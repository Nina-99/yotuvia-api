package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type ReservationController struct {
	reservationService service.ReservationService
}

func NewReservationController(service service.ReservationService) *ReservationController {
	return &ReservationController{reservationService: service}
}

// writeError maps any service error through apperrors.MapToHTTP so clients get
// the canonical status + stable message (403 for ownership, 404 for missing,
// etc.) and internal details never leak.
func (c *ReservationController) writeError(ctx *gin.Context, err error) {
	status, msg := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": msg})
}

// parsePositiveID parses a path parameter that must be a positive integer.
func parsePositiveID(raw string) (uint, bool) {
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, false
	}
	return uint(id), true
}

// ──────────────────────────────────────────────
//  CREATE
// ──────────────────────────────────────────────

func (c *ReservationController) Create(ctx *gin.Context) {
	clientID := ctx.GetUint("clientID")

	var req request.CreateReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.ClientID = clientID

	if err := c.reservationService.Create(req); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Reservation created successfully"})
}

// ──────────────────────────────────────────────
//  UPDATE
// ──────────────────────────────────────────────

func (c *ReservationController) Update(ctx *gin.Context) {
	reservationID, ok := parsePositiveID(ctx.Param("id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req request.CreateReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := c.reservationService.Update(
		reservationID,
		req,
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
		ctx.GetString("role"),
	); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Reservation updated successfully"})
}

// ──────────────────────────────────────────────
//  CANCEL — Client cancels own reservation
// ──────────────────────────────────────────────

func (c *ReservationController) Cancel(ctx *gin.Context) {
	clientID := ctx.GetUint("clientID")

	reservationID, ok := parsePositiveID(ctx.Param("id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req request.CancelReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := c.reservationService.Cancel(reservationID, clientID, req.Reason); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Reservation cancelled successfully"})
}

// ──────────────────────────────────────────────
//  DELETE
// ──────────────────────────────────────────────

func (c *ReservationController) Delete(ctx *gin.Context) {
	reservationID, ok := parsePositiveID(ctx.Param("id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := c.reservationService.Delete(
		reservationID,
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
		ctx.GetString("role"),
	); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Reservation deleted successfully"})
}

// ──────────────────────────────────────────────
//  FIND
// ──────────────────────────────────────────────

func (c *ReservationController) FindAll(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Scoped to the caller's centerID claim — never the global list.
	reservations, total, err := c.reservationService.FindAll(page, limit, ctx.GetUint("centerID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, response.PaginatedResponse{
		Data:  reservations,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

func (c *ReservationController) FindById(ctx *gin.Context) {
	reservationID, ok := parsePositiveID(ctx.Param("id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	reservation, err := c.reservationService.FindById(
		reservationID,
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
		ctx.GetString("role"),
	)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, reservation)
}

func (c *ReservationController) FindByClient(ctx *gin.Context) {
	clientID, ok := parsePositiveID(ctx.Param("clientId"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID format"})
		return
	}

	reservations, err := c.reservationService.FindByClientID(clientID, ctx.GetUint("clientID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, reservations)
}

func (c *ReservationController) FindUpcomingByClient(ctx *gin.Context) {
	clientID, ok := parsePositiveID(ctx.Param("clientId"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID format"})
		return
	}

	reservation, err := c.reservationService.FindUpcomingByClientID(clientID, ctx.GetUint("clientID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, reservation)
}

func (c *ReservationController) FindByBeautyCenter(ctx *gin.Context) {
	centerID, ok := parsePositiveID(ctx.Param("centerId"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID format"})
		return
	}

	reservations, err := c.reservationService.FindByBeautyCenterID(centerID, ctx.GetUint("centerID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, reservations)
}

// ──────────────────────────────────────────────
//  STATUS — Beauty center updates status
// ──────────────────────────────────────────────

func (c *ReservationController) UpdateStatus(ctx *gin.Context) {
	reservationID, ok := parsePositiveID(ctx.Param("id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := c.reservationService.UpdateStatus(reservationID, req.Status, ctx.GetUint("centerID")); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Status updated successfully"})
}

// ──────────────────────────────────────────────
//  ASSIGN PERSONNEL
// ──────────────────────────────────────────────

func (c *ReservationController) AssignPersonnel(ctx *gin.Context) {
	reservationID, ok := parsePositiveID(ctx.Param("id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req struct {
		PersonnelID uint `json:"personnel_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := c.reservationService.AssignPersonnel(reservationID, req.PersonnelID, ctx.GetUint("centerID")); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Personnel assigned successfully"})
}

// ──────────────────────────────────────────────
//  QR
// ──────────────────────────────────────────────

func (c *ReservationController) GenerateQR(ctx *gin.Context) {
	reservationID, ok := parsePositiveID(ctx.Param("id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	token, err := c.reservationService.GenerateVerificationToken(
		reservationID,
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
		ctx.GetString("role"),
	)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"token": token})
}

func (c *ReservationController) VerifyQR(ctx *gin.Context) {
	token := ctx.Query("token")
	if token == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "token query param is required"})
		return
	}

	beautyCenterID := ctx.GetUint("centerID")

	if err := c.reservationService.VerifyQR(token, beautyCenterID); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Reservation verified successfully"})
}
