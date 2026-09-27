package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
)

// RankingRow is one rank of the beauty-center leaderboard. Score is the
// composite weighted score; the individual components are exposed for
// frontend display (e.g. "★4.8 · 120 turnos · 340 views").
type RankingRow struct {
	CenterID       uint    `gorm:"column:center_id"`
	CenterName     string  `gorm:"column:center_name"`
	Address        string  `gorm:"column:center_address"`
	AvgRating      float64 `gorm:"column:avg_rating"`
	ReviewCount    int64   `gorm:"column:review_count"`
	ReservationCnt int64   `gorm:"column:reservation_cnt"`
	CompletionRate float64 `gorm:"column:completion_rate"`
	VideoViews     int64   `gorm:"column:video_views"`
	VideoLikes     int64   `gorm:"column:video_likes"`
	FavoritesCount int64   `gorm:"column:favorites_count"`
	Score          float64 `gorm:"column:score"`
}

type BeautyCenterRepository interface {
	Create(center *models.BeautyCenter) error
	Update(center *models.BeautyCenter) error
	Delete(id uint) error
	FindByID(id uint) (*models.BeautyCenter, error)
	FindAll(page, limit int) ([]models.BeautyCenter, int64, error)
	FindAllActiveForClients() ([]models.BeautyCenter, error)
	FindByEmail(email string) (*models.BeautyCenter, error)
	ReplaceSchedules(centerID uint, schedules []models.WorkingHour) error
	ReplaceImages(centerID uint, images []models.BeautyCenterImage) error
	FindWorkingHours(centerID uint, dayOfWeek int) ([]models.WorkingHour, error)
	FindReservationsForDate(centerID uint, date time.Time, personnelID uint) ([]models.Reservation, error)
	// FindCentersForAvailability loads the candidate centers for the
	// cross-center availability search in ONE query. centerID == 0 means "all
	// active centers" (capped at ListQueryLimit); a positive centerID narrows
	// to that active center. Respects is_active and soft-deletes.
	FindCentersForAvailability(centerID uint) ([]models.BeautyCenter, error)
	// FindWorkingHoursForCenters batch-loads the working blocks for every
	// candidate center across the requested weekdays in ONE query, so a
	// 14-day range never fans out into a per-center/per-day query storm.
	FindWorkingHoursForCenters(centerIDs []uint, days []int) ([]models.WorkingHour, error)
	// FindReservationsForRange batch-loads the reservations of every candidate
	// center across an inclusive date range in ONE query (excluding CANCELLED,
	// optionally scoped to one personnel), mirroring FindReservationsForDate's
	// semantics for the whole range at once.
	FindReservationsForRange(centerIDs []uint, from, to time.Time, personnelID uint) ([]models.Reservation, error)
	// FindReservationsForRangeForPersonnel is the multi-professional twin of
	// FindReservationsForRange: it loads, in ONE query (plus the line preload),
	// the range's reservations that involve ANY of the given professionals —
	// either as their lead or as a service line — with lines preloaded so the
	// availability chain can project each professional's own busy intervals in
	// memory without a query per professional.
	FindReservationsForRangeForPersonnel(centerIDs []uint, from, to time.Time, personnelIDs []uint) ([]models.Reservation, error)
	Search(query string, page, limit int) ([]models.BeautyCenter, int64, error)
	FindTopRated(limit int) ([]models.BeautyCenter, error)
	FindNearby(latitude, longitude float64, limit int) ([]models.BeautyCenter, error)
	FindByReferralCode(code string) (*models.BeautyCenter, error)
	// HasSchedules reports whether the center has at least one working hour
	// row (profile-completeness checklist, feat-token-missions D4).
	HasSchedules(centerID uint) (bool, error)
	// HasImages reports whether the center has at least one image row.
	HasImages(centerID uint) (bool, error)
	// VerifyCenter stamps verified_at on first verification inside a
	// transaction and resolves the referrer from an active referral row.
	// Returns transitioned=false when the center was already verified;
	// referrerID is nil when no referral links the center.
	VerifyCenter(centerID uint) (transitioned bool, referrerID *uint, err error)
	// FindRanking returns beauty centers ranked by a composite score within
	// the given time window. from is nil for all-time ranking.
	FindRanking(from *time.Time, limit int) ([]RankingRow, error)
}
