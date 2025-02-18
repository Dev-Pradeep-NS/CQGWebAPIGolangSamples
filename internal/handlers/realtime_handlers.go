package handlers

import (
	"fmt"
	"go-websocket/internal/client"
	"go-websocket/internal/services"
	pb "go-websocket/proto/WebAPI"
	"log"
	"os"
	"strconv"

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
	clientAppId := os.Getenv("CLIENT_APP_ID")
	clientVersion := os.Getenv("CLIENT_VERSION")
	protocolVersionMajor := os.Getenv("PROTOCOL_VERSION_MAJOR")
	protocolVersionMinor := os.Getenv("PROTOCOL_VERSION_MINOR")

	protocolMajor, err := strconv.ParseUint(protocolVersionMajor, 10, 32)
	if err != nil {
		c.WriteJSON(fiber.Map{"error": "Invalid PROTOCOL_VERSION_MAJOR: " + err.Error()})
		c.Close()
		return
	}

	protocolMinor, err := strconv.ParseUint(protocolVersionMinor, 10, 32)
	if err != nil {
		c.WriteJSON(fiber.Map{"error": "Invalid PROTOCOL_VERSION_MINOR: " + err.Error()})
		c.Close()
		return
	}

	if err := cqgClient.Logon(userName, password, clientAppId, clientVersion, uint32(protocolMajor), uint32(protocolMinor)); err != nil {
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
	go handleRealtimeMessages(c, cqgClient, done, contractID)
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			log.Println("client read:", err)
			break
		}
	}
	<-done
}

func handleRealtimeMessages(c *websocket.Conn, cqgClient *client.CQGClient, done chan bool, contractID uint32) {
	priceScale := cqgClient.ContractMetadata.GetCorrectPriceScale()
	log.Printf("Using price scale: %v for contract: %v", priceScale, contractID)

	lastMarketValues := fiber.Map{
		"open":    0.0,
		"high":    0.0,
		"low":     0.0,
		"close":   0.0,
		"last":    0.0,
		"volume":  int64(0),
		"oi":      0,
		"utctime": int64(0),
	}

	var firstTradeOfSession bool = true

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

		if rtData := serverMsg.GetRealTimeMarketData(); rtData != nil {
			for _, rtDataEntry := range rtData {
				response := fiber.Map{
					"bids":          make([]fiber.Map, 0),
					"asks":          make([]fiber.Map, 0),
					"trades":        make([]fiber.Map, 0),
					"corrections":   make([]fiber.Map, 0),
					"market_values": lastMarketValues,
					"contract_id":   contractID,
				}

				trades := make([]fiber.Map, 0)

				for _, quote := range rtDataEntry.Quotes {
					if quote.GetType() == uint32(pb.Quote_TYPE_TRADE) {
						price := float64(quote.GetScaledPrice()) * priceScale
						volume := quote.Volume.GetSignificand()

						if firstTradeOfSession {
							lastMarketValues["open"] = price
							firstTradeOfSession = false
						}

						if price > lastMarketValues["high"].(float64) || lastMarketValues["high"].(float64) == 0 {
							lastMarketValues["high"] = price
						}
						if price < lastMarketValues["low"].(float64) || lastMarketValues["low"].(float64) == 0 {
							lastMarketValues["low"] = price
						}

						lastMarketValues["last"] = price
						lastMarketValues["close"] = price
						lastMarketValues["volume"] = lastMarketValues["volume"].(int64) + volume

						trades = append(trades, fiber.Map{
							"price":    fmt.Sprintf("%.4f", price),
							"volume":   volume,
							"utc_time": quote.GetQuoteUtcTime(),
						})
					}
				}

				if len(rtDataEntry.MarketValues) > 0 {
					for _, mv := range rtDataEntry.MarketValues {
						if mv.GetDayIndex() == 0 && (mv.GetScaledLastPriceNoSettlement() != 0 || mv.TotalVolume.GetSignificand() != 0) {
							lastMarketValues["open"] = float64(mv.GetScaledOpenPrice()) * priceScale
							lastMarketValues["high"] = float64(mv.GetScaledHighPrice()) * priceScale
							lastMarketValues["low"] = float64(mv.GetScaledLowPrice()) * priceScale
							lastMarketValues["close"] = float64(mv.GetScaledClosePrice()) * priceScale
							lastMarketValues["last"] = float64(mv.GetScaledLastPriceNoSettlement()) * priceScale
							lastMarketValues["volume"] = mv.TotalVolume.GetSignificand()
							lastMarketValues["oi"] = mv.OpenInterest.GetSignificand()
							lastMarketValues["utctime"] = mv.GetLastTradeUtcTimestamp()
							break
						}
					}
				}

				corrections := make([]fiber.Map, 0)
				for _, corr := range rtDataEntry.Corrections {
					corrections = append(corrections, fiber.Map{
						"type":      pb.Quote_Type(corr.GetType()).String(),
						"old_price": float64(corr.GetScaledSourcePrice()) * priceScale,
						"new_price": float64(corr.GetScaledPrice()) * priceScale,
						"timestamp": corr.GetQuoteUtcTime(),
						"is_cancel": corr.GetVolume().GetSignificand() == 0,
					})
				}

				if dom := rtDataEntry.GetDetailedDom(); dom != nil {
					response["dom"] = fiber.Map{
						"price_levels": processDOMLevels(dom, priceScale),
					}
				}

				response["trades"] = trades
				response["corrections"] = corrections

				c.WriteJSON(response)

				if len(trades) > 0 && trades[0]["utc_time"].(int64) > 0 {
					if err := services.SaveToPocketBase(response); err != nil {
						log.Println("Failed to save data into pocketbase:", err)
					}
				}
			}
		}
	}
}

func processDOMLevels(dom *pb.DetailedDOM, priceScale float64) []fiber.Map {
	var levels []fiber.Map
	for _, level := range dom.GetPriceLevels() {
		var bidQty, askQty int64
		for _, order := range level.GetOrders() {
			if level.GetSide() == 1 {
				bidQty += order.GetVolume().GetSignificand()
			} else if level.GetSide() == 2 {
				askQty += order.GetVolume().GetSignificand()
			}
		}

		levels = append(levels, fiber.Map{
			"price":   float64(level.GetScaledPrice()) * priceScale,
			"bid_qty": bidQty,
			"ask_qty": askQty,
		})
	}
	return levels
}
