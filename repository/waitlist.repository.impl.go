package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
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

// FindWaiting implements WaitlistRepository. Day matching uses the
// half-open range idiom (see reservation FindConfirmedInDateRange): exact
// YYYY-MM-DD equality fails on SQLite datetime strings.
func (r *WaitlistRepositoryImpl) FindWaiting(centerID uint, date time.Time, startTime string) ([]models.WaitlistEntry, error) {
	var out []models.WaitlistEntry
	day := date.Format("2006-01-02")
	next, _ := time.Parse("2006-01-02", day)
	err := r.db.Where("beauty_center_id = ? AND date >= ? AND date < ? AND start_time = ? AND status = ?",
		centerID, day, next.Add(24*time.Hour).Format("2006-01-02"), startTime, models.WaitlistWaiting).
		Order("created_at ASC").Find(&out).Error
	return out, err
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
		Order("created_at DESC").Find(&out).Error
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
