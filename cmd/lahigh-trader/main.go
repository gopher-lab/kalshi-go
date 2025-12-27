// Package main provides an automated trading bot for the LA High Temperature market.
// It monitors weather data and places trades when conditions are met.
package main

import (
	"bufio"
	"context"
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
	"github.com/brendanplayford/kalshi-go/pkg/ws"
)

// Configuration
var (
	maxPositionSize = 10               // Max contracts per position
	maxRiskCents    = 5000             // Max $50 at risk per trade
	minEdge         = 0.05             // Minimum 5% edge to trade
	cliCalibration  = 1.0              // METAR to CLI adjustment
	pollInterval    = 30 * time.Second // Fast polling for price changes
)

// Trading state
type TradingState struct {
	// Weather
	CurrentTempF      int
	RunningMaxF       int
	ExpectedMaxF      int
	NWSForecastF      int
	LastWeatherUpdate time.Time

	// Market
	Markets   map[string]*MarketState
	Positions map[string]*rest.Position
	Balance   int // cents

	// Trading
	PendingOrders map[string]*rest.Order
	ExecutedToday int
}

type MarketState struct {
	Ticker    string
	Strike    string
	LowBound  int
	HighBound int
	YesBid    int
	YesAsk    int
	NoBid     int
	NoAsk     int
	LastPrice int
	ModelProb float64
	Edge      float64
	Signal    string
	Crossed   bool
	CrossedAt time.Time
}

// METAR observation
type METARObservation struct {
	IcaoID   string  `json:"icaoId"`
	ObsTime  int64   `json:"obsTime"`
	Temp     float64 `json:"temp"`
	WxString string  `json:"wxString"`
}

// NWS Forecast
type NWSForecast struct {
	Properties struct {
		Periods []struct {
			Name        string `json:"name"`
			Temperature int    `json:"temperature"`
			IsDaytime   bool   `json:"isDaytime"`
		} `json:"periods"`
	} `json:"properties"`
}

const (
	metarAPIURL    = "https://aviationweather.gov/api/data/metar?ids=KLAX&hours=3&format=json"
	nwsForecastURL = "https://api.weather.gov/gridpoints/LOX/154,44/forecast"
)

func main() {
	// Parse flags
	eventTicker := flag.String("event", "KXHIGHLAX-25DEC27", "Event ticker")
	demo := flag.Bool("demo", false, "Use demo environment (no real money)")
	autoTrade := flag.Bool("auto", false, "Enable auto-trading (default: manual confirmation)")
	maxRisk := flag.Int("max-risk", 50, "Maximum risk per trade in dollars")
	maxContracts := flag.Int("max-contracts", 10, "Maximum contracts per position")
	pollSecs := flag.Int("poll", 30, "Polling interval in seconds (default: 30)")
	flag.Parse()

	pollInterval = time.Duration(*pollSecs) * time.Second

	maxRiskCents = *maxRisk * 100
	maxPositionSize = *maxContracts

	// Header
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("🤖 LA HIGH TEMPERATURE - AUTOMATED TRADER")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if !cfg.IsAuthenticated() {
		fmt.Println("❌ No Kalshi credentials found in .env file")
		fmt.Println("   Please set KALSHI_API_KEY and KALSHI_PRIVATE_KEY")
		os.Exit(1)
	}

	// Create REST client
	var restOpts []rest.Option
	if *demo {
		restOpts = append(restOpts, rest.WithDemo())
		fmt.Println("🧪 DEMO MODE - No real money at risk")
	} else {
		fmt.Println("💰 PRODUCTION MODE - Real money trading")
	}

	if *autoTrade {
		fmt.Println("🤖 AUTO-TRADE ENABLED - Orders will be placed automatically")
	} else {
		fmt.Println("👤 MANUAL MODE - You will confirm each trade")
	}

	fmt.Printf("💵 Max Risk: $%d per trade\n", *maxRisk)
	fmt.Printf("📊 Max Contracts: %d per position\n", *maxContracts)
	fmt.Printf("📈 Min Edge: %.0f%%\n", minEdge*100)
	fmt.Printf("⏱️  Poll Interval: %v\n", pollInterval)
	fmt.Println()

	client := rest.New(cfg.APIKey, cfg.PrivateKey, restOpts...)

	// Initialize state
	state := &TradingState{
		Markets:       make(map[string]*MarketState),
		Positions:     make(map[string]*rest.Position),
		PendingOrders: make(map[string]*rest.Order),
	}

	// Verify connection and get balance
	fmt.Println("→ Connecting to Kalshi...")
	balance, err := client.GetBalance()
	if err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		os.Exit(1)
	}
	state.Balance = balance.Balance
	fmt.Printf("✓ Connected! Balance: $%.2f\n", float64(balance.Balance)/100)
	fmt.Println()

	// Fetch markets for the event
	fmt.Printf("→ Fetching markets for %s...\n", *eventTicker)
	markets, err := client.GetMarkets(*eventTicker)
	if err != nil {
		fmt.Printf("❌ Failed to fetch markets: %v\n", err)
		os.Exit(1)
	}

	if len(markets) == 0 {
		fmt.Printf("❌ No markets found for event %s\n", *eventTicker)
		fmt.Println("   Try a different event ticker, e.g., KXHIGHLAX-25DEC27")
		os.Exit(1)
	}

	fmt.Printf("✓ Found %d markets\n", len(markets))

	// Initialize market states
	for _, m := range markets {
		strike := m.YesSubTitle
		if strike == "" {
			strike = parseStrike(m.Title, m.Subtitle)
		}
		low, high := parseStrikeBoundsFromAPI(m.FloorStrike, m.CapStrike, strike)
		state.Markets[m.Ticker] = &MarketState{
			Ticker:    m.Ticker,
			Strike:    strike,
			LowBound:  low,
			HighBound: high,
			YesBid:    m.YesBid,
			YesAsk:    m.YesAsk,
			NoBid:     m.NoBid,
			NoAsk:     m.NoAsk,
			LastPrice: m.LastPrice,
		}
		fmt.Printf("  📊 %s: %s (Bid: %d¢, Ask: %d¢)\n", m.Ticker, strike, m.YesBid, m.YesAsk)
	}
	fmt.Println()

	// Initial weather update
	updateWeather(state)
	updateMarketProbabilities(state)
	printStatus(state, client)

	// Set up WebSocket for real-time market updates
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		wsClient := ws.New(
			ws.WithAPIKeyOption(cfg.APIKey, cfg.PrivateKey),
		)

		if err := wsClient.Connect(ctx); err != nil {
			fmt.Printf("⚠ WebSocket connection failed: %v\n", err)
			return
		}
		defer wsClient.Close()

		// Subscribe to all market tickers
		for ticker := range state.Markets {
			wsClient.Subscribe(ctx, ticker, ws.ChannelTicker)
		}
	}()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Polling loop
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	fmt.Println()
	fmt.Println("📡 Trading bot started. Press Ctrl+C to stop.")
	fmt.Println(strings.Repeat("=", 80))

	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-ticker.C:
			// Update weather
			prevMax := state.RunningMaxF
			updateWeather(state)

			// Refresh market prices
			refreshMarketPrices(state, client, *eventTicker)
			updateMarketProbabilities(state)

			// Check for threshold crossings
			checkThresholds(state, prevMax)

			// Look for trading opportunities
			opportunities := findOpportunities(state)

			if len(opportunities) > 0 {
				printOpportunities(opportunities)

				if *autoTrade {
					for _, opp := range opportunities {
						executeTrade(client, state, opp)
					}
				} else {
					for _, opp := range opportunities {
						fmt.Printf("\n🔔 TRADING OPPORTUNITY: %s\n", opp.Description)
						fmt.Printf("   Execute trade? (y/n): ")

						input, _ := reader.ReadString('\n')
						input = strings.TrimSpace(strings.ToLower(input))

						if input == "y" || input == "yes" {
							executeTrade(client, state, opp)
						} else {
							fmt.Println("   Skipped.")
						}
					}
				}
			}

			printUpdate(state)

		case <-sigCh:
			fmt.Println("\n→ Shutting down...")
			printFinalSummary(state, client)
			return
		}
	}
}

func updateWeather(state *TradingState) {
	loc, _ := time.LoadLocation("America/Los_Angeles")

	// Fetch latest METAR
	resp, err := http.Get(metarAPIURL)
	if err != nil {
		fmt.Printf("⚠ METAR fetch failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var observations []METARObservation
	json.Unmarshal(body, &observations)

	if len(observations) > 0 {
		obs := observations[0]
		tempF := int((obs.Temp * 9.0 / 5.0) + 32.5)
		state.CurrentTempF = tempF
		state.LastWeatherUpdate = time.Unix(obs.ObsTime, 0).In(loc)

		if tempF > state.RunningMaxF {
			state.RunningMaxF = tempF
		}
	}

	// Fetch NWS forecast
	resp2, err := http.Get(nwsForecastURL)
	if err == nil {
		defer resp2.Body.Close()
		body2, _ := io.ReadAll(resp2.Body)
		var forecast NWSForecast
		if json.Unmarshal(body2, &forecast) == nil {
			for _, period := range forecast.Properties.Periods {
				if period.IsDaytime {
					state.NWSForecastF = period.Temperature
					break
				}
			}
		}
	}

	// Expected CLI
	state.ExpectedMaxF = int(math.Max(float64(state.RunningMaxF), float64(state.NWSForecastF)) + cliCalibration)
}

func updateMarketProbabilities(state *TradingState) {
	expectedCLI := float64(state.ExpectedMaxF)
	stdDev := 2.0

	// Adjust uncertainty based on time of day
	loc, _ := time.LoadLocation("America/Los_Angeles")
	hour := time.Now().In(loc).Hour()

	switch {
	case hour >= 20:
		stdDev = 0.5
	case hour >= 18:
		stdDev = 1.0
	case hour >= 16:
		stdDev = 1.5
	}

	for _, m := range state.Markets {
		var prob float64
		if m.HighBound >= 999 {
			prob = 1 - normalCDF(float64(m.LowBound)-0.5, expectedCLI, stdDev)
		} else if m.LowBound <= 0 {
			prob = normalCDF(float64(m.HighBound)+0.5, expectedCLI, stdDev)
		} else {
			prob = normalCDF(float64(m.HighBound)+0.5, expectedCLI, stdDev) -
				normalCDF(float64(m.LowBound)-0.5, expectedCLI, stdDev)
		}
		m.ModelProb = prob

		// Calculate edge vs market
		if m.YesAsk > 0 {
			impliedProb := float64(m.YesAsk) / 100.0
			m.Edge = prob - impliedProb
		}

		// Determine signal
		if m.Edge > minEdge {
			m.Signal = "🟢 BUY YES"
		} else if m.Edge < -minEdge {
			m.Signal = "🔴 BUY NO"
		} else {
			m.Signal = "⚪ HOLD"
		}

		// Check if threshold crossed
		cliMax := state.RunningMaxF + int(cliCalibration)
		if !m.Crossed && cliMax > m.LowBound {
			m.Crossed = true
			m.CrossedAt = time.Now()
		}
	}
}

func refreshMarketPrices(state *TradingState, client *rest.Client, eventTicker string) {
	markets, err := client.GetMarkets(eventTicker)
	if err != nil {
		return
	}

	for _, m := range markets {
		if ms, ok := state.Markets[m.Ticker]; ok {
			ms.YesBid = m.YesBid
			ms.YesAsk = m.YesAsk
			ms.NoBid = m.NoBid
			ms.NoAsk = m.NoAsk
			ms.LastPrice = m.LastPrice
		}
	}
}

func checkThresholds(state *TradingState, prevMax int) {
	cliMax := state.RunningMaxF + int(cliCalibration)
	prevCLI := prevMax + int(cliCalibration)

	for _, m := range state.Markets {
		if prevCLI <= m.LowBound && cliMax > m.LowBound {
			fmt.Println()
			fmt.Println(strings.Repeat("!", 80))
			fmt.Printf("🚨 THRESHOLD CROSSED: %d°F (CLI) > %s strike!\n", cliMax, m.Strike)
			fmt.Printf("   → %s is now LOCKED IN for YES\n", m.Strike)
			fmt.Println(strings.Repeat("!", 80))
		}
	}
}

type Opportunity struct {
	Ticker      string
	Strike      string
	Action      string // "BUY_YES" or "BUY_NO"
	Side        rest.Side
	Price       int // in cents
	Contracts   int
	Edge        float64
	Description string
	Confidence  string
}

func findOpportunities(state *TradingState) []Opportunity {
	var opps []Opportunity

	for _, m := range state.Markets {
		// Skip if already crossed (YES is locked)
		if m.Crossed && m.Edge > 0 {
			continue
		}

		absEdge := math.Abs(m.Edge)
		if absEdge < minEdge {
			continue
		}

		var opp Opportunity
		opp.Ticker = m.Ticker
		opp.Strike = m.Strike
		opp.Edge = m.Edge

		if m.Edge > 0 {
			// BUY YES
			opp.Action = "BUY_YES"
			opp.Side = rest.SideYes
			opp.Price = m.YesAsk
			if opp.Price == 0 {
				continue
			}
			opp.Contracts = calculatePosition(opp.Price, state.Balance)
			opp.Description = fmt.Sprintf("BUY YES on \"%s\" @ %d¢ (Edge: +%.0f%%)",
				m.Strike, opp.Price, m.Edge*100)
		} else {
			// BUY NO
			opp.Action = "BUY_NO"
			opp.Side = rest.SideNo
			opp.Price = m.NoAsk
			if opp.Price == 0 {
				continue
			}
			opp.Contracts = calculatePosition(opp.Price, state.Balance)
			opp.Description = fmt.Sprintf("BUY NO on \"%s\" @ %d¢ (Edge: +%.0f%%)",
				m.Strike, opp.Price, absEdge*100)
		}

		if opp.Contracts > 0 {
			// Confidence level
			switch {
			case absEdge > 0.20:
				opp.Confidence = "HIGH"
			case absEdge > 0.10:
				opp.Confidence = "MEDIUM"
			default:
				opp.Confidence = "LOW"
			}
			opps = append(opps, opp)
		}
	}

	return opps
}

func calculatePosition(priceCents, balanceCents int) int {
	// Max contracts based on risk
	maxByRisk := maxRiskCents / priceCents

	// Max contracts based on position size
	contracts := maxPositionSize
	if maxByRisk < contracts {
		contracts = maxByRisk
	}

	// Max based on balance
	maxByBalance := balanceCents / priceCents
	if maxByBalance < contracts {
		contracts = maxByBalance
	}

	return contracts
}

func executeTrade(client *rest.Client, state *TradingState, opp Opportunity) {
	fmt.Printf("\n→ Executing: %s\n", opp.Description)
	fmt.Printf("  Contracts: %d @ %d¢ = $%.2f\n", opp.Contracts, opp.Price,
		float64(opp.Contracts*opp.Price)/100)

	var order *rest.Order
	var err error

	if opp.Side == rest.SideYes {
		order, err = client.BuyYes(opp.Ticker, opp.Contracts, opp.Price)
	} else {
		order, err = client.BuyNo(opp.Ticker, opp.Contracts, opp.Price)
	}

	if err != nil {
		fmt.Printf("  ❌ Order failed: %v\n", err)
		return
	}

	fmt.Printf("  ✅ Order placed! ID: %s\n", order.OrderID)
	fmt.Printf("     Status: %s\n", order.Status)

	state.PendingOrders[order.OrderID] = order
	state.ExecutedToday++
}

func printStatus(state *TradingState, client *rest.Client) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	now := time.Now().In(loc)

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("CURRENT STATUS")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("📅 %s\n", now.Format("Monday, January 2, 2006 3:04 PM MST"))
	fmt.Println()

	fmt.Println("WEATHER:")
	fmt.Printf("  🌡️  Current: %d°F\n", state.CurrentTempF)
	fmt.Printf("  📈 Running Max: %d°F (METAR) → %d°F (Est. CLI)\n",
		state.RunningMaxF, state.RunningMaxF+int(cliCalibration))
	fmt.Printf("  🌤️  NWS Forecast: %d°F\n", state.NWSForecastF)
	fmt.Printf("  🎯 Expected CLI: %d°F\n", state.ExpectedMaxF)
	fmt.Println()

	fmt.Println("MARKETS:")
	fmt.Printf("%-18s %-8s %-8s %-10s %-8s %-12s\n",
		"Strike", "Bid", "Ask", "Model", "Edge", "Signal")
	fmt.Printf("%-18s %-8s %-8s %-10s %-8s %-12s\n",
		"------", "---", "---", "-----", "----", "------")

	for _, m := range getSortedMarkets(state) {
		edgeStr := fmt.Sprintf("%+.0f%%", m.Edge*100)
		fmt.Printf("%-18s %-8d %-8d %-10.0f%% %-8s %-12s\n",
			m.Strike, m.YesBid, m.YesAsk, m.ModelProb*100, edgeStr, m.Signal)
	}
	fmt.Println()

	// Show existing positions
	positions, err := client.GetPositions()
	if err == nil && len(positions) > 0 {
		fmt.Println("POSITIONS:")
		for _, p := range positions {
			if p.YesPosition > 0 || p.NoPosition > 0 {
				fmt.Printf("  %s: YES=%d, NO=%d, Cost=$%.2f\n",
					p.Ticker, p.YesPosition, p.NoPosition, float64(p.TotalCost)/100)
			}
		}
		fmt.Println()
	}
}

func printOpportunities(opps []Opportunity) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("🎯 TRADING OPPORTUNITIES")
	fmt.Println(strings.Repeat("=", 80))

	for _, opp := range opps {
		fmt.Printf("\n%s\n", opp.Description)
		fmt.Printf("  Contracts: %d\n", opp.Contracts)
		fmt.Printf("  Confidence: %s\n", opp.Confidence)
	}
}

func printUpdate(state *TradingState) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	now := time.Now().In(loc)

	// Find best opportunity
	var bestEdge float64
	var bestStrike string
	for _, m := range state.Markets {
		if math.Abs(m.Edge) > math.Abs(bestEdge) {
			bestEdge = m.Edge
			bestStrike = m.Strike
		}
	}

	fmt.Printf("[%s] Temp: %d°F | Max: %d°F | Expected: %d°F | Best: %s (%+.0f%%)\n",
		now.Format("15:04"),
		state.CurrentTempF,
		state.RunningMaxF,
		state.ExpectedMaxF,
		bestStrike,
		bestEdge*100)
}

func printFinalSummary(state *TradingState, client *rest.Client) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("TRADING SESSION SUMMARY")
	fmt.Println(strings.Repeat("=", 80))

	fmt.Printf("📊 Orders Executed: %d\n", state.ExecutedToday)

	// Get final balance
	balance, err := client.GetBalance()
	if err == nil {
		fmt.Printf("💰 Current Balance: $%.2f\n", float64(balance.Balance)/100)
	}

	// Show positions
	positions, err := client.GetPositions()
	if err == nil && len(positions) > 0 {
		fmt.Println("\n📈 Open Positions:")
		for _, p := range positions {
			if p.YesPosition > 0 || p.NoPosition > 0 {
				fmt.Printf("  %s\n", p.Ticker)
				fmt.Printf("    YES: %d, NO: %d\n", p.YesPosition, p.NoPosition)
				fmt.Printf("    Cost: $%.2f, Realized P&L: $%.2f\n",
					float64(p.TotalCost)/100, float64(p.RealizedPnl)/100)
			}
		}
	}
	fmt.Println()
}

func getSortedMarkets(state *TradingState) []*MarketState {
	result := make([]*MarketState, 0, len(state.Markets))
	for _, m := range state.Markets {
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LowBound < result[j].LowBound
	})
	return result
}

func parseStrike(title, subtitle string) string {
	// Try to extract strike from title/subtitle
	combined := title + " " + subtitle

	// Common patterns: "56-57", "55 or below", "64 or above"
	if strings.Contains(combined, "below") {
		return "55 or below"
	}
	if strings.Contains(combined, "above") || strings.Contains(combined, "higher") {
		return "64 or above"
	}

	// Look for number ranges
	for _, s := range []string{"56-57", "58-59", "60-61", "62-63"} {
		if strings.Contains(combined, s) {
			return s
		}
	}

	return subtitle
}

func parseStrikeBounds(strike string) (int, int) {
	switch strike {
	case "55 or below", "55° or below":
		return 0, 55
	case "56-57", "56° to 57°":
		return 56, 57
	case "58-59", "58° to 59°":
		return 58, 59
	case "60-61", "60° to 61°":
		return 60, 61
	case "62-63", "62° to 63°":
		return 62, 63
	case "64 or above", "64° or above":
		return 64, 999
	default:
		return 0, 999
	}
}

func parseStrikeBoundsFromAPI(floor, cap float64, strike string) (int, int) {
	// Use API floor/cap if available
	if floor > 0 || cap > 0 {
		low := int(floor)
		high := int(cap)
		if high == 0 {
			high = 999 // "X or above"
		}
		if low == 0 && high > 0 {
			high-- // "X or below" means <= high-1
		}
		return low, high
	}
	return parseStrikeBounds(strike)
}

func normalCDF(x, mean, stdDev float64) float64 {
	return 0.5 * (1 + math.Erf((x-mean)/(stdDev*math.Sqrt2)))
}
