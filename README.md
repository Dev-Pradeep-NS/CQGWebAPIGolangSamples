# CQG WebSocket Market Data Service

A real-time market data service that connects to CQG's WebSocket API and stores data in PocketBase.

## Features

- Real-time market data streaming via WebSocket
- Historical data retrieval
- Data persistence using PocketBase
- RESTful API endpoints
- Configurable symbols and timeframes

## Tech Stack

- Go 1.21+
- Fiber (Web Framework)
- PocketBase (Database)
- Protocol Buffers
- WebSocket

## Project Structure

CQGWebAPIGolangSamples/
├── cmd/

│ └── server/
│ └── main.go
├── internal/

│ ├── client/
│ │ └── cqg_client.go
│ ├── handlers/
│ │ ├── logon_handler.go
│ │ ├── realtime_handler.go
│ │ └── historical_handler.go
│ ├── models/
│ │ └── models.go
│ └── services/
│ └── pocketbase.go
├── pkg/
│ └── proto/
├── .env
└── README.md

## Getting Started

1. Clone the repository:

   git clone https://github.com/yourusername/CQGWebAPIGolangSamples.git

2. Install dependencies:

   go mod tidy

3. Set up environment variables in `.env`:

   HOST_NAME=your_cqg_websocket_url
   USERNAME=your_username
   PASSWORD=your_password

4. Start PocketBase:

   ./pocketbase serve

5. Run the server:

   go run cmd/server/main.go

## API Endpoints

### WebSocket Endpoints

- `/realtime?symbol=<symbol>` - Real-time market data stream
- `/historical?symbol=<symbol>&barType=<type>&period=<period>&number=<number>` - Historical data

### HTTP Endpoints

- `/logon` - Test CQG connection

## Data Models

### Realtime Data

{
"timestamp": "2024-02-15T19:27:27Z",
"contract_id": 123,
"open_price": 72.479,
"high_price": 72.598,
"low_price": 72.419,
"last_price": 72.479,
"total_volume": 46383,
"open_interest": 108220,
"tick_volume": 10494,
"trade_date": 5346902000
}

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a new Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details
