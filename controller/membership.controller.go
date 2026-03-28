package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type MembershipController struct {
	MembershipService service.MembershipService
}

func NewMembershipController(membershipService service.MembershipService) *MembershipController {
	return &MembershipController{
		MembershipService: membershipService,
	}
}

// Create a new membership
func (c *MembershipController) Create(ctx *gin.Context) {
	var req request.CreateMembershipRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.MembershipService.Create(req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Membership created successfully"})
}

// Update an existing membership
func (c *MembershipController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid membership ID"})
		return
	}

	var req request.UpdateMembershipRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.MembershipService.Update(uint(id), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Membership updated successfully"})
}

// Delete a membership
func (c *MembershipController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid membership ID"})
		return
	}

	if err := c.MembershipService.Delete(uint(id)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Membership deleted successfully"})
}

// FindByID retrieves a membership by ID
func (c *MembershipController) FindByID(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid membership ID"})
		return
	}

	membership, err := c.MembershipService.FindByID(uint(id))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Membership not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": membership})
}

// FindAll retrieves all memberships
func (c *MembershipController) FindAll(ctx *gin.Context) {
	memberships, err := c.MembershipService.FindAll()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": memberships})
}
