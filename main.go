package main

import (
	"log"
	"os"

	"github.com/Nina-99/yotuvia-api/configs"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// version is stamped at release-build time with
// `-ldflags "-X main.version=v1.2.3"`. A local `go build` / `go run` keeps the
// "dev" value, so a logged version always tells you which binary is running.
var version = "dev"

func main() {
	// Production default: release mode. Gin's debug mode prints the route table
	// and verbose request diagnostics at boot; `GIN_MODE=debug` opts back in for
	// local work.
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	if err := godotenv.Load(); err != nil {
		log.Fatal("🚀 Could not load environment variables", err)
	}

	utils.InitLogger(utils.LoggerConfig{
		Level:  os.Getenv("LOG_LEVEL"),  // "debug", "info"
		Format: os.Getenv("LOG_FORMAT"), // "console" or "json"
	})
	utils.Log.Infof("[BOOT] yotuvia-api %s", version)

	// Transactional email transport. SMTP when SMTP_HOST is configured,
	// otherwise the log-only dev mailer (the automatic fallback). Log the active
	// mode loudly ONCE so an operator can tell whether real mail is going out.
	mailer, err := utils.MailerFromEnv()
	if err != nil {
		utils.Log.Fatalf("Invalid email configuration: %v", err)
	}
	utils.SetDefaultMailer(mailer)
	utils.Log.Infof("[MAILER] active mode: %s", mailer.Mode())

	// NOTE: DataBase
	// validate := validator.New()
	configs.DBConnection()

	// Phase 3 maintenance loop (reminders, no-shows, holds, waitlist).
	// Non-blocking; disable with SCHEDULER_ENABLED=false.
	configs.StartBackgroundScheduler(configs.DB)

	// NOTE: Use port of the .env or 8080 for default
	// Usar el puerto del .env o 8080 por defecto
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// NOTE: Here the server is located
	// Aqui se levanta el servidor
	router.Run(":" + port)
}
