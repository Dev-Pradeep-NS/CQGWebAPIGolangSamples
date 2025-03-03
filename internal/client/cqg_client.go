package client

import (
	"fmt"
	"log"
	"os"
	"time"

	"go-websocket/internal/models"
	pb "go-websocket/proto/WebAPI"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

// CQGClient represents a WebSocket client for connecting to CQG's trading platform
type CQGClient struct {
	WS               *websocket.Conn      // WebSocket connection
	BaseTime         int64                // Base time received from server for time synchronization
	ContractMetadata *pb.ContractMetadata // Metadata about the trading contract
}

// NewCQGClient creates and initializes a new CQG client with WebSocket connection
func NewCQGClient() (*CQGClient, error) {
	// Load environment variables from .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("Error loading .env file")
	}

	// Get hostname from environment variables
	hostName := os.Getenv("HOST_NAME")
	if hostName == "" {
		return nil, fmt.Errorf("HOST_NAME environment variable not set")
	}

	// Establish WebSocket connection
	ws, _, err := websocket.DefaultDialer.Dial(hostName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to establish WebSocket connection: %w", err)
	}

	return &CQGClient{WS: ws}, nil
}

// Logon authenticates the client with the CQG server
func (c *CQGClient) Logon(userName, password, clientAppId, clientVersion string, protocolVersionMajor uint32, protocolVersionMinor uint32) error {
	// Validate required credentials
	if userName == "" || password == "" || clientAppId == "" || clientVersion == "" {
		return fmt.Errorf("neccessary creds are not provided")
	}

	// Create logon message
	logon := &pb.Logon{
		UserName:             proto.String(userName),
		Password:             proto.String(password),
		ClientAppId:          proto.String(clientAppId),
		ClientVersion:        proto.String(clientVersion),
		ProtocolVersionMajor: proto.Uint32(protocolVersionMajor),
		ProtocolVersionMinor: proto.Uint32(protocolVersionMinor),
	}

	clientMsg := &pb.ClientMsg{
		Logon: logon,
	}

	// Marshal and send logon message
	data, err := proto.Marshal(clientMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal logon message: %w", err)
	}

	if err = c.WS.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("failed to send logon message: %w", err)
	}

	// Read and process server response
	_, msg, err := c.WS.ReadMessage()
	if err != nil {
		return fmt.Errorf("websocket read error: %w", err)
	}

	serverMsg := &pb.ServerMsg{}
	if err := proto.Unmarshal(msg, serverMsg); err != nil {
		return fmt.Errorf("protobuf unmarshal error: %w", err)
	}

	log.Printf("Raw server response: %+v", serverMsg)

	// Handle logon result
	if logonResult := serverMsg.GetLogonResult(); logonResult != nil {
		log.Printf("Logon result code: %d, message: %s",
			logonResult.GetResultCode(),
			logonResult.GetTextMessage(),
		)

		if logonResult.GetResultCode() != 0 {
			return fmt.Errorf("server rejection: %s (code %d)",
				logonResult.GetTextMessage(),
				logonResult.GetResultCode(),
			)
		}

		// Parse and store base time for time synchronization
		baseTimeStr := logonResult.GetBaseTime()
		if baseTimeStr == "" {
			return fmt.Errorf("empty base time received from server")
		}

		parsedTime, err := time.Parse("2006-01-02T15:04:05", baseTimeStr)
		if err != nil {
			return fmt.Errorf("invalid base time format: %w", err)
		}

		c.BaseTime = parsedTime.UnixNano() / int64(time.Millisecond)
	} else {
		return fmt.Errorf("unexpected response type: %T", serverMsg)
	}

	return nil
}

// Logoff sends a logoff request to the CQG server to terminate the session
func (c *CQGClient) Logoff() error {
	logoff := &pb.Logoff{
		TextMessage: proto.String("logoff test"),
	}

	clientMsg := &pb.ClientMsg{
		Logoff: logoff,
	}

	// Marshal the client message to binary format
	data, err := proto.Marshal(clientMsg)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	// Send the logoff message over websocket connection
	if err := c.WS.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("write message error: %w", err)
	}
	// Read and process server response
	_, msg, err := c.WS.ReadMessage()
	if err != nil {
		return fmt.Errorf("websocket read error: %w", err)
	}

	serverMsg := &pb.ServerMsg{}
	if err := proto.Unmarshal(msg, serverMsg); err != nil {
		return fmt.Errorf("protobuf unmarshal error: %w", err)
	}

	log.Printf("Raw server response: %+v", serverMsg)

	return nil
}

// ResolveSymbol resolves a trading symbol and returns its contract ID
func (c *CQGClient) ResolveSymbol(symbolName string, msgID uint32, subscribe bool) (uint32, error) {
	if symbolName == "" {
		return 0, fmt.Errorf("symbol name cannot be empty")
	}

	// Create symbol resolution request
	informationRequest := &pb.InformationRequest{
		Id:        proto.Uint32(msgID),
		Subscribe: proto.Bool(subscribe),
		SymbolResolutionRequest: &pb.SymbolResolutionRequest{
			Symbol: proto.String(symbolName),
		},
	}

	clientMsg := &pb.ClientMsg{
		InformationRequests: []*pb.InformationRequest{informationRequest},
	}

	log.Printf("Client message sent:\n%+v\n", clientMsg)

	// Marshal and send request
	data, err := proto.Marshal(clientMsg)
	if err != nil {
		return 0, fmt.Errorf("marshal error: %w", err)
	}

	if err := c.WS.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return 0, fmt.Errorf("write message error: %w", err)
	}

	// Read and process server response
	_, msg, err := c.WS.ReadMessage()
	if err != nil {
		return 0, fmt.Errorf("read message error: %w", err)
	}

	serverMsg := &pb.ServerMsg{}
	if err := proto.Unmarshal(msg, serverMsg); err != nil {
		return 0, fmt.Errorf("unmarshal error: %w", err)
	}

	log.Printf("Server message received:\n%+v\n", serverMsg)
	if len(serverMsg.InformationReports) == 0 {
		return 0, fmt.Errorf("no information reports received")
	}

	// Extract contract metadata from response
	infoReport := serverMsg.InformationReports[0]
	if resReport := infoReport.GetSymbolResolutionReport(); resReport != nil {
		if resReport.GetContractMetadata() == nil {
			return 0, fmt.Errorf("no contract metadata in response")
		}
		c.ContractMetadata = resReport.GetContractMetadata()
		return c.ContractMetadata.GetContractId(), nil
	}

	return 0, fmt.Errorf("symbol resolution failed")
}

// SubscribeMarketData subscribes to market data updates for a specific contract
func (c *CQGClient) SubscribeMarketData(contractID, msgID, level uint32) error {
	if contractID == 0 {
		return fmt.Errorf("invalid contract ID")
	}

	// Create market data subscription request
	subscription := &pb.MarketDataSubscription{
		ContractId:        proto.Uint32(contractID),
		RequestId:         proto.Uint32(msgID),
		Level:             proto.Uint32(level),
		IncludePastQuotes: proto.Bool(true),
	}

	clientMsg := &pb.ClientMsg{
		MarketDataSubscriptions: []*pb.MarketDataSubscription{subscription},
	}

	// Marshal and send subscription request
	data, err := proto.Marshal(clientMsg)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	if err := c.WS.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("write message error: %w", err)
	}

	return nil
}

// RequestBarTime requests historical bar data for a specific time range
func (c *CQGClient) RequestBarTime(msgID uint32, contractID uint32, barUnit uint32, timeRange models.TimeRange, requestType uint32) error {
	if contractID == 0 {
		return fmt.Errorf("invalid contract ID")
	}

	if timeRange.Number <= 0 {
		return fmt.Errorf("invalid time range number")
	}

	var barsNumber int
	var intervalMillis int64

	// Calculate number of bars and interval based on bar unit and time range
	switch barUnit {
	case models.DailyIndex:
		intervalMillis = models.MillisecondsInDay
		switch timeRange.Period {
		case "day":
			barsNumber = timeRange.Number
		case "month":
			barsNumber = timeRange.Number * models.DaysInMonth
		case "year":
			barsNumber = timeRange.Number * models.DaysInYear
		default:
			return fmt.Errorf("invalid time period for daily bars")
		}

	case models.HourlyIndex:
		intervalMillis = models.MillisecondsInHour
		switch timeRange.Period {
		case "day":
			barsNumber = timeRange.Number * models.HoursInDay
		case "month":
			barsNumber = timeRange.Number * models.DaysInMonth * models.HoursInDay
		case "year":
			barsNumber = timeRange.Number * models.DaysInYear * models.HoursInDay
		default:
			return fmt.Errorf("invalid time period for hourly bars")
		}

	case models.MinutelyIndex:
		intervalMillis = models.MillisecondsInMinute
		switch timeRange.Period {
		case "day":
			barsNumber = timeRange.Number * models.HoursInDay * models.MinutesInHour
		case "month":
			barsNumber = timeRange.Number * models.DaysInMonth * models.HoursInDay * models.MinutesInHour
		case "year":
			barsNumber = timeRange.Number * models.DaysInYear * models.HoursInDay * models.MinutesInHour
		default:
			return fmt.Errorf("invalid time period for minutely bars")
		}

	default:
		return fmt.Errorf("invalid bar unit")
	}

	// Calculate time range and create request
	currentTimeMillis := time.Now().UTC().UnixNano() / int64(time.Millisecond)
	fromUtcTime := currentTimeMillis - c.BaseTime - (int64(barsNumber) * intervalMillis)

	tbRequest := &pb.TimeBarRequest{
		RequestId: proto.Uint32(msgID),
		TimeBarParameters: &pb.TimeBarParameters{
			ContractId:  proto.Uint32(contractID),
			BarUnit:     proto.Uint32(barUnit),
			FromUtcTime: proto.Int64(fromUtcTime),
		},
		RequestType: proto.Uint32(requestType),
	}

	clientMsg := &pb.ClientMsg{
		TimeBarRequests: []*pb.TimeBarRequest{tbRequest},
	}

	log.Printf("Requesting historical data:\n%s", PrettyPrintProto(clientMsg))

	// Marshal and send request
	data, err := proto.Marshal(clientMsg)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	if err := c.WS.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("write message error: %w", err)
	}

	return nil
}

// HandleMessages continuously reads and processes incoming server messages
func (c *CQGClient) HandleMessages(handler func(*pb.ServerMsg)) {
	if handler == nil {
		log.Printf("error: message handler is nil")
		return
	}

	for {
		_, msg, err := c.WS.ReadMessage()
		if err != nil {
			log.Printf("read message error: %v", err)
			return
		}

		serverMsg := &pb.ServerMsg{}
		if err := proto.Unmarshal(msg, serverMsg); err != nil {
			log.Printf("unmarshal error: %v", err)
			continue
		}

		handler(serverMsg)
	}
}

// PrettyPrintProto formats a protobuf message for readable output
func PrettyPrintProto(msg proto.Message) string {
	return prototext.Format(msg)
}

// Close cleanly closes the WebSocket connection
func (c *CQGClient) Close() {
	if c.WS != nil {
		c.WS.Close()
	}
}
