// Package main provides a "set and forget" trading bot for LA High Temperature.
// Start it now and it will trade tomorrow's market automatically.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/brendanplayford/kalshi-go/internal/config"
	"github.com/brendanplayford/kalshi-go/pkg/rest"
)

// Configuration
var (
	maxPositionSize  = 10   // Max contracts per position
	maxRiskCents     = 5000 // Max $50 at risk
	minEdge          = 0.05 // 5% minimum edge
	maxEntryPrice    = 80   // Don't buy above 80¢
	cliCalibration   = 1.0  // METAR to CLI adjustment
	pollInterval     = 30 * time.Second
	tradingStartHour = 7  // 7 AM PT - start trading
	tradingEndHour   = 12 // 12 PM PT - stop adding positions
)

type MarketState struct {
	Ticker    string
	Strike    string
	LowBound  int
	HighBound int
	YesBid    int
	YesAsk    int
	ModelProb float64
	Edge      float64
}

func main() {
	// Parse flags
	eventTicker := flag.String("event", "", "Event ticker (e.g., KXHIGHLAX-25DEC27)")
	maxRisk := flag.Int("max-risk", 50, "Maximum risk per trade in dollars")
	dryRun := flag.Bool("dry-run", false, "Simulate trades without executing")
	flag.Parse()

	if *eventTicker == "" {
		// Auto-detect tomorrow's market
		tomorrow := time.Now().AddDate(0, 0, 1)
		*eventTicker = fmt.Sprintf("KXHIGHLAX-%s", strings.ToUpper(tomorrow.Format("06Jan02")))
	}

	maxRiskCents = *maxRisk * 100

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("🤖 LA HIGH TEMPERATURE - AUTO TRADING BOT")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Printf("📅 Target Market: %s\n", *eventTicker)
	fmt.Printf("💵 Max Risk: $%d per trade\n", *maxRisk)
	fmt.Printf("📈 Min Edge: %.0f%%\n", minEdge*100)
	fmt.Printf("💰 Max Entry Price: %d¢\n", maxEntryPrice)
	fmt.Printf("⏱️  Poll Interval: %v\n", pollInterval)
	if *dryRun {
		fmt.Println("🧪 DRY RUN MODE - No real trades")
	}
	fmt.Println()

	// Load config
	cfg, err := config.Load()
	if err != nil || !cfg.IsAuthenticated() {
		fmt.Println("❌ No Kalshi credentials found")
		os.Exit(1)
	}

	// Connect to Kalshi
	client := rest.New(cfg.APIKey, cfg.PrivateKey)

	// Check balance
	balance, err := client.GetBalance()
	if err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Connected! Balance: $%.2f\n", float64(balance.Balance)/100)

	if balance.Balance == 0 && !*dryRun {
		fmt.Println("⚠️  Warning: $0 balance - add funds to trade")
	}

	// Get markets
	markets, err := client.GetMarkets(*eventTicker)
	if err != nil || len(markets) == 0 {
		fmt.Printf("❌ No markets found for %s\n", *eventTicker)
		os.Exit(1)
	}
	fmt.Printf("✅ Found %d brackets\n", len(markets))
	fmt.Println()

	// Parse target date from event ticker
	targetDate := parseTargetDate(*eventTicker)
	fmt.Printf("📆 Target Date: %s\n", targetDate.Format("Monday, January 2, 2006"))
	fmt.Println()

	// State
	var tradedBrackets = make(map[string]bool)
	var totalPnL float64

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("🚀 BOT RUNNING - Will trade when edge is detected")
	fmt.Println("   Press Ctrl+C to stop")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	// Initial check
	checkAndTrade(client, *eventTicker, targetDate, tradedBrackets, &totalPnL, *dryRun, balance.Balance)

	for {
		select {
		case <-ticker.C:
			checkAndTrade(client, *eventTicker, targetDate, tradedBrackets, &totalPnL, *dryRun, balance.Balance)

		case <-sigCh:
			fmt.Println("\n→ Shutting down...")
			fmt.Println()
			fmt.Println(strings.Repeat("=", 70))
			fmt.Println("SESSION SUMMARY")
			fmt.Println(strings.Repeat("=", 70))
			fmt.Printf("Brackets traded: %d\n", len(tradedBrackets))
			fmt.Printf("Estimated P&L: $%.2f (pending settlement)\n", totalPnL)
			return
		}
	}
}

func checkAndTrade(client *rest.Client, eventTicker string, targetDate time.Time, tradedBrackets map[string]bool, totalPnL *float64, dryRun bool, balance int) {
	la, _ := time.LoadLocation("America/Los_Angeles")
	now := time.Now().In(la)

	// Determine if we're trading for today or a future day
	isTargetToday := now.Format("2006-01-02") == targetDate.Format("2006-01-02")

	// Determine trading window status
	var tradingStatus string
	var canTrade bool

	if !isTargetToday {
		// Target is in the future - monitor only
		targetStart := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(),
			tradingStartHour, 0, 0, 0, la)
		until := targetStart.Sub(now).Round(time.Minute)
		tradingStatus = fmt.Sprintf("⏳ WAITING - Trading starts in %v", until)
		canTrade = false
	} else if now.Hour() < tradingStartHour {
		// Before trading window
		minsUntil := (tradingStartHour-now.Hour())*60 - now.Minute()
		tradingStatus = fmt.Sprintf("⏳ WAITING - Trading starts in %d minutes", minsUntil)
		canTrade = false
	} else if now.Hour() >= tradingEndHour {
		// After trading window
		tradingStatus = "🔒 HOLDING - Trading window closed (after 12 PM)"
		canTrade = false
	} else {
		// In trading window!
		minsLeft := (tradingEndHour-now.Hour())*60 - now.Minute()
		tradingStatus = fmt.Sprintf("🟢 TRADING WINDOW ACTIVE (%d min remaining)", minsLeft)
		canTrade = true
	}

	// Get expected temperature
	var expectedCLI int
	var source string

	if isTargetToday {
		// Use METAR running max for today
		metar := fetchMETAR()
		expectedCLI = metar + int(cliCalibration)
		source = fmt.Sprintf("METAR running max: %d°F", metar)
	} else {
		// Use NWS forecast for future days
		forecast := fetchNWSForecast(targetDate)
		expectedCLI = forecast + int(cliCalibration)
		source = fmt.Sprintf("NWS forecast: %d°F", forecast)
	}

	// Get current market prices
	markets, err := client.GetMarkets(eventTicker)
	if err != nil {
		fmt.Printf("[%s] ⚠️ Failed to fetch markets\n", now.Format("15:04:05"))
		return
	}

	// Calculate probabilities and find opportunities
	var states []MarketState
	stdDev := 2.0

	// Adjust uncertainty based on time until target
	hoursUntil := targetDate.Sub(now).Hours()
	if hoursUntil < 0 {
		hoursUntil = 0
	}
	if hoursUntil > 24 {
		stdDev = 3.0 // More uncertainty for future days
	} else if hoursUntil < 6 {
		stdDev = 1.5 // Less uncertainty as we approach
	}

	for _, m := range markets {
		if m.YesSubTitle == "" {
			continue
		}

		low, high := parseBracket(m.YesSubTitle)
		prob := calculateProb(low, high, float64(expectedCLI), stdDev)

		edge := prob - float64(m.YesAsk)/100

		states = append(states, MarketState{
			Ticker:    m.Ticker,
			Strike:    m.YesSubTitle,
			LowBound:  low,
			HighBound: high,
			YesBid:    m.YesBid,
			YesAsk:    m.YesAsk,
			ModelProb: prob,
			Edge:      edge,
		})
	}

	// Sort by edge
	sort.Slice(states, func(i, j int) bool {
		return states[i].Edge > states[j].Edge
	})

	// Print status
	fmt.Printf("[%s] %s\n", now.Format("15:04:05"), tradingStatus)
	fmt.Printf("         Expected CLI: %d°F (%s) | StdDev: %.1f\n", expectedCLI, source, stdDev)

	// Show best opportunity
	var bestOpp *MarketState
	for i := range states {
		s := &states[i]
		if !tradedBrackets[s.Ticker] && s.Edge >= minEdge && s.YesAsk <= maxEntryPrice && s.YesAsk > 0 {
			bestOpp = s
			break
		}
	}

	if bestOpp != nil {
		fmt.Printf("         Best opportunity: %s @ %d¢ (Edge: +%.0f%%)\n",
			bestOpp.Strike, bestOpp.YesAsk, bestOpp.Edge*100)
	}

	// Only execute trades during trading window
	if !canTrade {
		if bestOpp != nil {
			fmt.Printf("         → Will trade when window opens\n")
		}
		fmt.Println()
		return
	}

	// Find best opportunity and trade
	for _, s := range states {
		if tradedBrackets[s.Ticker] {
			continue // Already traded this bracket
		}

		if s.Edge >= minEdge && s.YesAsk <= maxEntryPrice && s.YesAsk > 0 {
			fmt.Println()
			fmt.Println(strings.Repeat("!", 70))
			fmt.Printf("🎯 OPPORTUNITY: %s @ %d¢\n", s.Strike, s.YesAsk)
			fmt.Printf("   Our Probability: %.0f%%\n", s.ModelProb*100)
			fmt.Printf("   Market Price: %d¢\n", s.YesAsk)
			fmt.Printf("   Edge: +%.0f%%\n", s.Edge*100)

			// Calculate position size
			contracts := maxRiskCents / s.YesAsk
			if contracts > maxPositionSize {
				contracts = maxPositionSize
			}
			if contracts > balance/s.YesAsk && !dryRun {
				contracts = balance / s.YesAsk
			}

			if contracts == 0 {
				fmt.Println("   ⚠️ Insufficient balance")
				fmt.Println(strings.Repeat("!", 70))
				continue
			}

			cost := contracts * s.YesAsk
			fmt.Printf("   Contracts: %d @ %d¢ = $%.2f\n", contracts, s.YesAsk, float64(cost)/100)

			if dryRun {
				fmt.Println("   🧪 DRY RUN - Would execute trade")
				tradedBrackets[s.Ticker] = true
				*totalPnL += float64(100-s.YesAsk) * float64(contracts) * 0.93 / 100 // Estimated profit
			} else {
				// Execute trade
				order, err := client.BuyYes(s.Ticker, contracts, s.YesAsk)
				if err != nil {
					fmt.Printf("   ❌ Order failed: %v\n", err)
				} else {
					fmt.Printf("   ✅ ORDER PLACED! ID: %s\n", order.OrderID)
					tradedBrackets[s.Ticker] = true
					*totalPnL += float64(100-s.YesAsk) * float64(contracts) * 0.93 / 100
				}
			}
			fmt.Println(strings.Repeat("!", 70))
			fmt.Println()
			break // Only trade one bracket per tick
		}
	}
}

func fetchMETAR() int {
	resp, err := http.Get("https://aviationweather.gov/api/data/metar?ids=KLAX&hours=24&format=json")
	if err != nil {
		return 60 // Default
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var obs []struct {
		Temp float64 `json:"temp"`
	}
	json.Unmarshal(body, &obs)

	// Find max
	var maxTemp float64
	for _, o := range obs {
		tempF := o.Temp*9/5 + 32
		if tempF > maxTemp {
			maxTemp = tempF
		}
	}

	return int(maxTemp)
}

func fetchNWSForecast(targetDate time.Time) int {
	resp, err := http.Get("https://api.weather.gov/gridpoints/LOX/154,44/forecast")
	if err != nil {
		return 62 // Default
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var forecast struct {
		Properties struct {
			Periods []struct {
				Name        string `json:"name"`
				Temperature int    `json:"temperature"`
				IsDaytime   bool   `json:"isDaytime"`
			} `json:"periods"`
		} `json:"properties"`
	}
	json.Unmarshal(body, &forecast)

	// Find the forecast for target date
	targetStr := strings.ToLower(targetDate.Format("Monday"))
	for _, p := range forecast.Properties.Periods {
		if p.IsDaytime && strings.Contains(strings.ToLower(p.Name), targetStr) {
			return p.Temperature
		}
	}

	// Fallback to first daytime
	for _, p := range forecast.Properties.Periods {
		if p.IsDaytime {
			return p.Temperature
		}
	}

	return 62
}

func parseBracket(bracket string) (int, int) {
	// "64° or above" -> 64, 999
	// "55° or below" -> 0, 55
	// "62° to 63°" -> 62, 63
	if strings.Contains(bracket, "above") {
		var low int
		fmt.Sscanf(bracket, "%d°", &low)
		return low, 999
	}
	if strings.Contains(bracket, "below") {
		var high int
		fmt.Sscanf(bracket, "%d°", &high)
		return 0, high
	}
	var low, high int
	fmt.Sscanf(bracket, "%d° to %d°", &low, &high)
	return low, high
}

func calculateProb(low, high int, expected, stdDev float64) float64 {
	if high == 999 {
		return 1 - normalCDF(float64(low)-0.5, expected, stdDev)
	}
	if low == 0 {
		return normalCDF(float64(high)+0.5, expected, stdDev)
	}
	return normalCDF(float64(high)+0.5, expected, stdDev) - normalCDF(float64(low)-0.5, expected, stdDev)
}

func normalCDF(x, mean, stdDev float64) float64 {
	return 0.5 * (1 + math.Erf((x-mean)/(stdDev*math.Sqrt2)))
}

func parseTargetDate(ticker string) time.Time {
	// KXHIGHLAX-25DEC27 -> 2025-12-27
	parts := strings.Split(ticker, "-")
	if len(parts) < 2 {
		return time.Now().AddDate(0, 0, 1)
	}

	dateStr := parts[1]
	if len(dateStr) < 7 {
		return time.Now().AddDate(0, 0, 1)
	}

	year := 2000 + int(dateStr[0]-'0')*10 + int(dateStr[1]-'0')
	monthStr := dateStr[2:5]
	day := int(dateStr[5]-'0')*10 + int(dateStr[6]-'0')

	months := map[string]time.Month{
		"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
		"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
	}

	return time.Date(year, months[monthStr], day, 0, 0, 0, 0, time.UTC)
}
