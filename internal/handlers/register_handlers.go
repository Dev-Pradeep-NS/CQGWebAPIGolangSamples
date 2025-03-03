package handlers

import (
	"fmt"
	"go-websocket/internal/client"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// RegisterLogonHandler registers the logon endpoint with the Fiber application
func RegisterHandler(app *fiber.App) {
	app.Get("/logon", handleLogon)
	app.Get("/logoff", handleLogoff)
}

// handleLogon processes the logon request by retrieving credentials and client information
// from environment variables, establishing a connection with CQG, and attempting to log in
func handleLogon(c *fiber.Ctx) error {
	// Retrieve credentials and client information from environment variables
	userName := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	clientAppId := os.Getenv("CLIENT_APP_ID")
	clientVersion := os.Getenv("CLIENT_VERSION")
	protocolVersionMajor := os.Getenv("PROTOCOL_VERSION_MAJOR")
	protocolVersionMinor := os.Getenv("PROTOCOL_VERSION_MINOR")

	// Parse protocol version major number from string to uint
	protocolMajor, err := strconv.ParseUint(protocolVersionMajor, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid PROTOCOL_VERSION_MAJOR: %v", err)
	}

	// Parse protocol version minor number from string to uint
	protocolMinor, err := strconv.ParseUint(protocolVersionMinor, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid PROTOCOL_VERSION_MINOR: %v", err)
	}

	// Validate that required environment variables are set
	if userName == "" || password == "" || clientAppId == "" || clientVersion == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Username or password or clientAppId or clientVersion not found in environment variables",
		})
	}

	// Create a new CQG client instance
	cqgClient, err := client.NewCQGClient()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Connection failed: " + err.Error(),
		})
	}

	// Attempt to log in to CQG with provided credentials and client information
	err = cqgClient.Logon(userName, password, clientAppId, clientVersion, uint32(protocolMajor), uint32(protocolMinor))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Logon failed: " + err.Error(),
		})
	}

	// Return success response if logon is successful
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Logon successful",
	})
}

func handleLogoff(c *fiber.Ctx) error {
	// Create a new CQG client instance
	cqgClient, err := client.NewCQGClient()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Connection failed: " + err.Error(),
		})
	}

	// Attempt to log off from CQG
	err = cqgClient.Logoff()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Logoff failed: " + err.Error(),
		})
	}

	// Return success response if logoff is successful
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Logoff successful",
	})
}
