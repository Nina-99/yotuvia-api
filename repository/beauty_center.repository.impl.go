package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

// ratingAggregateJoin computes avg_rating and review_count for a whole page of
// centres in ONE pass over reviews, using the same grouped LEFT JOIN shape as
// the ranking query. A correlated subquery (the previous implementation) would
// re-scan reviews once per centre per column — and reviews has no index on
// beauty_center_id. Grouping once and joining is the batched form: the number of
// review scans stays constant no matter how many centres the page holds.
const ratingAggregateJoin = "LEFT JOIN (" +
	"SELECT beauty_center_id, AVG(rating) AS avg_rating, COUNT(*) AS review_count " +
	"FROM reviews WHERE deleted_at IS NULL GROUP BY beauty_center_id" +
	") rating_agg ON rating_agg.beauty_center_id = beauty_centers.id"

// ratingAggregateSelect projects the joined aggregate onto the model's
// read-only AvgRating/ReviewCount fields (see models.BeautyCenter).
const ratingAggregateSelect = "beauty_centers.*, " +
	"COALESCE(rating_agg.avg_rating, 0) AS avg_rating, " +
	"COALESCE(rating_agg.review_count, 0) AS review_count"

// withAverageRating is the shared GORM scope behind every beauty-center
// listing/detail query: it joins the rating aggregate and exposes avg_rating /
// review_count. A trailing SELECT expression with bind arguments (e.g. the
// distance calculation in FindNearby) is appended verbatim after them.
func withAverageRating(extraSelect string, args ...interface{}) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		selects := ratingAggregateSelect
		if extraSelect != "" {
			selects += ", " + extraSelect
		}
		return db.Joins(ratingAggregateJoin).Select(selects, args...)
	}
}

type BeautyCenterRepositoryImpl struct {
	db *gorm.DB
}

// DB returns the underlying *gorm.DB for queries outside the repository
// scope (e.g. image fetching in the ranking service).
func (r *BeautyCenterRepositoryImpl) DB() *gorm.DB { return r.db }

func NewBeautyCenterRepositoryImpl(db *gorm.DB) BeautyCenterRepository {
	return &BeautyCenterRepositoryImpl{db: db}
}

// Create implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Create(center *models.BeautyCenter) error {
	return r.db.Create(center).Error
}

// Update implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Update(center *models.BeautyCenter) error {
	return r.db.Save(center).Error
}

// Delete implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.BeautyCenter{}, id).Error
}

// FindByID implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindByID(id uint) (*models.BeautyCenter, error) {
	var center models.BeautyCenter
	err := r.db.
		Scopes(withAverageRating("")).
		Preload("SocialMedia").
		Preload("WorkingHour").
		Preload("BeautyCenterImage").
		First(&center, id).Error
	if err != nil {
		return nil, err
	}
	return &center, nil
} // FindAll implements BeautyCenterRepository — only active centers
// are returned so inactive/deleted centers never appear on the client map.
func (r *BeautyCenterRepositoryImpl) FindAll(page, limit int) ([]models.BeautyCenter, int64, error) {
	var centers []models.BeautyCenter
	var total int64

	r.db.Model(&models.BeautyCenter{}).Where("is_active = ?", true).Count(&total)

	offset := (page - 1) * limit
	err := r.db.
		Scopes(withAverageRating("")).
		Preload("BeautyCenterImage").
		Preload("WorkingHour").
		Where("is_active = ?", true).
		Offset(offset).Limit(limit).
		Find(&centers).Error
	return centers, total, err
}

func (r *BeautyCenterRepositoryImpl) FindAllActiveForClients() ([]models.BeautyCenter, error) {
	var centers []models.BeautyCenter
	err := r.db.Where("is_active = ?", true).Find(&centers).Error
	if err != nil {
		return nil, err
	}
	return centers, nil
}

// FindAllSummaries returns id + name for every centre (including inactive ones),
// for the platform admin's centre picker.
func (r *BeautyCenterRepositoryImpl) FindAllSummaries() ([]models.BeautyCenter, error) {
	var centers []models.BeautyCenter
	err := r.db.Select("id", "name").Order("name ASC").Find(&centers).Error
	if err != nil {
		return nil, err
	}
	return centers, nil
}

// ReplaceSchedules implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) ReplaceSchedules(centerID uint, schedules []models.WorkingHour) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("beauty_center_id = ?", centerID).Delete(&models.WorkingHour{}).Error; err != nil {
			return err
		}
		if len(schedules) > 0 {
			if err := tx.Create(schedules).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ReplaceImages implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) ReplaceImages(centerID uint, images []models.BeautyCenterImage) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("beauty_center_id = ?", centerID).Delete(&models.BeautyCenterImage{}).Error; err != nil {
			return err
		}
		if len(images) > 0 {
			if err := tx.Create(images).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// FindByEmail implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindByEmail(email string) (*models.BeautyCenter, error) {
	var center models.BeautyCenter
	err := r.db.Where("email = ?", email).First(&center).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &center, nil
}

// FindWorkingHours implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindWorkingHours(centerID uint, dayOfWeek int) ([]models.WorkingHour, error) {
	var whs []models.WorkingHour
	err := r.db.Where("beauty_center_id = ? AND day_of_week = ?", centerID, dayOfWeek).Find(&whs).Error
	if err != nil {
		return nil, err
	}
	return whs, nil
}

// FindReservationsForDate implements BeautyCenterRepository. When scoped to a
// professional, the filter is a SUPERSET: a reservation qualifies if the
// professional is its derived lead OR has a service line attributed to them,
// and the service lines are preloaded so the caller can refine to that
// professional's own segments (service.PersonnelBusyReservations). A
// professional-scoped read must never miss a reservation where they are a
// non-lead line.
func (r *BeautyCenterRepositoryImpl) FindReservationsForDate(centerID uint, date time.Time, personnelID uint) ([]models.Reservation, error) {
	var reservations []models.Reservation
	q := r.db.Where("beauty_center_id = ? AND DATE(date) = DATE(?) AND status != ?", centerID, date, "CANCELLED")
	if personnelID > 0 {
		q = q.
			Where("(personnel_id = ? OR id IN (SELECT reservation_id FROM reservation_services WHERE personnel_id = ? AND deleted_at IS NULL))",
				personnelID, personnelID).
			Preload("ReservationServices", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") })
	}
	err := q.Find(&reservations).Error
	return reservations, err
}

// FindCentersForAvailability implements BeautyCenterRepository. One SELECT for
// every candidate center of the availability search.
func (r *BeautyCenterRepositoryImpl) FindCentersForAvailability(centerID uint) ([]models.BeautyCenter, error) {
	var centers []models.BeautyCenter
	q := r.db.Model(&models.BeautyCenter{}).
		Select("id, name, currency, is_active").
		Where("is_active = ?", true)
	if centerID > 0 {
		q = q.Where("id = ?", centerID)
	}
	err := q.Order("id ASC").Limit(ListQueryLimit).Find(&centers).Error
	if err != nil {
		return nil, err
	}
	return centers, nil
}

// FindWorkingHoursForCenters implements BeautyCenterRepository. One SELECT for
// the working blocks of every candidate center across the given weekdays.
func (r *BeautyCenterRepositoryImpl) FindWorkingHoursForCenters(centerIDs []uint, days []int) ([]models.WorkingHour, error) {
	if len(centerIDs) == 0 || len(days) == 0 {
		return []models.WorkingHour{}, nil
	}
	var whs []models.WorkingHour
	err := r.db.
		Where("beauty_center_id IN ? AND day_of_week IN ?", centerIDs, days).
		Order("beauty_center_id ASC, day_of_week ASC, start_time ASC").
		Find(&whs).Error
	if err != nil {
		return nil, err
	}
	return whs, nil
}

// FindReservationsForRange implements BeautyCenterRepository. One SELECT for
// every candidate center's reservations over the inclusive [from, to] range,
// excluding CANCELLED — the batch twin of FindReservationsForDate, with the
// same professional superset filter and line preload.
func (r *BeautyCenterRepositoryImpl) FindReservationsForRange(centerIDs []uint, from, to time.Time, personnelID uint) ([]models.Reservation, error) {
	if len(centerIDs) == 0 {
		return []models.Reservation{}, nil
	}
	var reservations []models.Reservation
	q := r.db.Where(
		"beauty_center_id IN ? AND DATE(date) BETWEEN DATE(?) AND DATE(?) AND status != ?",
		centerIDs, from, to, "CANCELLED",
	)
	if personnelID > 0 {
		q = q.
			Where("(personnel_id = ? OR id IN (SELECT reservation_id FROM reservation_services WHERE personnel_id = ? AND deleted_at IS NULL))",
				personnelID, personnelID).
			Preload("ReservationServices", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") })
	}
	err := q.Find(&reservations).Error
	if err != nil {
		return nil, err
	}
	return reservations, nil
}

// FindReservationsForRangeForPersonnel implements BeautyCenterRepository. It is
// the multi-professional twin of FindReservationsForRange: ONE query for the
// range's reservations that involve ANY of the given professionals (as lead or
// as a service line), with the service lines preloaded so the availability
// chain can project each professional's own busy intervals in memory. Keeping
// it batched is what stops the chain from querying per professional.
func (r *BeautyCenterRepositoryImpl) FindReservationsForRangeForPersonnel(centerIDs []uint, from, to time.Time, personnelIDs []uint) ([]models.Reservation, error) {
	if len(centerIDs) == 0 || len(personnelIDs) == 0 {
		return []models.Reservation{}, nil
	}
	var reservations []models.Reservation
	err := r.db.Where(
		"beauty_center_id IN ? AND DATE(date) BETWEEN DATE(?) AND DATE(?) AND status != ?",
		centerIDs, from, to, "CANCELLED",
	).
		Where("(personnel_id IN ? OR id IN (SELECT reservation_id FROM reservation_services WHERE personnel_id IN ? AND deleted_at IS NULL))",
			personnelIDs, personnelIDs).
		Preload("ReservationServices", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Find(&reservations).Error
	if err != nil {
		return nil, err
	}
	return reservations, nil
}

// Search implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Search(query string, page, limit int) ([]models.BeautyCenter, int64, error) {
	var centers []models.BeautyCenter
	var total int64

	offset := (page - 1) * limit
	searchPattern := "%" + query + "%"

	base := r.db.Model(&models.BeautyCenter{}).
		Where("is_active = ?", true).
		Where("LOWER(name) LIKE LOWER(?) OR LOWER(address) LIKE LOWER(?) OR LOWER(description) LIKE LOWER(?)", searchPattern, searchPattern, searchPattern)

	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := base.
		Scopes(withAverageRating("")).
		Preload("BeautyCenterImage").
		Preload("WorkingHour").
		Offset(offset).Limit(limit).
		Find(&centers).Error

	return centers, total, err
}

// FindTopRated returns top-rated active beauty centers.
func (r *BeautyCenterRepositoryImpl) FindTopRated(limit int) ([]models.BeautyCenter, error) {
	var centers []models.BeautyCenter

	err := r.db.
		Scopes(withAverageRating("")).
		Preload("BeautyCenterImage").
		Preload("WorkingHour").
		Where("is_active = ?", true).
		Order("avg_rating DESC, review_count DESC").
		Limit(limit).
		Find(&centers).Error

	return centers, err
}

// FindNearby returns active beauty centers sorted by distance.
func (r *BeautyCenterRepositoryImpl) FindNearby(latitude, longitude float64, limit int) ([]models.BeautyCenter, error) {
	var centers []models.BeautyCenter

	err := r.db.
		Scopes(withAverageRating(
			"(6371 * acos(cos(radians(?)) * cos(radians(latitude)) * cos(radians(longitude) - radians(?)) + sin(radians(?)) * sin(radians(latitude)))) AS distance",
			latitude, longitude, latitude)).
		Preload("BeautyCenterImage").
		Preload("WorkingHour").
		Where("is_active = ? AND latitude != 0 AND longitude != 0", true).
		Order("distance ASC").
		Limit(limit).
		Find(&centers).Error

	return centers, err
}

func (r *BeautyCenterRepositoryImpl) FindByReferralCode(code string) (*models.BeautyCenter, error) {
	var center models.BeautyCenter
	err := r.db.Where("referral_code = ?", code).First(&center).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &center, nil
}

// HasSchedules reports whether at least one working hour exists (profile
// completeness, feat-token-missions D4).
func (r *BeautyCenterRepositoryImpl) HasSchedules(centerID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.WorkingHour{}).
		Where("beauty_center_id = ?", centerID).
		Count(&count).Error
	return count > 0, err
}

// HasImages reports whether at least one image row exists.
func (r *BeautyCenterRepositoryImpl) HasImages(centerID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.BeautyCenterImage{}).
		Where("beauty_center_id = ?", centerID).
		Count(&count).Error
	return count > 0, err
}

// MarkWelcomeBonusPaidTx flips welcome_bonus_paid exactly once, in the caller's
// transaction, so the welcome credit and its guard commit together.
func (r *BeautyCenterRepositoryImpl) MarkWelcomeBonusPaidTx(tx *gorm.DB, centerID uint) (bool, error) {
	res := tx.Model(&models.BeautyCenter{}).
		Where("id = ? AND welcome_bonus_paid = ?", centerID, false).
		Update("welcome_bonus_paid", true)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// VerifyCenter stamps verified_at on the first verification (NULL → set) and
// resolves the referrer from an active referral row in the same transaction.
// Already-verified centers return transitioned=false without touching data;
// a center without a referral row returns transitioned=true with nil referrer
// (verification itself is platform-ops driven — feat-token-missions D5).
func (r *BeautyCenterRepositoryImpl) VerifyCenter(centerID uint) (bool, *uint, error) {
	var referrerID *uint
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.BeautyCenter{}).
			Where("id = ? AND verified_at IS NULL", centerID).
			Update("verified_at", time.Now().UTC())
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // already verified — idempotent no-op
		}

		var referral models.Referral
		err := tx.Where("referred_id = ? AND deleted_at IS NULL", centerID).
			First(&referral).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil // verified without referral — nothing to attribute
			}
			return err
		}
		id := referral.ReferrerID
		referrerID = &id
		return nil
	})
	if err != nil {
		return false, nil, err
	}
	transitioned := referrerID != nil
	// A verified-without-referral center also "transitioned"; re-derive by
	// checking the column so callers can distinguish cleanly.
	if !transitioned {
		var center models.BeautyCenter
		if err := r.db.Select("verified_at").First(&center, centerID).Error; err == nil && center.VerifiedAt != nil {
			transitioned = true
		}
	}
	return transitioned, referrerID, nil
}

// FindRanking returns beauty centers ranked by a composite score within the
// given time window. When from is nil the ranking is all-time (no date filter).
//
// Score = (reviews × 0.35) + (reservations × 0.30) + (engagement × 0.20) + (favorites × 0.15)
//
// Each component is normalized to 0–100 via max-value scaling within the result set.
func (r *BeautyCenterRepositoryImpl) FindRanking(from *time.Time, limit int) ([]RankingRow, error) {
	// Each subquery has its own table — no shared alias.  We build the
	// date condition per-table and duplicate the arg for every placeholder.
	//
	// When from is nil (all-time) the filters are empty strings and no
	// date args are appended — no placeholders to fill.
	reviewDateFilter := ""
	resDateFilter := ""
	videoDateFilter := ""
	favDateFilter := ""
	var allArgs []interface{}

	if from != nil {
		reviewDateFilter = "AND reviews.created_at >= ?"
		resDateFilter = "AND reservations.created_at >= ?"
		videoDateFilter = "AND feed_posts.created_at >= ?"
		favDateFilter = "AND favorites.created_at >= ?"
		// Each ? needs its own copy of the date value.
		allArgs = []interface{}{*from, *from, *from, *from}
	}

	query := `
		SELECT
			bc.id AS center_id,
			bc.name AS center_name,
			bc.address AS center_address,
			COALESCE(rev.avg_rating, 0) AS avg_rating,
			COALESCE(rev.review_count, 0) AS review_count,
			COALESCE(res.reservation_cnt, 0) AS reservation_cnt,
			COALESCE(res.completion_rate, 0) AS completion_rate,
			COALESCE(vid.video_views, 0) AS video_views,
			COALESCE(vid.video_likes, 0) AS video_likes,
			COALESCE(fav.favorites_count, 0) AS favorites_count,
			-- Composite score: each component weighted
			(
				COALESCE(rev.avg_rating, 0) * LOG(COALESCE(rev.review_count, 0) + 1) * 0.35 +
				COALESCE(res.completion_rate, 0) * LOG(COALESCE(res.reservation_cnt, 0) + 1) * 0.30 +
				(COALESCE(vid.video_views, 0) + COALESCE(vid.video_likes, 0) * 3) * 0.20 +
				COALESCE(fav.favorites_count, 0) * 0.15
			) AS score
		FROM beauty_centers bc
		-- Reviews subquery (period-filtered)
		LEFT JOIN (
			SELECT beauty_center_id,
				AVG(rating) AS avg_rating,
				COUNT(*) AS review_count
			FROM reviews
			WHERE deleted_at IS NULL ` + reviewDateFilter + `
			GROUP BY beauty_center_id
		) rev ON rev.beauty_center_id = bc.id
		-- Reservations subquery (period-filtered)
		LEFT JOIN (
			SELECT beauty_center_id,
				COUNT(*) AS reservation_cnt,
				CASE WHEN COUNT(*) > 0
					THEN CAST(SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*)
					ELSE 0
				END AS completion_rate
			FROM reservations
			WHERE deleted_at IS NULL ` + resDateFilter + `
			GROUP BY beauty_center_id
		) res ON res.beauty_center_id = bc.id
		-- Video engagement subquery (period-filtered)
		LEFT JOIN (
			SELECT beauty_center_id,
				SUM(views_count) AS video_views,
				SUM(likes_count) AS video_likes
			FROM feed_posts
			WHERE deleted_at IS NULL AND is_active = true ` + videoDateFilter + `
			GROUP BY beauty_center_id
		) vid ON vid.beauty_center_id = bc.id
		-- Favorites subquery (period-filtered)
		LEFT JOIN (
			SELECT beauty_center_id,
				COUNT(*) AS favorites_count
			FROM favorites
			WHERE deleted_at IS NULL ` + favDateFilter + `
			GROUP BY beauty_center_id
		) fav ON fav.beauty_center_id = bc.id
		WHERE bc.is_active = true AND bc.deleted_at IS NULL
		ORDER BY score DESC
		LIMIT ?
	`

	allArgs = append(allArgs, limit)

	var rows []RankingRow
	if err := r.db.Raw(query, allArgs...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
