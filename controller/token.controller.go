package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
)

type TokenController struct {
	accountSvc    service.TokenAccountService
	txSvc         service.TokenTransactionService
	missionSvc    service.TokenMissionService
	referralSvc   service.ReferralService
	topUpRepo     repository.TokenTopUpRepository
	centerRepo    repository.BeautyCenterRepository
	uploadCtrl    *UploadController
}

func NewTokenController(
	accountSvc service.TokenAccountService,
	txSvc service.TokenTransactionService,
	missionSvc service.TokenMissionService,
	referralSvc service.ReferralService,
	topUpRepo repository.TokenTopUpRepository,
	centerRepo repository.BeautyCenterRepository,
	uploadCtrl *UploadController,
) *TokenController {
	return &TokenController{
		accountSvc:  accountSvc,
		txSvc:       txSvc,
		missionSvc:  missionSvc,
		referralSvc: referralSvc,
		topUpRepo:   topUpRepo,
		centerRepo:  centerRepo,
		uploadCtrl:  uploadCtrl,
	}
}

// GET /api/v1/tokens/balance
func (c *TokenController) GetBalance(ctx *gin.Context) {
	centerID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "center ID not found in token"})
		return
	}

	balance, overdraft, currency, err := c.accountSvc.GetBalance(centerID.(uint))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response.NewTokenBalanceResponse(balance, overdraft, currency))
}

// GET /api/v1/tokens/history
func (c *TokenController) GetHistory(ctx *gin.Context) {
	centerID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "center ID not found in token"})
		return
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))

	txs, total, err := c.txSvc.GetHistory(centerID.(uint), page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var txResponses []response.TokenTransactionResponse
	for _, tx := range txs {
		txResponses = append(txResponses, response.NewTokenTransactionResponse(tx))
	}

	ctx.JSON(http.StatusOK, response.TokenHistoryResponse{
		Transactions: txResponses,
		Total:        total,
		Page:         page,
		Limit:        limit,
	})
}

// POST /api/v1/tokens/topup
func (c *TokenController) CreateTopUp(ctx *gin.Context) {
	centerID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "center ID not found in token"})
		return
	}

	// Ensure token account exists and get currency
	center, err := c.centerRepo.FindByID(centerID.(uint))
	if err != nil || center == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "beauty center not found"})
		return
	}

	_, err = c.accountSvc.GetOrCreate(centerID.(uint), center.Currency)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	contentType := ctx.GetHeader("Content-Type")

	// Handle multipart/form-data (file upload)
	if len(contentType) > 0 && contentType[:30] == "multipart/form-data" {
		c.createTopUpMultipart(ctx, centerID.(uint), center.Currency)
		return
	}

	// Handle application/json (existing behavior)
	var req request.CreateTopUpRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, bonus := utils.CalculateTier(req.AmountBs, center.Currency)

	topup := &models.TokenTopUp{
		BeautyCenterID: centerID.(uint),
		AmountBs:       req.AmountBs,
		TokensReceived: utils.ToMilli(tokens),
		BonusTokens:    utils.ToMilli(bonus),
		ReceiptURL:     req.ReceiptURL,
		Status:         "pending",
	}

	if err := c.topUpRepo.Create(topup); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":         "Top-up request created",
		"topup_id":        topup.ID,
		"tokens_received": utils.FromMilli(topup.TokensReceived),
		"bonus_tokens":    utils.FromMilli(topup.BonusTokens),
		"total_tokens":    utils.FromMilli(topup.TokensReceived + topup.BonusTokens),
	})
}

func (c *TokenController) createTopUpMultipart(ctx *gin.Context, centerID uint, currency string) {
	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	amountBsStr := ctx.PostForm("amount_bs")
	if amountBsStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "amount_bs is required"})
		return
	}

	amountBs := 0.0
	if _, err := fmt.Sscanf(amountBsStr, "%f", &amountBs); err != nil || amountBs < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount_bs"})
		return
	}

	// Upload receipt
	if c.uploadCtrl == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "upload service not available"})
		return
	}

	receiptURL, err := c.uploadCtrl.UploadReceiptFromFile(file)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, bonus := utils.CalculateTier(amountBs, currency)

	topup := &models.TokenTopUp{
		BeautyCenterID: centerID,
		AmountBs:       amountBs,
		TokensReceived: utils.ToMilli(tokens),
		BonusTokens:    utils.ToMilli(bonus),
		ReceiptURL:     receiptURL,
		Status:         "pending",
	}

	if err := c.topUpRepo.Create(topup); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":         "Top-up request created",
		"topup_id":        topup.ID,
		"tokens_received": utils.FromMilli(topup.TokensReceived),
		"bonus_tokens":    utils.FromMilli(topup.BonusTokens),
		"total_tokens":    utils.FromMilli(topup.TokensReceived + topup.BonusTokens),
	})
}

// GET /api/v1/tokens/missions
func (c *TokenController) GetMissions(ctx *gin.Context) {
	centerID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "center ID not found in token"})
		return
	}

	progress, err := c.missionSvc.GetProgress(centerID.(uint))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var resp []response.TokenMissionResponse
	for _, p := range progress {
		resp = append(resp, response.NewTokenMissionResponse(p))
	}

	ctx.JSON(http.StatusOK, resp)
}

// POST /api/v1/tokens/missions/:id/claim
func (c *TokenController) ClaimMission(ctx *gin.Context) {
	centerID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "center ID not found in token"})
		return
	}

	missionID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || missionID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid mission ID"})
		return
	}

	if err := c.missionSvc.ClaimReward(centerID.(uint), uint(missionID)); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "reward claimed successfully"})
}

// GET /api/v1/tokens/referral-info
func (c *TokenController) GetReferralInfo(ctx *gin.Context) {
	centerID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "center ID not found in token"})
		return
	}

	code, count, err := c.referralSvc.GetReferralInfo(centerID.(uint))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"referral_code":  code,
		"referral_count": count,
	})
}

// POST /api/v1/tokens/referrals
func (c *TokenController) CreateReferral(ctx *gin.Context) {
	centerID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "center ID not found in token"})
		return
	}

	var req request.CreateReferralByCodeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.referralSvc.CreateReferralByCode(centerID.(uint), req.Code); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "referral created"})
}

// GET /api/v1/tokens/referrals
func (c *TokenController) GetReferrals(ctx *gin.Context) {
	centerID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "center ID not found in token"})
		return
	}

	referrals, err := c.referralSvc.GetReferrals(centerID.(uint))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, referrals)
}

// Admin: GET /api/v1/admin/tokens/topups
func (c *TokenController) AdminListTopUps(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))

	topups, total, err := c.topUpRepo.FindAll(page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var resp []response.TokenTopUpResponse
	for _, t := range topups {
		resp = append(resp, response.NewTokenTopUpResponse(t))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"topups": resp,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

// Admin: PUT /api/v1/admin/tokens/topups/:id
func (c *TokenController) AdminProcessTopUp(ctx *gin.Context) {
	adminID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "admin ID not found in token"})
		return
	}

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

	if err := c.accountSvc.ProcessTopUp(uint(topUpID), adminID.(uint), req.Status); err != nil {
		// D5: only the ownership violation maps to 403 — the legacy 400 path
		// (e.g. "top-up already processed") is preserved unchanged.
		if errors.Is(err, apperrors.ErrForbidden) {
			ctx.JSON(http.StatusForbidden, gin.H{"error": apperrors.ErrForbidden.Error()})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "top-up " + req.Status})
}

// GET /api/v1/tokens/topups (Center: My TopUps)
func (c *TokenController) FindMyTopUps(ctx *gin.Context) {
	centerID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "center ID not found in token"})
		return
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if limit > 100 {
		limit = 100
	}
	status := ctx.Query("status")

	topups, total, err := c.topUpRepo.FindByCenterIDWithStatus(centerID.(uint), page, limit, status)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var resp []response.TokenTopUpResponse
	for _, t := range topups {
		resp = append(resp, response.NewTokenTopUpResponse(t))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"topups": resp,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

// GET /api/v1/config/tiers
func (c *TokenController) GetTierConfig(ctx *gin.Context) {
	currency := ctx.DefaultQuery("currency", "VES")

	tiers, err := utils.GetTierConfig(currency)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tier config"})
		return
	}

	if tiers == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "currency not supported"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"currency": currency,
		"tiers":    tiers,
	})
}
