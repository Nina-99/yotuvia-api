// Package apperrors defines sentinel errors for the API's business rules and
// a single mapping from errors to HTTP responses. Controllers route every
// error through MapToHTTP so clients get a stable, leak-free contract.
package apperrors

import (
	"errors"
	"net/http"

	"github.com/Nina-99/yotuvia-api/utils"
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
	// ErrEmailTaken signals a registration attempt with an email that is
	// already registered. Kept distinct from ErrUsernameTaken (and from the
	// generic ErrConflict) so a client can render a targeted message.
	ErrEmailTaken = errors.New("email already exists")
	// ErrUsernameTaken signals a registration attempt with a username that is
	// already registered.
	ErrUsernameTaken = errors.New("username already exists")
	// ErrInvalidToken signals an email-verification or password-reset token that
	// is unknown, expired or already used. The three cases collapse into one
	// client-visible message so a caller cannot probe token state.
	ErrInvalidToken = errors.New("invalid or expired token")
	// ErrTooManyRequests signals a per-account throttle (e.g. login-code
	// requests) exceeded. The middleware limiter still covers per-IP floods.
	ErrTooManyRequests = errors.New("too many requests")
	// ErrCenterInactive signals a booking against a centre that is not active.
	// The message is deliberately neutral: the caller may be a client.
	ErrCenterInactive = errors.New("center is not active")
	// ErrCenterUnavailable signals a centre that cannot fund the commission of a
	// new visit (no balance and no overdraft headroom left). Neutral on purpose:
	// the client must not learn the centre's financial state, and the centre's
	// own wrap replaces it with an actionable message.
	ErrCenterUnavailable = errors.New("this centre is not accepting reservations right now")
	// ErrCenterSuspended signals a centre suspended for staying in deficit past
	// the platform's window. Same neutral client copy as unavailable, but the
	// owner gets a distinct, actionable message.
	ErrCenterSuspended = errors.New("this centre is temporarily unavailable")
)

// UserFacingError is a business-rule failure whose message is DELIBERATELY shown
// to the client ("you must complete a booking before leaving a review"). It is
// the explicit opt-in that lets a service keep human-readable copy without
// forwarding arbitrary internal text: anything NOT wrapped in this type reaches
// the client as a generic 500.
type UserFacingError struct {
	Message string
}

func (e *UserFacingError) Error() string { return e.Message }

// NewUserFacing builds a UserFacingError. Use it only for text written for the
// end user, never for a wrapped driver/repository error.
func NewUserFacing(message string) error { return &UserFacingError{Message: message} }

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
	case errors.Is(err, ErrEmailTaken):
		return http.StatusConflict, ErrEmailTaken.Error()
	case errors.Is(err, ErrUsernameTaken):
		return http.StatusConflict, ErrUsernameTaken.Error()
	case errors.Is(err, ErrInvalidToken):
		return http.StatusBadRequest, ErrInvalidToken.Error()
	case errors.Is(err, ErrTooManyRequests):
		return http.StatusTooManyRequests, ErrTooManyRequests.Error()
	case errors.Is(err, ErrCenterInactive), errors.Is(err, ErrCenterUnavailable), errors.Is(err, ErrCenterSuspended):
		return http.StatusConflict, err.Error()
	case errors.Is(err, ErrFileTooLarge):
		return http.StatusRequestEntityTooLarge, ErrFileTooLarge.Error()
	default:
		// A deliberate, user-safe business message. Checked before the generic
		// 500 so services can keep copy their UI displays ("you must complete a
		// booking first") without leaking anything else.
		var userFacing *UserFacingError
		if errors.As(err, &userFacing) {
			return http.StatusConflict, userFacing.Message
		}
		utils.Log.Errorf("[apperrors] internal error: %v", err)
		return http.StatusInternalServerError, "internal server error"
	}
}
