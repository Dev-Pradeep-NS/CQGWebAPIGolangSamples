package handlers

import (
	"go-websocket/internal/client"
	"go-websocket/internal/services"
	pb "go-websocket/proto/WebAPI"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"google.golang.org/protobuf/proto"
)

func RegisterRealtimeHandler(app *fiber.App) {
	app.Get("/realtime", websocket.New(handleRealtime))
}

func handleRealtime(c *websocket.Conn) {
	symbol := c.Query("symbol")
	if symbol == "" {
		c.WriteJSON(fiber.Map{"error": "Symbol parameter is required"})
		c.Close()
		return
	}

	cqgClient, err := client.NewCQGClient()
	if err != nil {
		c.WriteJSON(fiber.Map{"error": "Connection failed: " + err.Error()})
		c.Close()
		return
	}
	defer cqgClient.Close()

	userName := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")

	if err := cqgClient.Logon(userName, password); err != nil {
		c.WriteJSON(fiber.Map{"error": "Logon failed: " + err.Error()})
		c.Close()
		return
	}

	contractID, err := cqgClient.ResolveSymbol(symbol, 1, true)
	if err != nil {
		c.WriteJSON(fiber.Map{"error": "Symbol resolution failed: " + err.Error()})
		c.Close()
		return
	}

	if err := cqgClient.SubscribeMarketData(contractID, 2, 1); err != nil {
		c.WriteJSON(fiber.Map{"error": "Subscription failed: " + err.Error()})
		c.Close()
		return
	}

	done := make(chan bool)
	go handleRealtimeMessages(c, cqgClient, done)

	for {
		if _, _, err := c.ReadMessage(); err != nil {
			log.Println("client read:", err)
			break
		}
	}
	<-done
}

func handleRealtimeMessages(c *websocket.Conn, cqgClient *client.CQGClient, done chan bool) {
	for {
		_, msg, err := cqgClient.WS.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			c.WriteJSON(fiber.Map{"error": "Connection closed"})
			c.Close()
			done <- true
			return
		}

		serverMsg := &pb.ServerMsg{}
		if err := proto.Unmarshal(msg, serverMsg); err != nil {
			log.Println("unmarshal:", err)
			continue
		}

		if len(serverMsg.RealTimeMarketData) > 0 {
			rtData := serverMsg.RealTimeMarketData[0]
			marketValues := rtData.GetMarketValues()[0]

			priceScale := rtData.GetCorrectPriceScale()
			openPrice := float64(marketValues.GetScaledOpenPrice()) * priceScale
			highPrice := float64(marketValues.GetScaledHighPrice()) * priceScale
			lowPrice := float64(marketValues.GetScaledLowPrice()) * priceScale
			lastPrice := float64(marketValues.GetScaledLastPrice()) * priceScale

			data := map[string]interface{}{
				"timestamp":     time.Now().Format(time.RFC3339),
				"contract_id":   rtData.GetContractId(),
				"open_price":    openPrice,
				"high_price":    highPrice,
				"low_price":     lowPrice,
				"last_price":    lastPrice,
				"total_volume":  marketValues.GetTotalVolume().GetSignificand(),
				"open_interest": marketValues.GetOpenInterest().GetSignificand(),
				"tick_volume":   marketValues.GetTickVolume(),
				"trade_date":    marketValues.GetTradeDate(),
			}

			if err := services.SaveRealtimeData(data); err != nil {
				log.Printf("Failed to save to PocketBase: %v", err)
			}

			if err := c.WriteJSON(rtData); err != nil {
				log.Println("write json:", err)
				done <- true
				return
			}
		}
	}
}
