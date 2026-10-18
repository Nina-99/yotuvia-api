package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Time blocks (Phase C2): a professional blocks their OWN time; the owner blocks
// one professional of its centre OR the whole centre. The actor id/centre come
// from the JWT, never the body.

func timeBlockRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()
	r := gin.New()
	api := r.Group("/api")
	router.TimeBlockRoutes(api, db, validate)
	return r, db
}

func timeBlockBody(dateKey, start, end string, personnelID *uint) string {
	personnel := "null"
	if personnelID != nil {
		personnel = fmt.Sprintf("%d", *personnelID)
	}
	return fmt.Sprintf(
		`{"date":"%s","start_time":"%s","end_time":"%s","personnel_id":%s,"reason":"break"}`,
		dateKey, start, end, personnel,
	)
}

func TestTimeBlock_StaffBlocksOwnTime(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := timeBlockRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)

	w := do(r, http.MethodPost, "/api/time-blocks", roleToken(t, 7, 1, "personnel"),
		timeBlockBody(bookingTestDateKey, "12:00", "13:00", nil))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var stored models.TimeBlock
	require.NoError(t, db.First(&stored).Error)
	require.NotNil(t, stored.PersonnelID)
	assert.Equal(t, uint(7), *stored.PersonnelID, "the block is forced onto the caller")
	assert.Equal(t, uint(1), stored.BeautyCenterID)
}

func TestTimeBlock_StaffCannotBlockAnotherProfessional(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := timeBlockRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)
	seedDashboardPersonnel(t, db, 9, 1, true)

	other := uint(9)
	w := do(r, http.MethodPost, "/api/time-blocks", roleToken(t, 7, 1, "personnel"),
		timeBlockBody(bookingTestDateKey, "12:00", "13:00", &other))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTimeBlock_OwnerBlocksWholeCentre(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := timeBlockRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)

	w := do(r, http.MethodPost, "/api/time-blocks", roleToken(t, 1, 1, "beauty_center"),
		timeBlockBody(bookingTestDateKey, "00:00", "23:59", nil))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var stored models.TimeBlock
	require.NoError(t, db.First(&stored).Error)
	assert.Nil(t, stored.PersonnelID, "omitted personnel_id = centre-wide")
}

func TestTimeBlock_OwnerCannotBlockForeignProfessional(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := timeBlockRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardCenter(t, db, 2, "Salon Y", "salony@test.com", true)
	seedDashboardPersonnel(t, db, 9, 2, true) // belongs to centre 2

	foreign := uint(9)
	w := do(r, http.MethodPost, "/api/time-blocks", roleToken(t, 1, 1, "beauty_center"),
		timeBlockBody(bookingTestDateKey, "12:00", "13:00", &foreign))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTimeBlock_StaffCannotDeleteCentreBlock(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := timeBlockRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)

	// The owner created a centre-wide block.
	w := do(r, http.MethodPost, "/api/time-blocks", roleToken(t, 1, 1, "beauty_center"),
		timeBlockBody(bookingTestDateKey, "00:00", "23:59", nil))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var block models.TimeBlock
	require.NoError(t, db.First(&block).Error)

	// staff 7 cannot remove it.
	w = do(r, http.MethodDelete, fmt.Sprintf("/api/time-blocks/%d", block.ID), roleToken(t, 7, 1, "personnel"), "")
	assert.Equal(t, http.StatusForbidden, w.Code)

	// The owner can.
	w = do(r, http.MethodDelete, fmt.Sprintf("/api/time-blocks/%d", block.ID), roleToken(t, 1, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
