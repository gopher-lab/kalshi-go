# kalshi-go

A Go trading bot for [Kalshi](https://kalshi.com) prediction markets, with a validated strategy for the **LA High Temperature** market.

## 🎯 LA High Temperature Strategy

A backtested and validated trading strategy for the KXHIGHLAX (Highest Temperature in LA) market.

**📚 Full documentation: [docs/LAHIGH-STRATEGY.md](docs/LAHIGH-STRATEGY.md)**

### Validated Results (14 Days of Real Kalshi Trade Data)

| Metric | Value |
|--------|-------|
| Average edge at first trade | **71¢** |
| Days with 50%+ edge | **93%** (13/14 days) |
| First trade timing | **7-9 AM PT day before** |
| METAR→CLI accuracy | 96.2% |

### Key Insight: Edge is on the DAY BEFORE

The market for tomorrow's weather opens at ~7 AM PT **today**. First trades on the winning bracket are typically at **5-40¢**, providing massive edge.

```
Timeline for Dec 27's weather:
┌────────────────────────────────────────────────────────┐
│ Dec 26, 7 AM:   Market opens, first trade @ 20-40¢    │
│ Dec 26, PM:     Trades at 40-70¢                      │
│ Dec 27, AM:     Hits 80-90% as temp confirmed         │
│ Dec 28, 10 AM:  SETTLEMENT - pays $1.00               │
└────────────────────────────────────────────────────────┘
```

### Entry Recommendations

| Entry Price | When | Expected Profit |
|-------------|------|-----------------|
| **< 40¢** | Day before morning | 🟢 **60-95¢** |
| 40-70¢ | Day before afternoon | 🟡 30-60¢ |
| 70-90¢ | Target day morning | 🟠 10-30¢ |
| > 90¢ | Target day afternoon | 🔴 NO EDGE |

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
│   ├── lahigh-autorun/          # Set-and-forget trading bot
│   ├── lahigh-trader/           # LA High Temperature trader
│   ├── lahigh-backtest-real/    # Real Kalshi trade data backtest
│   ├── lahigh-backtest-validated/ # Backtest with real prices
│   ├── lahigh-montecarlo/       # Monte Carlo simulation
│   ├── lahigh-predict-v2/       # Temperature prediction (NWS + METAR)
│   └── lahigh-status/           # Check bot readiness
├── pkg/
│   ├── ws/                      # WebSocket client
│   └── rest/                    # REST API client
├── internal/
│   └── config/                  # Configuration handling
├── docs/
│   └── LAHIGH-STRATEGY.md       # Full strategy documentation
├── Dockerfile                   # Docker build
├── docker-compose.yml           # Docker compose config
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

1. **Edge timing**: The biggest edge is on the **DAY BEFORE**, not the day of
2. **First trade prices**: Winning brackets start at 5-40¢ when market opens
3. **Daily max timing**: 76.5% of daily highs occur between 10 AM - 12 PM PT
4. **METAR→CLI calibration**: +1°F adjustment from METAR to NWS CLI (96.2% accuracy)
5. **Edge frequency**: 93% of days have 50%+ edge at first trade

## License

MIT
