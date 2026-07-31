package controller

import (
	"log"
	"net/http"
	"strconv"

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

// ──────────────────────────────────────────────
//  CREATE
// ──────────────────────────────────────────────

func (c *ReservationController) Create(ctx *gin.Context) {
	clientID := ctx.GetUint("clientID")

	var req request.CreateReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.ClientID = clientID

	if err := c.reservationService.Create(req); err != nil {
		log.Printf("Error creating reservation: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Reservation created successfully"})
}

// ──────────────────────────────────────────────
//  UPDATE
// ──────────────────────────────────────────────

func (c *ReservationController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	reservationID, err := strconv.Atoi(idParam)
	if err != nil || reservationID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req request.CreateReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.ClientID = ctx.GetUint("clientID")

	if err := c.reservationService.Update(uint(reservationID), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Reservation updated successfully"})
}

// ──────────────────────────────────────────────
//  CANCEL — Client cancels own reservation
// ──────────────────────────────────────────────

func (c *ReservationController) Cancel(ctx *gin.Context) {
	clientID := ctx.GetUint("clientID")
	if clientID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "client ID not found in token"})
		return
	}

	idParam := ctx.Param("id")
	reservationID, err := strconv.Atoi(idParam)
	if err != nil || reservationID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req request.CancelReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.reservationService.Cancel(uint(reservationID), clientID, req.Reason); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Reservation cancelled successfully"})
}

// ──────────────────────────────────────────────
//  DELETE
// ──────────────────────────────────────────────

func (c *ReservationController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	reservationID, err := strconv.Atoi(idParam)
	if err != nil || reservationID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := c.reservationService.Delete(uint(reservationID)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	reservations, total, err := c.reservationService.FindAll(page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	idParam := ctx.Param("id")
	reservationID, err := strconv.Atoi(idParam)
	if err != nil || reservationID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	reservation, err := c.reservationService.FindById(uint(reservationID))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
		return
	}

	ctx.JSON(http.StatusOK, reservation)
}

func (c *ReservationController) FindByClient(ctx *gin.Context) {
	clientIDParam := ctx.Param("clientId")
	clientID, err := strconv.Atoi(clientIDParam)
	if err != nil || clientID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID format"})
		return
	}

	reservations, err := c.reservationService.FindByClientID(uint(clientID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, reservations)
}

func (c *ReservationController) FindUpcomingByClient(ctx *gin.Context) {
	clientIDParam := ctx.Param("clientId")
	clientID, err := strconv.Atoi(clientIDParam)
	if err != nil || clientID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID format"})
		return
	}

	reservation, err := c.reservationService.FindUpcomingByClientID(uint(clientID))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "No upcoming reservation found"})
		return
	}

	ctx.JSON(http.StatusOK, reservation)
}

func (c *ReservationController) FindByBeautyCenter(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil || centerID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID format"})
		return
	}

	reservations, err := c.reservationService.FindByBeautyCenterID(uint(centerID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, reservations)
}

// ──────────────────────────────────────────────
//  STATUS — Beauty center updates status
// ──────────────────────────────────────────────

func (c *ReservationController) UpdateStatus(ctx *gin.Context) {
	idParam := ctx.Param("id")
	reservationID, err := strconv.Atoi(idParam)
	if err != nil || reservationID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.reservationService.UpdateStatus(uint(reservationID), req.Status); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Status updated successfully"})
}

// ──────────────────────────────────────────────
//  ASSIGN PERSONNEL
// ──────────────────────────────────────────────

func (c *ReservationController) AssignPersonnel(ctx *gin.Context) {
	idParam := ctx.Param("id")
	reservationID, err := strconv.Atoi(idParam)
	if err != nil || reservationID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req struct {
		PersonnelID uint `json:"personnel_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.reservationService.AssignPersonnel(uint(reservationID), req.PersonnelID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Personnel assigned successfully"})
}

// ──────────────────────────────────────────────
//  QR
// ──────────────────────────────────────────────

func (c *ReservationController) GenerateQR(ctx *gin.Context) {
	idParam := ctx.Param("id")
	reservationID, err := strconv.Atoi(idParam)
	if err != nil || reservationID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	token, err := c.reservationService.GenerateVerificationToken(uint(reservationID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Reservation verified successfully"})
}
