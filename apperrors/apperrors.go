// Package apperrors defines sentinel errors for the API's business rules and
// a single mapping from errors to HTTP responses. Controllers route every
// error through MapToHTTP so clients get a stable, leak-free contract.
package apperrors

import (
	"errors"
	"net/http"

	"github.com/Nina-99/yottu-api/utils"
)

var (
	// ErrBadRequest signals malformed or semantically invalid input.
	ErrBadRequest = errors.New("bad request")
	// ErrUnauthorized signals a missing/invalid identity. Reserved for the
	// middleware layer and future refresh-token flows; service paths return
	// ErrForbidden/ErrNotFound instead.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden signals an authenticated actor without permission (ownership mismatch).
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound signals a missing resource.
	ErrNotFound = errors.New("not found")
	// ErrConflict signals a state conflict (e.g. completing an already-completed reservation).
	ErrConflict = errors.New("conflict")
	// ErrValidation signals request validation failure.
	ErrValidation = errors.New("validation failed")
	// ErrFileTooLarge signals an upload exceeding the size cap (413).
	ErrFileTooLarge = errors.New("file too large")
	// ErrInsufficientBalance signals the client cannot afford the operation.
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// MapToHTTP returns the HTTP status code and a client-safe message for err.
// Known sentinels (and errors wrapping them) map to their canonical status and
// message; any other error is logged server-side and mapped to 500 with a
// generic message, so internal details never leak to clients.
func MapToHTTP(err error) (int, string) {
	switch {
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest, ErrBadRequest.Error()
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest, ErrValidation.Error()
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized, ErrUnauthorized.Error()
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden, ErrForbidden.Error()
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound, ErrNotFound.Error()
	case errors.Is(err, ErrConflict):
		return http.StatusConflict, ErrConflict.Error()
	case errors.Is(err, ErrInsufficientBalance):
		return http.StatusConflict, ErrInsufficientBalance.Error()
	case errors.Is(err, ErrFileTooLarge):
		return http.StatusRequestEntityTooLarge, ErrFileTooLarge.Error()
	default:
		utils.Log.Errorf("[apperrors] internal error: %v", err)
		return http.StatusInternalServerError, "internal server error"
	}
}
