package tests

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// seedReservationDTOData creates a beauty center, client, personnel, service,
// and a reservation with one reservation-service row so the DTO contract can be
// asserted end-to-end through the HTTP router.
func seedReservationDTOData(t *testing.T, db *gorm.DB) *models.Reservation {
	t.Helper()
	bc := models.BeautyCenter{
		Name:     "Salón Bella",
		Address:  "Calle 1",
		Email:    "bella@test.com",
		Phone:    "5550100",
		Currency: "USD",
		IsActive: true,
	}
	if err := db.Create(&bc).Error; err != nil {
		t.Fatalf("failed to seed beauty center: %v", err)
	}

	client := models.Client{
		Model:     gorm.Model{ID: 1},
		FirstName: "María",
		LastName:  "Gómez",
		Username:  "maria",
		Phone:     "5550001",
		Email:     "maria@test.com",
		Password:  "hashed",
		IsActive:  true,
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("failed to seed client: %v", err)
	}

	personnel := models.Personnel{
		Model:          gorm.Model{ID: 1},
		BeautyCenterID: bc.ID,
		FirstName:      "Jane",
		LastName:       "Doe",
		Email:          "jane.doe@example.com",
		Phone:          "5550002",
		IsActive:       true,
	}
	if err := db.Create(&personnel).Error; err != nil {
		t.Fatalf("failed to seed personnel: %v", err)
	}

	svc := models.Service{
		BeautyCenterID: bc.ID,
		CategoryID:     1,
		Name:           "Corte",
		Description:    "Corte de pelo",
		Duration:       30,
		Price:          50,
	}
	if err := db.Create(&svc).Error; err != nil {
		t.Fatalf("failed to seed service: %v", err)
	}

	res := &models.Reservation{
		BeautyCenterID: bc.ID,
		ClientID:       client.ID,
		PersonnelID:    personnel.ID,
		Date:           time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		StartTime:      "10:00",
		EndTime:        "10:30",
		TotalPrice:     100,
		Status:         "PENDING",
		ReservationServices: []models.ReservationService{{
			ServiceID:   svc.ID,
			ServiceName: "Corte",
			Price:       50,
			Duration:    30,
		}},
	}
	if err := db.Create(res).Error; err != nil {
		t.Fatalf("failed to seed reservation: %v", err)
	}
	return res
}

// TestReservationFindById_DTOContract — GET /reservation/:id must return the
// flattened ReservationResponse DTO: beauty_center_name, client_name, currency,
// services as {service_id, service_name, price, duration}, client as
// {first_name, last_name, phone}, personnel as {first_name, last_name}, and
// never the GORM model internals (verification_token, timestamps, soft-delete).
func TestReservationFindById_DTOContract(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db := authzTestRouter(t)
	res := seedReservationDTOData(t, db)

	owner := roleToken(t, 1, 1, "client")
	w := do(r, http.MethodGet, "/api/reservation/1", owner, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		ID               uint    `json:"id"`
		BeautyCenterID   uint    `json:"beauty_center_id"`
		BeautyCenterName string  `json:"beauty_center_name"`
		ClientID         uint    `json:"client_id"`
		ClientName       string  `json:"client_name"`
		Currency         string  `json:"currency"`
		PersonnelID      uint    `json:"personnel_id"`
		StartTime        string  `json:"start_time"`
		EndTime          string  `json:"end_time"`
		TotalPrice       float64 `json:"total_price"`
		Status           string  `json:"status"`
		Services         []struct {
			ServiceID   uint    `json:"service_id"`
			ServiceName string  `json:"service_name"`
			Price       float64 `json:"price"`
			Duration    int     `json:"duration"`
		} `json:"services"`
		Client *struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Phone     string `json:"phone"`
		} `json:"client"`
		Personnel *struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"personnel"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Flattened top-level fields come from the preloaded relations.
	assert.Equal(t, res.ID, body.ID)
	assert.Equal(t, uint(1), body.BeautyCenterID)
	assert.Equal(t, "Salón Bella", body.BeautyCenterName)
	assert.Equal(t, uint(1), body.ClientID)
	assert.Equal(t, "María Gómez", body.ClientName)
	assert.Equal(t, "USD", body.Currency)
	assert.Equal(t, uint(1), body.PersonnelID)
	assert.Equal(t, "10:00", body.StartTime)
	assert.Equal(t, "10:30", body.EndTime)
	assert.Equal(t, 100.0, body.TotalPrice)
	assert.Equal(t, "PENDING", body.Status)

	// Services are flattened to {service_id, service_name, price, duration}.
	if assert.Len(t, body.Services, 1) {
		assert.Equal(t, uint(1), body.Services[0].ServiceID)
		assert.Equal(t, "Corte", body.Services[0].ServiceName)
		assert.Equal(t, 50.0, body.Services[0].Price)
		assert.Equal(t, 30, body.Services[0].Duration)
	}

	// Client lite object contains only first_name/last_name/phone.
	if assert.NotNil(t, body.Client) {
		assert.Equal(t, "María", body.Client.FirstName)
		assert.Equal(t, "Gómez", body.Client.LastName)
		assert.Equal(t, "5550001", body.Client.Phone)
	}

	// Personnel lite object contains only first_name/last_name.
	if assert.NotNil(t, body.Personnel) {
		assert.Equal(t, "Jane", body.Personnel.FirstName)
		assert.Equal(t, "Doe", body.Personnel.LastName)
	}

	// GORM internals must never reach the wire.
	raw := w.Body.String()
	assert.NotContains(t, raw, "verification_token")
	assert.NotContains(t, raw, "token_expires_at")
	assert.NotContains(t, raw, "created_at")
	assert.NotContains(t, raw, "updated_at")
	assert.NotContains(t, raw, "deleted_at")
	assert.NotContains(t, raw, "password")
}

// TestReservationFindAll_EmptyReturnsEmptyArray — an empty result must serialize
// data as [] (never null) with total 0.
func TestReservationFindAll_EmptyReturnsEmptyArray(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, _ := authzTestRouter(t)

	admin := roleToken(t, 7, 1, "beauty_center")
	w := do(r, http.MethodGet, "/api/reservation", admin, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data  json.RawMessage `json:"data"`
		Total int64           `json:"total"`
		Page  int             `json:"page"`
		Limit int             `json:"limit"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	assert.Equal(t, "[]", string(body.Data))
	assert.Equal(t, int64(0), body.Total)
	assert.Equal(t, 1, body.Page)
	assert.Equal(t, 20, body.Limit)
}

// TestReservationFindByCenter_DTOList — GET /reservation/center/:centerId must
// return a DTO array (flattened fields, no model internals), never models.
func TestReservationFindByCenter_DTOList(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db := authzTestRouter(t)
	seedReservationDTOData(t, db)

	admin := roleToken(t, 7, 1, "beauty_center")
	w := do(r, http.MethodGet, "/api/reservation/center/1", admin, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var list []struct {
		ID               uint   `json:"id"`
		BeautyCenterName string `json:"beauty_center_name"`
		ClientName       string `json:"client_name"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if assert.Len(t, list, 1) {
		assert.Equal(t, uint(1), list[0].ID)
		assert.Equal(t, "Salón Bella", list[0].BeautyCenterName)
		assert.Equal(t, "María Gómez", list[0].ClientName)
	}
	assert.NotContains(t, w.Body.String(), "verification_token")
}

// TestReservationDTO_OmitemptySemantics — when a reservation has no assigned
// personnel row and no cancel reason, the nested personnel object and
// cancel_reason must be omitted (the scalar personnel_id is still emitted),
// while the client object stays present.
func TestReservationDTO_OmitemptySemantics(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db := authzTestRouter(t)

	// Client row exists; PersonnelID points at a row that does not exist, so
	// the preloaded Personnel is zero-valued and must be omitted.
	seedClient(t, db, 1, "maria", "maria@test.com")
	res := &models.Reservation{
		BeautyCenterID: 1,
		ClientID:       1,
		PersonnelID:    1,
		Date:           time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		StartTime:      "10:00",
		EndTime:        "10:30",
		TotalPrice:     100,
		Status:         "PENDING",
	}
	if err := db.Create(res).Error; err != nil {
		t.Fatalf("failed to seed reservation: %v", err)
	}

	owner := roleToken(t, 1, 1, "client")
	w := do(r, http.MethodGet, "/api/reservation/1", owner, "")

	assert.Equal(t, http.StatusOK, w.Code)

	raw := w.Body.String()
	// Scalar personnel_id is not omitempty and must remain.
	assert.Contains(t, raw, `"personnel_id":1`)
	// Nested personnel object and cancel_reason are omitted when absent.
	assert.NotContains(t, raw, `"personnel":{`)
	assert.NotContains(t, raw, "cancel_reason")
	// The client lite object is still emitted from the preloaded client row.
	assert.Contains(t, raw, `"client":{"first_name":"Test"`)
	// A reservation with no services omits the services key entirely.
	assert.NotContains(t, raw, "services")
}
