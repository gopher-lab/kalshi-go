# LA High Temperature Trading Strategy

## Overview

This document describes the trading strategy analysis for Kalshi's "Highest Temperature in LA" (LAHIGH) market series, including honest findings from rigorous backtesting.

## Market Mechanics

- **Market**: KXHIGHLAX series
- **Settlement**: Based on NWS Daily Climate Report (CLI) for Los Angeles Airport
- **Trading Hours**: 24/7
- **Settlement Time**: 10:00 AM ET the day after
- **Brackets**: 2°F ranges (e.g., 62-63°F, 64-65°F)

## ⚠️ Key Finding: Model Accuracy is Limited

Our rigorous backtest using real data revealed a sobering reality:

| Metric | Claimed | Actual |
|--------|---------|--------|
| +1°F calibration accuracy | 96% | **50%** (7/14 days) |
| Expected profit | $900+ | **$0.77** |
| ROI | 100%+ | **0.4%** |

### Why the Discrepancy?

The original 96% figure was for **post-hoc METAR→CLI comparison** (looking at data after the fact).

For **prediction** (forecasting BEFORE the event), the +1°F calibration is NOT reliable:

| Pattern | Frequency | Impact |
|---------|-----------|--------|
| CLI = METAR + 1°F | ~50% | ✅ Prediction correct |
| CLI = METAR + 2°F | ~25% | ❌ Off by 1 bracket |
| CLI = METAR - 1°F | ~10% | ❌ Off by 1 bracket |
| Wide brackets | ~15% | ⚠️ Hard to evaluate |

## First Trade Prices ARE Cheap (Real Edge)

Despite model limitations, early prices remain genuinely underpriced:

| Date | Winner | 1st Price | First Trade | Edge |
|------|--------|-----------|-------------|------|
| Dec 25 | 66-67°F | 40¢ | Dec 24, 7:15 AM | 60¢ 🎯 |
| Dec 24 | 64-65°F | 35¢ | Dec 23, 8:55 AM | 65¢ 🎯 |
| Dec 23 | 64-65°F | 39¢ | Dec 22, 7:39 AM | 61¢ 🎯 |
| Dec 22 | 64-65°F | 34¢ | Dec 21, 8:33 AM | 66¢ 🎯 |
| Dec 21 | 64-65°F | 36¢ | Dec 20, 8:01 AM | 64¢ 🎯 |
| Dec 20 | 65-66°F | 21¢ | Dec 19, 7:15 AM | 79¢ 🎯 |
| Dec 19 | 71-72°F | 6¢ | Dec 18, 7:04 AM | 94¢ 🎯 |

**The edge IS real** - winning brackets start at 6-40¢. 

**The problem**: Identifying WHICH bracket will win in advance is the hard part.

## Rigorous Backtest Results

### Methodology

```
For each of 14 historical days:
1. Fetch METAR max from Iowa State ASOS
2. Apply +1°F calibration → Predicted bracket
3. Fetch actual winning bracket from Kalshi
4. Fetch real first trade prices
5. Simulate 70/30 hedge and calculate P&L
```

### Detailed Results

| Date | METAR | +1°F | Winner | Predicted | Correct | Profit |
|------|-------|------|--------|-----------|---------|--------|
| 2025-12-25 | 66°F | 67°F | 66-67° | 66-67° | ✅ | -$0.84 |
| 2025-12-24 | 64°F | 65°F | 64-65° | 64-65° | ✅ | -$1.50 |
| 2025-12-23 | 63°F | 64°F | 64-65° | 64-65° | ✅ | +$26.60 |
| 2025-12-22 | 64°F | 65°F | 64-65° | 64-65° | ✅ | +$0.06 |
| 2025-12-21 | 64°F | 65°F | 64-65° | 64-65° | ✅ | +$3.41 |
| 2025-12-20 | 65°F | 66°F | 65-66° | 65-66° | ✅ | +$1.49 |
| 2025-12-19 | 69°F | 70°F | 71-72° | 69-70° | ❌ | -$9.69 |
| 2025-12-18 | 70°F | 71°F | ≤72° | 70-71° | ❌ | $0.00 |
| 2025-12-17 | 72°F | 73°F | 74-75° | 72-73° | ❌ | -$13.53 |
| 2025-12-16 | 62°F | 63°F | ≤71° | 62-63° | ❌ | $0.00 |
| 2025-12-15 | 66°F | 67°F | 65-66° | 67-68° | ❌ | -$8.35 |
| 2025-12-14 | 62°F | 63°F | ≤65° | 62-63° | ❌ | $0.00 |
| 2025-12-13 | 61°F | 62°F | ≤65° | 62-63° | ❌ | $0.00 |
| 2025-12-12 | 67°F | 68°F | 67-68° | 67-68° | ✅ | +$3.12 |

### Prediction Errors Analyzed

```
Dec 19: METAR=69° → +1°F = 70° → Predicted 69-70°
        BUT CLI settled at 71-72° (CLI was +2°F higher)

Dec 17: METAR=72° → +1°F = 73° → Predicted 72-73°
        BUT CLI settled at 74-75° (CLI was +2°F higher)

Dec 15: METAR=66° → +1°F = 67° → Predicted 67-68°
        BUT CLI settled at 65-66° (CLI was -1°F LOWER)
```

**Conclusion**: The +1°F calibration is an average, not a guarantee. Some days CLI is +2°F, some days -1°F.

## Trading Strategies

### Strategy 1: Thesis-Only (High Risk)

Put all capital on predicted bracket.

- **If correct (50%)**: ~$30-40 profit on $14
- **If wrong (50%)**: -$14 loss
- **EV**: ~$8-13 (marginally positive)
- **Risk**: Coin flip

### Strategy 2: 70/30 Hedge (Medium Risk)

- 70% on thesis bracket
- 30% on adjacent bracket (usually lower)

- **If thesis wins**: ~$15-25 profit
- **If adjacent wins**: ~-$5 to +$5
- **If neither wins**: -$14 loss
- **EV**: Break-even to small positive

### Strategy 3: Wide Hedge (Lower Risk)

Spread across 3+ brackets:
- 33% on market favorite
- 33% on model prediction
- 33% on protection bracket

- **Higher hit rate** (~70-80% coverage)
- **Lower per-trade profit** ($5-15)
- **Better for thesis testing**

### Strategy 4: Market Following

Bet on market favorite (highest probability bracket).

- **Pros**: Wisdom of crowds, no model needed
- **Cons**: Lower odds, smaller edge
- **EV**: Near zero (market is efficient)

## Recommended Approach

Given the model's 50% accuracy, we recommend:

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
│  This is DATA COLLECTION, not profit extraction.              │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

## What We Learned

### The Edge IS Real

1. Winning brackets start at 5-40¢
2. They settle at $1.00
3. First trades happen 7-9 AM PT the day before
4. There IS 60-95¢ of potential profit

### The Problem is Prediction

1. +1°F calibration only works ~50% of time
2. Weather is inherently variable
3. NWS forecasts themselves have error
4. Simple models can't beat sophisticated weather modeling

### Future Improvements Needed

1. **Better prediction model** - incorporate more weather variables
2. **Probabilistic approach** - assign probabilities to multiple brackets
3. **Ensemble methods** - combine multiple models
4. **Real-time adjustment** - update predictions as METAR data comes in

## Data Sources

| Source | Data | Used For |
|--------|------|----------|
| [Iowa State ASOS](https://mesonet.agron.iastate.edu/) | Historical METAR | Backtesting |
| [Aviation Weather Center](https://aviationweather.gov/) | Real-time METAR | Live monitoring |
| [NWS API](https://api.weather.gov/) | Forecasts | Predictions |
| Kalshi API | Trade history, prices | Validation |

## Files

| File | Purpose |
|------|---------|
| `cmd/lahigh-backtest-rigorous/main.go` | Rigorous backtest with real prediction simulation |
| `cmd/lahigh-backtest-real/main.go` | Real data backtesting with Kalshi trades |
| `cmd/lahigh-autorun/main.go` | Automated trading bot |
| `cmd/lahigh-predict-v2/main.go` | Prediction using NWS + METAR |
| `cmd/lahigh-status/main.go` | Check bot readiness |

## Running the Backtest

```bash
# Run the rigorous prediction-based backtest
go run ./cmd/lahigh-backtest-rigorous/

# Run the real trade data backtest (shows price evolution)
go run ./cmd/lahigh-backtest-real/
```

## Changelog

- **2025-12-26**: Added rigorous backtest with prediction simulation
- **2025-12-26**: **REVISED**: Model accuracy from 96% to 50%
- **2025-12-26**: Updated strategy recommendations to reflect reality
- **2025-12-26**: Documented prediction errors and their causes
- **2025-12-26**: Initial documentation based on 14-day real data backtest
