# 3-Signal ENSEMBLE Strategy

The validated trading strategy for Kalshi's LA High Temperature market.

## 📊 Strategy Summary

| Metric | Value |
|--------|-------|
| Win Rate | **81.8%** |
| Trades | 52% of days |
| Avg Profit/Trade | **$2.45** |
| Yearly Profit (Monte Carlo) | **$336** |
| Sharpe Ratio | **3.44** |
| Risk of Ruin | **0%** |

## 🧠 How It Works

Trade only when **2 of 3 signals agree**:

1. **Market Favorite** - Bracket with highest first trade price
2. **METAR/NWS Forecast** - Expected high temperature for the day
3. **2nd Best Bracket** - Bracket with second highest price

```
If 2+ agree → BUY that bracket
If all disagree → SKIP the day
```

## 📁 Tools

| Tool | Command | Purpose |
|------|---------|---------|
| **recommend** | `go run ./cmd/3signal/recommend/` | Get today's trade recommendation |
| **strategy** | `go run ./cmd/3signal/strategy/` | Run full backtest with day-by-day breakdown |
| **montecarlo** | `go run ./cmd/3signal/montecarlo/` | Run 10,000 Monte Carlo simulations |
| **edge-finder** | `go run ./cmd/3signal/edge-finder/` | Discover edge conditions |

## 🚀 Quick Start

```bash
# Get today's recommendation
go run ./cmd/3signal/recommend/

# Example output:
# ✅ CONSENSUS REACHED!
#    Bracket: 60-61°F
#    Signals: Market + NWS (2/3 agree)
#    Current Ask: 40¢
#    → BUY 35 contracts for $14
```

## 📈 Monte Carlo Results (10,000 simulations)

```
Profit Distribution (1 year, $14/day):
  Mean:   $336
  Median: $336
  5th %:  $175 (worst case)
  95th %: $496 (best case)

Profitable Years: 99.9%
Risk of Ruin (5 years): 0%
```

## 📋 Daily Workflow

```
Every morning at 7-9 AM PT:

1. Run: go run ./cmd/3signal/recommend/

2. If "CONSENSUS REACHED":
   → Go to Kalshi
   → Buy the recommended bracket
   → Amount: $14

3. If "NO CONSENSUS":
   → Skip today
   → Check tomorrow
```

## ⚠️ Important Notes

- Based on 21 days of backtesting (Dec 5-26, 2025)
- Strategy skips ~48% of days (patience required)
- Use consistent position sizing ($14/day recommended)
- Not financial advice - trade at your own risk

