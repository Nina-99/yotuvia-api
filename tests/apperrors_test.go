package tests

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestMapToHTTP(t *testing.T) {
	// MapToHTTP logs unknown errors via utils.Log; the global is nil until
	// InitLogger runs, so point it at a NOP logger to keep test output clean.
	utils.Log = zap.NewNop().Sugar()

	table := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"bad request", apperrors.ErrBadRequest, 400, "bad request"},
		{"validation", apperrors.ErrValidation, 400, "validation failed"},
		{"unauthorized", apperrors.ErrUnauthorized, 401, "unauthorized"},
		{"forbidden", apperrors.ErrForbidden, 403, "forbidden"},
		{"not found", apperrors.ErrNotFound, 404, "not found"},
		{"conflict", apperrors.ErrConflict, 409, "conflict"},
		{"insufficient balance", apperrors.ErrInsufficientBalance, 409, "insufficient balance"},
		{
			"wrapped sentinel resolves to sentinel status",
			fmt.Errorf("reservation 42: %w", apperrors.ErrNotFound),
			404, "not found",
		},
		{
			"unknown error maps to generic 500",
			errors.New("dial tcp 10.0.0.5:5432: connection refused"),
			500, "internal server error",
		},
		{
			"wrapped DB error maps to generic 500 with no SQL leak",
			fmt.Errorf("failed query: %w", errors.New("near \"::numeric\": syntax error")),
			500, "internal server error",
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			status, msg := apperrors.MapToHTTP(tt.err)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantBody, msg)
			if tt.wantStatus == 500 {
				// The client-facing message must never leak the underlying error text.
				assert.NotContains(t, msg, tt.err.Error())
			}
		})
	}
}

// 403 vs 404 must be distinct: ownership failures are Forbidden, missing
// resources are NotFound — clients must be able to tell them apart.
func TestMapToHTTP_ForbiddenVsNotFoundDistinct(t *testing.T) {
	statusF, _ := apperrors.MapToHTTP(apperrors.ErrForbidden)
	statusN, _ := apperrors.MapToHTTP(apperrors.ErrNotFound)
	assert.Equal(t, 403, statusF)
	assert.Equal(t, 404, statusN)
	assert.NotEqual(t, statusF, statusN)
}

// MapToHTTP must return nil-safe behavior for typed wrapped errors too.
func TestMapToHTTP_WrappedValidation(t *testing.T) {
	status, msg := apperrors.MapToHTTP(fmt.Errorf("invalid input: %w", apperrors.ErrValidation))
	assert.Equal(t, 400, status)
	assert.Equal(t, "validation failed", msg)
}
