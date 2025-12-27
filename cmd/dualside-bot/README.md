# Dual-Side Temperature Trading Bot

Automated trading bot that maximizes liquidity by trading **both YES and NO** on temperature markets.

## Strategy

When signals agree on bracket X winning:
1. **BUY YES** on bracket X (primary bet, best ROI)
2. **BUY NO** on losing brackets (additional exposure, 97.7% win rate)

### Why This Works

| Side | Win Rate | Avg Profit | Logic |
|------|----------|------------|-------|
| YES on winner | 62.2% | $25.62/trade | Market often priced in |
| NO on losers | **97.7%** | $17.67/trade | Almost always correct |

When we're confident X wins, ALL other brackets lose - so NO on them is nearly free money.

## Backtest Results (21 days, 7 cities)

| Metric | YES Only | YES + NO | Improvement |
|--------|----------|----------|-------------|
| Trades | 74 | 161 | +117% |
| Profit | $1,896 | **$3,433** | **+81%** |
| Annual | $10,835 | **$19,618** | **+81%** |

## Usage

### Dry Run (Recommended First)

```bash
go run ./cmd/dualside-bot/ --dry-run --bet-yes=300 --bet-no=100
```

### Live Trading

```bash
go run ./cmd/dualside-bot/ --bet-yes=300 --bet-no=100
```

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `--bet-yes` | $300 | YES trade size |
| `--bet-no` | $100 | Each NO trade size |
| `--max-no` | 3 | Max NO trades per event |
| `--min-no-price` | 50¢ | Minimum NO price to trade |
| `--max-no-price` | 90¢ | Maximum NO price to trade |
| `--interval` | 5m | Polling interval |
| `--dry-run` | false | Simulate without executing |

## Example Output

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                    DUAL-SIDE TEMPERATURE TRADING BOT                        ║
╚══════════════════════════════════════════════════════════════════════════════╝

💰 Starting Balance: $500.00
📊 YES Bet: $300 | NO Bet: $100 (max 3 per event)

[09:15:00] Analyzing markets...
  Los Angeles: Fav=60-61°@41¢ METAR=61°→60-61° Agree=true
    → YES: BUY 731 contracts of 60-61° @ 41¢ ($299.71)
    → NO:  BUY 137 contracts of 62-63° @ 73¢ ($100.01)
    → NO:  BUY 122 contracts of 58-59° @ 82¢ ($100.04)
```

## Trading Logic

1. **Signal Check**: Market favorite must match METAR bracket
2. **Price Filter**: YES price must be 20-95¢
3. **NO Selection**: Only NO prices in 50-90¢ range
4. **Position Limit**: One set of positions per market per day

## Markets Covered

- Los Angeles (LAX)
- New York (JFK)  
- Chicago (ORD)
- Miami (MIA)
- Austin (AUS)
- Philadelphia (PHL)
- Denver (DEN)

## Risk Management

- Only trades when 2+ signals agree
- Limits NO trades to $100 each
- Maximum 3 NO trades per event
- Dry-run mode for testing

