// Package config provides configuration handling for the Kalshi trading bot.
package config

import (
	"bufio"
	"crypto/rsa"
	"errors"
	"os"
	"strings"

	"github.com/brendanplayford/kalshi-go/pkg/ws"
)

var (
	// ErrMissingAPIKey is returned when the API key is not configured.
	ErrMissingAPIKey = errors.New("config: KALSHI_API_KEY not set")

	// ErrMissingPrivateKey is returned when the private key is not configured.
	ErrMissingPrivateKey = errors.New("config: KALSHI_PRIVATE_KEY not set")

	// ErrInvalidPrivateKey is returned when the private key cannot be parsed.
	ErrInvalidPrivateKey = errors.New("config: failed to parse private key")
)

// Config holds the application configuration.
type Config struct {
	// APIKey is the Kalshi API key ID.
	APIKey string

	// PrivateKeyPEM is the raw PEM-encoded private key.
	PrivateKeyPEM string

	// PrivateKey is the parsed RSA private key.
	PrivateKey *rsa.PrivateKey

	// BaseURL is the WebSocket base URL (optional, uses default if empty).
	BaseURL string

	// Debug enables debug logging.
	Debug bool
}

// Load loads configuration from environment variables.
// It first attempts to load from a .env file if present.
func Load() (*Config, error) {
	// Try to load .env file with multiline support.
	envVars := loadEnvFile(".env")

	// Get values from env file or environment.
	getEnv := func(key string) string {
		if val, ok := envVars[key]; ok {
			return val
		}
		return os.Getenv(key)
	}

	cfg := &Config{
		APIKey:        getEnv("KALSHI_API_KEY"),
		PrivateKeyPEM: getEnv("KALSHI_PRIVATE_KEY"),
		BaseURL:       getEnv("KALSHI_WS_URL"),
		Debug:         getEnv("KALSHI_DEBUG") == "true",
	}

	// Parse the private key if provided.
	if cfg.PrivateKeyPEM != "" {
		key, err := ws.ParsePrivateKeyString(cfg.PrivateKeyPEM)
		if err != nil {
			return nil, errors.Join(ErrInvalidPrivateKey, err)
		}
		cfg.PrivateKey = key
	}

	return cfg, nil
}

// loadEnvFile reads a .env file with support for multiline values.
// Multiline values are detected when a line starts with a key= and the value
// spans multiple lines (like PEM-encoded keys).
func loadEnvFile(path string) map[string]string {
	result := make(map[string]string)

	file, err := os.Open(path)
	if err != nil {
		return result
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentKey string
	var currentValue strings.Builder
	inMultiline := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments when not in multiline.
		if !inMultiline && (line == "" || strings.HasPrefix(line, "#")) {
			continue
		}

		// Check if this is a new key=value pair.
		if !inMultiline && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := ""
			if len(parts) > 1 {
				value = parts[1]
			}

			// Check if this starts a multiline value (PEM key).
			if strings.Contains(value, "-----BEGIN") {
				currentKey = key
				currentValue.Reset()
				currentValue.WriteString(value)
				currentValue.WriteString("\n")
				inMultiline = true
				continue
			}

			// Single line value.
			result[key] = value
			continue
		}

		// We're in a multiline value.
		if inMultiline {
			currentValue.WriteString(line)
			currentValue.WriteString("\n")

			// Check if this ends the multiline value.
			if strings.Contains(line, "-----END") {
				result[currentKey] = strings.TrimSuffix(currentValue.String(), "\n")
				inMultiline = false
				currentKey = ""
				currentValue.Reset()
			}
		}
	}

	return result
}

// Validate checks that required configuration is present.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return ErrMissingAPIKey
	}
	if c.PrivateKey == nil {
		return ErrMissingPrivateKey
	}
	return nil
}

// IsAuthenticated returns true if authentication credentials are configured.
func (c *Config) IsAuthenticated() bool {
	return c.APIKey != "" && c.PrivateKey != nil
}
