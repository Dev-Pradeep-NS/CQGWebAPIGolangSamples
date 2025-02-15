package handlers

import (
	"go-websocket/internal/client"
	"os"

	"github.com/gofiber/fiber/v2"
)

func RegisterLogonHandler(app *fiber.App) {
	app.Get("/logon", handleLogon)
}

func handleLogon(c *fiber.Ctx) error {
	userName := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")

	if userName == "" || password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Username or password not found in environment variables",
		})
	}

	cqgClient, err := client.NewCQGClient()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Connection failed: " + err.Error(),
		})
	}

	err = cqgClient.Logon(userName, password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Logon failed: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Logon successful",
	})
}
