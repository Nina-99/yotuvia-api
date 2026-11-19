package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
)

// sessionGate validates the token's session id against the store. It returns
// true when the caller may proceed.
//
// Three outcomes, deliberately distinct:
//   - no checker wired (tests / non-SetupRouter embeddings): proceed;
//   - session id missing or the session is gone: 401, the client must log in;
//   - the store itself failed: 503, so the client does NOT wipe credentials
//     over an infrastructure blip. Failing closed (denying access) still holds —
//     only the status differs.
func sessionGate(ctx *gin.Context, sessionID string) bool {
	if sessionChecker == nil {
		return true
	}
	if sessionID == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		ctx.Abort()
		return false
	}
	id, err := strconv.ParseUint(sessionID, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		ctx.Abort()
		return false
	}

	active, err := sessionChecker.IsActive(ctx.Request.Context(), uint(id), time.Now(), sessionRevokeGrace())
	if err != nil {
		if utils.Log != nil {
			utils.Log.Errorf("[AUTH] session check failed for sid %s: %v", sessionID, err)
		}
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "authentication temporarily unavailable"})
		ctx.Abort()
		return false
	}
	if !active {
		if utils.Log != nil {
			utils.Log.Warnf("[AUTH] rejected access token for revoked/expired session %s on %s %s",
				sessionID, ctx.Request.Method, ctx.Request.URL.Path)
		}
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		ctx.Abort()
		return false
	}
	return true
}

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
		if !sessionGate(ctx, claims.SessionID) {
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
//
// A revoked session here simply degrades to "anonymous": the caller is a public
// endpoint, so there is nothing to protect and nothing to tell the visitor.
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
		if sessionChecker != nil {
			id, parseErr := strconv.ParseUint(claims.SessionID, 10, 64)
			if parseErr != nil {
				ctx.Next()
				return
			}
			active, checkErr := sessionChecker.IsActive(ctx.Request.Context(), uint(id), time.Now(), sessionRevokeGrace())
			if checkErr != nil || !active {
				ctx.Next()
				return
			}
		}
		ctx.Set("clientID", claims.UserID)
		ctx.Set("role", claims.Role)
		if claims.CenterID != 0 {
			ctx.Set("centerID", claims.CenterID)
		}
		ctx.Next()
	}
}
