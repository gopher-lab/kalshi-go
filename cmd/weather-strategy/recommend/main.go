// Multi-city weather strategy recommendation tool
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/brendanplayford/kalshi-go/internal/config"
	"github.com/brendanplayford/kalshi-go/pkg/market"
	"github.com/brendanplayford/kalshi-go/pkg/rest"
	"github.com/brendanplayford/kalshi-go/pkg/strategy"
	"github.com/brendanplayford/kalshi-go/pkg/weather"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     MULTI-CITY WEATHER STRATEGY - RECOMMENDATIONS                ║")
	fmt.Println("║     3-Signal Ensemble Strategy                                   ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	// Initialize Kalshi client
	client := rest.New(cfg.APIKey, cfg.PrivateKey)

	// Get tomorrow's date
	tomorrow := time.Now().AddDate(0, 0, 1)
	fmt.Printf("📅 Analyzing markets for: %s\n\n", tomorrow.Format("Monday, January 2, 2006"))

	// Create ensemble strategy
	ensemble := strategy.NewEnsemble()

	// Track recommendations
	type CityResult struct {
		Station *weather.Station
		Type    weather.MarketType
		Result  *strategy.EnsembleResult
		Error   error
	}

	var results []CityResult

	// Analyze each city
	for code, station := range weather.Stations {
		fmt.Printf("🏙️  Analyzing %s (%s)...\n", station.City, code)

		// Try HIGH temperature market
		eventTickerHigh := station.HighEventTicker(tomorrow)
		tmHigh, err := market.FetchTempMarket(client, station, weather.MarketTypeHigh, tomorrow)
		if err != nil {
			fmt.Printf("   ❌ HIGH: No market found (%s)\n", eventTickerHigh)
		} else {
			result, err := ensemble.Analyze(station, weather.MarketTypeHigh, tomorrow, tmHigh)
			results = append(results, CityResult{
				Station: station,
				Type:    weather.MarketTypeHigh,
				Result:  result,
				Error:   err,
			})
			if result != nil && result.Recommendation != nil {
				rec := result.Recommendation
				if rec.Action == "BUY" {
					fmt.Printf("   ✅ HIGH: %s @ %d¢ (%s)\n", rec.Bracket, rec.Price, rec.Reason)
				} else {
					fmt.Printf("   ⚪ HIGH: %s\n", rec.Reason)
				}
			}
		}

		// Try LOW temperature market
		eventTickerLow := station.LowEventTicker(tomorrow)
		tmLow, err := market.FetchTempMarket(client, station, weather.MarketTypeLow, tomorrow)
		if err != nil {
			fmt.Printf("   ❌ LOW:  No market found (%s)\n", eventTickerLow)
		} else {
			result, err := ensemble.Analyze(station, weather.MarketTypeLow, tomorrow, tmLow)
			results = append(results, CityResult{
				Station: station,
				Type:    weather.MarketTypeLow,
				Result:  result,
				Error:   err,
			})
			if result != nil && result.Recommendation != nil {
				rec := result.Recommendation
				if rec.Action == "BUY" {
					fmt.Printf("   ✅ LOW:  %s @ %d¢ (%s)\n", rec.Bracket, rec.Price, rec.Reason)
				} else {
					fmt.Printf("   ⚪ LOW:  %s\n", rec.Reason)
				}
			}
		}

		fmt.Println()
	}

	// Summary of actionable trades
	fmt.Println("════════════════════════════════════════════════════════════════════")
	fmt.Println("📊 ACTIONABLE TRADES")
	fmt.Println("════════════════════════════════════════════════════════════════════")

	actionCount := 0
	totalCost := 0.0
	potentialProfit := 0.0

	for _, r := range results {
		if r.Result == nil || r.Result.Recommendation == nil {
			continue
		}
		rec := r.Result.Recommendation
		if rec.Action != "BUY" {
			continue
		}

		actionCount++
		cost := float64(rec.Price*rec.Quantity) / 100
		profit := float64((100-rec.Price)*rec.Quantity) / 100

		totalCost += cost
		potentialProfit += profit

		typeStr := "HIGH"
		if r.Type == weather.MarketTypeLow {
			typeStr = "LOW"
		}

		fmt.Printf("\n%d. %s %s Temperature\n", actionCount, r.Station.City, typeStr)
		fmt.Printf("   Bracket: %s\n", rec.Bracket)
		fmt.Printf("   Ticker:  %s\n", rec.Ticker)
		fmt.Printf("   Price:   %d¢ (BUY YES)\n", rec.Price)
		fmt.Printf("   Qty:     %d contracts\n", rec.Quantity)
		fmt.Printf("   Cost:    $%.2f\n", cost)
		fmt.Printf("   Profit:  $%.2f (if wins)\n", profit)
		fmt.Printf("   Edge:    %.1f%%\n", rec.ExpectedEdge)
		fmt.Printf("   Reason:  %s\n", rec.Reason)

		// Show signal breakdown
		fmt.Printf("   Signals:\n")
		for _, sig := range r.Result.Signals {
			match := "❌"
			if sig.Bracket == rec.Bracket {
				match = "✅"
			}
			fmt.Printf("     %s %s: %s (%.0f°F)\n", match, sig.Name, sig.Bracket, sig.Temperature)
		}
	}

	if actionCount == 0 {
		fmt.Println("\n⚠️  No trades recommended at this time.")
		fmt.Println("   Signals are not in agreement or prices are out of range.")
	} else {
		fmt.Printf("\n────────────────────────────────────────────────────────────────────\n")
		fmt.Printf("TOTAL: %d trades | Cost: $%.2f | Potential Profit: $%.2f\n",
			actionCount, totalCost, potentialProfit)
	}

	fmt.Println()

	// Check current balance
	balanceResp, err := client.GetBalance()
	if err != nil {
		fmt.Printf("⚠️  Could not fetch balance: %v\n", err)
	} else {
		balance := float64(balanceResp.Balance) / 100.0
		fmt.Printf("💰 Account Balance: $%.2f\n", balance)
		if totalCost > 0 && balance >= totalCost {
			fmt.Printf("✅ Sufficient funds for all trades\n")
		} else if totalCost > 0 {
			fmt.Printf("⚠️  Insufficient funds (need $%.2f more)\n", totalCost-balance)
		}
	}

	os.Exit(0)
}

