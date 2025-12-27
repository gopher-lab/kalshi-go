# Production Dual-Side Trading Bot

Autonomous, always-on trading bot for temperature markets with optimized dual-side strategy.

## Features

- ✅ **Autonomous Operation** - Runs 24/7 without intervention
- ✅ **Graceful Shutdown** - Clean exit on SIGTERM/SIGINT
- ✅ **Health Checks** - HTTP endpoints for monitoring
- ✅ **Retry Logic** - Automatic retry on order failures
- ✅ **Optimized Parameters** - 95.8% win rate, $97K/year projected
- ✅ **Docker Ready** - One-command deployment

## Quick Start

### Local Development

```bash
# Set environment variables
export KALSHI_API_KEY=your-key
export KALSHI_PRIVATE_KEY='-----BEGIN RSA PRIVATE KEY-----...'

# Dry run (no real trades)
go run ./cmd/dualside-bot/production --dry-run

# Live trading
go run ./cmd/dualside-bot/production
```

### Docker Deployment

```bash
# Navigate to production folder
cd cmd/dualside-bot/production

# Create .env file with credentials
cat > .env << EOF
KALSHI_API_KEY=your-key
KALSHI_PRIVATE_KEY=your-private-key-single-line
EOF

# Start the bot
docker-compose up -d

# Check logs
docker-compose logs -f

# Check health
curl http://localhost:8080/health

# Check stats
curl http://localhost:8080/stats

# Stop
docker-compose down
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `KALSHI_API_KEY` | Required | Kalshi API key |
| `KALSHI_PRIVATE_KEY` | Required | RSA private key |
| `BET_YES` | $500 | YES trade size |
| `BET_NO` | $150 | Each NO trade size |
| `MIN_YES_PRICE` | 50¢ | Minimum YES price to trade |
| `MAX_YES_PRICE` | 95¢ | Maximum YES price to trade |
| `MIN_NO_PRICE` | 40¢ | Minimum NO price to trade |
| `MAX_NO_PRICE` | 95¢ | Maximum NO price to trade |
| `MAX_NO_TRADES` | 4 | Max NO trades per event |
| `TRADING_START_HOUR` | 7 | Start hour (local time) |
| `TRADING_END_HOUR` | 14 | End hour (local time) |
| `POLL_INTERVAL` | 60 | Polling interval (seconds) |
| `HTTP_PORT` | 8080 | Health check port |

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check (returns 200 if running) |
| `GET /stats` | Trading statistics JSON |

### Example `/stats` Response

```json
{
  "total_trades": 12,
  "yes_trades": 4,
  "no_trades": 8,
  "daily_pnl": 245.50,
  "open_positions": 2
}
```

## Strategy

### Dual-Side Trading
When signals agree on bracket X winning:
1. **BUY YES** on bracket X ($500 @ 50-95¢)
2. **BUY NO** on losing brackets ($150 each @ 40-95¢)

### Signal Agreement
Trade only when:
- Market favorite bracket == METAR temperature bracket
- YES price in 50-95¢ range

### Markets
- Los Angeles (LAX)
- New York (JFK)
- Chicago (ORD)
- Miami (MIA)
- Austin (AUS)
- Philadelphia (PHL)
- Denver (DEN)

## Backtest Results

| Metric | Value |
|--------|-------|
| Win Rate | 95.8% |
| Trades (21 days) | 48 |
| Profit | $5,635 |
| Sharpe Ratio | 10.02 |
| Annual Projection | $97,938 |

## Monitoring

### Docker Health Check
The container includes a health check that pings `/health` every 30 seconds.
If unhealthy, Docker will restart the container.

### Log Output
```
[Main] Configuration: Config{BetYes:$500, BetNo:$150, ...}
[Main] Account balance: $1000.00
[Engine] Starting trading engine...
[Engine] Tick at 09:15:00
[Engine] LAX: Fav=60-61°@55¢ METAR=61°→60-61° Agree=true
[Trade] Los Angeles: yes 60-61° 909 @ 55¢ = $500.00
[Trade] Los Angeles: no 62-63° 205 @ 73¢ = $150.00
```

## Deployment Options

### Option 1: Local Docker
```bash
docker-compose up -d
```

### Option 2: Cloud VM (DigitalOcean $6/mo)
```bash
# SSH to droplet
ssh root@your-droplet

# Clone and run
git clone https://github.com/gopher-lab/kalshi-go.git
cd kalshi-go/cmd/dualside-bot/production
docker-compose up -d
```

### Option 3: Fly.io
```bash
fly launch
fly secrets set KALSHI_API_KEY=xxx KALSHI_PRIVATE_KEY=xxx
fly deploy
```

## Troubleshooting

### Bot not trading?
1. Check trading window (7 AM - 2 PM local time)
2. Check signal agreement in logs
3. Check account balance

### API errors?
1. Verify API key and private key
2. Check Kalshi account status
3. Review retry logs

### Container restarting?
1. Check logs: `docker-compose logs`
2. Check health: `curl localhost:8080/health`
3. Verify environment variables

