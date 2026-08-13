package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type PersonnelController struct {
	personnelService service.PersonnelService
}

func NewPersonnelController(service service.PersonnelService) *PersonnelController {
	return &PersonnelController{personnelService: service}
}

// writeError maps any service error through apperrors.MapToHTTP so clients get
// the canonical status + stable message (404 for missing personnel, etc.) and
// internal details never leak.
func (c *PersonnelController) writeError(ctx *gin.Context, err error) {
	status, msg := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": msg})
}

func (c *PersonnelController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}

	var req request.UpdatePersonnelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := c.personnelService.Update(uint(id), req)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// UpdateMe is the staff self-service profile edit. The actor is the JWT claim
// (ctx clientID) — never a path parameter — and the allow-listed DTO makes
// email/is_active/beauty_center_id structurally impossible, so a personnel
// token cannot mutate its own login identity or center affiliation.
func (c *PersonnelController) UpdateMe(ctx *gin.Context) {
	personnelID := ctx.GetUint("clientID")

	// Spec contract (api-authz): sending identity/center fields must be
	// rejected with 403 even though the DTO cannot bind them. Detect them in
	// the raw payload so the stored value stays unchanged and the caller is
	// told the attempt failed — defense in depth on top of the DTO firewall.
	var raw map[string]json.RawMessage
	if err := ctx.ShouldBindJSON(&raw); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	for _, forbidden := range []string{"email", "is_active", "beauty_center_id"} {
		if _, present := raw[forbidden]; present {
			c.writeError(ctx, apperrors.ErrForbidden)
			return
		}
	}

	// Rebind the validated payload into the allow-listed DTO.
	body, _ := json.Marshal(raw)
	var req request.UpdateMePersonnelRequest
	if err := json.Unmarshal(body, &req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	res, err := c.personnelService.UpdateMe(personnelID, req)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (c *PersonnelController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}

	if err := c.personnelService.Delete(uint(id)); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func (c *PersonnelController) FindByID(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}

	res, err := c.personnelService.FindByID(uint(id))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (c *PersonnelController) FindAll(ctx *gin.Context) {
	beautyCenterIDParam := ctx.Query("beauty_center_id")
	var beautyCenterID uint
	if beautyCenterIDParam != "" {
		id, err := strconv.Atoi(beautyCenterIDParam)
		if err != nil || id <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of beauty_center_id invalid"})
			return
		}
		beautyCenterID = uint(id)
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	res, total, err := c.personnelService.FindAll(beautyCenterID, page, limit)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, response.PaginatedResponse{
		Data:  res,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}
