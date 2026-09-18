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
