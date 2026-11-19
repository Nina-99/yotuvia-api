package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type TokenTopUpRepositoryImpl struct {
	db *gorm.DB
}

func NewTokenTopUpRepositoryImpl(db *gorm.DB) TokenTopUpRepository {
	return &TokenTopUpRepositoryImpl{db: db}
}

func (r *TokenTopUpRepositoryImpl) Create(topup *models.TokenTopUp) error {
	return r.db.Create(topup).Error
}

func (r *TokenTopUpRepositoryImpl) Update(topup *models.TokenTopUp) error {
	return r.db.Save(topup).Error
}

func (r *TokenTopUpRepositoryImpl) FindByID(id uint) (*models.TokenTopUp, error) {
	var topup models.TokenTopUp
	err := r.db.First(&topup, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &topup, nil
}

func (r *TokenTopUpRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint) ([]models.TokenTopUp, error) {
	var topups []models.TokenTopUp
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).Order("created_at DESC").Find(&topups).Error
	return topups, err
}

func (r *TokenTopUpRepositoryImpl) FindPending() ([]models.TokenTopUp, error) {
	var topups []models.TokenTopUp
	err := r.db.Where("status = ?", "pending").Order("created_at ASC").Find(&topups).Error
	return topups, err
}

func (r *TokenTopUpRepositoryImpl) FindAll(page, limit int) ([]models.TokenTopUp, int64, error) {
	var topups []models.TokenTopUp
	var total int64
	r.db.Model(&models.TokenTopUp{}).Count(&total)
	offset := (page - 1) * limit
	err := r.db.Offset(offset).Limit(limit).Order("created_at DESC").Find(&topups).Error
	return topups, total, err
}

func (r *TokenTopUpRepositoryImpl) FindByCenterIDWithStatus(beautyCenterID uint, page, limit int, status string) ([]models.TokenTopUp, int64, error) {
	var topups []models.TokenTopUp
	var total int64

	query := r.db.Model(&models.TokenTopUp{}).Where("beauty_center_id = ?", beautyCenterID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&topups).Error
	return topups, total, err
}

// FindAllWithFilters is the platform-wide queue read. Both filters are honoured
// by the query itself (never faked in the handler): a nil pointer means "every
// centre", an empty status means "every status".
func (r *TokenTopUpRepositoryImpl) FindAllWithFilters(page, limit int, beautyCenterID *uint, status string) ([]models.TokenTopUp, int64, error) {
	var topups []models.TokenTopUp
	var total int64

	query := r.db.Model(&models.TokenTopUp{})
	if beautyCenterID != nil {
		query = query.Where("beauty_center_id = ?", *beautyCenterID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&topups).Error
	return topups, total, err
}

// Reconciliation aggregates the manual money path. Every query filters
// deleted_at explicitly because it bypasses the model scope (Table/Joins).
func (r *TokenTopUpRepositoryImpl) Reconciliation(now time.Time) (*TopUpReconciliation, error) {
	rec := &TopUpReconciliation{}

	// Per-status totals in one group-by.
	type row struct {
		Status   string
		Cnt      int64
		AmountBs float64
		Tokens   int64
	}
	var rows []row
	if err := r.db.Table("token_top_ups").
		Select("status, COUNT(*) AS cnt, COALESCE(SUM(amount_bs), 0) AS amount_bs, COALESCE(SUM(tokens_received + bonus_tokens), 0) AS tokens").
		Where("deleted_at IS NULL").
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, s := range rows {
		switch s.Status {
		case "pending":
			rec.PendingCount = s.Cnt
			rec.PendingAmountBs = s.AmountBs
		case "approved":
			rec.ApprovedCount = s.Cnt
			rec.ApprovedAmountBs = s.AmountBs
			rec.ApprovedTokens = s.Tokens
		case "rejected":
			rec.RejectedCount = s.Cnt
			rec.RejectedAmountBs = s.AmountBs
		}
	}

	// Pending aging: how many are older than 24h and how old the oldest is.
	// Read the oldest row through the model (not MIN()) so the driver's date
	// parsing is used — a raw MIN(created_at) comes back as a string in SQLite.
	var oldestRow models.TokenTopUp
	err := r.db.Where("status = ?", "pending").Order("created_at ASC").First(&oldestRow).Error
	switch {
	case err == nil:
		rec.OldestPendingAge = now.Sub(oldestRow.CreatedAt).Hours()
	case err != gorm.ErrRecordNotFound:
		return nil, err
	}
	if err := r.db.Table("token_top_ups").
		Where("status = ? AND deleted_at IS NULL AND created_at < ?", "pending", now.Add(-24*time.Hour)).
		Count(&rec.PendingOver24h).Error; err != nil {
		return nil, err
	}

	// Ledger drift: approved top-ups whose credited tokens differ from what the
	// row declares. A LEFT JOIN keeps top-ups with no ledger row (credited 0).
	err = r.db.Table("token_top_ups AS t").
		Joins(`LEFT JOIN (SELECT reference_id, SUM(amount) AS credited
			FROM token_transactions WHERE type = ? AND reference_id IS NOT NULL
			GROUP BY reference_id) l ON l.reference_id = t.id`, "topup").
		Where("t.status = ? AND t.deleted_at IS NULL", "approved").
		Where("COALESCE(l.credited, 0) <> (t.tokens_received + t.bonus_tokens)").
		Count(&rec.DriftCount).Error
	if err != nil {
		return nil, err
	}

	return rec, nil
}
