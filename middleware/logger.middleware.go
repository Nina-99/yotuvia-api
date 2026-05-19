package middleware

import (
	"time"

	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
)

func GinLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 1.  Capture the start time
		start := time.Now()
		path := ctx.Request.URL.Path
		query := ctx.Request.URL.RawQuery

		// 2. Process the request (pass the request to the controller)
		ctx.Next()

		// 3. Cature the data after the controller finishes
		end := time.Now()
		latency := end.Sub(start)
		status := ctx.Writer.Status()
		clientIP := ctx.ClientIP()
		method := ctx.Request.Method

		// 4. Determine thw log nivel based on the request result
		// if the status is 5xxx, it's a system error
		// if it's 4xx, it's a client error.
		// if it's 2xx/3xx, it's a normal operation
		if len(ctx.Errors) > 0 {
			// if Gin caught errors during execution
			for _, e := range ctx.Errors.Errors() {
				utils.Log.Errorf("[API ERROR] Path: %s", path, e)
			}
		} else if status >= 500 {
			utils.Log.Errorf("[HTTP 5XX] %s %s | Status: %d | Latency: %s | IP: %s | QUery: %s", method, path, status, latency, clientIP, query)
		} else if status >= 400 {
			utils.Log.Warnf("[HTTP 4XX] %s %s | Status: %d | Latency: %s | IP: %s ", method, path, status, latency, clientIP)
		} else {
			utils.Log.Infof("[HTTP 2XX] %s %s | Status: %d | Latency: %s | IP: %s ", method, path, status, latency, clientIP)
		}
	}
}
