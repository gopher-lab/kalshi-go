# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-12-26

### Added

#### LA High Temperature Trading System
- `cmd/lahigh-trader/` - Semi-automated trading bot for KXHIGHLAX market
- `cmd/lahigh-monitor/` - Real-time temperature monitoring with alerts
- `cmd/lahigh-backtest-validated/` - Backtest using actual Kalshi trade prices
- `cmd/lahigh-backtest-full/` - Comprehensive backtest with historical METAR data
- `cmd/lahigh-montecarlo/` - Monte Carlo simulation for strategy validation
- `cmd/lahigh-predict/` - Temperature prediction models (v1 and v2)

#### REST API Client
- `pkg/rest/client.go` - HTTP client with RSA-PSS authentication
- `pkg/rest/orders.go` - Order placement, cancellation, and management
- `pkg/rest/markets.go` - Market data, positions, and balance queries

#### WebSocket Client (Previous)
- `pkg/ws/` - Full WebSocket client for Kalshi streaming API
- RSA-PSS authentication
- Channel subscriptions (ticker, orderbook, trades, fills, positions)

### Validated Results

Based on 53 days of real Kalshi trade prices:

| Metric | Value |
|--------|-------|
| Days with big edge (<50¢ entry) | 71.7% |
| Average profit per trade | 65.7¢ |
| Best strategy Sharpe ratio | 85.0 |
| Win rate (on winning bracket) | 100% |

### Key Findings

1. Daily max temperature at LAX occurs 10-11 AM PT (62.5% of days)
2. METAR→CLI calibration: +1°F adjustment validated
3. Edge exists on 72% of trading days
4. "Only trade <50¢" strategy has Sharpe ratio of 85.0

## [0.1.0] - 2025-12-25

### Added
- Initial WebSocket client implementation
- RSA-PSS authentication for Kalshi API
- Basic bot framework (`cmd/kalshi-bot/`)
- Configuration handling (`internal/config/`)

