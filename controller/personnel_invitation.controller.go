package controller

import (
	"net/http"
	"os"

	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type PersonnelInvitationController struct {
	invitationService service.PersonnelInvitationService
}

func NewPersonnelInvitationController(svc service.PersonnelInvitationService) *PersonnelInvitationController {
	return &PersonnelInvitationController{invitationService: svc}
}

// ──────────────────────────────────────────────
//  GENERATE — Centro crea una invitación
// ──────────────────────────────────────────────

func (c *PersonnelInvitationController) Generate(ctx *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Valid email is required"})
		return
	}

	beautyCenterID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// INVITE_BASE_URL env: set to "https://yottu.app/join/" in production
	token, err := c.invitationService.Generate(beautyCenterID.(uint), req.Email)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"token": token,
		"link":  getInviteBaseURL() + token,
	})
}

// ──────────────────────────────────────────────
//  GET — Obtiene info de invitación por token
// ──────────────────────────────────────────────

func (c *PersonnelInvitationController) GetByToken(ctx *gin.Context) {
	token := ctx.Param("token")
	if token == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	invitation, err := c.invitationService.GetByToken(token)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, invitation)
}

// ──────────────────────────────────────────────
//  ACCEPT — El profesional acepta la invitación
// ──────────────────────────────────────────────

func (c *PersonnelInvitationController) Accept(ctx *gin.Context) {
	var req struct {
		Token     string `json:"token" binding:"required"`
		FirstName string `json:"first_name" binding:"required"`
		LastName  string `json:"last_name" binding:"required"`
		Phone     string `json:"phone"`
		Password  string `json:"password" binding:"required,min=8"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.invitationService.Accept(req.Token, req.FirstName, req.LastName, req.Phone, req.Password); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Invitation accepted successfully"})
}

// ──────────────────────────────────────────────
//  LIST — Lista invitaciones del centro autenticado
// ──────────────────────────────────────────────

func (c *PersonnelInvitationController) ListByCenter(ctx *gin.Context) {
	beautyCenterID, exists := ctx.Get("centerID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	invitations, err := c.invitationService.ListByCenter(beautyCenterID.(uint))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, invitations)
}

// Returns the base URL for invitation links.
// Defaults to deep link scheme; set INVITE_BASE_URL env for universal links.
func getInviteBaseURL() string {
	url := os.Getenv("INVITE_BASE_URL")
	if url == "" {
		return "yottumobile://join/"
	}
	// Ensure trailing slash
	if url[len(url)-1] != '/' {
		url += "/"
	}
	return url
}
