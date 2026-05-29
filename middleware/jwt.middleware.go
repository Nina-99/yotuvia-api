package middleware

import (
	"net/http"
	"strings"

	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
)

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			ctx.Abort()
			return
		}
		token := strings.Split(authHeader, "Bearer ")
		if len(token) < 2 {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			ctx.Abort()
			return
		}
		claims, err := utils.ValidateToken(token[1])
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			ctx.Abort()
			return
		}
		ctx.Set("clientID", claims.ID)
		ctx.Set("userRole", claims.Role)
		ctx.Next()
	}
}
