// Package main provides a quick status check for the trading bot.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/brendanplayford/kalshi-go/internal/config"
	"github.com/brendanplayford/kalshi-go/pkg/rest"
)

func main() {
	fmt.Println(strings.Repeat("=", 65))
	fmt.Println("🔍 LA HIGH TEMPERATURE BOT - READINESS CHECK")
	fmt.Println(strings.Repeat("=", 65))
	fmt.Println()

	allGood := true

	// Check 1: Config
	fmt.Print("1. API Credentials............ ")
	cfg, err := config.Load()
	if err != nil || !cfg.IsAuthenticated() {
		fmt.Println("❌ Not configured")
		fmt.Println("   → Create .env file with KALSHI_API_KEY and KALSHI_PRIVATE_KEY")
		return
	}
	fmt.Println("✅ Loaded from .env")

	// Check 2: REST API Connection
	fmt.Print("2. Kalshi API Connection...... ")
	client := rest.New(cfg.APIKey, cfg.PrivateKey)
	balance, err := client.GetBalance()
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		return
	}
	fmt.Println("✅ Connected")

	// Check 3: Balance
	fmt.Print("3. Account Balance............ ")
	if balance.Balance == 0 {
		fmt.Printf("⚠️  $0.00 - NEED TO ADD FUNDS\n")
		allGood = false
	} else {
		fmt.Printf("✅ $%.2f available\n", float64(balance.Balance)/100)
	}

	// Check 4: Tomorrow's market
	fmt.Print("4. Dec 27 Market Status....... ")
	markets, err := client.GetMarkets("KXHIGHLAX-25DEC27")
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		allGood = false
	} else if len(markets) == 0 {
		fmt.Println("❌ No markets found")
		allGood = false
	} else {
		fmt.Printf("✅ %d brackets available\n", len(markets))
	}

	// Check 5: METAR API
	fmt.Print("5. Weather Data (METAR)....... ")
	resp, err := http.Get("https://aviationweather.gov/api/data/metar?ids=KLAX&format=json")
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		allGood = false
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var obs []struct {
			Temp float64 `json:"temp"`
		}
		json.Unmarshal(body, &obs)
		if len(obs) > 0 {
			tempF := int(obs[0].Temp*9/5 + 32)
			fmt.Printf("✅ Current LAX temp: %d°F\n", tempF)
		} else {
			fmt.Println("⚠️  No data")
		}
	}

	// Check 6: Show market prices
	fmt.Println()
	fmt.Println("📊 CURRENT MARKET PRICES (Dec 27):")
	fmt.Printf("   %-15s %8s %8s\n", "Bracket", "YES Bid", "YES Ask")
	fmt.Printf("   %-15s %8s %8s\n", "-------", "-------", "-------")
	for _, m := range markets {
		if m.YesSubTitle != "" {
			fmt.Printf("   %-15s %7d¢ %7d¢\n", m.YesSubTitle, m.YesBid, m.YesAsk)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 65))

	if !allGood {
		fmt.Println("⚠️  ACTION REQUIRED:")
		if balance.Balance == 0 {
			fmt.Println("   → Add funds: https://kalshi.com/portfolio/funds")
			fmt.Println("   → Recommended: At least $50 to start")
		}
	} else {
		fmt.Println("✅ BOT IS READY TO TRADE!")
		fmt.Println()
		fmt.Println("📅 Run tomorrow (Dec 27) starting at 7 AM PT:")
		fmt.Println()
		fmt.Println("   # Monitor only (you trade manually):")
		fmt.Println("   go run ./cmd/lahigh-monitor/")
		fmt.Println()
		fmt.Println("   # Semi-auto (bot suggests, you confirm):")
		fmt.Println("   go run ./cmd/lahigh-trader/ -event KXHIGHLAX-25DEC27")
		fmt.Println()
		fmt.Println("   # Full auto (bot trades for you):")
		fmt.Println("   go run ./cmd/lahigh-trader/ -event KXHIGHLAX-25DEC27 -auto")
	}
	fmt.Println(strings.Repeat("=", 65))
}
