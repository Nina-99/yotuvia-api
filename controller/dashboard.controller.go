package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

const dateLayout = "2006-01-02"

// DashboardController serves the beauty_center admin dashboard stats endpoint.
type DashboardController struct {
	dashboardService service.DashboardService
}

func NewDashboardController(service service.DashboardService) *DashboardController {
	return &DashboardController{dashboardService: service}
}

// writeError maps any service error through apperrors.MapToHTTP so clients get
// the canonical status + stable message and internal details never leak.
func (c *DashboardController) writeError(ctx *gin.Context, err error) {
	status, msg := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": msg})
}

// parseDateOrDefault parses a date string in "2006-01-02" format.
// Returns (zero time, error) if parsing fails.
func parseDateOrDefault(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(dateLayout, dateStr)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// startOfDayUTC returns midnight UTC for the day containing t.
func startOfDayUTC(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// GetStats handles GET /dashboard/stats. JWTAuthMiddleware stores the caller's
// centerID claim only when it is nonzero; RequireRole("beauty_center") runs
// before this handler. A missing or malformed centerID claim is treated as a
// scoping failure (403, per api-authz spec) because the query would otherwise
// be unscoped.
//
// Optional query params:
//   - from: start date (inclusive) in YYYY-MM-DD UTC
//   - to:   end date (inclusive) in YYYY-MM-DD UTC
//
// Default (both absent): last 30 days [startOfDayUTC(now)-29d, startOfDayUTC(now)]
// Only from provided:    31 days [from, from+30d]
// Validation: malformed date → 400; from > to → 400; future to allowed (zero-filled).
func (c *DashboardController) GetStats(ctx *gin.Context) {
	raw, exists := ctx.Get("centerID")
	if !exists {
		c.writeError(ctx, apperrors.ErrForbidden)
		return
	}
	centerID, ok := raw.(uint)
	if !ok || centerID == 0 {
		c.writeError(ctx, apperrors.ErrForbidden)
		return
	}

	fromStr := ctx.Query("from")
	toStr := ctx.Query("to")

	var from, to time.Time
	var err error

	// Parse dates if provided
	if fromStr != "" {
		from, err = parseDateOrDefault(fromStr)
		if err != nil {
			c.writeError(ctx, apperrors.ErrBadRequest)
			return
		}
	}
	if toStr != "" {
		to, err = parseDateOrDefault(toStr)
		if err != nil {
			c.writeError(ctx, apperrors.ErrBadRequest)
			return
		}
	}

	// Determine date range
	now := time.Now().UTC()
	todayStart := startOfDayUTC(now)

	if fromStr == "" && toStr == "" {
		// Default: last 30 days [today-29d, today]
		from = todayStart.AddDate(0, 0, -29)
		to = todayStart
	} else if fromStr != "" && toStr == "" {
		// Only from provided: 31 days [from, from+30d]
		to = from.AddDate(0, 0, 30)
	} else if fromStr == "" && toStr != "" {
		// Only to provided: default from = to - 30 days
		from = to.AddDate(0, 0, -30)
	}
	// Both provided: use as-is

	// Validate from <= to
	if from.After(to) {
		c.writeError(ctx, apperrors.ErrBadRequest)
		return
	}

	// Optional churnDays (feat-admin-analytics Phase 3): at-risk window in
	// days. Default 30; accepted 1–365 → 400 otherwise (api-error-contract).
	churnDays := 30
	if rawChurn := ctx.Query("churnDays"); rawChurn != "" {
		parsed, parseErr := strconv.Atoi(rawChurn)
		if parseErr != nil || parsed < 1 || parsed > 365 {
			c.writeError(ctx, apperrors.ErrBadRequest)
			return
		}
		churnDays = parsed
	}

	resp, err := c.dashboardService.GetStatsWithRange(centerID, from, to, churnDays)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, resp)
}
