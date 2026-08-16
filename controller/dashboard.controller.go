package controller

import (
	"net/http"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

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

// GetStats handles GET /dashboard/stats. JWTAuthMiddleware stores the caller's
// centerID claim only when it is nonzero; RequireRole("beauty_center") runs
// before this handler. A missing or malformed centerID claim is treated as a
// scoping failure (403, per api-authz spec) because the query would otherwise
// be unscoped.
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

	resp, err := c.dashboardService.GetStats(centerID)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, resp)
}
