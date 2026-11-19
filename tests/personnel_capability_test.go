package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"testing"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  Personnel capability at booking + picker filter
// ──────────────────────────────────────────────
//
// The rule (backward compatible): a professional with NO capability rows is
// unrestricted; one WITH rows may only be booked for services among them. The
// picker filter (GET /api/personnel?service_ids=) and the booking validation
// (POST /api/reservation/) must agree, and this file pins that agreement.

// capabilityRouter mounts the picker read path AND the client booking write path
// over the same SQLite database.
func capabilityRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()
	r := gin.New()
	r.Use(gin.Recovery())
	api := r.Group("/api")
	router.PersonnelRoutes(api, db, validate)
	router.ReservationRoutes(api, db, validate)
	router.ServiceRoutes(api, db, validate)
	return r, db
}

// capabilityData seeds one centre/one client and three professionals plus three
// services (ids 1..3). It does NOT seed working hours: callers that book add
// them.
func capabilityData(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Capability Center", "capability@test.com", true)
	seedDashboardClient(t, db, 1, "Cap", "Client")
	// p1 — no capability rows (unrestricted)
	seedDashboardPersonnel(t, db, 1, 1, true)
	// p2 — rows {1,2} (covers {1,2}, missing 3)
	seedDashboardPersonnel(t, db, 2, 1, true)
	// p3 — row {1} (covers {1}, missing 2 and 3)
	seedDashboardPersonnel(t, db, 3, 1, true)
	seedServiceRow(t, db, 1, 1, 30)
	seedServiceRow(t, db, 2, 1, 30)
	seedServiceRow(t, db, 3, 1, 30)
	// A centre that accepts bookings can settle their commission. Without a
	// wallet the solvency gate (correctly) refuses carts above the new-centre
	// overdraft, and this fixture is about personnel capability, not funds.
	require.NoError(t, db.Create(&models.TokenAccount{
		BeautyCenterID: 1, Balance: 1_000_000, Currency: "USD",
	}).Error)
}

func seedCapability(t *testing.T, db *gorm.DB, personnelID, serviceID uint) {
	t.Helper()
	require.NoError(t, db.Create(&models.PersonnelService{
		PersonnelID: personnelID, ServiceID: serviceID,
	}).Error)
}

func pickerIDs(t *testing.T, r *gin.Engine, token, query string) []uint {
	t.Helper()
	w := do(r, http.MethodGet, "/api/personnel?beauty_center_id=1&"+query, token, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var page struct {
		Data []response.PersonnelResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	ids := make([]uint, 0, len(page.Data))
	for _, p := range page.Data {
		ids = append(ids, p.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func bookingIDs(t *testing.T, r *gin.Engine, token, date, start string, serviceIDs []uint, personnelID uint) int {
	t.Helper()
	servicesJSON := "["
	for i, id := range serviceIDs {
		if i > 0 {
			servicesJSON += ","
		}
		servicesJSON += fmt.Sprintf("%d", id)
	}
	servicesJSON += "]"
	body := fmt.Sprintf(
		`{"beauty_center_id":1,"personnel_id":%d,"service_ids":%s,"date":%q,"start_time":%q}`,
		personnelID, servicesJSON, date, start)
	w := do(r, http.MethodPost, "/api/reservation/", token, body)
	return w.Code
}

// TestClientBooking_CapabilityRule covers the three cases of the rule directly
// on the booking endpoint.
func TestClientBooking_CapabilityRule(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	tests := []struct {
		name        string
		capability  [][2]uint // personnelID, serviceID rows
		personnelID uint
		services    []uint
		want        int
	}{
		{
			name:        "no rows is unrestricted",
			personnelID: 1,
			services:    []uint{1},
			want:        http.StatusCreated,
		},
		{
			name:        "rows covering all requested services",
			capability:  [][2]uint{{2, 1}, {2, 2}},
			personnelID: 2,
			services:    []uint{1, 2},
			want:        http.StatusCreated,
		},
		{
			name:        "rows missing one requested service",
			capability:  [][2]uint{{3, 1}},
			personnelID: 3,
			services:    []uint{1, 2},
			want:        http.StatusBadRequest,
		},
		{
			name:        "single service outside the rows",
			capability:  [][2]uint{{3, 1}},
			personnelID: 3,
			services:    []uint{3},
			want:        http.StatusBadRequest,
		},
		{
			name:        "no professional preference is never constrained",
			capability:  [][2]uint{{3, 1}},
			personnelID: 0,
			services:    []uint{2, 3},
			want:        http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, db := capabilityRouter(t)
			capabilityData(t, db)
			for _, c := range tt.capability {
				seedCapability(t, db, c[0], c[1])
			}
			date := bookingTestDate
			seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

			code := bookingIDs(t, r, roleToken(t, 1, 0, "client"),
				date.Format("2006-01-02"), "10:00", tt.services, tt.personnelID)
			assert.Equal(t, tt.want, code)
		})
	}
}

// TestPersonnelPicker_ServiceIDsFilter covers the query-param contract.
func TestPersonnelPicker_ServiceIDsFilter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := capabilityRouter(t)
	capabilityData(t, db)
	seedCapability(t, db, 2, 1)
	seedCapability(t, db, 2, 2)
	seedCapability(t, db, 3, 1)
	token := roleToken(t, 1, 0, "client")

	tests := []struct {
		name  string
		query string
		want  []uint
	}{
		{"absent filter returns everyone (legacy)", "", []uint{1, 2, 3}},
		{"covers {1,2}: unrestricted + full coverage", "service_ids=1,2", []uint{1, 2}},
		{"covers {1}: unrestricted + p2 + p3", "service_ids=1", []uint{1, 2, 3}},
		{"covers {3}: only unrestricted", "service_ids=3", []uint{1}},
		{"covers {1,2,3}: only unrestricted", "service_ids=1,2,3", []uint{1}},
		{"duplicate ids do not inflate coverage", "service_ids=1,1", []uint{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pickerIDs(t, r, token, tt.query))
		})
	}
}

// TestPersonnelPicker_ServiceIDsInvalid covers malformed and unknown filters.
func TestPersonnelPicker_ServiceIDsInvalid(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := capabilityRouter(t)
	capabilityData(t, db)
	token := roleToken(t, 1, 0, "client")

	for _, query := range []string{"service_ids=abc", "service_ids=0", "service_ids=1,,2", "service_ids=999"} {
		t.Run(query, func(t *testing.T) {
			w := do(r, http.MethodGet, "/api/personnel?beauty_center_id=1&"+query, token, "")
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestPersonnelPicker_LegacyUnchanged proves that omitting service_ids leaves the
// existing pagination/filter behaviour untouched.
func TestPersonnelPicker_LegacyUnchanged(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := capabilityRouter(t)
	capabilityData(t, db)
	seedCapability(t, db, 3, 1) // a restricted professional must NOT be hidden when no filter is present
	token := roleToken(t, 1, 0, "client")

	w := do(r, http.MethodGet, "/api/personnel?beauty_center_id=1&page=1&limit=2", token, "")
	require.Equal(t, http.StatusOK, w.Code)
	var page struct {
		Data  []response.PersonnelResponse `json:"data"`
		Total int64                        `json:"total"`
		Page  int                          `json:"page"`
		Limit int                          `json:"limit"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(3), page.Total, "all three personnel are present without a filter")
	assert.Len(t, page.Data, 2, "limit still applies")
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 2, page.Limit)
}

// TestPersonnelPicker_AgreesWithBooking is THE acceptance test: for a centre and
// a requested service set, the professionals the filtered picker offers are
// EXACTLY the professionals the booking validation accepts — across the three
// cases (no rows, rows covering, rows missing one). If the two could disagree,
// the app would show a professional it later rejects.
func TestPersonnelPicker_AgreesWithBooking(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	setup := func(t *testing.T) (*gin.Engine, *gorm.DB, string, string) {
		t.Helper()
		r, db := capabilityRouter(t)
		capabilityData(t, db)
		// p2 covers {1,2}; p3 covers {1} only; p1 is unrestricted.
		seedCapability(t, db, 2, 1)
		seedCapability(t, db, 2, 2)
		seedCapability(t, db, 3, 1)
		date := bookingTestDate
		seedWorkingHour(t, db, 1, int(date.Weekday()), "08:00", "18:00")
		return r, db, date.Format("2006-01-02"), roleToken(t, 1, 0, "client")
	}

	for _, requested := range [][]uint{{1, 2}, {1}, {3}, {1, 2, 3}} {
		requested := requested
		t.Run(fmt.Sprint(requested), func(t *testing.T) {
			// A fresh database per case: booking attempts must not overlap and
			// make a later acceptance a false 409.
			r, _, day, token := setup(t)

			// READ: what the picker offers.
			query := "service_ids="
			for i, id := range requested {
				if i > 0 {
					query += ","
				}
				query += fmt.Sprint(id)
			}
			offered := pickerIDs(t, r, token, query)

			// WRITE: what the booking actually accepts, computed by attempting a
			// booking for every professional the picker could return.
			accepted := []uint{}
			for i, personnelID := range []uint{1, 2, 3} {
				start := fmt.Sprintf("%02d:00", 9+i)
				if bookingIDs(t, r, token, day, start, requested, personnelID) == http.StatusCreated {
					accepted = append(accepted, personnelID)
				}
			}
			sort.Slice(accepted, func(i, j int) bool { return accepted[i] < accepted[j] })

			assert.Equal(t, offered, accepted,
				"picker offers exactly the professionals the booking accepts for %v", requested)
		})
	}
}
