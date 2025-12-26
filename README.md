# kalshi-go

A Go trading bot for [Kalshi](https://kalshi.com) prediction markets.

## Project Structure

```
kalshi-go/
├── cmd/
│   └── kalshi-bot/           # CLI entry point
├── pkg/
│   └── ws/                   # WebSocket client package
│       ├── auth.go           # RSA-PSS authentication
│       ├── client.go         # WebSocket client
│       ├── channels.go       # Channel definitions
│       ├── messages.go       # Message types
│       ├── options.go        # Configuration options
│       └── README.md         # Package documentation
├── internal/
│   └── config/               # Configuration handling
└── go.mod
```

## Installation

```bash
go mod download
```

## Quick Start

### 1. Set up credentials

Create a `.env` file with your Kalshi API credentials:

```
KALSHI_API_KEY=your-api-key-id
KALSHI_PRIVATE_KEY=-----BEGIN RSA PRIVATE KEY-----
...your private key...
-----END RSA PRIVATE KEY-----
```

### 2. Run the bot

```bash
# Just connect
go run ./cmd/kalshi-bot

# Subscribe to a market
go run ./cmd/kalshi-bot -market "KXBTC-25DEC31-T50000" -channel "ticker"

# Available channels: ticker, orderbook_delta, trade, fill, positions
```

## WebSocket Package

See [`pkg/ws/README.md`](pkg/ws/README.md) for full package documentation.

### Quick Example

```go
package main

import (
    "context"
    "log"

    "github.com/brendanplayford/kalshi-go/pkg/ws"
)

func main() {
    // Parse your RSA private key
    privateKey, _ := ws.ParsePrivateKeyString(pemKey)

    // Create authenticated client
    client := ws.New(
        ws.WithAPIKeyOption("your-api-key-id", privateKey),
    )

    // Connect
    ctx := context.Background()
    client.Connect(ctx)
    defer client.Close()

    // Subscribe to ticker updates
    client.Subscribe(ctx, "MARKET-TICKER", ws.ChannelTicker)

    select {}
}
```

### Channels

| Channel | Auth | Description |
|---------|------|-------------|
| `orderbook_delta` | No | Real-time orderbook updates |
| `ticker` | No | Market ticker data |
| `trade` | No | Public trade executions |
| `lifecycle` | No | Market & event lifecycle |
| `fill` | Yes | Your trade fills |
| `positions` | Yes | Your market positions |

## Testing

```bash
# Run unit tests (49 tests)
go test ./pkg/ws/...

# Run integration tests (requires credentials)
go test -tags=integration ./pkg/ws/...
```

## API Reference

- [Kalshi WebSocket Documentation](https://docs.kalshi.com/websockets/websocket-connection)
- [Package Documentation](pkg/ws/README.md)

## License

MIT
