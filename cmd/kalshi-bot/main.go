// Package main provides the entry point for the Kalshi trading bot.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brendanplayford/kalshi-go/internal/config"
	"github.com/brendanplayford/kalshi-go/pkg/ws"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run() error {
	// Parse flags.
	marketTicker := flag.String("market", "", "Market ticker to subscribe to (e.g., KXBTC-25DEC31-T50000)")
	channel := flag.String("channel", "ticker", "Channel to subscribe to (ticker, orderbook_delta, trade)")
	flag.Parse()

	// Load configuration from environment/.env file.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Build WebSocket client options.
	opts := []ws.Option{
		ws.WithCallbacks(
			func() { log.Println("✓ connected to Kalshi WebSocket") },
			func(err error) { log.Printf("✗ disconnected: %v", err) },
			func(err error) { log.Printf("⚠ error: %v", err) },
		),
	}

	// Add authentication if credentials are available.
	if cfg.IsAuthenticated() {
		opts = append(opts, ws.WithAPIKeyOption(cfg.APIKey, cfg.PrivateKey))
		log.Println("→ using authenticated connection")
	} else {
		log.Println("→ using unauthenticated connection (public channels only)")
	}

	// Override base URL if configured.
	if cfg.BaseURL != "" {
		opts = append(opts, ws.WithBaseURLOption(cfg.BaseURL))
	}

	// Create WebSocket client.
	client := ws.New(opts...)

	// Set up message handler.
	client.SetMessageHandler(func(msg *ws.Response) {
		data, _ := json.MarshalIndent(msg, "", "  ")
		log.Printf("← received:\n%s", data)
	})

	// Connect to WebSocket.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("→ connecting to Kalshi...")
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	log.Println("✓ kalshi-bot ready")

	// Subscribe if a market ticker was provided.
	if *marketTicker != "" {
		ch := ws.Channel(*channel)
		if !ch.IsValid() {
			return fmt.Errorf("invalid channel: %s", *channel)
		}

		log.Printf("→ subscribing to %s on %s...", ch, *marketTicker)

		// Small delay to ensure connection is stable.
		time.Sleep(500 * time.Millisecond)

		id, err := client.Subscribe(ctx, *marketTicker, ch)
		if err != nil {
			return fmt.Errorf("subscribe: %w", err)
		}
		log.Printf("→ subscription request sent (id=%d)", id)
	}

	// Wait for interrupt signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("→ shutting down...")
	return nil
}
