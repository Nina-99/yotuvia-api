package middleware

import (
	"net/http"

	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

func RequireActiveMembership(membershipService service.CenterMembershipService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		role, exists := ctx.Get("role")
		if !exists {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized access, center ID missing"})
			ctx.Abort()
			return
		}

		if role == "client" {
			ctx.Next()
			return
		}

		id, exists := ctx.Get("centerID")
		if !exists || id.(uint) == 0 {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "center ID missing in token"})
			ctx.Abort()
			return
		}
		centerID := id.(uint)

		if !membershipService.HasActiveAccess(centerID) {
			ctx.JSON(http.StatusPaymentRequired, gin.H{"error": "the beauty center requires an active membership to perform this action"})
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}
