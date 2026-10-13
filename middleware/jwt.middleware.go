package middleware

import (
	"net/http"
	"strings"

	"github.com/Nina-99/yotuvia-api/utils"
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
		ctx.Set("clientID", claims.UserID)
		ctx.Set("role", claims.Role)
		if claims.CenterID != 0 {
			ctx.Set("centerID", claims.CenterID)
		}
		ctx.Next()
	}
}

// OptionalJWTAuthMiddleware extracts clientID from the JWT if present and valid,
// but does NOT abort when the token is missing or invalid. This allows public
// endpoints (e.g. the video feed) to personalize responses for authenticated
// users while still serving anonymous visitors.
func OptionalJWTAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			ctx.Next()
			return
		}
		token := strings.Split(authHeader, "Bearer ")
		if len(token) < 2 {
			ctx.Next()
			return
		}
		claims, err := utils.ValidateToken(token[1])
		if err != nil {
			ctx.Next()
			return
		}
		ctx.Set("clientID", claims.UserID)
		ctx.Set("role", claims.Role)
		if claims.CenterID != 0 {
			ctx.Set("centerID", claims.CenterID)
		}
		ctx.Next()
	}
}
