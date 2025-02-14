package main

import (
	"log"
	"os"

	"go-websocket/pkg/client"
	pb "go-websocket/proto/WebAPI"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/joho/godotenv"
	"google.golang.org/protobuf/proto"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	app := fiber.New()

	app.Get("/logon", func(c *fiber.Ctx) error {
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
	})

	app.Get("/realtime", websocket.New(func(c *websocket.Conn) {
		// Get symbol from query parameters
		symbol := c.Query("symbol")
		if symbol == "" {
			c.WriteJSON(fiber.Map{"error": "Symbol parameter is required"})
			c.Close()
			return
		}

		// Create CQG client
		cqgClient, err := client.NewCQGClient()
		if err != nil {
			c.WriteJSON(fiber.Map{"error": "Connection failed: " + err.Error()})
			c.Close()
			return
		}
		defer cqgClient.Close()

		// Perform logon
		userName := os.Getenv("USERNAME")
		password := os.Getenv("PASSWORD")
		if err := cqgClient.Logon(userName, password); err != nil {
			c.WriteJSON(fiber.Map{"error": "Logon failed: " + err.Error()})
			c.Close()
			return
		}

		// Resolve symbol
		contractID, err := cqgClient.ResolveSymbol(symbol, 1, true)
		if err != nil {
			c.WriteJSON(fiber.Map{"error": "Symbol resolution failed: " + err.Error()})
			c.Close()
			return
		}

		// Subscribe to market data
		if err := cqgClient.SubscribeMarketData(contractID, 2, 1); err != nil {
			c.WriteJSON(fiber.Map{"error": "Subscription failed: " + err.Error()})
			c.Close()
			return
		}

		// Start real-time updates
		go func() {
			for {
				_, msg, err := cqgClient.WS.ReadMessage()
				if err != nil {
					log.Println("read:", err) // Log the error for debugging
					c.WriteJSON(fiber.Map{"error": "Connection closed"})
					c.Close()
					return
				}

				serverMsg := &pb.ServerMsg{}
				if err := proto.Unmarshal(msg, serverMsg); err != nil {
					log.Println("unmarshal:", err) // Log the error for debugging
					continue
				}

				if len(serverMsg.RealTimeMarketData) > 0 {
					rtData := serverMsg.RealTimeMarketData[0] // Access the first element in the slice

					// Send the entire RealTimeMarketData message as JSON
					if err := c.WriteJSON(rtData); err != nil {
						log.Println("write json:", err)
						return
					}

				} else {
					//Log that the message was empty or not of the expected type.
					log.Println("Received a message but it did not contain RealTimeMarketData or was empty")
				}
			}
		}()
		// Keep connection alive
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				log.Println("client read:", err) //Log client read error
				break
			}
		}
	}))

	log.Fatal(app.Listen(":3000"))
}
