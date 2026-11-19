package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

type PaymentConfigController struct {
	paymentSvc service.PaymentConfigService
	uploadCtrl *UploadController
}

func NewPaymentConfigController(
	paymentSvc service.PaymentConfigService,
	uploadCtrl *UploadController,
) *PaymentConfigController {
	return &PaymentConfigController{
		paymentSvc: paymentSvc,
		uploadCtrl: uploadCtrl,
	}
}

// resolvePublicCenterID reads the REQUIRED `beauty_center_id` query param used
// by the public payment reads. A missing or malformed value (including 0 and
// negatives) is a 400: the client paying must name the centre whose QR it wants,
// and the endpoint must never silently fall back to another centre's QR.
func resolvePublicCenterID(ctx *gin.Context) (uint, bool) {
	raw := ctx.Query("beauty_center_id")
	if raw == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "beauty_center_id is required"})
		return 0, false
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "beauty_center_id is invalid"})
		return 0, false
	}
	return uint(id), true
}

// callerCenterID reads the admin's own centre from the token claim. A
// beauty_center token without a centreID claim (0) cannot own a config row, so
// it is rejected with 403 — the same guard loyalty/promotion create use. The
// body/params are never consulted for the owner.
func callerCenterID(ctx *gin.Context) (uint, bool) {
	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return 0, false
	}
	return centerID, true
}

// GetPaymentConfig returns the PUBLIC payment configuration of one centre
// (QR URL, phone, tiers).
//
// GET /config/payment?beauty_center_id=<id>
func (c *PaymentConfigController) GetPaymentConfig(ctx *gin.Context) {
	centerID, ok := resolvePublicCenterID(ctx)
	if !ok {
		return
	}

	config, err := c.paymentSvc.Get(centerID)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	tiers, err := c.paymentSvc.GetTiers(centerID)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"beauty_center_id": centerID,
		"payment_qr_url":   config.PaymentQRURL,
		"payment_phone":    config.PaymentPhone,
		"tiers":            tiers,
	})
}

type updatePaymentConfigRequest struct {
	PaymentPhone string                    `json:"payment_phone"`
	Tiers        models.PaymentConfigTiers `json:"tiers"`
}

// UpdatePaymentConfig updates the CALLER'S OWN centre payment configuration
// (admin only). The owner comes from the token claim, never the body.
//
// PUT /admin/config/payment
func (c *PaymentConfigController) UpdatePaymentConfig(ctx *gin.Context) {
	centerID, ok := callerCenterID(ctx)
	if !ok {
		return
	}

	var req updatePaymentConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeBadRequest(ctx, err)
		return
	}

	if err := c.paymentSvc.Update(centerID, "", req.PaymentPhone, req.Tiers); err != nil {
		writeInternalError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "payment config updated"})
}

// UploadPaymentQR handles the QR image upload (multipart) for the CALLER'S OWN
// centre. The owner comes from the token claim, never the body.
//
// POST /admin/config/payment/qr
func (c *PaymentConfigController) UploadPaymentQR(ctx *gin.Context) {
	centerID, ok := callerCenterID(ctx)
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
		writeBadRequest(ctx, err)
		return
	}

	// Persist the URL against the caller's centre.
	if err := c.paymentSvc.Update(centerID, url, "", nil); err != nil {
		writeInternalError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"url": url})
}

// GetTierConfig returns the requested centre's tier ladder for a currency
// (public).
//
// GET /config/tiers?beauty_center_id=<id>&currency=USD
func (c *PaymentConfigController) GetTierConfig(ctx *gin.Context) {
	centerID, ok := resolvePublicCenterID(ctx)
	if !ok {
		return
	}

	currency := ctx.DefaultQuery("currency", "USD")

	tiers, err := c.paymentSvc.GetTiers(centerID)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	currencyTiers, ok := tiers[currency]
	if !ok {
		// Fallback to first available currency
		for _, v := range tiers {
			currencyTiers = v
			break
		}
	}

	// Convert to the format expected by the frontend
	type tierResp struct {
		Min      float64 `json:"min"`
		BonusPct float64 `json:"bonusPct"`
	}
	var resp []tierResp
	for _, t := range currencyTiers {
		resp = append(resp, tierResp{Min: t.Min, BonusPct: t.BonusPct})
	}

	ctx.JSON(http.StatusOK, gin.H{"beauty_center_id": centerID, "tiers": resp})
}
