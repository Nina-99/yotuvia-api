package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type ClientRepositoryImpl struct {
	db *gorm.DB
}

func NewClientRepositoryImpl(db *gorm.DB) ClientRepository {
	return &ClientRepositoryImpl{db: db}
}

// Create implements ClientRepository
func (r *ClientRepositoryImpl) Create(client *models.Client) error {
	return r.db.Create(&client).Error
}

// Update implements ClientRepository
func (r *ClientRepositoryImpl) Update(client *models.Client) error {
	return r.db.Save(client).Error
}

// Delete implements ClientRepository
func (r *ClientRepositoryImpl) Delete(client *models.Client) error {
	return r.db.Delete(client).Error
}

// AdjustTrustScore implements ClientRepository. Single UPDATE with a CASE
// clamp so concurrent no-show and attendance writes never escape [0, 100]
// and never overwrite each other (no read-modify-write race). CASE WHEN is
// used instead of GREATEST/LEAST so the statement runs on both Postgres
// (production) and SQLite (tests).
func (r *ClientRepositoryImpl) AdjustTrustScore(clientID uint, delta int) (int, error) {
	var client models.Client
	if err := r.db.First(&client, clientID).Error; err != nil {
		return 0, err
	}
	if err := r.db.Model(&models.Client{}).Where("id = ?", clientID).
		Update("trust_score", gorm.Expr("CASE WHEN trust_score + ? < 0 THEN 0 WHEN trust_score + ? > 100 THEN 100 ELSE trust_score + ? END", delta, delta, delta)).Error; err != nil {
		return 0, err
	}
	if err := r.db.First(&client, clientID).Error; err != nil {
		return 0, err
	}
	return client.TrustScore, nil
}

// adminClientRow pairs a client row with its aggregate metrics from the
// FindAllAdmin join query — the internal Scan target (design D1). LastVisit is
// kept as sql.NullString because the SQLite driver returns date expressions as
// strings that GORM cannot scan into time values; the value is parsed back
// into *time.Time after scanning (CAST AS TEXT also normalizes Postgres
// date-only results).
type adminClientRow struct {
	models.Client
	VisitsCount int
	TotalSpent  float64
	LastVisit   sql.NullString
}

// lastVisitLayouts covers the formats a date expression can arrive in:
// SQLite keeps the full "2006-01-02 15:04:05+00:00" timestamp, while Postgres
// CAST(date AS TEXT) yields the date-only form.
var lastVisitLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05-07:00",
	time.RFC3339,
	"2006-01-02",
}

func parseAdminLastVisit(raw string) (*time.Time, error) {
	for _, layout := range lastVisitLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("cannot parse last_visit %q", raw)
}

// adminReservationsJoin scopes the client list through the reservations
// table: a client only appears when it has at least one non-deleted
// reservation (of any status) with the given center. This join IS the
// authorization boundary — center A can never see center B's clients.
const adminReservationsJoin = "INNER JOIN reservations ON reservations.client_id = clients.id AND reservations.beauty_center_id = ? AND reservations.deleted_at IS NULL"

// FindAllAdmin implements ClientRepository. A single aggregate query computes
// the COMPLETED-only metrics (COUNT/SUM/MAX) per client; the pagination total
// counts distinct clients scoped to the center, never the global table.
func (r *ClientRepositoryImpl) FindAllAdmin(page, limit int, centerID uint) ([]models.Client, []ClientMetrics, int64, error) {
	var total int64
	if err := r.db.Table("clients").
		Joins(adminReservationsJoin, centerID).
		Where("clients.deleted_at IS NULL").
		Distinct("clients.id").
		Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}

	offset := (page - 1) * limit
	var rows []adminClientRow
	err := r.db.Table("clients").
		Select("clients.*, "+
			"COALESCE(COUNT(CASE WHEN reservations.status = 'COMPLETED' THEN 1 END), 0) AS visits_count, "+
			"COALESCE(SUM(CASE WHEN reservations.status = 'COMPLETED' THEN reservations.total_price END), 0) AS total_spent, "+
			"CAST(MAX(CASE WHEN reservations.status = 'COMPLETED' THEN reservations.date END) AS TEXT) AS last_visit").
		Joins(adminReservationsJoin, centerID).
		Where("clients.deleted_at IS NULL").
		Group("clients.id").
		Order("clients.id ASC").
		Offset(offset).
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, nil, 0, err
	}

	clients := make([]models.Client, 0, len(rows))
	metrics := make([]ClientMetrics, 0, len(rows))
	for i := range rows {
		clients = append(clients, rows[i].Client)
		var lastVisit *time.Time
		if rows[i].LastVisit.Valid {
			parsed, err := parseAdminLastVisit(rows[i].LastVisit.String)
			if err != nil {
				return nil, nil, 0, err
			}
			lastVisit = parsed
		}
		metrics = append(metrics, ClientMetrics{
			VisitsCount: rows[i].VisitsCount,
			TotalSpent:  rows[i].TotalSpent,
			LastVisit:   lastVisit,
		})
	}
	return clients, metrics, total, nil
}

// FindById implements ClientRepository
func (r *ClientRepositoryImpl) FindById(clienId uint) (*models.Client, error) {
	var user models.Client
	err := r.db.First(&user, clienId).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail implements ClientRepository
func (r *ClientRepositoryImpl) FindByEmail(email string) (*models.Client, error) {
	var client models.Client
	err := r.db.Where("email = ?", email).First(&client).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &client, nil
}

// FindByUsername implements ClientRepository
func (r *ClientRepositoryImpl) FindByUsername(username string) (*models.Client, error) {
	var client models.Client
	err := r.db.Where("username = ?", username).First(&client).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &client, nil
}
