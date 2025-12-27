package main

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all production bot configuration
type Config struct {
	// Trading Parameters (optimized defaults from backtest)
	BetYes      float64
	BetNo       float64
	MinYesPrice int
	MaxYesPrice int
	MinNoPrice  int
	MaxNoPrice  int
	MaxNoTrades int

	// Trading Window
	TradingStartHour int
	TradingEndHour   int

	// Polling (fallback when WS unavailable)
	PollInterval int // seconds

	// Notifications
	SlackWebhookURL   string
	DiscordWebhookURL string

	// Server
	HTTPPort int
	LogLevel string

	// Persistence
	DataDir string
}

// DefaultConfig returns optimized defaults from backtest
func DefaultConfig() *Config {
	return &Config{
		// Trading Parameters (from optimizer backtest)
		BetYes:      500,
		BetNo:       150,
		MinYesPrice: 50,
		MaxYesPrice: 95,
		MinNoPrice:  40,
		MaxNoPrice:  95,
		MaxNoTrades: 4,

		// Trading Window (local time)
		TradingStartHour: 7,
		TradingEndHour:   14,

		// Polling
		PollInterval: 60, // 1 minute

		// Server
		HTTPPort: 8080,
		LogLevel: "info",

		// Persistence
		DataDir: "./data",
	}
}

// LoadConfig loads configuration from environment with defaults
// Note: Kalshi API credentials are loaded separately via internal/config
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	// Optional overrides
	if v := os.Getenv("BET_YES"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.BetYes = f
		}
	}
	if v := os.Getenv("BET_NO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.BetNo = f
		}
	}
	if v := os.Getenv("MIN_YES_PRICE"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.MinYesPrice = i
		}
	}
	if v := os.Getenv("MAX_YES_PRICE"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.MaxYesPrice = i
		}
	}
	if v := os.Getenv("MIN_NO_PRICE"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.MinNoPrice = i
		}
	}
	if v := os.Getenv("MAX_NO_PRICE"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.MaxNoPrice = i
		}
	}
	if v := os.Getenv("MAX_NO_TRADES"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.MaxNoTrades = i
		}
	}
	if v := os.Getenv("TRADING_START_HOUR"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.TradingStartHour = i
		}
	}
	if v := os.Getenv("TRADING_END_HOUR"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.TradingEndHour = i
		}
	}
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.PollInterval = i
		}
	}
	if v := os.Getenv("SLACK_WEBHOOK_URL"); v != "" {
		cfg.SlackWebhookURL = v
	}
	if v := os.Getenv("DISCORD_WEBHOOK_URL"); v != "" {
		cfg.DiscordWebhookURL = v
	}
	if v := os.Getenv("HTTP_PORT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.HTTPPort = i
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	return cfg, nil
}

// String returns a safe string representation (no secrets)
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{BetYes:$%.0f, BetNo:$%.0f, YesRange:%d-%d¢, NoRange:%d-%d¢, MaxNo:%d, Window:%d-%d, Port:%d}",
		c.BetYes, c.BetNo,
		c.MinYesPrice, c.MaxYesPrice,
		c.MinNoPrice, c.MaxNoPrice,
		c.MaxNoTrades,
		c.TradingStartHour, c.TradingEndHour,
		c.HTTPPort,
	)
}

