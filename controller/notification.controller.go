package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/service"
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

	if err := c.notificationService.RegisterPushDevice(clientID, req.PushToken, req.Platform); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Device registered"})
}

// ─── Helpers ─────────────────────────────────

func mapNotificationToResponse(n *models.Notification) response.NotificationResponse {
	resp := response.NotificationResponse{
		ID:          n.ID,
		ClientID:    n.ClientID,
		PersonnelID: n.PersonnelID,
		Type:        n.Type,
		Title:     n.Title,
		Message:   n.Message,
		Data:      n.Data,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
		ReadAt:    n.ReadAt,
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
