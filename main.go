package main

import (
	"log"
	"os"

	"github.com/Nina-99/yottu-api/configs"
	"github.com/Nina-99/yottu-api/router"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("🚀 Could not load environment variables", err)
	}

	// NOTE: DataBase
	// validate := validator.New()
	configs.DBConnection()

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
