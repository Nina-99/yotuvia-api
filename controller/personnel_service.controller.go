package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

type PersonnelServiceController struct {
	svc service.PersonnelServiceService
}

func NewPersonnelServiceController(svc service.PersonnelServiceService) *PersonnelServiceController {
	return &PersonnelServiceController{svc: svc}
}

// writeError maps a service error through apperrors.MapToHTTP so clients get the
// canonical status + stable message (403 for a foreign centre, 409 for a
// duplicate, 404 for a missing row) and internal details never leak.
func (c *PersonnelServiceController) writeError(ctx *gin.Context, err error) {
	status, msg := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": msg})
}

// Create godoc
// @Summary Assign a service to a personnel
// @Description Link a service to a staff member, with optional price/duration overrides
// @Tags personnel-services
// @Accept json
// @Produce json
// @Param body body request.CreatePersonnelServiceRequest true "Assignment details"
// @Success 201 {object} response.PersonnelServiceResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /personnel-services [post]
func (c *PersonnelServiceController) Create(ctx *gin.Context) {
	var req request.CreatePersonnelServiceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeBadRequest(ctx, err)
		return
	}

	// The acting centre is the JWT claim, never a body field: the write is
	// ownership-checked against it in the service.
	res, err := c.svc.Create(req, ctx.GetUint("centerID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

// Delete godoc
// @Summary Remove a service from a personnel
// @Description Unlink a service from a staff member
// @Tags personnel-services
// @Param id path int true "Assignment ID"
// @Success 204
// @Failure 404 {object} map[string]string
// @Router /personnel-services/{id} [delete]
func (c *PersonnelServiceController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	if err := c.svc.Delete(uint(id), ctx.GetUint("centerID")); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// List godoc
// @Summary List personnel-service assignments
// @Description Filter by personnel_id or service_id to get services per staff or staff per service
// @Tags personnel-services
// @Produce json
// @Param personnel_id query int false "Filter by personnel ID"
// @Param service_id query int false "Filter by service ID"
// @Success 200 {array} response.PersonnelServiceResponse
// @Failure 400 {object} map[string]string
// @Router /personnel-services [get]
func (c *PersonnelServiceController) List(ctx *gin.Context) {
	personnelIDParam := ctx.Query("personnel_id")
	serviceIDParam := ctx.Query("service_id")

	var res []response.PersonnelServiceResponse
	var err error

	switch {
	case personnelIDParam != "":
		id, parseErr := strconv.Atoi(personnelIDParam)
		if parseErr != nil || id <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid personnel_id"})
			return
		}
		res, err = c.svc.FindByPersonnelID(uint(id))

	case serviceIDParam != "":
		id, parseErr := strconv.Atoi(serviceIDParam)
		if parseErr != nil || id <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid service_id"})
			return
		}
		res, err = c.svc.FindByServiceID(uint(id))

	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provide either personnel_id or service_id"})
		return
	}

	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if res == nil {
		res = []response.PersonnelServiceResponse{}
	}

	ctx.JSON(http.StatusOK, res)
}
