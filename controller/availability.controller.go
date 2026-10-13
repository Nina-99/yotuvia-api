package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

// AvailabilityController exposes the cross-center availability search.
type AvailabilityController struct {
	availabilityService service.AvailabilityService
}

func NewAvailabilityController(svc service.AvailabilityService) *AvailabilityController {
	return &AvailabilityController{availabilityService: svc}
}

// Search handles GET /availability/search. It only parses the raw query into
// the typed request and maps service errors through apperrors; every semantic
// rule (dates, range, period, unknown service) lives in the service.
func (c *AvailabilityController) Search(ctx *gin.Context) {
	serviceIDs, ok := parseIDList(ctx.Query("service_ids"))
	// Absent service_ids is fine when treatment_ids is supplied: the two are
	// alternative sizing strategies. Malformed service_ids is always 400.
	if !ok && ctx.Query("service_ids") != "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "service_ids must be a comma-separated list of positive integers"})
		return
	}
	treatmentIDs, ok := parseIDList(ctx.Query("treatment_ids"))
	if !ok && ctx.Query("treatment_ids") != "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "treatment_ids must be a comma-separated list of positive integers"})
		return
	}
	assignments, ok := parseAssignments(ctx.Query("assignments"))
	if !ok && ctx.Query("assignments") != "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "assignments must be a comma-separated list of service_id:personnel_id pairs"})
		return
	}
	if len(serviceIDs) == 0 && len(treatmentIDs) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "service_ids or treatment_ids query param is required (comma-separated)"})
		return
	}

	from := ctx.Query("from")
	if from == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "from query param is required (YYYY-MM-DD)"})
		return
	}
	to := ctx.Query("to")
	if to == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "to query param is required (YYYY-MM-DD)"})
		return
	}

	centerID, ok := parseOptionalUint(ctx.Query("center_id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "center_id must be a positive integer"})
		return
	}
	personnelID, ok := parseOptionalUint(ctx.Query("personnel_id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "personnel_id must be a positive integer"})
		return
	}

	limit := 0
	if raw := ctx.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "limit must be an integer"})
			return
		}
		limit = parsed
	}

	req := request.AvailabilitySearchRequest{
		ServiceIDs:   serviceIDs,
		TreatmentIDs: treatmentIDs,
		Assignments:  assignments,
		From:         from,
		To:           to,
		CenterID:     centerID,
		PersonnelID:  personnelID,
		Period:       ctx.Query("period"),
		Limit:        limit,
	}

	res, err := c.availabilityService.Search(req)
	if err != nil {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// parseIDList parses a comma-separated list of positive integers. An empty or
// malformed entry returns ok=false.
func parseIDList(raw string) ([]uint, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	parts := strings.Split(raw, ",")
	ids := make([]uint, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n <= 0 {
			return nil, false
		}
		ids = append(ids, uint(n))
	}
	return ids, true
}

// parseOptionalUint parses an optional non-negative integer. An absent/empty
// value or explicit 0 both mean "not set" and return (0, true); a negative or
// non-numeric value is (0, false).
func parseOptionalUint(raw string) (uint, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, false
	}
	return uint(n), true
}

// parseAssignments parses the OPTIONAL multi-professional chain query param:
// `serviceId:personnelId,serviceId:personnelId`. Order is preserved (it defines
// the chain). An empty value means "not set" and returns (nil, false); any
// malformed pair or a non-positive id is (nil, false) and the caller answers
// 400.
func parseAssignments(raw string) ([]request.ServiceAssignment, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	parts := strings.Split(raw, ",")
	out := make([]request.ServiceAssignment, 0, len(parts))
	for _, part := range parts {
		pair := strings.Split(strings.TrimSpace(part), ":")
		if len(pair) != 2 {
			return nil, false
		}
		serviceID, errSvc := strconv.Atoi(strings.TrimSpace(pair[0]))
		personnelID, errPers := strconv.Atoi(strings.TrimSpace(pair[1]))
		if errSvc != nil || errPers != nil || serviceID <= 0 || personnelID <= 0 {
			return nil, false
		}
		out = append(out, request.ServiceAssignment{ServiceID: uint(serviceID), PersonnelID: uint(personnelID)})
	}
	return out, true
}
