package main

import (
	"log"

	"go-websocket/internal/handlers"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	app := fiber.New()

	handlers.RegisterLogonHandler(app)
	handlers.RegisterRealtimeHandler(app)
	handlers.RegisterHistoricalHandler(app)

	log.Fatal(app.Listen(":3000"))
}
