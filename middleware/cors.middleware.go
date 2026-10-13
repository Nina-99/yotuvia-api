package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS lets browser clients (the web app) call this API from a different
// origin. The Flutter/React Native clients never needed it because native
// fetch does not send a preflight, so the web login surfaced the gap: the
// browser answered its own OPTIONS preflight with the API's 404 because Gin has
// no OPTIONS route registered for /api/auth-accounts/login.
//
// Allowed origins come from CORS_ALLOWED_ORIGINS (comma-separated). When unset
// it falls back to FRONT_HOST (the web dev server), and finally to
// http://localhost:5173 so local development works with no configuration.
//
// Requests from an origin that is not allowed receive no
// Access-Control-Allow-Origin header, so the browser blocks them. The
// preflight itself is answered with 204 for every origin: a disallowed origin
// still gets a response, but without the header the browser refuses the
// follow-up request.
func CORS() gin.HandlerFunc {
	allowed := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(allowed) == 0 {
		allowed = parseAllowedOrigins(os.Getenv("FRONT_HOST"))
	}
	if len(allowed) == 0 {
		allowed = []string{"http://localhost:5173"}
	}

	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")

		// Always vary on Origin: the response headers depend on it.
		ctx.Writer.Header().Add("Vary", "Origin")

		if origin != "" && originAllowed(origin, allowed) {
			header := ctx.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
			header.Set("Access-Control-Max-Age", "600")
		}

		if ctx.Request.Method == http.MethodOptions {
			// Preflight: answer here, before Gin can fall through to 404.
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}

func parseAllowedOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func originAllowed(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		if candidate == "*" || strings.EqualFold(candidate, origin) {
			return true
		}
	}
	return false
}
