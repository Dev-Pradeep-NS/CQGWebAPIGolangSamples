# CQG WebSocket Market Data Service

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue)](https://golang.org/)
[![PocketBase](https://img.shields.io/badge/PocketBase-0.22%2B-green)](https://pocketbase.io/)
[![Fiber](https://img.shields.io/badge/Fiber-2.4%2B-orange)](https://gofiber.io/)

Real-time market data streaming solution integrating CQG's WebSocket API with PocketBase persistence. Built for high-frequency trading data processing.

## Table of Contents
- [Features](#features)
- [Architecture](#architecture)
- [Data Models](#data-models)
- [Protocol Buffers](#protocol-buffers)
- [Installation](#installation)
- [Configuration](#configuration)
- [API Documentation](#api-documentation)
- [Database Schema](#database-schema)
- [Sample Data](#sample-data)
- [Development](#development)
- [License](#license)
- [WebSocket Access](#websocket-access)

## Features

| Feature                      | Description                                                                 |
|------------------------------|-----------------------------------------------------------------------------|
| Real-time Streaming          | WebSocket-based market data updates (bids/asks/trades)                     |
| Historical Data API          | REST endpoints for historical market analysis                              |
| Data Persistence             | PocketBase with PostgreSQL backend for time-series storage                 |
| Protocol Buffers             | Efficient binary serialization for high-frequency updates                  |
| Authentication               | JWT-based secure access to market data streams                             |
| Rate Limiting                | Built-in API request throttling                                            |

## Architecture

```text
CQGWebAPIGolangSamples/
├── cmd/                  # Application entry points
│   └── server/
│       └── main.go       # Main server configuration
├── internal/             # Core application logic
│   ├── client/           # CQG WebSocket client implementation
│   ├── handlers/         # API endpoint controllers
│   │   ├── historical_handlers.go # Historical data handlers
│   │   ├── logon_handlers.go      # Authentication handlers
│   │   └── realtime_handlers.go    # Real-time WebSocket handlers
│   ├── models/           # Data structures and constants
│   │   └── models.go     # Time interval configurations
│   └── services/         # Business logic services
│       └── pocketbase.go # PocketBase integration layer
├── pocketbase/           # Embedded database configuration
│   ├── pb_migrations/    # Database schema versions
│   │   └── 1739788064_created_market_data.js # Initial schema
│   └── pocketbase        # PocketBase executable
└── proto/                # Protocol Buffer definitions
    └── WebAPI/
        └── market_data_2.pb.go # Generated Protobuf code
```

## Data Models

### Time Constants (`internal/models/models.go`)
```go
// TimeRange defines a period for historical data requests
type TimeRange struct {
    Period string // "minute", "hour", "day"
    Number int    // Quantity of periods
}

// Time conversion constants (milliseconds)
const (
    MillisecondsInDay    = 86400000  // 24 * 60 * 60 * 1000
    MillisecondsInHour   = 3600000   // 60 * 60 * 1000
    MillisecondsInMinute = 60000     // 60 * 1000
    DaysInMonth          = 31
    DaysInYear           = 365
)
```

## Protocol Buffers

### Market Data Structure (`proto/WebAPI/market_data_2.pb.go`)
```go
// Real-time market data message
type RealTimeMarketData struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields

    MarketValues []*MarketValues `protobuf:"bytes,5,rep,name=market_values,json=marketValues" json:"market_values,omitempty"`
    Trades       []*Trade        `protobuf:"bytes,6,rep,name=trades" json:"trades,omitempty"`
    Corrections  []*Correction   `protobuf:"bytes,7,rep,name=corrections" json:"corrections,omitempty"`
}

// Individual market values snapshot
type MarketValues struct {
    OpenPrice  float64 `protobuf:"fixed64,1,opt,name=open_price,json=openPrice" json:"open_price,omitempty"`
    HighPrice  float64 `protobuf:"fixed64,2,opt,name=high_price,json=highPrice" json:"high_price,omitempty"`
    LowPrice   float64 `protobuf:"fixed64,3,opt,name=low_price,json=lowPrice" json:"low_price,omitempty"`
    ClosePrice float64 `protobuf:"fixed64,4,opt,name=close_price,json=closePrice" json:"close_price,omitempty"`
}
```

## Installation

### Prerequisites
- Go 1.21+
- PocketBase 0.22+
- CQG API Credentials

### Step-by-Step Setup
1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/cqg-websocket-service.git
   cd cqg-websocket-service
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Configure environment variables (create `.env`):
   ```env
   # CQG API Configuration
   CQG_API_KEY=your_api_key_here
   CQG_API_SECRET=your_api_secret_here

   # PocketBase Configuration
   POCKETBASE_DATA_DIR=./pocketbase/pb_data
   POCKETBASE_MIGRATIONS_DIR=./pocketbase/pb_migrations
   ```

4. Initialize PocketBase database:
   ```bash
   cd pocketbase
   ./pocketbase migrate up
   ./pocketbase serve
   ```

5. Start the application:
   ```bash
   go run cmd/server/main.go
   ```

### Real-time Data Stream
```bash
wscat -c "ws://localhost:3000/realtime?symbol=ZUC"
```

### Historical Bar Data
```bash
# Hourly bars for last 2 days
wscat -c "ws://localhost:3000/historical?symbol=EUC&barType=hourly&period=day&number=2"

# Minutely bars for last day
wscat -c "ws://localhost:3000/historical?symbol=EUC&barType=minutely&period=day&number=1"

# Daily bars for last month
wscat -c "ws://localhost:3000/historical?symbol=EUC&barType=daily&period=month&number=1"
```

**Parameters**:
- `symbol`: Contract identifier (e.g. ZUC, EUC)
- `barType`: `minutely` | `hourly` | `daily`
- `period`: `day` | `month` | `year`
- `number`: Quantity of periods to fetch

**Requirements**:
1. Install `wscat` (WebSocket client):
```bash
npm install -g wscat
```

2. Replace `ZUC`/`EUC` with your actual contract symbols
3. Ensure service is running on port 3000

## Database Schema

### Market Data Collection (`pocketbase/pb_migrations/1739788064_created_market_data.js`)
```javascript
{
  "name": "market_data",
  "schema": [
    {
      "name": "contract_id",
      "type": "number",
      "options": {
        "min": null,
        "max": null,
        "noDecimal": false
      }
    },
    {
      "name": "is_snapshot",
      "type": "bool"
    },
    {
      "name": "corrections",
      "type": "json",
      "options": {
        "maxSize": 2000000
      }
    },
    {
      "name": "market_values",
      "type": "json",
      "options": {
        "maxSize": 2000000
      }
    },
    {
      "name": "trades", 
      "type": "json",
      "options": {
        "maxSize": 2000000
      }
    },
    {
      "name": "timestamp",
      "type": "date"
    }
  ],
  "indexes": []
}
```

**Key Fields**:
- `contract_id`: Numeric identifier for financial contracts
- `is_snapshot`: Boolean flag for full market state snapshots
- `market_values`: JSON storage of OHLC data (2MB max)
- `trades`: JSON array of trade records (2MB max) 
- `corrections`: JSON array of data corrections
- `timestamp`: Precise timestamp of market update

**Storage Limits**:
- JSON fields support up to 2MB of data
- All fields are optional for partial updates

## Sample Data

### Real-time Update
```json
{
  "id": "a1b2c3d4",
  "contract_id": 12345,
  "is_snapshot": false,
  "market_values": {
    "open": 7.251,
    "high": 7.2598,
    "low": 7.2327,
    "close": 7.2407
  },
  "trades": [
    {
      "price": "7.2407",
      "volume": 1,
      "utc_time": 1684612800000
    }
  ],
  "corrections": [],
  "created": "2024-05-20T15:30:45.123Z"
}
```

## Development

### Building from Source
```bash
go build -o bin/server cmd/server/main.go
```

### Running Tests
```bash
go test -v ./internal/...
```

### Generating Protobuf Code
```bash
protoc --go_out=. --go_opt=paths=source_relative proto/WebAPI/market_data_2.proto
```

## License
MIT License - See [LICENSE](LICENSE) for full text

---

**Disclaimer**: This project is not affiliated with CQG, Inc. Use of CQG's API requires proper authorization.