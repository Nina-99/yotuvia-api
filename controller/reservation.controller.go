package controller

import (
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

func (c *ReservationController) Create(ctx *gin.Context) {
	var req request.CreateReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.reservationService.Create(req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Created"})
}

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

	if err := c.reservationService.Update(uint(reservationID), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Update"})
}

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
	ctx.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

func (c *ReservationController) FindAll(ctx *gin.Context) {
	reservations, err := c.reservationService.FindAll()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var res []response.ReservationResponse
	for _, reservation := range reservations {
		res = append(res, response.ReservationResponse{ID: reservation.ID, BeautyCenterID: reservation.BeautyCenterID, ClientID: reservation.ClientID, Date: reservation.Date, StartTime: reservation.StartTime, EndTime: reservation.EndTime, Status: reservation.Status})
	}
	ctx.JSON(http.StatusOK, res)
}

func (c *ReservationController) FindById(ctx *gin.Context) {
	idParam := ctx.Param("id")
	reservationID, err := strconv.Atoi(idParam)
	if err != nil || reservationID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	s, err := c.reservationService.FindById(uint(reservationID))
	if err != nil || s == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
		return
	}

	res := response.ReservationResponse{
		ID:             s.ID,
		BeautyCenterID: s.BeautyCenterID,
		ClientID:       s.ClientID,
		Date:           s.Date,
		StartTime:      s.StartTime,
		EndTime:        s.EndTime,
		Status:         s.Status,
	}

	ctx.JSON(http.StatusOK, res)
}
