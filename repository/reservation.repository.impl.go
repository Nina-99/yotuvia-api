package repository

import (
	"errors"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type ReservationRepositoryImpl struct {
	db *gorm.DB
}

func NewReservationRepositoryImpl(db *gorm.DB) ReservationRepository {
	return &ReservationRepositoryImpl{db: db}
}

func (r *ReservationRepositoryImpl) Create(reservation *models.Reservation) error {
	return r.db.Create(reservation).Error
}

func (r *ReservationRepositoryImpl) Update(reservation *models.Reservation) error {
	return r.db.Save(reservation).Error
}

func (r *ReservationRepositoryImpl) Delete(reservation *models.Reservation) error {
	return r.db.Delete(reservation).Error
}

func (r *ReservationRepositoryImpl) FindById(reservationID uint) (*models.Reservation, error) {
	var reservation models.Reservation
	err := r.db.
		Preload("ReservationServices.Service.Category").Preload("Client").Preload("Personnel").Preload("BeautyCenter").
		First(&reservation, reservationID).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("reservation not found")
		}
		return nil, err
	}
	return &reservation, nil
}

func (r *ReservationRepositoryImpl) FindUpcomingByClientID(clientID uint) (*models.Reservation, error) {
	var out []models.Reservation
	today := time.Now().Format("2006-01-02")
	// Find (unlike First) does not treat an empty result as an error, so a
	// client with no upcoming reservation is a normal empty state instead of
	// a spurious "record not found" error line.
	err := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("client_id = ? AND date >= ? AND status NOT IN (?)", clientID, today, []string{"COMPLETED", "CANCELLED"}).
		Order("date ASC, start_time ASC").
		Limit(1).
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
}

func (r *ReservationRepositoryImpl) FindByClientID(clientID uint) ([]models.Reservation, error) {
	var reservations []models.Reservation
	err := r.db.
		Preload("ReservationServices.Service.Category").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("client_id = ?", clientID).
		Order("date DESC, start_time DESC").
		Find(&reservations).Error
	return reservations, err
}

func (r *ReservationRepositoryImpl) FindByBeautyCenterId(beautyCenterID uint, from, to *time.Time, clientID *uint) ([]models.Reservation, error) {
	var reservations []models.Reservation
	query := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("beauty_center_id = ?", beautyCenterID)

	if from != nil && to != nil {
		// Range-scoped agenda (design D4): inclusive range, chronological ordering.
		query = query.Where("date >= ? AND date <= ?", *from, *to).Order("start_time ASC")
	} else if from == nil && to == nil && clientID == nil {
		// No filters: return all reservations for this center (unchanged behavior).
		// The existing callers that only pass `date` will now pass nil, nil.
	} else if clientID != nil {
		// Client-scoped history (design D3): strict center+client filter with
		// reverse chronological order. Takes precedence over the date-agenda
		// ordering when both params are present.
		query = query.Where("client_id = ?", *clientID).Order("date DESC, start_time DESC")
} else if from != nil && to == nil {
		// Single from-date: treat as exact day query.
		query = query.Where("date = ?", *from).Order("start_time ASC")
	} else if from == nil && to != nil {
		// Single to-date: treat as exact day query.
		query = query.Where("date = ?", *to).Order("start_time ASC")
	}

	err := query.Find(&reservations).Error
	return reservations, err
}

// FindByPersonnelId implements ReservationRepository. A professional's agenda
// must contain every reservation that has a service line attributed to them —
// not only the ones where they are the derived lead. The OR keeps lead
// membership (and line-less legacy rows) working, while the subquery adds the
// non-lead lines. The where clause is assembled conditionally so nil from/to
// stay open-ended while GORM still handles the bound parameters.
func (r *ReservationRepositoryImpl) FindByPersonnelId(personnelID uint, from, to *time.Time) ([]models.Reservation, error) {
	var reservations []models.Reservation
	query := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("(personnel_id = ? OR id IN (SELECT reservation_id FROM reservation_services WHERE personnel_id = ? AND deleted_at IS NULL))",
			personnelID, personnelID)
	if from != nil {
		query = query.Where("date >= ?", *from)
	}
	if to != nil {
		query = query.Where("date <= ?", *to)
	}
	err := query.Order("date ASC, start_time ASC").Find(&reservations).Error
	return reservations, err
}

func (r *ReservationRepositoryImpl) FindAll(page, limit int, centerID uint) ([]models.Reservation, int64, error) {
	var reservations []models.Reservation
	var total int64

	base := r.db.Model(&models.Reservation{}).Where("beauty_center_id = ?", centerID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("beauty_center_id = ?", centerID).
		Offset(offset).
		Limit(limit).
		Find(&reservations).Error
	return reservations, total, err
}

// CheckOverlap reports whether personnelID is busy inside [startTime, endTime)
// on date at the center. A professional is busy only for THEIR OWN segments of
// a reservation, not for the whole envelope of a shared reservation.
//
// Strategy (superset SQL prefilter + exact Go refinement):
//
//  1. SQL narrows to envelope-overlapping, non-cancelled reservations at the
//     center (exactly the historical predicate). For a professional it also
//     requires the reservation to carry them — either as its derived lead or
//     as the professional of one of its service lines. This is a SUPERSET of
//     the true conflicts: any segment that overlaps the window lies inside its
//     reservation's envelope, so no real conflict can be filtered out.
//  2. Go derives each candidate's segments (models.DeriveReservationSegments)
//     and checks overlap against the professional's own segments, which can
//     only narrow the set.
//
// For a single-professional reservation the segments tile the envelope
// contiguously, so step 2 returns the same answer the envelope check always
// did — a byte-for-byte no-op for all existing data.
//
// personnelID == 0 keeps the historical center-wide semantics: any
// envelope-overlapping reservation blocks.
func (r *ReservationRepositoryImpl) CheckOverlap(centerID uint, date time.Time, startTime, endTime string, personnelID, excludeID uint) (bool, error) {
	query := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND date = ? AND status != 'CANCELLED'", centerID, date).
		Where("((start_time < ? AND end_time > ?) OR (start_time >= ? AND start_time < ?) OR (end_time > ? AND end_time <= ?))",
			endTime, startTime, startTime, endTime, startTime, endTime)

	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	if personnelID == 0 {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	}

	var candidates []models.Reservation
	err := query.
		Where("(personnel_id = ? OR id IN (SELECT reservation_id FROM reservation_services WHERE personnel_id = ? AND deleted_at IS NULL))",
			personnelID, personnelID).
		Preload("ReservationServices", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Find(&candidates).Error
	if err != nil {
		return false, err
	}

	startMinutes := models.ParseClockMinutes(startTime)
	endMinutes := models.ParseClockMinutes(endTime)
	for i := range candidates {
		if models.ReservationBusyForPersonnel(&candidates[i], personnelID, startMinutes, endMinutes) {
			return true, nil
		}
	}
	return false, nil
}

// UpdatePersonnelID sets the reservation's professional: the DERIVED LEAD and
// every one of its service lines, in one transaction, so the two can never
// diverge. Until per-line assignment exists, reassigning a reservation means
// reassigning all of it - and the line-level attribution we now expose would
// otherwise keep naming the previous professional.
func (r *ReservationRepositoryImpl) UpdatePersonnelID(reservationID uint, personnelID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Reservation{}).
			Where("id = ?", reservationID).
			Update("personnel_id", personnelID).Error; err != nil {
			return err
		}
		return tx.Model(&models.ReservationService{}).
			Where("reservation_id = ?", reservationID).
			Update("personnel_id", personnelID).Error
	})
}

func (r *ReservationRepositoryImpl) DeleteReservationServices(reservationID uint) error {
	return r.db.Where("reservation_id = ?", reservationID).Delete(&models.ReservationService{}).Error
}

// FindExpiredHolds implements ReservationRepository. Only live holds
// (PENDING/CONFIRMED) qualify — terminal rows are never rewritten.
func (r *ReservationRepositoryImpl) FindExpiredHolds(now time.Time) ([]models.Reservation, error) {
	var out []models.Reservation
	err := r.db.Where("payment_status = ?", "PENDING_PROOF").
		Where("status IN ?", []string{"PENDING", "CONFIRMED"}).
		Where("hold_expires_at IS NOT NULL AND hold_expires_at < ?", now).
		Find(&out).Error
	return out, err
}

// FindConfirmedInDateRange implements ReservationRepository. The upper
// bound is exclusive next-day-start ("date < to+1d") instead of "<= to":
// SQLite compares datetimes lexicographically, so a same-day row stored as
// "2026-09-22 00:00:00+00:00" would fail "<= 2026-09-22". Both Postgres
// (date type) and SQLite (string compare) evaluate the exclusive bound
// correctly.
func (r *ReservationRepositoryImpl) FindConfirmedInDateRange(from, to time.Time) ([]models.Reservation, error) {
	var out []models.Reservation
	err := r.db.Where("status = ?", "CONFIRMED").
		Where("date >= ? AND date < ?", from.Format("2006-01-02"), to.Add(24*time.Hour).Format("2006-01-02")).
		Find(&out).Error
	return out, err
}

func (r *ReservationRepositoryImpl) FindByToken(token string) (*models.Reservation, error) {
	var reservation models.Reservation
	err := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("verification_token = ?", token).
		First(&reservation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &reservation, nil
}

// CompleteVerification performs the atomic exactly-once update: only a
// reservation that is still PENDING or CONFIRMED is completed, the token and
// expiry are cleared, and the status is persisted in canonical UPPERCASE.
func (r *ReservationRepositoryImpl) CompleteVerification(reservation *models.Reservation) (bool, error) {
	result := r.db.Model(&models.Reservation{}).
		Where("id = ? AND status IN ?", reservation.ID, []string{"PENDING", "CONFIRMED"}).
		Updates(map[string]interface{}{
			"status":             "COMPLETED",
			"verification_token": gorm.Expr("NULL"),
			"token_expires_at":   gorm.Expr("NULL"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// Transact exposes the repository's database transaction so the QR
// verification service can compose completion + token debit atomically
// (feat-token-economy-config D4).
func (r *ReservationRepositoryImpl) Transact(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// CompleteVerificationTx is the tx-aware variant of CompleteVerification:
// same conditional update (PENDING/CONFIRMED → COMPLETED, token cleared),
// executed inside the caller's transaction. RowsAffected zero means another
// scan already completed it.
func (r *ReservationRepositoryImpl) CompleteVerificationTx(tx *gorm.DB, reservationID uint) (bool, error) {
	result := tx.Model(&models.Reservation{}).
		Where("id = ? AND status IN ?", reservationID, []string{"PENDING", "CONFIRMED"}).
		Updates(map[string]interface{}{
			"status":             "COMPLETED",
			"verification_token": gorm.Expr("NULL"),
			"token_expires_at":   gorm.Expr("NULL"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
