package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole gates a route to authenticated actors whose role claim is in the
// allowed set. JWTAuthMiddleware MUST run first: it validates identity and
// stores the role claim in ctx. A missing or non-allowed role aborts with 403
// before the handler runs. Roles: "client", "personnel", "beauty_center".
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(ctx *gin.Context) {
		role, exists := ctx.Get("role")
		if !exists {
			ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			ctx.Abort()
			return
		}
		roleStr, ok := role.(string)
		if !ok {
			ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			ctx.Abort()
			return
		}
		if _, ok := allowed[roleStr]; !ok {
			ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}
