package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type FeedPostController struct {
	feedPostService service.FeedPostService
}

func NewFeedPostController(service service.FeedPostService) *FeedPostController {
	return &FeedPostController{feedPostService: service}
}

// feedActor derives the write identity exclusively from JWT claims
// (feat-video-feed-unification D2): the client never declares authorship.
func feedActor(ctx *gin.Context) service.FeedActor {
	return service.FeedActor{
		Role:     ctx.GetString("role"),
		UserID:   ctx.GetUint("clientID"),
		CenterID: ctx.GetUint("centerID"),
	}
}

// Create handles POST /feed-posts (multipart/form-data):
// caption, service_id, personnel_id (centers only), is_promo, video file.
func (c *FeedPostController) Create(ctx *gin.Context) {
	caption := ctx.PostForm("caption")
	serviceID, _ := strconv.Atoi(ctx.PostForm("service_id"))
	personnelID, _ := strconv.Atoi(ctx.PostForm("personnel_id"))
	isPromo := ctx.PostForm("is_promo") == "true"

	file, err := ctx.FormFile("video")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "video file is required"})
		return
	}

	req := request.CreateFeedPostRequest{
		Caption:     caption,
		ServiceID:   uint(serviceID),
		PersonnelID: uint(personnelID),
		IsPromo:     isPromo,
	}

	res, err := c.feedPostService.CreateFromUpload(feedActor(ctx), req, file)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, res)
}

// Update allows the author to edit caption/promo of their own post.
func (c *FeedPostController) Update(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req request.UpdateFeedPostRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := c.feedPostService.Update(feedActor(ctx), uint(id), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// Delete allows ONLY the author to remove their post.
func (c *FeedPostController) Delete(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}
	if err := c.feedPostService.Delete(feedActor(ctx), uint(id)); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "post deleted"})
}

// Suspend handles PUT /feed-posts/:id/suspend — owning center toggles a
// staff-authored post of their own center. Body {note} suspends with note;
// empty body re-enables.
func (c *FeedPostController) Suspend(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	centerID := ctx.GetUint("centerID")
	var req struct {
		Note string `json:"note"`
	}
	_ = ctx.ShouldBindJSON(&req) // empty body = re-enable

	suspend := true
	if req.Note == "" {
		suspend = false
	}
	if err := c.feedPostService.SetSuspended(centerID, uint(id), suspend, req.Note); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	state := "suspended"
	if !suspend {
		state = "re-enabled"
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "post " + state})
}

func (c *FeedPostController) FindByID(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}
	ctx.JSON(http.StatusNotImplemented, gin.H{"error": "deprecated — use the feed list"})
}

func (c *FeedPostController) FindAll(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))

	// Optionally extract clientID if JWT is present (feed is public, auth is optional)
	var clientID *uint
	if id, exists := ctx.Get("clientID"); exists {
		if uid, ok := id.(uint); ok {
			clientID = &uid
		}
	}

	res, total, err := c.feedPostService.FindAll(page, limit, clientID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"posts": res, "total": total, "page": page, "limit": limit})
}

// FindMine serves management lists for the authenticated author.
func (c *FeedPostController) FindMine(ctx *gin.Context) {
	res, err := c.feedPostService.FindMine(feedActor(ctx))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"posts": res})
}

func (c *FeedPostController) FindByBeautyCenterID(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}
	res, err := c.feedPostService.FindByBeautyCenterID(uint(id))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// IncrementViews handles POST /feed-posts/:id/views (public, best-effort).
func (c *FeedPostController) IncrementViews(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}
	if err := c.feedPostService.IncrementViews(uint(id)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "view counted"})
}
