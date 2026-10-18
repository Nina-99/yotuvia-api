package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

type NotificationController struct {
	notificationService service.NotificationService
}

func NewNotificationController(service service.NotificationService) *NotificationController {
	return &NotificationController{notificationService: service}
}

// writeError maps any service error through apperrors.MapToHTTP so clients get
// the canonical status + stable message (403 for ownership, 404 for missing,
// etc.) and internal details never leak.
func (c *NotificationController) writeError(ctx *gin.Context, err error) {
	status, msg := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": msg})
}

// GET /notifications/client/:clientId
func (c *NotificationController) GetByClient(ctx *gin.Context) {
	clientIDStr := ctx.Param("clientId")
	clientID, err := strconv.Atoi(clientIDStr)
	if err != nil || clientID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID"})
		return
	}

	notifications, err := c.notificationService.GetByClientID(uint(clientID), ctx.GetUint("clientID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, mapNotificationsToResponse(notifications))
}

// GET /notifications/client/:clientId/unread
func (c *NotificationController) GetUnreadByClient(ctx *gin.Context) {
	clientIDStr := ctx.Param("clientId")
	clientID, err := strconv.Atoi(clientIDStr)
	if err != nil || clientID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID"})
		return
	}

	notifications, err := c.notificationService.GetUnreadByClientID(uint(clientID), ctx.GetUint("clientID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, mapNotificationsToResponse(notifications))
}

// GET /notifications/client/:clientId/count
func (c *NotificationController) CountUnread(ctx *gin.Context) {
	clientIDStr := ctx.Param("clientId")
	clientID, err := strconv.Atoi(clientIDStr)
	if err != nil || clientID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID"})
		return
	}

	count, err := c.notificationService.CountUnread(uint(clientID), ctx.GetUint("clientID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"count": count})
}

// PATCH /notifications/:id/read
func (c *NotificationController) MarkAsRead(ctx *gin.Context) {
	idParam := ctx.Param("id")
	notifID, err := strconv.Atoi(idParam)
	if err != nil || notifID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	if err := c.notificationService.MarkAsRead(uint(notifID), ctx.GetUint("clientID")); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Marked as read"})
}

// POST /notifications/client/:clientId/read-all
func (c *NotificationController) MarkAllAsRead(ctx *gin.Context) {
	clientIDStr := ctx.Param("clientId")
	clientID, err := strconv.Atoi(clientIDStr)
	if err != nil || clientID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID"})
		return
	}

	if err := c.notificationService.MarkAllAsRead(uint(clientID), ctx.GetUint("clientID")); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "All marked as read"})
}

// ──────────────────────────────────────────────
//  PERSONNEL NOTIFICATIONS (design D5)
// ──────────────────────────────────────────────

// GET /notifications/personnel — staff reads its own notifications. The actor
// claim (personnelID in ctx "clientID") is the scope; no path param needed.
func (c *NotificationController) GetByPersonnel(ctx *gin.Context) {
	notifications, err := c.notificationService.GetByPersonnelID(ctx.GetUint("clientID"), ctx.GetUint("clientID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, mapNotificationsToResponse(notifications))
}

// GET /notifications/personnel/unread
func (c *NotificationController) GetUnreadByPersonnel(ctx *gin.Context) {
	notifications, err := c.notificationService.GetUnreadByPersonnelID(ctx.GetUint("clientID"), ctx.GetUint("clientID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, mapNotificationsToResponse(notifications))
}

// GET /notifications/personnel/count
func (c *NotificationController) CountUnreadByPersonnel(ctx *gin.Context) {
	count, err := c.notificationService.CountUnreadByPersonnelID(ctx.GetUint("clientID"), ctx.GetUint("clientID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"count": count})
}

// PATCH /notifications/personnel/:id/read
func (c *NotificationController) MarkAsReadByPersonnel(ctx *gin.Context) {
	idParam := ctx.Param("id")
	notifID, err := strconv.Atoi(idParam)
	if err != nil || notifID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	if err := c.notificationService.MarkAsReadByPersonnel(uint(notifID), ctx.GetUint("clientID")); err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Marked as read"})
}

// ──────────────────────────────────────────────
//  CENTER NOTIFICATIONS (design D6)
// ──────────────────────────────────────────────

// centerIDClaimOrForbidden returns the actor's centerID claim or writes a 403
// (design D6). RequireRole guarantees the role but not a nonzero claim —
// jwt.middleware.go stores centerID only when nonzero, and an unscoped query
// would expose every center's rows — so the handlers mirror the dashboard
// precedent (dashboard.controller.go:33-42).
func (c *NotificationController) centerIDClaimOrForbidden(ctx *gin.Context) (uint, bool) {
	raw, exists := ctx.Get("centerID")
	if !exists {
		c.writeError(ctx, apperrors.ErrForbidden)
		return 0, false
	}
	centerID, ok := raw.(uint)
	if !ok || centerID == 0 {
		c.writeError(ctx, apperrors.ErrForbidden)
		return 0, false
	}
	return centerID, true
}

// GET /notifications/center — the actor's centerID claim is the scope; no path
// param is exposed, and cross-center reads are rejected by the service.
func (c *NotificationController) GetByCenter(ctx *gin.Context) {
	centerID, ok := c.centerIDClaimOrForbidden(ctx)
	if !ok {
		return
	}

	notifications, err := c.notificationService.GetByCenterID(centerID, centerID)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, mapNotificationsToResponse(notifications))
}

// GET /notifications/center/unread
func (c *NotificationController) GetUnreadByCenter(ctx *gin.Context) {
	centerID, ok := c.centerIDClaimOrForbidden(ctx)
	if !ok {
		return
	}

	notifications, err := c.notificationService.GetUnreadByCenterID(centerID, centerID)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, mapNotificationsToResponse(notifications))
}

// GET /notifications/center/count
func (c *NotificationController) CountUnreadByCenter(ctx *gin.Context) {
	centerID, ok := c.centerIDClaimOrForbidden(ctx)
	if !ok {
		return
	}

	count, err := c.notificationService.CountUnreadByCenterID(centerID, centerID)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"count": count})
}

// PATCH /notifications/center/:id/read
func (c *NotificationController) MarkAsReadByCenter(ctx *gin.Context) {
	centerID, ok := c.centerIDClaimOrForbidden(ctx)
	if !ok {
		return
	}

	idParam := ctx.Param("id")
	notifID, err := strconv.Atoi(idParam)
	if err != nil || notifID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	if err := c.notificationService.MarkAsReadByCenter(uint(notifID), centerID); err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Marked as read"})
}

// POST /push-devices/register
func (c *NotificationController) RegisterPushDevice(ctx *gin.Context) {
	var req request.RegisterPushDeviceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "push_token and platform are required"})
		return
	}

	clientID := ctx.GetUint("clientID")
	if clientID == 0 {
		c.writeError(ctx, apperrors.ErrUnauthorized)
		return
	}

	if err := c.notificationService.RegisterPushDevice(clientID, req.PushToken, req.Platform, req.P256dh, req.Auth); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Device registered"})
}

// GET /push-devices/vapid-public-key
//
// PUBLIC (no JWT): a signed-out browser may need the application server key
// before it can even ask for notification permission. When Web Push is not
// configured the key is empty and the web client hides the affordance.
func (c *NotificationController) VapidPublicKey(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"public_key": c.notificationService.VapidPublicKey()})
}

// ─── Helpers ─────────────────────────────────

func mapNotificationToResponse(n *models.Notification) response.NotificationResponse {
	resp := response.NotificationResponse{
		ID:             n.ID,
		ClientID:       n.ClientID,
		PersonnelID:    n.PersonnelID,
		BeautyCenterID: n.BeautyCenterID,
		Type:           n.Type,
		Title:          n.Title,
		Message:        n.Message,
		Data:           n.Data,
		IsRead:         n.IsRead,
		CreatedAt:      n.CreatedAt,
		ReadAt:         n.ReadAt,
	}
	return resp
}

func mapNotificationsToResponse(notifications []models.Notification) []response.NotificationResponse {
	result := make([]response.NotificationResponse, len(notifications))
	for i, n := range notifications {
		result[i] = mapNotificationToResponse(&n)
	}
	return result
}
