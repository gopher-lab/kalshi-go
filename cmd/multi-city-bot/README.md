# Multi-City Temperature Trading Bot

Automated trading bot for 7 HIGH temperature markets using the 3-Signal ENSEMBLE strategy.

## Strategy

**3-Signal ENSEMBLE:**
1. **Market Favorite** - Highest priced bracket
2. **Second Best** - Confirms market uncertainty level
3. **METAR Observation** - Real-time temperature data

**Trade when all signals agree** + price is 20-95¢ + confidence > 10¢ spread

## Supported Markets

| City | Event Prefix | Timezone |
|------|--------------|----------|
| Los Angeles | KXHIGHLAX | America/Los_Angeles |
| New York | KXHIGHNY | America/New_York |
| Chicago | KXHIGHCHI | America/Chicago |
| Miami | KXHIGHMIA | America/New_York |
| Austin | KXHIGHAUS | America/Chicago |
| Philadelphia | KXHIGHPHIL | America/New_York |
| Denver | KXHIGHDEN | America/Denver |

## Usage

```bash
# Dry run (no real trades)
go run ./cmd/multi-city-bot/ --dry-run --bet=50

# Live trading with $100/trade
go run ./cmd/multi-city-bot/ --bet=100

# Custom poll interval
go run ./cmd/multi-city-bot/ --bet=50 --interval=10m
```

## Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `--bet` | 50 | Bet size per trade in dollars |
| `--dry-run` | false | Simulate without executing |
| `--interval` | 5m | Polling interval |

## Trading Window

The bot only trades during local daytime hours:
- **Start:** 7:00 AM local time
- **End:** 2:00 PM local time

This ensures METAR data is available for the day.

## Expected Returns

Based on backtesting (21 days, Dec 2025):

| Bet Size | Monthly | Yearly |
|----------|---------|--------|
| $50 | $350 | $4,200 |
| $100 | $700 | $8,400 |
| $200 | $1,400 | $16,800 |
| $500 | $3,500 | $42,000 |

## Risk Management

- **Win Rate:** 97.4% (backtest)
- **Max Drawdown:** Depends on bet size
- **Position Limit:** $25,000 per strike (Kalshi limit)

## Configuration

Requires `.env` file with:
```
KALSHI_API_KEY=your-key
KALSHI_PRIVATE_KEY=-----BEGIN RSA PRIVATE KEY-----
...
-----END RSA PRIVATE KEY-----
```

## Logging

All trades are logged to stdout. Consider redirecting to file:
```bash
go run ./cmd/multi-city-bot/ --bet=100 2>&1 | tee trading.log
```

