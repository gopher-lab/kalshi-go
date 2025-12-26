# kalshi-go

A Go trading bot for [Kalshi](https://kalshi.com) prediction markets.

## Project Structure

```
kalshi-go/
├── cmd/
│   └── kalshi-bot/       # CLI entry point
├── pkg/
│   └── ws/               # WebSocket client package
│       ├── client.go     # WebSocket client implementation
│       ├── channels.go   # Channel type definitions
│       ├── messages.go   # Message types and parsing
│       └── options.go    # Connection options
├── internal/
│   └── config/           # Configuration handling
└── go.mod
```

## Installation

```bash
go mod download
```

## Usage

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `KALSHI_API_KEY` | For auth | Your Kalshi API key |
| `KALSHI_PRIVATE_KEY` | For auth | Your Kalshi private key |
| `KALSHI_WS_URL` | No | Custom WebSocket URL (defaults to production) |
| `KALSHI_DEBUG` | No | Enable debug logging (`true`/`false`) |

### Running the Bot

```bash
# Unauthenticated (public channels only)
go run ./cmd/kalshi-bot

# Authenticated
export KALSHI_API_KEY="your-api-key"
export KALSHI_PRIVATE_KEY="your-private-key"
go run ./cmd/kalshi-bot
```

## WebSocket Package (`pkg/ws`)

The `ws` package provides a clean, idiomatic Go client for the Kalshi WebSocket API.

### Channels

| Channel | Auth Required | Description |
|---------|--------------|-------------|
| `orderbook_delta` | No | Orderbook updates |
| `ticker` | No | Market ticker |
| `trade` | No | Public trades |
| `lifecycle` | No | Market & event lifecycle |
| `fill` | Yes | User fills |
| `positions` | Yes | Market positions |

### Example

```go
package main

import (
    "context"
    "log"

    "github.com/brendanplayford/kalshi-go/pkg/ws"
)

func main() {
    // Create client with options
    client := ws.New(
        ws.WithAPIKeyOption("your-api-key", "your-private-key"),
        ws.WithCallbacks(
            func() { log.Println("connected") },
            func(err error) { log.Printf("disconnected: %v", err) },
            func(err error) { log.Printf("error: %v", err) },
        ),
    )

    // Connect
    ctx := context.Background()
    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Set message handler
    client.SetMessageHandler(func(msg *ws.Response) {
        log.Printf("received: %+v", msg)
    })

    // Subscribe to orderbook updates
    _, err := client.Subscribe(ctx, "TICKER-EXAMPLE", ws.ChannelOrderbookDelta)
    if err != nil {
        log.Fatal(err)
    }

    // Keep running...
    select {}
}
```

## API Reference

See [Kalshi WebSocket Documentation](https://docs.kalshi.com/websockets/websocket-connection) for full API details.

## License

MIT

