package client

import (
	"fmt"
	"log"
	"os"

	pb "go-websocket/proto/WebAPI"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"google.golang.org/protobuf/proto"
)

type CQGClient struct {
	WS *websocket.Conn
}

func NewCQGClient() (*CQGClient, error) {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	hostName := os.Getenv("HOST_NAME")

	// Establish WebSocket connection
	ws, _, err := websocket.DefaultDialer.Dial(hostName, nil)
	if err != nil {
		return nil, err
	}

	return &CQGClient{WS: ws}, nil
}

func (c *CQGClient) Logon(userName, password string) error {
	// Create Logon message with proper pointer fields
	logon := &pb.Logon{
		UserName:             proto.String(userName),
		Password:             proto.String(password),
		ClientAppId:          proto.String("WebApiTest"),
		ClientVersion:        proto.String("python-client-test-2-230"),
		ProtocolVersionMajor: proto.Uint32(2),
		ProtocolVersionMinor: proto.Uint32(230),
	}

	// Wrap Logon message in ClientMsg using the correct oneof field name
	clientMsg := &pb.ClientMsg{
		Logon: logon, // Directly assign to Logon field instead of using Message
	}

	// Serialize message
	data, err := proto.Marshal(clientMsg)
	if err != nil {
		return err
	}

	// Send message
	err = c.WS.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		return err
	}

	// Read response
	_, msg, err := c.WS.ReadMessage()
	if err != nil {
		return fmt.Errorf("websocket read error: %w", err)
	}

	// Deserialize response
	serverMsg := &pb.ServerMsg{}
	if err := proto.Unmarshal(msg, serverMsg); err != nil {
		return fmt.Errorf("protobuf unmarshal error: %w", err)
	}

	// Add detailed logging
	log.Printf("Raw server response: %+v", serverMsg)

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
	} else {
		return fmt.Errorf("unexpected response type: %T", serverMsg)
	}

	return nil
}

// ResolveSymbol sends a symbol resolution request and returns the contract ID.
func (c *CQGClient) ResolveSymbol(symbolName string, msgID uint32, subscribe bool) (uint32, error) {
	// Create InformationRequest with SymbolResolutionRequest
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

	// Serialize and send the message
	data, err := proto.Marshal(clientMsg)
	if err != nil {
		return 0, fmt.Errorf("marshal error: %w", err)
	}

	if err := c.WS.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return 0, fmt.Errorf("write message error: %w", err)
	}

	// Read and process the response
	_, msg, err := c.WS.ReadMessage()
	if err != nil {
		return 0, fmt.Errorf("read message error: %w", err)
	}

	serverMsg := &pb.ServerMsg{}
	if err := proto.Unmarshal(msg, serverMsg); err != nil {
		return 0, fmt.Errorf("unmarshal error: %w", err)
	}

	// Extract the contract ID from the response
	if len(serverMsg.InformationReports) > 0 {
		infoReport := serverMsg.InformationReports[0]
		if resReport := infoReport.GetSymbolResolutionReport(); resReport != nil {
			return resReport.GetContractMetadata().GetContractId(), nil
		}
	}

	return 0, fmt.Errorf("symbol resolution failed")
}

// SubscribeMarketData subscribes to real-time market data for a given contract ID.
func (c *CQGClient) SubscribeMarketData(contractID, msgID, level uint32) error {
	subscription := &pb.MarketDataSubscription{
		ContractId: proto.Uint32(contractID),
		RequestId:  proto.Uint32(msgID),
		Level:      proto.Uint32(level),
	}

	clientMsg := &pb.ClientMsg{
		MarketDataSubscriptions: []*pb.MarketDataSubscription{subscription},
	}

	data, err := proto.Marshal(clientMsg)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	if err := c.WS.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("write message error: %w", err)
	}

	return nil
}

// HandleMessages processes incoming messages from the WebSocket connection.
func (c *CQGClient) HandleMessages(handler func(*pb.ServerMsg)) {
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

func (c *CQGClient) Close() {
	c.WS.Close()
}
