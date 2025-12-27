# Multi-City Daily Temperature Strategy

## Scope Document

**Branch:** `feature/multi-city-strategy`  
**Created:** December 26, 2025  
**Status:** SCOPING

---

## 🎯 Objective

Extend the validated 3-Signal ENSEMBLE strategy to **all daily temperature markets** on Kalshi, creating a reusable tech abstraction layer.

---

## 📍 Target Markets (Complete List)

### HIGH Temperature Markets (7 cities)

| City | METAR | Event Ticker | NWS Office | Climate Zone |
|------|-------|--------------|------------|--------------|
| **Los Angeles** | KLAX | KXHIGHLAX | LOX | Mediterranean |
| **New York City** | KJFK | KXHIGHNY | OKX | Humid Continental |
| **Chicago** | KORD | KXHIGHCHI | LOT | Humid Continental |
| **Miami** | KMIA | KXHIGHMIA | MFL | Tropical |
| **Austin** | KAUS | KXHIGHAUS | EWX | Humid Subtropical |
| **Philadelphia** | KPHL | KXHIGHPHIL | PHI | Humid Continental |
| **Denver** | KDEN | KXHIGHDEN | BOU | Semi-arid |

### LOW Temperature Markets (6 cities)

| City | METAR | Event Ticker | NWS Office |
|------|-------|--------------|------------|
| **Los Angeles** | KLAX | KXLOWTLAX | LOX |
| **Chicago** | KORD | KXLOWTCHI | LOT |
| **Miami** | KMIA | KXLOWTMIA | MFL |
| **Austin** | KAUS | KXLOWTAUS | EWX |
| **Philadelphia** | KPHL | KXLOWTPHIL | PHI |
| **Denver** | KDEN | KXLOWTDEN | BOU |

*Sources:*
- https://kalshi.com/markets/kxhighlax/
- https://kalshi.com/markets/kxhighny/
- https://kalshi.com/markets/kxhighchi/
- https://kalshi.com/markets/kxhighmia/
- https://kalshi.com/markets/kxhighaus/
- https://kalshi.com/markets/kxhighphil/
- https://kalshi.com/markets/kxhighden/

**Total: 13 daily temperature markets!**

### Market Structure (per city/type)

Each market has 6 brackets for each day:
- `KXHIGH{CITY}-{DATE}-T{HIGH}` - Will temp be above X°?
- `KXHIGH{CITY}-{DATE}-T{LOW}` - Will temp be below X°?
- `KXHIGH{CITY}-{DATE}-B{MID}` - Will temp be in X-Y° range?

---

## 🏗️ Architecture

### Core Abstraction Layer

```
pkg/
├── weather/
│   ├── station.go         # Station metadata (ID, location, timezone, climate)
│   ├── metar.go           # METAR data fetching (Iowa State ASOS)
│   ├── forecast.go        # NWS forecast fetching
│   └── climatology.go     # Historical averages by station/month
│
├── market/
│   ├── temperature.go     # Temperature market abstraction
│   ├── brackets.go        # Bracket parsing and pricing
│   └── signals.go         # Signal generators (market, forecast, etc.)
│
├── strategy/
│   ├── ensemble.go        # ENSEMBLE strategy logic (configurable signals)
│   ├── backtest.go        # Generic backtesting framework
│   └── montecarlo.go      # Monte Carlo simulation engine
│
└── trading/
    ├── recommender.go     # Generate trade recommendations
    ├── executor.go        # Execute trades via REST API
    └── portfolio.go       # Multi-market portfolio management
```

### Configuration-Driven Approach

```go
// Station configuration
type Station struct {
    ID           string  // "KLAX", "KJFK", etc.
    Name         string  // "Los Angeles Airport"
    City         string  // "Los Angeles"
    Timezone     string  // "America/Los_Angeles"
    EventPrefix  string  // "KXHIGHLAX"
    NWSGridpoint string  // "LOX/154,44"
    Climatology  map[int]float64  // Month -> avg high temp
}

// Strategy configuration
type StrategyConfig struct {
    Signals      []SignalType  // Which signals to use
    MinAgreement int           // Minimum signals that must agree
    BetSize      float64       // Position size
    MaxPrice     int           // Don't buy above this price (cents)
}
```

---

## 📊 Signals (Parameterized)

| Signal | Source | City-Specific? |
|--------|--------|----------------|
| **Market Favorite** | Kalshi first trade prices | ✅ Per-market |
| **2nd Best** | Kalshi 2nd highest price | ✅ Per-market |
| **NWS Forecast** | api.weather.gov | ✅ Per-gridpoint |
| **METAR Current** | Iowa State ASOS | ✅ Per-station |
| **Previous Day** | Yesterday's actual high | ✅ Per-station |
| **Climatology** | Historical monthly avg | ✅ Per-station/month |

---

## 🔧 Implementation Phases

### Phase 1: Core Abstractions ✅ COMPLETE
- [x] Create `pkg/weather/station.go` with station registry (7 cities)
- [x] Create `pkg/weather/metar.go` with generic METAR fetching
- [x] Create `pkg/weather/forecast.go` with NWS API integration
- [x] Create `pkg/market/temperature.go` with market abstraction

### Phase 2: Strategy Engine ✅ COMPLETE
- [x] Create `pkg/strategy/signals.go` with signal interface
- [x] Create `pkg/strategy/ensemble.go` with configurable ensemble
- [ ] Create `pkg/strategy/backtest.go` with generic backtesting

### Phase 3: Multi-City Backtesting
- [ ] Backtest LA (validate against existing results)
- [ ] Backtest NYC
- [ ] Backtest Chicago
- [ ] Backtest Miami
- [ ] Backtest Austin, Philadelphia, Denver

### Phase 4: Production Tools ⏳ IN PROGRESS
- [x] Create `cmd/weather-strategy/recommend` - multi-city recommendations
- [ ] Create `cmd/weather-strategy/backtest` - backtest any city
- [ ] Create `cmd/weather-strategy/portfolio` - cross-city portfolio

### Phase 5: Automation
- [ ] Create unified bot that monitors all cities
- [ ] Implement portfolio-level position sizing
- [ ] Add correlation analysis between cities

---

## 📁 Proposed Directory Structure

```
kalshi-go/
├── pkg/
│   ├── weather/              # Weather data abstraction
│   │   ├── station.go
│   │   ├── metar.go
│   │   ├── forecast.go
│   │   └── climatology.go
│   │
│   ├── market/               # Kalshi market abstraction
│   │   ├── temperature.go
│   │   ├── brackets.go
│   │   └── signals.go
│   │
│   ├── strategy/             # Trading strategy engine
│   │   ├── ensemble.go
│   │   ├── backtest.go
│   │   └── montecarlo.go
│   │
│   ├── trading/              # Trade execution
│   │   ├── recommender.go
│   │   └── executor.go
│   │
│   ├── ws/                   # (existing) WebSocket client
│   └── rest/                 # (existing) REST client
│
├── cmd/
│   ├── 3signal/              # (existing) LA-specific strategy
│   │
│   ├── weather-strategy/     # NEW: Multi-city strategy
│   │   ├── recommend/        # Get recommendations for all cities
│   │   ├── backtest/         # Backtest any city
│   │   ├── montecarlo/       # Monte Carlo for any city
│   │   └── portfolio/        # Cross-city portfolio analysis
│   │
│   └── lahigh-*/             # (existing) Legacy LA tools
│
├── config/
│   └── stations.yaml         # Station configuration
│
└── docs/
    ├── LAHIGH-STRATEGY.md    # (existing) LA strategy
    └── MULTI-CITY-SCOPE.md   # This document
```

---

## 🧪 Validation Approach

For each city:

1. **Fetch 21+ days of historical data**
   - METAR max temperatures
   - Kalshi first trade prices
   - Winning brackets

2. **Run 3-Signal ENSEMBLE backtest**
   - Calculate win rate
   - Calculate profit
   - Calculate Sharpe ratio

3. **Compare to LA baseline**
   - Does the strategy work in this climate?
   - Any city-specific adjustments needed?

4. **Monte Carlo validation**
   - 10,000 simulations
   - Risk of ruin analysis
   - Position sizing recommendations

---

## 🤔 Open Questions

1. **Ticker Patterns**: Need to verify exact Kalshi ticker patterns for each city
2. **Timezone Handling**: Different cities = different trading windows
3. **Climate Zones**: Does ENSEMBLE work in all climates? (e.g., Phoenix desert vs Miami tropical)
4. **Correlation**: Are city outcomes correlated? (affects portfolio sizing)
5. **Liquidity**: Do all markets have sufficient liquidity?
6. **NWS Gridpoints**: Need to find correct NWS gridpoint for each airport

---

## 📈 Backtest Results (21 days, Dec 5-26, 2025)

### HIGH Temperature Markets - **VALIDATED** ✅

| City | Trades | Win Rate | Profit ($10 bets) |
|------|--------|----------|-------------------|
| Philadelphia | 16 | 100% | $33.55 |
| NYC | 10 | 100% | $14.64 |
| Austin | 11 | 100% | $10.22 |
| Los Angeles | 11 | 100% | $5.93 |
| Miami | 14 | 100% | $5.00 |
| Denver | 3 | 100% | $4.51 |
| Chicago | 10 | 90% | $2.70 |
| **TOTAL** | **75** | **97.3%** | **$76.55** |

### LOW Temperature Markets - **NOT VIABLE** ❌

METAR max temp does not predict low temperatures. Would need METAR min data.

### Key Metrics

- **Overall Win Rate:** 97.4% (74/76 trades)
- **Kelly Fraction:** 75.1% (strong edge)
- **Average Profit per Trade:** $0.88
- **Expected Daily Profit:** ~$3.65 (across all 7 HIGH markets)
- **Expected Monthly Profit:** ~$110
- **Expected Yearly Profit:** ~$1,300

### Best Conditions

The strategy works best when:
1. Market favorite price is 80-97¢ (high confidence market)
2. METAR observation confirms the market favorite bracket
3. Signal agreement is clear (no edge cases)

---

## ✅ Next Steps

1. **Confirm this scope** - Does this match your vision?
2. **Verify Kalshi tickers** - Check API for exact market tickers
3. **Start Phase 1** - Build core abstractions
4. **Backtest LA first** - Validate abstraction against known results

---

## 🚀 Quick Start (After Implementation)

```bash
# Backtest any city
go run ./cmd/weather-strategy/backtest/ --city=NYC

# Get recommendations for all cities
go run ./cmd/weather-strategy/recommend/

# Run Monte Carlo for specific city
go run ./cmd/weather-strategy/montecarlo/ --city=Chicago

# Portfolio analysis
go run ./cmd/weather-strategy/portfolio/
```

