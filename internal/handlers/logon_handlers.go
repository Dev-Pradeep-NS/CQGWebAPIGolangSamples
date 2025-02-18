package handlers

import (
	"fmt"
	"go-websocket/internal/client"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func RegisterLogonHandler(app *fiber.App) {
	app.Get("/logon", handleLogon)
}

func handleLogon(c *fiber.Ctx) error {
	userName := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	clientAppId := os.Getenv("CLIENT_APP_ID")
	clientVersion := os.Getenv("CLIENT_VERSION")
	protocolVersionMajor := os.Getenv("PROTOCOL_VERSION_MAJOR")
	protocolVersionMinor := os.Getenv("PROTOCOL_VERSION_MINOR")

	protocolMajor, err := strconv.ParseUint(protocolVersionMajor, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid PROTOCOL_VERSION_MAJOR: %v", err)
	}

	protocolMinor, err := strconv.ParseUint(protocolVersionMinor, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid PROTOCOL_VERSION_MINOR: %v", err)
	}

	if userName == "" || password == "" || clientAppId == "" || clientVersion == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Username or password or clientAppId or clientVersion not found in environment variables",
		})
	}

	cqgClient, err := client.NewCQGClient()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Connection failed: " + err.Error(),
		})
	}

	err = cqgClient.Logon(userName, password, clientAppId, clientVersion, uint32(protocolMajor), uint32(protocolMinor))
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
