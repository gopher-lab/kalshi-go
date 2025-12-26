// Package main provides the entry point for the Kalshi trading bot.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/brendanplayford/kalshi-go/internal/config"
	"github.com/brendanplayford/kalshi-go/pkg/ws"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run() error {
	// Load configuration from environment.
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Build WebSocket client options.
	opts := []ws.Option{
		ws.WithCallbacks(
			func() { log.Println("connected to Kalshi WebSocket") },
			func(err error) { log.Printf("disconnected: %v", err) },
			func(err error) { log.Printf("error: %v", err) },
		),
	}

	// Add authentication if credentials are available.
	if cfg.IsAuthenticated() {
		opts = append(opts, ws.WithAPIKeyOption(cfg.APIKey, cfg.PrivateKey))
		log.Println("using authenticated connection")
	} else {
		log.Println("using unauthenticated connection (public channels only)")
	}

	// Override base URL if configured.
	if cfg.BaseURL != "" {
		opts = append(opts, ws.WithBaseURLOption(cfg.BaseURL))
	}

	// Create WebSocket client.
	client := ws.New(opts...)

	// Set up message handler.
	client.SetMessageHandler(func(msg *ws.Response) {
		log.Printf("received: type=%s id=%d sid=%d", msg.Type, msg.ID, msg.SID)
	})

	// Connect to WebSocket.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	log.Println("kalshi-bot started")

	// Wait for interrupt signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")
	return nil
}

