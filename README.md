# kalshi-go

A Go trading bot for [Kalshi](https://kalshi.com) prediction markets, with a validated strategy for the **LA High Temperature** market.

## 🎯 LA High Temperature Strategy

A backtested and validated trading strategy for the KXHIGHLAX (Highest Temperature in LA) market.

### Validated Results (53 Days of Real Data)

| Metric | Value |
|--------|-------|
| Days with big edge (<50¢ entry) | 71.7% |
| Average profit per trade | 65.7¢ |
| Best strategy Sharpe ratio | 85.0 |
| Win rate (on winning bracket) | 100% |

### The Strategy

The daily high temperature at LAX typically occurs by **10-11 AM PT** (62.5% of the time). By monitoring METAR data early, you can identify the winning bracket before the market prices it in.

```
Entry Price    Action          Expected Profit
-----------    ------          ---------------
< 50¢          🟢 STRONG BUY   ~70¢ profit
50-80¢         🟡 BUY          ~35¢ profit  
> 80¢          🔴 SKIP         Edge too small
```

### Quick Start

```bash
# Run the validated backtest
go run ./cmd/lahigh-backtest-validated/

# Monitor today's temperature
go run ./cmd/lahigh-monitor/

# Run the trading bot
go run ./cmd/lahigh-trader/ -event KXHIGHLAX-25DEC27
```

## Project Structure

```
kalshi-go/
├── cmd/
│   ├── kalshi-bot/              # Generic WebSocket bot
│   ├── lahigh-trader/           # LA High Temperature trader
│   ├── lahigh-monitor/          # Real-time temperature monitor
│   ├── lahigh-backtest-validated/ # Backtest with real prices
│   ├── lahigh-backtest-full/    # Full historical backtest
│   ├── lahigh-montecarlo/       # Monte Carlo simulation
│   └── lahigh-predict/          # Temperature prediction
├── pkg/
│   ├── ws/                      # WebSocket client
│   └── rest/                    # REST API client
├── internal/
│   └── config/                  # Configuration handling
└── go.mod
```

## Installation

```bash
go mod download
```

## Configuration

Create a `.env` file with your Kalshi API credentials:

```
KALSHI_API_KEY=your-api-key-id
KALSHI_PRIVATE_KEY=-----BEGIN RSA PRIVATE KEY-----
...your private key...
-----END RSA PRIVATE KEY-----
```

## Commands

### LA High Temperature Trading

```bash
# Run validated backtest (uses real Kalshi prices)
go run ./cmd/lahigh-backtest-validated/

# Monitor real-time temperature at LAX
go run ./cmd/lahigh-monitor/

# Run the trading bot (manual confirmation mode)
go run ./cmd/lahigh-trader/ -event KXHIGHLAX-25DEC27

# Run with auto-trading (be careful!)
go run ./cmd/lahigh-trader/ -event KXHIGHLAX-25DEC27 -auto

# Use demo environment (no real money)
go run ./cmd/lahigh-trader/ -event KXHIGHLAX-25DEC27 -demo
```

### Generic Kalshi Bot

```bash
# Connect and subscribe to a market
go run ./cmd/kalshi-bot -market "KXBTC-25DEC31-T50000" -channel "ticker"
```

## Packages

### pkg/ws - WebSocket Client

Full-featured WebSocket client for Kalshi's streaming API.

```go
client := ws.New(
    ws.WithAPIKeyOption("your-api-key", privateKey),
)
client.Connect(ctx)
client.Subscribe(ctx, "MARKET-TICKER", ws.ChannelTicker)
```

### pkg/rest - REST API Client

REST client for order placement and market data.

```go
client := rest.New(apiKey, privateKey)

// Get balance
balance, _ := client.GetBalance()

// Place an order
order, _ := client.BuyYes("KXHIGHLAX-25DEC27-B62.5", 10, 50)

// Get positions
positions, _ := client.GetPositions()
```

## Data Sources

| Source | Data | Used For |
|--------|------|----------|
| [Iowa State ASOS](https://mesonet.agron.iastate.edu/) | Historical METAR | Backtesting |
| [Aviation Weather Center](https://aviationweather.gov/) | Real-time METAR | Live monitoring |
| [NWS API](https://api.weather.gov/) | Forecasts | Predictions |
| Kalshi API | Trade history, prices | Validation |

## Testing

```bash
# Run unit tests
go test ./pkg/ws/...

# Run integration tests (requires credentials)
go test -tags=integration ./pkg/ws/...
```

## Key Findings

1. **Daily max timing**: 62.5% of daily highs occur between 10-11 AM PT
2. **METAR→CLI calibration**: +1°F adjustment from METAR to NWS CLI
3. **Edge frequency**: 72% of days have significant edge (<50¢ entry)
4. **No-edge days**: Only 4% of days have no trading opportunity

## License

MIT
