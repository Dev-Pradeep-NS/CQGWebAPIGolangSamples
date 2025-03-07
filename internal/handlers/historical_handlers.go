package handlers

import (
	"fmt"
	"go-websocket/internal/client"
	"go-websocket/internal/models"
	pb "go-websocket/proto/WebAPI"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"google.golang.org/protobuf/proto"
)

// RegisterHistoricalHandler registers the WebSocket endpoint for historical data
func RegisterHistoricalHandler(app *fiber.App) {
	app.Get("/historical", websocket.New(handleHistorical))
}

// handleHistorical processes WebSocket connections for historical data requests
// It validates input parameters and initializes the CQG client connection
func handleHistorical(c *websocket.Conn) {
	symbol := c.Query("symbol")
	barType := c.Query("barType")
	period := c.Query("period")
	number := c.Query("number")

	// Validate required parameters
	if symbol == "" || period == "" || number == "" {
		c.WriteJSON(fiber.Map{"error": "Required parameters missing"})
		c.Close()
		return
	}

	// Convert number parameter to integer
	numberInt, err := strconv.Atoi(number)
	if err != nil {
		c.WriteJSON(fiber.Map{"error": "Invalid number format"})
		c.Close()
		return
	}

	barUnit := getBarUnit(barType)
	timeRange := models.TimeRange{
		Period: period,
		Number: numberInt,
	}

	// Initialize CQG client
	cqgClient, err := client.NewCQGClient()
	if err != nil {
		c.WriteJSON(fiber.Map{"error": "Connection failed: " + err.Error()})
		c.Close()
		return
	}
	defer cqgClient.Close()

	if err := handleHistoricalData(c, cqgClient, symbol, barUnit, timeRange); err != nil {
		c.WriteJSON(fiber.Map{"error": err.Error()})
		c.Close()
		return
	}
}

// getBarUnit converts string bar type to corresponding uint32 index
func getBarUnit(barType string) uint32 {
	switch barType {
	case "hourly":
		return models.HourlyIndex
	case "minutely":
		return models.MinutelyIndex
	default:
		return models.DailyIndex
	}
}

// handleHistoricalData manages the main flow of historical data retrieval
// It handles authentication, symbol resolution, and data request
func handleHistoricalData(c *websocket.Conn, cqgClient *client.CQGClient, symbol string, barUnit uint32, timeRange models.TimeRange) error {
	// Get authentication credentials from environment
	userName := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	clientAppId := os.Getenv("CLIENT_APP_ID")
	clientVersion := os.Getenv("CLIENT_VERSION")
	protocolVersionMajor := os.Getenv("PROTOCOL_VERSION_MAJOR")
	protocolVersionMinor := os.Getenv("PROTOCOL_VERSION_MINOR")

	// Parse protocol version numbers
	protocolMajor, err := strconv.ParseUint(protocolVersionMajor, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid PROTOCOL_VERSION_MAJOR: %v", err)
	}

	protocolMinor, err := strconv.ParseUint(protocolVersionMinor, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid PROTOCOL_VERSION_MINOR: %v", err)
	}

	// Authenticate with CQG
	if err := cqgClient.Logon(userName, password, clientAppId, clientVersion, uint32(protocolMajor), uint32(protocolMinor)); err != nil {
		return err
	}

	// Resolve symbol to contract ID
	contractID, err := cqgClient.ResolveSymbol(symbol, 1, true)
	if err != nil {
		return err
	}

	// Request historical bar data
	// requestTYpe => 2 -> subscribe, 3 -> drop, 1 -> get
	msgID := uint32(2)
	if err := cqgClient.RequestBarTime(msgID, contractID, barUnit, timeRange, 1); err != nil {
		return err
	}

	// Start processing messages
	done := make(chan bool)
	go processHistoricalMessages(c, cqgClient, done)

	// Keep connection alive until client disconnects
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			log.Println("client read error:", err)
			break
		}
	}
	<-done
	return nil
}

// processHistoricalMessages handles incoming messages from CQG
// It processes time bar reports and sends them to the client
func processHistoricalMessages(c *websocket.Conn, cqgClient *client.CQGClient, done chan bool) {
	for {
		// Read message from CQG
		_, msg, err := cqgClient.WS.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			c.WriteJSON(fiber.Map{"error": "Connection closed"})
			c.Close()
			done <- true
			return
		}

		// Unmarshal protobuf message
		serverMsg := &pb.ServerMsg{}
		if err := proto.Unmarshal(msg, serverMsg); err != nil {
			log.Println("unmarshal error:", err)
			continue
		}

		// Process time bar reports
		if len(serverMsg.TimeBarReports) > 0 {
			for _, report := range serverMsg.TimeBarReports {
				response := createHistoricalResponse(report)
				if err := c.WriteJSON(response); err != nil {
					log.Println("write error:", err)
					done <- true
					return
				}

				// Check if all data has been received
				if report.GetIsReportComplete() {
					log.Println("Historical data complete")
					done <- true
					return
				}
			}
		}
	}
}

// createHistoricalResponse creates a map of time bar report data
// to be sent to the client
func createHistoricalResponse(report *pb.TimeBarReport) map[string]interface{} {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		// Fallback: if location loading fails, use UTC
		loc = time.UTC
	}

	// Convert each bar to include IST time
	newBars := make([]map[string]interface{}, 0, len(report.GetTimeBars()))
	for _, bar := range report.GetTimeBars() {
		utcTime := time.Unix(int64(bar.GetBarUtcTime()), 0).UTC()
		istTime := utcTime.In(loc).Format("2006-01-02 15:04:05")

		// Build a new map for this bar, copying available fields and adding the IST time.
		barMap := map[string]interface{}{
			"bar_utc_time":       bar.GetBarUtcTime(),
			"bar_ist_time":       istTime,
			"scaled_open_price":  bar.GetScaledOpenPrice(),
			"scaled_high_price":  bar.GetScaledHighPrice(),
			"scaled_low_price":   bar.GetScaledLowPrice(),
			"scaled_close_price": bar.GetScaledClosePrice(),
			"volume":             bar.GetVolume(), // if volume is a nested message, depending on your needs you may have to extract its fields
			"tick_volume":        bar.GetTickVolume(),
		}

		newBars = append(newBars, barMap)
	}

	return map[string]interface{}{
		"request_id":         report.GetRequestId(),
		"status_code":        report.GetStatusCode(),
		"up_to_utc_time":     report.GetUpToUtcTime(),
		"is_report_complete": report.GetIsReportComplete(),
		"bars":               newBars,
	}
}
