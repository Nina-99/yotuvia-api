package controller

import (
	"net/http"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/service"
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

// GetPaymentConfig returns the public payment configuration (QR URL, phone, tiers).
// GET /config/payment
func (c *PaymentConfigController) GetPaymentConfig(ctx *gin.Context) {
	config, err := c.paymentSvc.Get()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tiers, err := c.paymentSvc.GetTiers()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"payment_qr_url": config.PaymentQRURL,
		"payment_phone":  config.PaymentPhone,
		"tiers":          tiers,
	})
}

type updatePaymentConfigRequest struct {
	PaymentPhone string                       `json:"payment_phone"`
	Tiers        models.PaymentConfigTiers    `json:"tiers"`
}

// UpdatePaymentConfig updates the payment configuration (admin only).
// PUT /admin/config/payment
func (c *PaymentConfigController) UpdatePaymentConfig(ctx *gin.Context) {
	var req updatePaymentConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.paymentSvc.Update("", req.PaymentPhone, req.Tiers); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "payment config updated"})
}

// UploadPaymentQR handles the QR image upload (multipart).
// POST /admin/config/payment/qr
func (c *PaymentConfigController) UploadPaymentQR(ctx *gin.Context) {
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

	// Persist the URL in the config
	if err := c.paymentSvc.Update(url, "", nil); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"url": url})
}

// GetTierConfig returns tier config for a specific currency (public).
// This replaces the old file-based tier endpoint.
// GET /config/tiers?currency=USD
func (c *PaymentConfigController) GetTierConfig(ctx *gin.Context) {
	currency := ctx.DefaultQuery("currency", "USD")

	tiers, err := c.paymentSvc.GetTiers()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	ctx.JSON(http.StatusOK, gin.H{"tiers": resp})
}
