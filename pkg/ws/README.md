# ws - Kalshi WebSocket Client

A clean, idiomatic Go client for the [Kalshi WebSocket API](https://docs.kalshi.com/websockets/websocket-connection).

## Features

- **RSA-PSS Authentication** - Secure signing with your Kalshi API credentials
- **Channel Subscriptions** - Subscribe to orderbook, ticker, trades, fills, and positions
- **Automatic Keep-Alive** - Built-in ping/pong handling
- **Thread-Safe** - Safe for concurrent use
- **Functional Options** - Flexible configuration pattern

## Installation

```go
import "github.com/brendanplayford/kalshi-go/pkg/ws"
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/brendanplayford/kalshi-go/pkg/ws"
)

func main() {
    // Parse your RSA private key
    privateKey, err := ws.ParsePrivateKeyString(pemEncodedKey)
    if err != nil {
        log.Fatal(err)
    }

    // Create client with authentication
    client := ws.New(
        ws.WithAPIKeyOption("your-api-key-id", privateKey),
        ws.WithCallbacks(
            func() { log.Println("connected") },
            func(err error) { log.Printf("disconnected: %v", err) },
            func(err error) { log.Printf("error: %v", err) },
        ),
    )

    // Set up message handler
    client.SetMessageHandler(func(msg *ws.Response) {
        log.Printf("received: type=%s", msg.Type)
    })

    // Connect
    ctx := context.Background()
    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Subscribe to a channel
    _, err = client.Subscribe(ctx, "TICKER-EXAMPLE", ws.ChannelTicker)
    if err != nil {
        log.Fatal(err)
    }

    // Keep running...
    select {}
}
```

## Channels

| Channel | Constant | Auth Required | Description |
|---------|----------|---------------|-------------|
| `orderbook_delta` | `ChannelOrderbookDelta` | No | Real-time orderbook updates |
| `ticker` | `ChannelTicker` | No | Market ticker data |
| `trade` | `ChannelTrade` | No | Public trade executions |
| `lifecycle` | `ChannelLifecycle` | No | Market & event lifecycle |
| `fill` | `ChannelFill` | Yes | Your trade fills |
| `positions` | `ChannelPositions` | Yes | Your market positions |

## Client Methods

### Connection

```go
// Connect to WebSocket
err := client.Connect(ctx)

// Check connection status
if client.IsConnected() { ... }

// Close connection
err := client.Close()
```

### Subscriptions

```go
// Subscribe to channels for a market
id, err := client.Subscribe(ctx, "MARKET-TICKER", ws.ChannelTicker, ws.ChannelOrderbookDelta)

// Unsubscribe by subscription ID
id, err := client.Unsubscribe(ctx, sid1, sid2)

// List all active subscriptions
id, err := client.ListSubscriptions(ctx)

// Add markets to existing subscription
id, err := client.AddMarkets(ctx, []int64{sid}, []string{"MARKET-1", "MARKET-2"})

// Remove markets from subscription
id, err := client.RemoveMarkets(ctx, []int64{sid}, []string{"MARKET-1"})

// Get locally tracked subscriptions
subs := client.GetActiveSubscriptions() // map[int64]Channel
```

### Message Handling

```go
// Handle all incoming messages
client.SetMessageHandler(func(msg *ws.Response) {
    switch msg.Type {
    case ws.MessageTypeSubscribed:
        subMsg, _ := ws.ParseSubscribedMsg(msg.Msg)
        log.Printf("subscribed: channel=%s sid=%d", subMsg.Channel, subMsg.SID)
    case ws.MessageTypeError:
        errMsg, _ := ws.ParseErrorMsg(msg.Msg)
        log.Printf("error: code=%d msg=%s", errMsg.Code, errMsg.Msg)
    case ws.MessageTypeData:
        // Handle data updates
    }
})
```

## Configuration Options

```go
// Default options
client := ws.New()

// With authentication
client := ws.New(
    ws.WithAPIKeyOption(apiKey, privateKey),
)

// With custom base URL
client := ws.New(
    ws.WithBaseURLOption("wss://custom.endpoint"),
)

// With custom ping interval
client := ws.New(
    ws.WithPingIntervalOption(15 * time.Second),
)

// With auto-reconnect settings
client := ws.New(
    ws.WithAutoReconnectOption(true, 10), // enabled, max 10 attempts
)

// With callbacks
client := ws.New(
    ws.WithCallbacks(onConnect, onDisconnect, onError),
)

// Or use explicit options struct
opts := ws.DefaultOptions().
    WithAPIKey(apiKey, privateKey).
    WithBaseURL("wss://custom.endpoint")
client := ws.NewWithOptions(opts)
```

## Authentication

Kalshi uses RSA-PSS signing for WebSocket authentication. You need:
1. **API Key ID** - Your Kalshi API key identifier
2. **RSA Private Key** - PEM-encoded private key from Kalshi

```go
// Parse from PEM string
privateKey, err := ws.ParsePrivateKeyString(`-----BEGIN RSA PRIVATE KEY-----
...
-----END RSA PRIVATE KEY-----`)

// Parse from bytes
privateKey, err := ws.ParsePrivateKey(pemBytes)

// Generate signature manually (if needed)
sig, err := ws.GenerateSignature(privateKey, timestamp, "GET", "/trade-api/ws/v2")
```

## Message Types

### Request Types

```go
// Subscribe command
ws.CommandSubscribe

// Unsubscribe command  
ws.CommandUnsubscribe

// List subscriptions
ws.CommandListSubscriptions

// Update subscription
ws.CommandUpdateSubscription
```

### Response Types

```go
ws.MessageTypeSubscribed   // Subscription confirmed
ws.MessageTypeUnsubscribed // Unsubscription confirmed
ws.MessageTypeOK           // Operation successful
ws.MessageTypeError        // Error occurred
ws.MessageTypeData         // Data update from subscription
```

## Error Handling

```go
var (
    ws.ErrNotConnected     // Operation requires active connection
    ws.ErrAlreadyConnected // Connect called on active connection
    ws.ErrAuthRequired     // Channel requires authentication
    ws.ErrInvalidChannel   // Unknown channel specified
    ws.ErrConnectionClosed // Connection was closed
)
```

## Testing

```bash
# Run unit tests
go test ./pkg/ws/...

# Run integration tests (requires credentials)
export KALSHI_API_KEY="your-api-key"
export KALSHI_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----..."
go test -tags=integration ./pkg/ws/...
```

## Thread Safety

The client is safe for concurrent use. All public methods use appropriate locking:
- Connection state is protected by `sync.RWMutex`
- Message ID generation uses `atomic.Int64`
- Subscription tracking uses `sync.Map`

## License

MIT

