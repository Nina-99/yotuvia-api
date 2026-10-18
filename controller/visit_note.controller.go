package controller

import (
	"net/http"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

// VisitNoteController exposes the internal per-visit notes a professional keeps
// about their own appointments (see service.VisitNoteService).
type VisitNoteController struct {
	visitNoteService service.VisitNoteService
}

func NewVisitNoteController(svc service.VisitNoteService) *VisitNoteController {
	return &VisitNoteController{visitNoteService: svc}
}

func (c *VisitNoteController) writeError(ctx *gin.Context, err error) {
	status, msg := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": msg})
}

// Upsert handles PUT /reservation/visit-notes/:id. The author and centre come
// from the JWT; the body carries only the text.
func (c *VisitNoteController) Upsert(ctx *gin.Context) {
	id, ok := parsePositiveID(ctx.Param("id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}
	var req request.UpsertVisitNoteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	note, err := c.visitNoteService.Upsert(
		id,
		ctx.GetString("role"),
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
		req.Body,
	)
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, note)
}

// Delete handles DELETE /reservation/visit-notes/:id.
func (c *VisitNoteController) Delete(ctx *gin.Context) {
	id, ok := parsePositiveID(ctx.Param("id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}
	if err := c.visitNoteService.Delete(
		id,
		ctx.GetString("role"),
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
	); err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Visit note deleted"})
}

// List handles GET /reservation/visit-notes?from=&to=.
func (c *VisitNoteController) List(ctx *gin.Context) {
	notes, err := c.visitNoteService.List(
		ctx.GetString("role"),
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
		ctx.Query("from"),
		ctx.Query("to"),
	)
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, notes)
}
