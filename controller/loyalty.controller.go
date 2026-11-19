package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

type LoyaltyController struct {
	loyaltyService service.LoyaltyService
}

func NewLoyaltyController(service service.LoyaltyService) *LoyaltyController {
	return &LoyaltyController{loyaltyService: service}
}

// writeError maps any service error through apperrors.MapToHTTP so clients get
// the canonical status + stable message (403 for ownership, 404 for missing,
// etc.) and internal details never leak.
func (c *LoyaltyController) writeError(ctx *gin.Context, err error) {
	status, msg := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": msg})
}

// CreateProgram creates a new loyalty program for a beauty center.
func (c *LoyaltyController) CreateProgram(ctx *gin.Context) {
	var req request.CreateLoyaltyProgrmaRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeBadRequest(ctx, err)
		return
	}

	// The program's owner is the caller's center from the token, never the body:
	// a center-A admin must not create a center-B program by sending
	// `beauty_center_id`. Same contract as reservation Create overriding
	// req.ClientID. A beauty_center token with no centerID claim (0) cannot own a
	// program, so it is rejected rather than persisting a zero-center orphan —
	// matching the actorCenterID == 0 guard the service uses for update/delete.
	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	req.BeautyCenterID = centerID

	res, err := c.loyaltyService.CreateProgram(req)
	if err != nil {
		writeBadRequest(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

// GetCLientCards returns all loyalty cards for a specific customer.
func (c *LoyaltyController) GetCLientCards(ctx *gin.Context) {
	clientIdParam := ctx.Param("clientId")
	clientId, err := strconv.Atoi(clientIdParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}

	res, err := c.loyaltyService.GetClientsCards(uint(clientId), ctx.GetUint("clientID"), ctx.GetString("role"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// GetClientHistory returns the client's punch timeline (newest first).
func (c *LoyaltyController) GetClientHistory(ctx *gin.Context) {
	clientIdParam := ctx.Param("clientId")
	clientId, err := strconv.Atoi(clientIdParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}

	res, err := c.loyaltyService.GetHistory(uint(clientId), ctx.GetUint("clientID"), ctx.GetString("role"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// GetPendingRewards returns the client's completed-but-unredeemed rewards.
func (c *LoyaltyController) GetPendingRewards(ctx *gin.Context) {
	clientIdParam := ctx.Param("clientId")
	clientId, err := strconv.Atoi(clientIdParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}

	res, err := c.loyaltyService.GetPendingRewards(uint(clientId), ctx.GetUint("clientID"), ctx.GetString("role"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// GetPublicPrograms returns anonymous-safe active program summaries for a
// center. No auth: this powers the pre-booking badge for guests.
func (c *LoyaltyController) GetPublicPrograms(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	res, err := c.loyaltyService.GetPublicPrograms(uint(centerID))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// FindByCenter returns all loyalty programs for a beauty center.
func (c *LoyaltyController) FindByCenter(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	res, err := c.loyaltyService.FindProgramsByCenter(uint(centerID), ctx.GetUint("centerID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// UpdateProgram updates an existing loyalty program.
func (c *LoyaltyController) UpdateProgram(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var req request.UpdateLoyaltyProgramRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeBadRequest(ctx, err)
		return
	}

	res, err := c.loyaltyService.UpdateProgram(uint(id), ctx.GetUint("centerID"), req)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// DeleteProgram soft-deletes a loyalty program.
func (c *LoyaltyController) DeleteProgram(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := c.loyaltyService.DeleteProgram(uint(id), ctx.GetUint("centerID")); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusNoContent, nil)
}

// ToggleProgram enables or disables a loyalty program.
func (c *LoyaltyController) ToggleProgram(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		writeBadRequest(ctx, err)
		return
	}

	if err := c.loyaltyService.ToggleProgram(uint(id), ctx.GetUint("centerID"), body.IsActive); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"is_active": body.IsActive})
}

// GetCenterSettings returns the loyalty settings for a beauty center.
func (c *LoyaltyController) GetCenterSettings(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	res, err := c.loyaltyService.GetCenterSettings(uint(centerID), ctx.GetUint("centerID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// UpdateCenterSettings updates the loyalty settings for a beauty center.
func (c *LoyaltyController) UpdateCenterSettings(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	var req request.UpdateLoyaltySettingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeBadRequest(ctx, err)
		return
	}

	if err := c.loyaltyService.UpdateCenterSettings(uint(centerID), ctx.GetUint("centerID"), req); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}

// GetMetrics returns per-program issued-vs-redeemed metrics for the acting
// center. RequireRole proves the beauty_center role; the service verifies the
// centerID claim equals :centerId (403 otherwise).
func (c *LoyaltyController) GetMetrics(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	res, err := c.loyaltyService.GetMetrics(uint(centerID), ctx.GetUint("centerID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}
