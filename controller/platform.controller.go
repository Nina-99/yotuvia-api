package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

// platformMaxPageSize caps the platform queue page size, mirroring the
// centre-scoped top-up list.
const platformMaxPageSize = 100

// PlatformController serves the /admin/platform surface: the platform-level
// principal's oversight of every beauty centre. It is intentionally a thin
// composition over the SAME services the centre-scoped routes use —
// TokenAccountService for top-up decisions and PaymentConfigService +
// UploadController for payment configuration + QR upload — so there is exactly
// one implementation of each business rule and the platform view cannot drift
// from the centre view.
type PlatformController struct {
	topUpSvc   service.TokenAccountService
	topUpRepo  repository.TokenTopUpRepository
	centerRepo repository.BeautyCenterRepository
	paymentSvc service.PaymentConfigService
	uploadCtrl *UploadController
}

func NewPlatformController(
	topUpSvc service.TokenAccountService,
	topUpRepo repository.TokenTopUpRepository,
	centerRepo repository.BeautyCenterRepository,
	paymentSvc service.PaymentConfigService,
	uploadCtrl *UploadController,
) *PlatformController {
	return &PlatformController{
		topUpSvc:   topUpSvc,
		topUpRepo:  topUpRepo,
		centerRepo: centerRepo,
		paymentSvc: paymentSvc,
		uploadCtrl: uploadCtrl,
	}
}

// platformPagination reads `page`/`limit`, defaulting to 20 and capping at 100.
func platformPagination(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > platformMaxPageSize {
		limit = platformMaxPageSize
	}
	return page, limit
}

// platformPathCenterID parses the `:centerId` path parameter. A missing, zero or
// malformed value is a client error: the platform admin must always name the
// centre it acts on, and the endpoint must never silently pick one.
func platformPathCenterID(ctx *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(ctx.Param("centerId"), 10, 64)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "beauty center id is invalid"})
		return 0, false
	}
	return uint(id), true
}

// resolveExistingCenter parses `:centerId` and proves the centre exists, so a
// platform read/update can never create a config row for a phantom centre.
func (c *PlatformController) resolveExistingCenter(ctx *gin.Context) (uint, bool) {
	centerID, ok := platformPathCenterID(ctx)
	if !ok {
		return 0, false
	}
	center, err := c.centerRepo.FindByID(centerID)
	if err != nil || center == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "beauty center not found"})
		return 0, false
	}
	return centerID, true
}

// ListTopUps is the GLOBAL queue: every centre's top-ups in one paginated list.
//
// GET /admin/platform/tokens/topups?page=&limit=&beauty_center_id=&status=
//
// Both filters are honoured by the SQL query itself (see
// TokenTopUpRepository.FindAllWithFilters), so `total` and the page contents
// always agree with the filter — nothing is filtered in memory after the query.
func (c *PlatformController) ListTopUps(ctx *gin.Context) {
	page, limit := platformPagination(ctx)

	var centerFilter *uint
	if raw := ctx.Query("beauty_center_id"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "beauty_center_id is invalid"})
			return
		}
		value := uint(id)
		centerFilter = &value
	}
	status := ctx.Query("status")

	topups, total, err := c.topUpRepo.FindAllWithFilters(page, limit, centerFilter, status)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := make([]response.TokenTopUpResponse, 0, len(topups))
	for _, topup := range topups {
		resp = append(resp, response.NewTokenTopUpResponse(topup))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"topups": resp,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

// ProcessTopUp approves or rejects ANY centre's top-up.
//
// PUT /admin/platform/tokens/topups/:id  body: {"status":"approved|rejected"}
//
// It reuses the existing top-up contract verbatim — same request body, same
// "top-up already processed" / "not found" answers, same credit + notification
// side effects — and only lifts the ownership check that scopes the
// beauty_center route (TokenAccountService.ProcessTopUpAsPlatform). The
// platform admin's own id is recorded as the validator.
func (c *PlatformController) ProcessTopUp(ctx *gin.Context) {
	adminID := ctx.GetUint("clientID")

	topUpID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || topUpID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid top-up ID"})
		return
	}

	var req request.ApproveTopUpRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.topUpSvc.ProcessTopUpAsPlatform(uint(topUpID), adminID, req.Status); err != nil {
		if errors.Is(err, apperrors.ErrForbidden) {
			ctx.JSON(http.StatusForbidden, gin.H{"error": apperrors.ErrForbidden.Error()})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "top-up " + req.Status})
}

// ListCenters returns id + name for every centre, so the platform admin can pick
// the one whose payment config it wants to inspect or manage.
//
// GET /admin/platform/centers
func (c *PlatformController) ListCenters(ctx *gin.Context) {
	centers, err := c.centerRepo.FindAllSummaries()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type centerSummary struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	resp := make([]centerSummary, 0, len(centers))
	for _, center := range centers {
		resp = append(resp, centerSummary{ID: center.ID, Name: center.Name})
	}

	ctx.JSON(http.StatusOK, gin.H{"centers": resp})
}

// GetCenterPayment reads the payment configuration of the CHOSEN centre.
//
// GET /admin/platform/centers/:centerId/payment
//
// The centre comes from the path, never from a claim (a super_admin has none),
// and the response mirrors the public read so a client can render it the same
// way: beauty_center_id, payment_qr_url, payment_phone, tiers.
func (c *PlatformController) GetCenterPayment(ctx *gin.Context) {
	centerID, ok := c.resolveExistingCenter(ctx)
	if !ok {
		return
	}

	config, err := c.paymentSvc.Get(centerID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tiers, err := c.paymentSvc.GetTiers(centerID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"beauty_center_id": centerID,
		"payment_qr_url":   config.PaymentQRURL,
		"payment_phone":    config.PaymentPhone,
		"tiers":            tiers,
	})
}

// UpdateCenterPayment updates the payment configuration (phone + tiers) of the
// CHOSEN centre. The body shares the centre-scoped DTO — the owner comes from
// the path instead of the centreID claim, which is the ONLY difference from
// PUT /admin/config/payment.
//
// PUT /admin/platform/centers/:centerId/payment
func (c *PlatformController) UpdateCenterPayment(ctx *gin.Context) {
	centerID, ok := c.resolveExistingCenter(ctx)
	if !ok {
		return
	}

	var req updatePaymentConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.paymentSvc.Update(centerID, "", req.PaymentPhone, req.Tiers); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "payment config updated"})
}

// UploadCenterPaymentQR uploads and persists the payment QR of the CHOSEN centre.
// It reuses the very same upload path as the centre-scoped handler
// (UploadController.UploadReceiptFromFile → PaymentConfigService.Update), with
// the centre coming from the path.
//
// POST /admin/platform/centers/:centerId/payment/qr
func (c *PlatformController) UploadCenterPaymentQR(ctx *gin.Context) {
	centerID, ok := c.resolveExistingCenter(ctx)
	if !ok {
		return
	}

	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	url, err := c.uploadCtrl.UploadReceiptFromFile(file)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.paymentSvc.Update(centerID, url, "", nil); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"url": url})
}
