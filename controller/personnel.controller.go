package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type PersonnelController struct {
	personnelService service.PersonnelService
	followService    service.PersonnelFollowService
}

func NewPersonnelController(personnelService service.PersonnelService, followService service.PersonnelFollowService) *PersonnelController {
	return &PersonnelController{personnelService: personnelService, followService: followService}
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

	// is_following is derived from the caller's JWT claim (never the body).
	// The route runs JWTAuthMiddleware, so a claim is present; an absent/zero
	// claim or a lookup failure simply leaves the field false.
	if c.followService != nil {
		if clientID := ctx.GetUint("clientID"); clientID != 0 {
			if following, ferr := c.followService.IsFollowing(clientID, uint(id)); ferr == nil {
				res.IsFollowing = following
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

// Follow handles POST /personnel/:id/follow (client-only). The actor is the
// JWT clientID claim — never the body. Idempotent: a repeat stays true.
func (c *PersonnelController) Follow(ctx *gin.Context) {
	id, ok := c.parsePersonnelID(ctx)
	if !ok {
		return
	}
	if err := c.followService.Follow(ctx.GetUint("clientID"), id); err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"following": true})
}

// Unfollow handles DELETE /personnel/:id/follow (client-only). Idempotent:
// removing an edge that does not exist still answers 200 with false.
func (c *PersonnelController) Unfollow(ctx *gin.Context) {
	id, ok := c.parsePersonnelID(ctx)
	if !ok {
		return
	}
	if err := c.followService.Unfollow(ctx.GetUint("clientID"), id); err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"following": false})
}

// parsePersonnelID reads and validates the :id path param, writing a 400 on a
// malformed value. ok is false when the response has already been written.
func (c *PersonnelController) parsePersonnelID(ctx *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return 0, false
	}
	return uint(id), true
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

	// service_ids is the optional picker capability filter: a comma-separated
	// list of service ids. Omitted → unchanged behaviour. A malformed element
	// (non-numeric or <= 0) is invalid input (400); an id that is well-formed
	// but unknown is rejected by the service layer with the same 400.
	serviceIDs, ok := parseServiceIDs(ctx.Query("service_ids"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of service_ids invalid"})
		return
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	res, total, err := c.personnelService.FindAll(beautyCenterID, serviceIDs, page, limit)
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

// parseServiceIDs parses a comma-separated service_ids query value. An empty
// value yields a nil slice and ok=true (filter omitted). Each element must be a
// positive integer; a bad element yields ok=false so the controller answers 400.
func parseServiceIDs(raw string) ([]uint, bool) {
	if raw == "" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	ids := make([]uint, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, false
		}
		id, err := strconv.Atoi(trimmed)
		if err != nil || id <= 0 {
			return nil, false
		}
		ids = append(ids, uint(id))
	}
	return ids, true
}
