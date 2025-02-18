package handlers

import (
	"fmt"
	"go-websocket/internal/client"
	"go-websocket/internal/models"
	pb "go-websocket/proto/WebAPI"
	"log"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"google.golang.org/protobuf/proto"
)

func RegisterHistoricalHandler(app *fiber.App) {
	app.Get("/historical", websocket.New(handleHistorical))
}

func handleHistorical(c *websocket.Conn) {
	symbol := c.Query("symbol")
	barType := c.Query("barType")
	period := c.Query("period")
	number := c.Query("number")

	if symbol == "" || period == "" || number == "" {
		c.WriteJSON(fiber.Map{"error": "Required parameters missing"})
		c.Close()
		return
	}

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

func handleHistoricalData(c *websocket.Conn, cqgClient *client.CQGClient, symbol string, barUnit uint32, timeRange models.TimeRange) error {
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

	if err := cqgClient.Logon(userName, password, clientAppId, clientVersion, uint32(protocolMajor), uint32(protocolMinor)); err != nil {
		return err
	}

	contractID, err := cqgClient.ResolveSymbol(symbol, 1, true)
	if err != nil {
		return err
	}

	msgID := uint32(2)
	if err := cqgClient.RequestBarTime(msgID, contractID, barUnit, timeRange); err != nil {
		return err
	}

	done := make(chan bool)
	go processHistoricalMessages(c, cqgClient, done)

	for {
		if _, _, err := c.ReadMessage(); err != nil {
			log.Println("client read error:", err)
			break
		}
	}
	<-done
	return nil
}

func processHistoricalMessages(c *websocket.Conn, cqgClient *client.CQGClient, done chan bool) {
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
			log.Println("unmarshal error:", err)
			continue
		}

		if len(serverMsg.TimeBarReports) > 0 {
			for _, report := range serverMsg.TimeBarReports {
				response := createHistoricalResponse(report)
				if err := c.WriteJSON(response); err != nil {
					log.Println("write error:", err)
					done <- true
					return
				}

				if report.GetIsReportComplete() {
					log.Println("Historical data complete")
					done <- true
					return
				}
			}
		}
	}
}

func createHistoricalResponse(report *pb.TimeBarReport) map[string]interface{} {
	return map[string]interface{}{
		"request_id":         report.GetRequestId(),
		"status_code":        report.GetStatusCode(),
		"up_to_utc_time":     report.GetUpToUtcTime(),
		"is_report_complete": report.GetIsReportComplete(),
		"bars":               report.GetTimeBars(),
	}
}
