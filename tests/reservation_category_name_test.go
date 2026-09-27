package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestClientReservationList_CategoryName pins the wire contract the mobile
// client reads: the client's own reservation list (`GET /reservation/client/:id`)
// carries each service line's category as a flat `category_name`, and omits the
// key when the line's service has no category. The list serialises the GORM
// model directly, so this asserts the model-side projection, not the DTO.
func TestClientReservationList_CategoryName(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db := authzTestRouter(t)

	bc := models.BeautyCenter{Name: "Salón Bella", Address: "Calle 1", Email: "bella@test.com", Phone: "5550100", Currency: "USD", IsActive: true}
	if err := db.Create(&bc).Error; err != nil {
		t.Fatalf("failed to seed beauty center: %v", err)
	}

	cat := models.Category{BeautyCenterID: bc.ID, Name: "Cabello"}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("failed to seed category: %v", err)
	}

	// One service WITH a category, one whose CategoryID points at no row.
	withCat := models.Service{BeautyCenterID: bc.ID, CategoryID: cat.ID, Name: "Corte", Duration: 30, Price: 50}
	if err := db.Create(&withCat).Error; err != nil {
		t.Fatalf("failed to seed service with category: %v", err)
	}
	noCat := models.Service{BeautyCenterID: bc.ID, CategoryID: 9999, Name: "Peinado", Duration: 30, Price: 20}
	if err := db.Create(&noCat).Error; err != nil {
		t.Fatalf("failed to seed service without category: %v", err)
	}

	seedClient(t, db, 1, "maria", "maria@test.com")

	res := &models.Reservation{
		BeautyCenterID: bc.ID,
		ClientID:       1,
		PersonnelID:    1,
		Date:           time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		StartTime:      "10:00",
		EndTime:        "11:00",
		TotalPrice:     70,
		Status:         "PENDING",
		ReservationServices: []models.ReservationService{
			{ServiceID: withCat.ID, ServiceName: "Corte", Price: 50, Duration: 30},
			{ServiceID: noCat.ID, ServiceName: "Peinado", Price: 20, Duration: 30},
		},
	}
	if err := db.Create(res).Error; err != nil {
		t.Fatalf("failed to seed reservation: %v", err)
	}

	owner := roleToken(t, 1, 1, "client")
	w := do(r, http.MethodGet, "/api/reservation/client/1", owner, "")
	assert.Equal(t, http.StatusOK, w.Code)

	var body []struct {
		Services []struct {
			ServiceName  string  `json:"service_name"`
			CategoryName *string `json:"category_name"`
		} `json:"services"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !assert.Len(t, body, 1) || !assert.Len(t, body[0].Services, 2) {
		t.Fatalf("unexpected response shape: %s", w.Body.String())
	}

	byName := map[string]*string{}
	for _, s := range body[0].Services {
		byName[s.ServiceName] = s.CategoryName
	}

	if assert.NotNil(t, byName["Corte"], "a line whose service HAS a category must carry category_name") {
		assert.Equal(t, "Cabello", *byName["Corte"])
	}
	assert.Nil(t, byName["Peinado"], "a line with an unavailable category must omit category_name")
}

// countingLogger counts every SQL trace GORM emits while delegating the rest of
// the logger contract to a silent default.
type countingLogger struct {
	logger.Interface
	count *int32
}

func (l countingLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	atomic.AddInt32(l.count, 1)
}

// TestClientReservationList_CategoryName_NoNPlusOne proves the category
// projection adds NO query: the client list issues the same fixed number of
// statements for a single reservation and for many, so no per-line query sneaks
// in for the label.
func TestClientReservationList_CategoryName_NoNPlusOne(t *testing.T) {
	var count int32
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: countingLogger{Interface: logger.Default.LogMode(logger.Silent), count: &count},
	})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	for _, m := range []interface{}{
		&models.BeautyCenter{}, &models.Client{}, &models.Personnel{},
		&models.Category{}, &models.Service{}, &models.Reservation{}, &models.ReservationService{},
	} {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatalf("failed to migrate %T: %v", m, err)
		}
	}

	bc := models.BeautyCenter{Name: "C", Address: "a", Email: "c@t.com", Phone: "1", Currency: "USD", IsActive: true}
	assert.NoError(t, db.Create(&bc).Error)
	cat := models.Category{BeautyCenterID: bc.ID, Name: "Cabello"}
	assert.NoError(t, db.Create(&cat).Error)
	svc := models.Service{BeautyCenterID: bc.ID, CategoryID: cat.ID, Name: "Corte", Duration: 30, Price: 50}
	assert.NoError(t, db.Create(&svc).Error)
	assert.NoError(t, db.Create(&models.Client{Model: gorm.Model{ID: 1}, FirstName: "T", LastName: "C", Username: "t", Phone: "1", Email: "t@t.com", Password: "h"}).Error)

	seedOne := func() {
		res := &models.Reservation{
			BeautyCenterID: bc.ID, ClientID: 1, PersonnelID: 1, Status: "PENDING", TotalPrice: 100,
			ReservationServices: []models.ReservationService{
				{ServiceID: svc.ID, ServiceName: "Corte", Price: 50, Duration: 30},
				{ServiceID: svc.ID, ServiceName: "Corte", Price: 50, Duration: 30},
			},
		}
		if err := db.Create(res).Error; err != nil {
			t.Fatalf("seed reservation: %v", err)
		}
	}

	repo := repository.NewReservationRepositoryImpl(db)

	measure := func() int32 {
		atomic.StoreInt32(&count, 0)
		rows, err := repo.FindByClientID(1)
		assert.NoError(t, err)
		assert.NotEmpty(t, rows)
		return atomic.LoadInt32(&count)
	}

	seedOne()
	one := measure()

	seedOne()
	seedOne()
	many := measure()

	t.Logf("client reservation list query count: 1 reservation -> %d, 3 reservations -> %d", one, many)
	assert.Equal(t, one, many, "the number of statements must not grow with the row count (no N+1)")
	assert.Less(t, many, int32(12), "the list must stay on a fixed preload budget")
}
