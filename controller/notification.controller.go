package controller

import (
	"net/http"
	"strconv"

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

// GET /notifications/client/:clientId
func (c *NotificationController) GetByClient(ctx *gin.Context) {
	clientIDStr := ctx.Param("clientId")
	clientID, err := strconv.Atoi(clientIDStr)
	if err != nil || clientID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID"})
		return
	}

	notifications, err := c.notificationService.GetByClientID(uint(clientID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	notifications, err := c.notificationService.GetUnreadByClientID(uint(clientID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	count, err := c.notificationService.CountUnread(uint(clientID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	if err := c.notificationService.MarkAsRead(uint(notifID)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	if err := c.notificationService.MarkAllAsRead(uint(clientID)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "All marked as read"})
}

// POST /push-devices/register
func (c *NotificationController) RegisterPushDevice(ctx *gin.Context) {
	var req request.RegisterPushDeviceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "push_token and platform are required"})
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := c.notificationService.RegisterPushDevice(clientID.(uint), req.PushToken, req.Platform); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Device registered"})
}

// ─── Helpers ─────────────────────────────────

func mapNotificationToResponse(n *models.Notification) response.NotificationResponse {
	resp := response.NotificationResponse{
		ID:        n.ID,
		ClientID:  n.ClientID,
		Type:      n.Type,
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
