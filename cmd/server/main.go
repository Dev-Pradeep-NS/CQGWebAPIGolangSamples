package main

import (
	"log"

	"go-websocket/internal/handlers"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Create a new Fiber app instance
	app := fiber.New()

	// Register route handlers for different endpoints
	handlers.RegisterLogonHandler(app)      // Authentication endpoints
	handlers.RegisterRealtimeHandler(app)   // Real-time data endpoints
	handlers.RegisterHistoricalHandler(app) // Historical data endpoints

	// Start the server on port 3000
	log.Fatal(app.Listen(":3000"))
}
