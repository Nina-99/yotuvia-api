package controller

import (
	"errors"
	"net/http"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
)

// writeInternalError maps an arbitrary error to its canonical HTTP status and
// client-safe message through apperrors.MapToHTTP.
//
// Controllers used to answer `gin.H{"error": err.Error()}` on internal paths,
// which forwarded raw GORM/Postgres text to the client (constraint names,
// column names, file paths) and handed an attacker a free map of the schema.
// Unknown errors now become 500 + "internal server error" and stay in the
// server log; the known sentinels keep their precise status.
func writeInternalError(ctx *gin.Context, err error) {
	status, message := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": message})
}

// writeBadRequest answers 400 without echoing the raw error.
//
// Binding and validation errors used to go out verbatim
// (`json: cannot unmarshal string into Go struct field ...`), which tells an
// attacker the exact Go type, field name and tag of every DTO. Known shapes keep
// a useful message; everything else collapses to a generic one, and the detail
// stays in the server log.
func writeBadRequest(ctx *gin.Context, err error) {
	var userFacing *apperrors.UserFacingError
	if errors.As(err, &userFacing) && userFacing.Message != "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": userFacing.Message})
		return
	}
	if errors.Is(err, apperrors.ErrValidation) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": apperrors.ErrValidation.Error()})
		return
	}
	if utils.Log != nil {
		utils.Log.Warnf("[400] %s %s: %v", ctx.Request.Method, ctx.Request.URL.Path, err)
	}
	ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
}
