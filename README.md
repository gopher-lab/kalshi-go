# kalshi-go

A Go trading bot for [Kalshi](https://kalshi.com) prediction markets, with analysis and tools for the **LA High Temperature** market.

## 🎯 LA High Temperature Strategy

Tools and backtesting for the KXHIGHLAX (Highest Temperature in LA) market.

**📚 Full documentation: [docs/LAHIGH-STRATEGY.md](docs/LAHIGH-STRATEGY.md)**

### ⚠️ Key Finding: Model Accuracy is Limited

Our rigorous backtest revealed honest results:

| Metric | Value |
|--------|-------|
| Model prediction accuracy | **50%** (7/14 days) |
| First trade prices | **5-40¢** (cheap!) |
| Potential edge (if correct) | **60-95¢** |
| +1°F calibration reliability | **~50%** (varies ±1-2°F) |

### The Edge IS Real, Prediction is Hard

```
✅ WHAT WORKS:
   - Winning brackets start at 5-40¢
   - First trades happen 7-9 AM PT day before
   - There IS 60-95¢ of potential profit per contract

❌ WHAT'S HARD:
   - +1°F calibration only works ~50% of time
   - Some days CLI = METAR +2°F, some days -1°F
   - Simple models can't reliably pick the winner
```

### Real Backtest Results (14 Days)

| Date | METAR | Predicted | Winner | Correct | Profit |
|------|-------|-----------|--------|---------|--------|
| Dec 25 | 66°F | 66-67° | 66-67° | ✅ | -$0.84 |
| Dec 24 | 64°F | 64-65° | 64-65° | ✅ | -$1.50 |
| Dec 23 | 63°F | 64-65° | 64-65° | ✅ | +$26.60 |
| Dec 22 | 64°F | 64-65° | 64-65° | ✅ | +$0.06 |
| Dec 21 | 64°F | 64-65° | 64-65° | ✅ | +$3.41 |
| Dec 20 | 65°F | 65-66° | 65-66° | ✅ | +$1.49 |
| Dec 19 | 69°F | 69-70° | 71-72° | ❌ | -$9.69 |
| ... | | | | | |
| **Total** | | | | **50%** | **$0.77** |

### Recommended Approach

```
┌────────────────────────────────────────────────────────────────┐
│  FOR LEARNING / THESIS TESTING                                 │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│  1. Use small position sizes ($5-15)                          │
│  2. Hedge across 2-3 brackets                                 │
│  3. Track predictions vs outcomes                             │
│  4. Iterate and improve model                                 │
│                                                                │
│  This is DATA COLLECTION, not guaranteed profit.              │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

### Quick Start

```bash
# Run the rigorous backtest (honest results)
go run ./cmd/lahigh-backtest-rigorous/

# Run the trade data backtest (price evolution)
go run ./cmd/lahigh-backtest-real/

# Monitor today's temperature
go run ./cmd/lahigh-monitor/
```

## Project Structure

```
kalshi-go/
├── cmd/
│   ├── kalshi-bot/              # Generic WebSocket bot
│   ├── lahigh-autorun/          # Set-and-forget trading bot
│   ├── lahigh-trader/           # LA High Temperature trader
│   ├── lahigh-backtest-rigorous/# Rigorous prediction backtest (NEW)
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
# Run rigorous backtest (simulates predictions, honest results)
go run ./cmd/lahigh-backtest-rigorous/

# Run real trade data backtest (shows price evolution)
go run ./cmd/lahigh-backtest-real/

# Monitor real-time temperature at LAX
go run ./cmd/lahigh-monitor/

# Run the trading bot
go run ./cmd/lahigh-trader/ -event KXHIGHLAX-25DEC27

# Run with Docker
docker-compose up --build -d
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

## Key Learnings

1. **Edge timing**: The biggest edge is on the **DAY BEFORE**, not the day of
2. **First trade prices**: Winning brackets start at 5-40¢ when market opens
3. **Prediction is hard**: +1°F calibration only works ~50% of the time
4. **Model limitations**: Simple METAR+calibration can't reliably beat the market
5. **Future work**: Need better prediction models, probabilistic approaches

## Honest Assessment

This project demonstrates:
- ✅ The infrastructure to trade Kalshi markets
- ✅ Real-time data fetching and parsing
- ✅ Comprehensive backtesting framework
- ⚠️ Model accuracy is limited (~50%)
- ⚠️ NOT a "money printer" without better prediction

The **tooling** is solid. The **prediction model** needs work.

## License

MIT
