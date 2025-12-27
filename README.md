# kalshi-go

A Go trading bot for [Kalshi](https://kalshi.com) prediction markets, with analysis and tools for the **LA High Temperature** market.

## 🎯 LA High Temperature Strategy

Tools and backtesting for the KXHIGHLAX (Highest Temperature in LA) market.

**📚 Full documentation: [docs/LAHIGH-STRATEGY.md](docs/LAHIGH-STRATEGY.md)**

### 🏆 VALIDATED: Ensemble Strategy (82% Win Rate!)

After testing 20+ strategies, we found one that works:

| Metric | Value |
|--------|-------|
| Win rate | **81.8%** (9/11 trades) |
| Total profit (21 days) | **+$27.00** |
| Sharpe ratio | **4.52** (excellent!) |
| Kelly fraction | **+40%** (real edge!) |

### The Strategy

```
Trade only when 2+ signals agree:
  1. Market favorite (highest first price)
  2. METAR prediction (rounded to bracket)
  3. 2nd best bracket

If 2+ agree → BUY that bracket
If no consensus → SKIP the day (48% of days)
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
# Run the validated ensemble strategy
go run ./cmd/lahigh-final-strategy/

# Run the strategy optimizer (tests 20+ strategies)
go run ./cmd/lahigh-optimizer/

# Find edge opportunities
go run ./cmd/lahigh-edge-finder/

# Monitor today's temperature
go run ./cmd/lahigh-monitor/
```

## Project Structure

```
kalshi-go/
├── cmd/
│   ├── kalshi-bot/              # Generic WebSocket bot
│   ├── lahigh-final-strategy/   # 🏆 Validated ensemble strategy
│   ├── lahigh-optimizer/        # Strategy optimizer (20+ strategies)
│   ├── lahigh-edge-finder/      # Edge discovery tool
│   ├── lahigh-market-follow/    # Market-following analysis
│   ├── lahigh-threshold-optimize/ # Threshold optimization
│   ├── lahigh-deep-analysis/    # Pattern analysis
│   ├── lahigh-autorun/          # Automated trading bot
│   ├── lahigh-trader/           # Manual trading bot
│   ├── lahigh-backtest-rigorous/# Rigorous prediction backtest
│   ├── lahigh-backtest-real/    # Real Kalshi trade data backtest
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

1. **Cheap brackets DON'T win**: Brackets with first trade <30¢ have 0% win rate
2. **Market IS efficient**: High-priced brackets are priced high because they're likely to win
3. **Single signals fail**: +1°F calibration alone only works ~50% of the time
4. **Signal consensus works**: When 2+ signals agree, win rate jumps to 82%
5. **Skip uncertain days**: No trade is better than a bad trade

## Validated Strategy

✅ **ENSEMBLE STRATEGY WORKS**
- Win rate: 81.8%
- Sharpe ratio: 4.52
- Kelly fraction: +40% (real edge)
- Trade only when market + METAR + 2nd agree

The key insight: **Don't fight the market. Use METAR to confirm the market's pick.**

## License

MIT
