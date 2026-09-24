package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WaitlistRepositoryImpl struct {
	db *gorm.DB
}

func NewWaitlistRepositoryImpl(db *gorm.DB) WaitlistRepository {
	return &WaitlistRepositoryImpl{db: db}
}

// Create implements WaitlistRepository
func (r *WaitlistRepositoryImpl) Create(entry *models.WaitlistEntry) error {
	return r.db.Create(entry).Error
}

// Update implements WaitlistRepository
func (r *WaitlistRepositoryImpl) Update(entry *models.WaitlistEntry) error {
	return r.db.Save(entry).Error
}

// FindByID implements WaitlistRepository
func (r *WaitlistRepositoryImpl) FindByID(entryID uint) (*models.WaitlistEntry, error) {
	var entry models.WaitlistEntry
	if err := r.db.First(&entry, entryID).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}

// OfferNextWaiting implements WaitlistRepository. The select-and-mark runs in
// one transaction with a row lock (clause.Locking{Strength: "UPDATE"}) so a
// second concurrent cancellation blocks on the row and then finds no WAITING
// victim. The lock is a no-op on SQLite (the driver strips FOR UPDATE), which
// serializes writers at the file level anyway, and a real row lock on
// PostgreSQL. Day matching uses the half-open range idiom (see reservation
// FindConfirmedInDateRange): exact YYYY-MM-DD equality fails on SQLite
// datetime strings.
func (r *WaitlistRepositoryImpl) OfferNextWaiting(centerID uint, date time.Time, startTime string, offeredAt, expiresAt time.Time) (*models.WaitlistEntry, error) {
	var offered *models.WaitlistEntry
	day := date.Format("2006-01-02")
	next, _ := time.Parse("2006-01-02", day)
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var rows []models.WaitlistEntry
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("beauty_center_id = ? AND date >= ? AND date < ? AND start_time = ? AND status = ?",
				centerID, day, next.Add(24*time.Hour).Format("2006-01-02"), startTime, models.WaitlistWaiting).
			Order("created_at ASC").Limit(1).Find(&rows).Error
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		rows[0].Status = models.WaitlistOffered
		rows[0].OfferedAt = &offeredAt
		rows[0].OfferExpiresAt = &expiresAt
		if err := tx.Save(&rows[0]).Error; err != nil {
			return err
		}
		offered = &rows[0]
		return nil
	})
	if err != nil {
		return nil, err
	}
	return offered, nil
}

// FindOfferedExpired implements WaitlistRepository
func (r *WaitlistRepositoryImpl) FindOfferedExpired(now time.Time) ([]models.WaitlistEntry, error) {
	var out []models.WaitlistEntry
	err := r.db.Where("status = ?", models.WaitlistOffered).
		Where("offer_expires_at IS NOT NULL AND offer_expires_at < ?", now).
		Find(&out).Error
	return out, err
}

// FindLive implements WaitlistRepository
func (r *WaitlistRepositoryImpl) FindLive(clientID uint) ([]models.WaitlistEntry, error) {
	var out []models.WaitlistEntry
	err := r.db.Where("client_id = ? AND status IN ?", clientID, []string{models.WaitlistWaiting, models.WaitlistOffered}).
		Preload("BeautyCenter").
		Order("created_at DESC").Find(&out).Error
	return out, err
}

// FindAllByClient implements WaitlistRepository: the full history of the
// client's entries, any status, newest first. The same Preload keeps the item
// shape identical to FindLive (recent fix: the center name must ship along).
func (r *WaitlistRepositoryImpl) FindAllByClient(clientID uint) ([]models.WaitlistEntry, error) {
	var out []models.WaitlistEntry
	err := r.db.Where("client_id = ?", clientID).
		Preload("BeautyCenter").
		Order("created_at DESC").Find(&out).Error
	return out, err
}

// FindLiveForSlots implements WaitlistRepository. One OR-of-AND query over
// the requested slots returns every live entry ordered by created_at ASC, so
// the service can rank a page of entries without a COUNT per row. Duplicate
// slots collapse via OR, so the result may repeat an entry; callers key by ID.
func (r *WaitlistRepositoryImpl) FindLiveForSlots(slots []WaitlistSlot) ([]models.WaitlistEntry, error) {
	var out []models.WaitlistEntry
	if len(slots) == 0 {
		return out, nil
	}
	var cond *gorm.DB
	for i, s := range slots {
		day := s.Date.Format("2006-01-02")
		next, _ := time.Parse("2006-01-02", day)
		c := r.db.Where("beauty_center_id = ? AND date >= ? AND date < ? AND start_time = ?",
			s.BeautyCenterID, day, next.Add(24*time.Hour).Format("2006-01-02"), s.StartTime)
		if i == 0 {
			cond = c
		} else {
			cond = cond.Or(c)
		}
	}
	err := r.db.Where("status IN ?", []string{models.WaitlistWaiting, models.WaitlistOffered}).
		Where(cond).
		Order("created_at ASC").Find(&out).Error
	return out, err
}

// ExistsLive implements WaitlistRepository (half-open day range, see above).
func (r *WaitlistRepositoryImpl) ExistsLive(centerID, clientID uint, date time.Time, startTime string) (bool, error) {
	var count int64
	day := date.Format("2006-01-02")
	next, _ := time.Parse("2006-01-02", day)
	err := r.db.Model(&models.WaitlistEntry{}).
		Where("beauty_center_id = ? AND client_id = ? AND date >= ? AND date < ? AND start_time = ? AND status IN ?",
			centerID, clientID, day, next.Add(24*time.Hour).Format("2006-01-02"), startTime,
			[]string{models.WaitlistWaiting, models.WaitlistOffered}).
		Count(&count).Error
	return count > 0, err
}
