// Production dual-side trading bot with graceful shutdown
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brendanplayford/kalshi-go/cmd/dualside-bot/production/engine"
	"github.com/brendanplayford/kalshi-go/internal/config"
)

var (
	dryRun bool
)

func init() {
	flag.BoolVar(&dryRun, "dry-run", false, "Simulate trades without executing")
}

func main() {
	flag.Parse()

	printBanner()

	// Load Kalshi credentials using internal config
	kalshiCfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load Kalshi config: %v", err)
	}
	if err := kalshiCfg.Validate(); err != nil {
		log.Fatalf("Invalid Kalshi config: %v", err)
	}

	// Load production bot configuration
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("[Main] Configuration: %s", cfg)

	// Create data directory
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize executor with parsed private key
	executor, err := engine.NewExecutor(kalshiCfg.APIKey, kalshiCfg.PrivateKey, dryRun)
	if err != nil {
		log.Fatalf("Failed to initialize executor: %v", err)
	}

	// Get initial balance
	balance, err := executor.GetBalance()
	if err != nil {
		log.Fatalf("Failed to get balance: %v", err)
	}
	log.Printf("[Main] Account balance: $%.2f", balance)

	if dryRun {
		log.Println("[Main] ⚠️  DRY RUN MODE - No real trades will be executed")
	}

	// Create trading engine
	tradingEngine := engine.NewEngine(engine.TradingConfig{
		BetYes:           cfg.BetYes,
		BetNo:            cfg.BetNo,
		MinYesPrice:      cfg.MinYesPrice,
		MaxYesPrice:      cfg.MaxYesPrice,
		MinNoPrice:       cfg.MinNoPrice,
		MaxNoPrice:       cfg.MaxNoPrice,
		MaxNoTrades:      cfg.MaxNoTrades,
		TradingStartHour: cfg.TradingStartHour,
		TradingEndHour:   cfg.TradingEndHour,
	}, executor)

	// Set up trade callback
	tradingEngine.SetTradeCallback(func(trade engine.Trade) {
		log.Printf("[Trade] %s: %s %s %d @ %d¢ = $%.2f",
			trade.City, trade.Side, trade.Bracket, trade.Quantity, trade.Price, trade.Cost)
		// TODO: Send notification
	})

	// Set up error callback
	tradingEngine.SetErrorCallback(func(err error) {
		log.Printf("[Error] %v", err)
		// TODO: Send alert
	})

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start HTTP server for health checks
	httpServer := startHTTPServer(cfg.HTTPPort, tradingEngine)

	// Start trading engine in goroutine
	go tradingEngine.Run(ctx, time.Duration(cfg.PollInterval)*time.Second)

	log.Println("[Main] ✅ Bot is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[Main] Shutdown signal received...")

	// Graceful shutdown
	cancel()
	tradingEngine.Stop()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Main] HTTP server shutdown error: %v", err)
	}

	// Print final stats
	stats := tradingEngine.GetStats()
	log.Printf("[Main] Final stats: %d trades, $%.2f daily P&L",
		stats["total_trades"], stats["daily_pnl"])

	log.Println("[Main] Goodbye!")
}

func printBanner() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              PRODUCTION DUAL-SIDE TRADING BOT                               ║")
	fmt.Println("║              Autonomous Temperature Market Trading                          ║")
	fmt.Println("║              7 Markets • YES + NO Strategy • 95.8% Win Rate                 ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func startHTTPServer(port int, eng *engine.Engine) *http.Server {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// Stats endpoint
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := eng.GetStats()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"total_trades":%d,"yes_trades":%d,"no_trades":%d,"daily_pnl":%.2f,"open_positions":%d}`,
			stats["total_trades"],
			stats["yes_trades"],
			stats["no_trades"],
			stats["daily_pnl"],
			stats["open_positions"])
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		log.Printf("[HTTP] Server starting on :%d", port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[HTTP] Server error: %v", err)
		}
	}()

	return server
}

