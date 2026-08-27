package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
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
//  CREATE ADMIN — Walk-in booking (beauty_center | personnel)
// ──────────────────────────────────────────────

// CreateAdmin books a walk-in reservation on behalf of an active client
// (design D5). The actor claims come from the JWT context — the body can never
// supply identity — and the created reservation is returned as the computed
// ReservationResponse DTO (design D6: 201 + status/price/end_time for
// immediate UI updates).
func (c *ReservationController) CreateAdmin(ctx *gin.Context) {
	var req request.CreateAdminReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	actor := service.ReservationActor{
		ClientID: ctx.GetUint("clientID"),
		CenterID: ctx.GetUint("centerID"),
		Role:     ctx.GetString("role"),
	}

	reservation, err := c.reservationService.CreateAdminReservation(req, actor)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, mapToReservationResponse(reservation))
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
		Data:  mapReservationsToResponses(reservations),
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

	ctx.JSON(http.StatusOK, mapToReservationResponse(reservation))
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

	// Optional date=YYYY-MM-DD scopes the agenda to one day (design D3).
	// Parsing mirrors /reservation/me: malformed → 400, absent → nil.
	var parsedDate *time.Time
	if raw := ctx.Query("date"); raw != "" {
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			c.writeError(ctx, apperrors.ErrBadRequest)
			return
		}
		parsedDate = &d
	}

	// Optional clientId=X scopes the agenda to one client's history at this
	// center (design D3). Parsing mirrors the date param: malformed or
	// non-positive → 400, absent → nil (unchanged behavior).
	var clientID *uint
	if raw := ctx.Query("clientId"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			c.writeError(ctx, apperrors.ErrBadRequest)
			return
		}
		parsed := uint(id)
		clientID = &parsed
	}

	reservations, err := c.reservationService.FindByBeautyCenterID(centerID, ctx.GetUint("centerID"), parsedDate, clientID)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, mapReservationsToResponses(reservations))
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
//  STATUS BY PERSONNEL — staff accepts/rejects own turno
// ──────────────────────────────────────────────

// UpdateMyStatus lets a staff member transition one of their OWN reservations
// (PENDING→CONFIRMED/CANCELLED, CONFIRMED→COMPLETED/CANCELLED). The personnel
// identity is the JWT clientID claim — never a path parameter (design D1).
func (c *ReservationController) UpdateMyStatus(ctx *gin.Context) {
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

	if err := c.reservationService.UpdateStatusByPersonnel(
		reservationID,
		req.Status,
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
	); err != nil {
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

	reservation, alreadyCompleted, err := c.reservationService.VerifyQR(token, beautyCenterID)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, response.VerifyQRResponse{
		Reservation:      mapToReservationResponse(reservation),
		AlreadyCompleted: alreadyCompleted,
	})
}

// FindMyReservations returns the caller's own agenda (staff self-service). The
// personnel identity is the JWT claim (ctx clientID) — never a path parameter —
// and optional from/to query params bound the date range, defaulting to
// [today, +90d] per the resolved design question.
func (c *ReservationController) FindMyReservations(ctx *gin.Context) {
	personnelID := ctx.GetUint("clientID")

	var from, to *time.Time
	if raw := ctx.Query("from"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			c.writeError(ctx, apperrors.ErrBadRequest)
			return
		}
		from = &parsed
	}
	if raw := ctx.Query("to"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			c.writeError(ctx, apperrors.ErrBadRequest)
			return
		}
		to = &parsed
	}
	if from == nil {
		today := time.Now().Truncate(24 * time.Hour)
		from = &today
	}
	if to == nil {
		plus90 := time.Now().Truncate(24*time.Hour).AddDate(0, 0, 90)
		to = &plus90
	}

	reservations, err := c.reservationService.FindByPersonnelID(personnelID, from, to)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, mapReservationsToResponses(reservations))
}

// ──────────────────────────────────────────────
//  DTO MAPPING
// ──────────────────────────────────────────────

// mapToReservationResponse flattens a Reservation (with preloaded Client,
// Personnel, BeautyCenter and ReservationServices) into the public
// ReservationResponse DTO so GORM model internals (verification token,
// timestamps, soft-delete, nested entities) never reach the wire. The DTO's
// omitempty semantics are preserved: personnel and cancel_reason are omitted
// when absent.
func mapToReservationResponse(r *models.Reservation) response.ReservationResponse {
	resp := response.ReservationResponse{
		ID:               r.ID,
		BeautyCenterID:   r.BeautyCenterID,
		BeautyCenterName: r.BeautyCenter.Name,
		ClientID:         r.ClientID,
		ClientName:       strings.TrimSpace(r.Client.FirstName + " " + r.Client.LastName),
		Currency:         r.BeautyCenter.Currency,
		PersonnelID:      r.PersonnelID,
		Date:             r.Date,
		StartTime:        r.StartTime,
		EndTime:          r.EndTime,
		TotalPrice:       r.TotalPrice,
		Status:           r.Status,
		CancelReason:     r.CancelReason,
	}

	for _, svc := range r.ReservationServices {
		resp.Services = append(resp.Services, response.ReservationServiceResponse{
			ServiceID:   svc.ServiceID,
			ServiceName: svc.ServiceName,
			Price:       svc.Price,
			Duration:    svc.Duration,
		})
	}

	if r.Client.ID != 0 {
		resp.Client = &response.ClientLite{
			FirstName: r.Client.FirstName,
			LastName:  r.Client.LastName,
			Phone:     r.Client.Phone,
		}
	}

	if r.Personnel.ID != 0 {
		resp.Personnel = &response.PersonnelLite{
			FirstName: r.Personnel.FirstName,
			LastName:  r.Personnel.LastName,
		}
	}

	return resp
}

// mapReservationsToResponses maps a reservation slice to DTOs, always returning
// a non-nil slice so an empty result serializes as [] instead of null.
func mapReservationsToResponses(reservations []models.Reservation) []response.ReservationResponse {
	resp := make([]response.ReservationResponse, 0, len(reservations))
	for i := range reservations {
		resp = append(resp, mapToReservationResponse(&reservations[i]))
	}
	return resp
}
