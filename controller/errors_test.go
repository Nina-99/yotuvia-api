package controller

// The 400 path must never echo engine text. `json: cannot unmarshal ... into Go
// struct field ServiceRequest.Price of type float64` is a free schema map for an
// attacker; the response has to be generic while the detail goes to the log.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func badRequestRouter(err error) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/probe", func(ctx *gin.Context) {
		writeBadRequest(ctx, err)
	})
	return r
}

func probeBody(t *testing.T, err error) string {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/probe", nil)
	badRequestRouter(err).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	return w.Body.String()
}

func TestWriteBadRequest_HidesEngineDetail(t *testing.T) {
	// The exact shape encoding/json produces for a type mismatch.
	raw := errors.New("json: cannot unmarshal string into Go struct field CreateServiceRequest.price of type float64")

	body := probeBody(t, raw)

	assert.Contains(t, body, "invalid request body")
	assert.NotContains(t, body, "json:")
	assert.NotContains(t, body, "Go struct field")
	assert.NotContains(t, body, "float64")

	var parsed map[string]string
	assert.NoError(t, json.Unmarshal([]byte(body), &parsed))
}

func TestWriteBadRequest_KeepsValidationSentinel(t *testing.T) {
	err := fmt.Errorf("%w: Key: 'Rating' Error:Field validation for 'Rating' failed on the 'max' tag", apperrors.ErrValidation)

	body := probeBody(t, err)

	assert.Contains(t, body, apperrors.ErrValidation.Error())
	assert.NotContains(t, body, "Key: 'Rating'", "validator internals must not reach the client")
}

func TestWriteBadRequest_KeepsUserFacingMessage(t *testing.T) {
	body := probeBody(t, apperrors.NewUserFacing("ya dejaste una reseña para este centro"))
	assert.Contains(t, body, "ya dejaste una reseña para este centro")
}

func TestWriteBadRequest_NeverLeaksAnArbitraryError(t *testing.T) {
	secret := "pq: duplicate key value violates unique constraint \"uniq_clients_email\""
	body := probeBody(t, errors.New(secret))

	assert.False(t, strings.Contains(body, "duplicate key"), body)
	assert.Contains(t, body, "invalid request body")
}
