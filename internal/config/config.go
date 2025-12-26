// Package config provides configuration handling for the Kalshi trading bot.
package config

import (
	"errors"
	"os"
)

var (
	// ErrMissingAPIKey is returned when the API key is not configured.
	ErrMissingAPIKey = errors.New("config: KALSHI_API_KEY environment variable not set")

	// ErrMissingPrivateKey is returned when the private key is not configured.
	ErrMissingPrivateKey = errors.New("config: KALSHI_PRIVATE_KEY environment variable not set")
)

// Config holds the application configuration.
type Config struct {
	// APIKey is the Kalshi API key.
	APIKey string

	// PrivateKey is the Kalshi private key for signing requests.
	PrivateKey string

	// BaseURL is the WebSocket base URL (optional, uses default if empty).
	BaseURL string

	// Debug enables debug logging.
	Debug bool
}

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		APIKey:     os.Getenv("KALSHI_API_KEY"),
		PrivateKey: os.Getenv("KALSHI_PRIVATE_KEY"),
		BaseURL:    os.Getenv("KALSHI_WS_URL"),
		Debug:      os.Getenv("KALSHI_DEBUG") == "true",
	}

	return cfg, nil
}

// Validate checks that required configuration is present.
// For authenticated operations, both APIKey and PrivateKey are required.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return ErrMissingAPIKey
	}
	if c.PrivateKey == "" {
		return ErrMissingPrivateKey
	}
	return nil
}

// IsAuthenticated returns true if authentication credentials are configured.
func (c *Config) IsAuthenticated() bool {
	return c.APIKey != "" && c.PrivateKey != ""
}

