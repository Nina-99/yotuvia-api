package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets the response headers a browser needs before it can be
// trusted to behave safely with this API's output.
//
// The Content-Security-Policy below is NOT cosmetic. This service also serves
// user-uploaded files under /uploads from the same origin as the web app, so a
// file that a browser decides to interpret as a document (a stored `.html`, or
// any future format that slips through) must be inert. `default-src 'none'` +
// `sandbox` denies it scripts, network access and same-origin privileges, which
// keeps the web app's localStorage tokens out of reach even if an upload bug
// regresses. JSON responses are unaffected by a CSP.
func SecurityHeaders() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Cross-Origin-Resource-Policy", "same-site")
		header.Set("Permissions-Policy", "geolocation=(self), camera=(), microphone=()")
		header.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; sandbox")

		// HSTS is only meaningful (and only honored) over TLS. Behind a proxy the
		// scheme arrives in X-Forwarded-Proto; locally there is no TLS and the
		// header is skipped so plain-HTTP development keeps working.
		if ctx.Request.TLS != nil || strings.EqualFold(ctx.GetHeader("X-Forwarded-Proto"), "https") {
			header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		ctx.Next()
	}
}

// BodyLimit caps how many request-body bytes Gin will read for a handler.
//
// Without it, `ShouldBindJSON` reads an arbitrarily large body into memory:
// a handful of concurrent multi-megabyte posts is enough to inflate the process.
// Multipart uploads are exempt because they carry their own per-file caps
// (5MB receipts, 10MB images, 100MB videos) and buffering them here would break
// the legitimate upload paths.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if maxBytes <= 0 {
			ctx.Next()
			return
		}
		if strings.HasPrefix(ctx.GetHeader("Content-Type"), "multipart/") {
			ctx.Next()
			return
		}
		if ctx.Request.ContentLength > maxBytes {
			ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxBytes)
		ctx.Next()
	}
}
