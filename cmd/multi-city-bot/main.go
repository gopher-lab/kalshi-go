// Package main provides an automated multi-city temperature trading bot
// using the 3-Signal ENSEMBLE strategy across 7 HIGH temperature markets.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/brendanplayford/kalshi-go/internal/config"
	"github.com/brendanplayford/kalshi-go/pkg/rest"
)

// Configuration
var (
	betSize       float64
	dryRun        bool
	pollInterval  time.Duration
	tradingWindow struct {
		startHour int
		endHour   int
	}
)

// Station configuration
type Station struct {
	Code       string
	City       string
	METAR      string
	EventPrefix string
	Timezone   string
}

var Stations = []Station{
	{"LAX", "Los Angeles", "LAX", "KXHIGHLAX", "America/Los_Angeles"},
	{"NYC", "New York", "JFK", "KXHIGHNY", "America/New_York"},
	{"CHI", "Chicago", "ORD", "KXHIGHCHI", "America/Chicago"},
	{"MIA", "Miami", "MIA", "KXHIGHMIA", "America/New_York"},
	{"AUS", "Austin", "AUS", "KXHIGHAUS", "America/Chicago"},
	{"PHIL", "Philadelphia", "PHL", "KXHIGHPHIL", "America/New_York"},
	{"DEN", "Denver", "DEN", "KXHIGHDEN", "America/Denver"},
}

// Market data structures
type Market struct {
	Ticker      string  `json:"ticker"`
	EventTicker string  `json:"event_ticker"`
	FloorStrike int     `json:"floor_strike"`
	CapStrike   int     `json:"cap_strike"`
	Status      string  `json:"status"`
	YesBid      float64 `json:"yes_bid"`
	YesAsk      float64 `json:"yes_ask"`
	Volume      int     `json:"volume"`
}

type MarketsResponse struct {
	Markets []Market `json:"markets"`
}

// Trade tracking
type TradeRecord struct {
	Timestamp   time.Time
	City        string
	EventTicker string
	Bracket     string
	Ticker      string
	Side        string
	Price       int
	Quantity    int
	Cost        float64
	OrderID     string
}

type BotState struct {
	StartTime      time.Time
	TotalTrades    int
	OpenPositions  map[string]TradeRecord
	ClosedTrades   []TradeRecord
	TotalProfit    float64
	CurrentBalance float64
}

var (
	client     *rest.Client
	httpClient = &http.Client{Timeout: 15 * time.Second}
	state      BotState
)

func init() {
	flag.Float64Var(&betSize, "bet", 50, "Bet size per trade in dollars")
	flag.BoolVar(&dryRun, "dry-run", false, "Simulate trades without executing")
	flag.DurationVar(&pollInterval, "interval", 5*time.Minute, "Polling interval")
	
	tradingWindow.startHour = 7  // 7 AM local
	tradingWindow.endHour = 14   // 2 PM local
}

func main() {
	flag.Parse()
	
	printBanner()
	
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}
	
	// Initialize client
	client = rest.New(cfg.APIKey, cfg.PrivateKey)
	
	// Initialize state
	state = BotState{
		StartTime:     time.Now(),
		OpenPositions: make(map[string]TradeRecord),
	}
	
	// Get initial balance
	balance, err := client.GetBalance()
	if err != nil {
		log.Fatalf("Failed to get balance: %v", err)
	}
	state.CurrentBalance = float64(balance.Balance) / 100.0
	
	fmt.Printf("\n💰 Starting Balance: $%.2f\n", state.CurrentBalance)
	fmt.Printf("📊 Bet Size: $%.2f per trade\n", betSize)
	fmt.Printf("🔄 Poll Interval: %v\n", pollInterval)
	fmt.Printf("⏰ Trading Window: %d:00 - %d:00 local time\n", tradingWindow.startHour, tradingWindow.endHour)
	
	if dryRun {
		fmt.Println("\n⚠️  DRY RUN MODE - No real trades will be executed")
	}
	
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("Starting trading loop...")
	fmt.Println(strings.Repeat("═", 80))
	
	// Main trading loop
	runTradingLoop()
}

func printBanner() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    MULTI-CITY TEMPERATURE TRADING BOT                       ║")
	fmt.Println("║                    3-Signal ENSEMBLE Strategy                               ║")
	fmt.Println("║                    7 HIGH Temperature Markets                               ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
}

func runTradingLoop() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	
	// Run immediately on start
	analyzeAndTrade()
	
	for range ticker.C {
		analyzeAndTrade()
	}
}

func analyzeAndTrade() {
	now := time.Now()
	fmt.Printf("\n[%s] Analyzing markets...\n", now.Format("15:04:05"))
	
	for _, station := range Stations {
		analyzeCity(station, now)
	}
	
	printStatus()
}

func analyzeCity(station Station, now time.Time) {
	loc, err := time.LoadLocation(station.Timezone)
	if err != nil {
		log.Printf("  %s: Failed to load timezone: %v", station.City, err)
		return
	}
	
	localTime := now.In(loc)
	localHour := localTime.Hour()
	
	// Check if within trading window
	if localHour < tradingWindow.startHour || localHour >= tradingWindow.endHour {
		fmt.Printf("  %s: Outside trading window (%d:00 local)\n", station.City, localHour)
		return
	}
	
	// Get today's event
	today := localTime
	dateCode := strings.ToUpper(today.Format("06Jan02"))
	eventTicker := fmt.Sprintf("%s-%s", station.EventPrefix, dateCode)
	
	// Check if we already have a position
	if _, exists := state.OpenPositions[eventTicker]; exists {
		fmt.Printf("  %s: Already have position in %s\n", station.City, eventTicker)
		return
	}
	
	// Fetch markets
	markets, err := fetchMarkets(eventTicker)
	if err != nil {
		fmt.Printf("  %s: No market (%v)\n", station.City, err)
		return
	}
	
	// Get bracket prices (using current bid as proxy for first trade price)
	bracketPrices := make(map[string]int)
	var brackets []Market
	for _, m := range markets {
		if m.Status != "active" {
			continue
		}
		price := int(m.YesBid * 100)
		if price > 0 {
			bracket := fmt.Sprintf("%d-%d°", m.FloorStrike, m.CapStrike)
			bracketPrices[bracket] = price
			brackets = append(brackets, m)
		}
	}
	
	if len(brackets) == 0 {
		fmt.Printf("  %s: No active brackets\n", station.City)
		return
	}
	
	// Signal 1: Market Favorite
	var favBracket string
	var favPrice int
	var favMarket Market
	for _, m := range brackets {
		price := int(m.YesBid * 100)
		if price > favPrice {
			favPrice = price
			favBracket = fmt.Sprintf("%d-%d°", m.FloorStrike, m.CapStrike)
			favMarket = m
		}
	}
	
	// Signal 2: Second Best (not used directly, but confirms market uncertainty)
	var secondPrice int
	for _, m := range brackets {
		price := int(m.YesBid * 100)
		if price > secondPrice && price < favPrice {
			secondPrice = price
		}
	}
	
	// Signal 3: METAR observation
	metarMax, err := getMETARMax(station, today)
	if err != nil {
		fmt.Printf("  %s: No METAR data (%v)\n", station.City, err)
		return
	}
	
	// Find METAR bracket
	var metarBracket string
	for _, m := range brackets {
		if m.FloorStrike <= metarMax && m.CapStrike >= metarMax {
			metarBracket = fmt.Sprintf("%d-%d°", m.FloorStrike, m.CapStrike)
			break
		}
	}
	
	// Check signal agreement
	signalsAgree := favBracket == metarBracket
	
	// Calculate confidence
	confidence := float64(favPrice - secondPrice)
	
	fmt.Printf("  %s: Fav=%s@%d¢ METAR=%d°→%s Agree=%v Conf=%.0f\n",
		station.City, favBracket, favPrice, metarMax, metarBracket, signalsAgree, confidence)
	
	// Trade decision
	if !signalsAgree {
		fmt.Printf("    → SKIP: Signals don't agree\n")
		return
	}
	
	if favPrice < 20 || favPrice > 95 {
		fmt.Printf("    → SKIP: Price out of range (%d¢)\n", favPrice)
		return
	}
	
	// Check market has enough spread for profit
	if confidence < 10 {
		fmt.Printf("    → SKIP: Low confidence (spread=%.0f)\n", confidence)
		return
	}
	
	// Execute trade
	executeTrade(station, eventTicker, favMarket, favBracket, favPrice)
}

func executeTrade(station Station, eventTicker string, market Market, bracket string, price int) {
	// Calculate contracts
	contracts := int(betSize * 100 / float64(price))
	if contracts < 1 {
		contracts = 1
	}
	
	cost := float64(contracts*price) / 100.0
	
	fmt.Printf("    → TRADE: BUY %d contracts of %s @ %d¢ (cost: $%.2f)\n",
		contracts, bracket, price, cost)
	
	if dryRun {
		fmt.Printf("    → DRY RUN: Would place order\n")
		// Record simulated trade
		state.OpenPositions[eventTicker] = TradeRecord{
			Timestamp:   time.Now(),
			City:        station.City,
			EventTicker: eventTicker,
			Bracket:     bracket,
			Ticker:      market.Ticker,
			Side:        "BUY",
			Price:       price,
			Quantity:    contracts,
			Cost:        cost,
			OrderID:     "DRY-RUN",
		}
		state.TotalTrades++
		return
	}
	
	// Place real order
	order := &rest.CreateOrderRequest{
		Ticker:    market.Ticker,
		Action:    "buy",
		Side:      "yes",
		Type:      "limit",
		YesPrice:  price,
		Count:     contracts,
	}
	
	resp, err := client.CreateOrder(order)
	if err != nil {
		fmt.Printf("    → ERROR: Failed to place order: %v\n", err)
		return
	}
	
	fmt.Printf("    → SUCCESS: Order %s placed\n", resp.OrderID)
	
	// Record trade
	state.OpenPositions[eventTicker] = TradeRecord{
		Timestamp:   time.Now(),
		City:        station.City,
		EventTicker: eventTicker,
		Bracket:     bracket,
		Ticker:      market.Ticker,
		Side:        "BUY",
		Price:       price,
		Quantity:    contracts,
		Cost:        cost,
		OrderID:     resp.OrderID,
	}
	state.TotalTrades++
}

func fetchMarkets(eventTicker string) ([]Market, error) {
	url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets?event_ticker=%s&limit=100", eventTicker)
	
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	var result MarketsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	
	// Filter to bracket markets only
	var brackets []Market
	for _, m := range result.Markets {
		parts := strings.Split(m.Ticker, "-")
		if len(parts) >= 3 && strings.HasPrefix(parts[len(parts)-1], "B") {
			brackets = append(brackets, m)
		}
	}
	
	if len(brackets) == 0 {
		return nil, fmt.Errorf("no bracket markets found")
	}
	
	// Sort by floor strike
	sort.Slice(brackets, func(i, j int) bool {
		return brackets[i].FloorStrike < brackets[j].FloorStrike
	})
	
	return brackets, nil
}

func getMETARMax(station Station, date time.Time) (int, error) {
	url := fmt.Sprintf(
		"https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py?station=%s&data=tmpf&year1=%d&month1=%d&day1=%d&year2=%d&month2=%d&day2=%d&tz=%s&format=onlycomma&latlon=no&elev=no&missing=M&trace=T&direct=no&report_type=3",
		station.METAR,
		date.Year(), int(date.Month()), date.Day(),
		date.Year(), int(date.Month()), date.Day()+1,
		station.Timezone,
	)
	
	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(string(body), "\n")
	maxTemp := -999.0
	
	for _, line := range lines {
		if strings.HasPrefix(line, station.METAR+",") {
			parts := strings.Split(line, ",")
			if len(parts) >= 3 {
				var temp float64
				fmt.Sscanf(parts[2], "%f", &temp)
				if temp > maxTemp {
					maxTemp = temp
				}
			}
		}
	}
	
	if maxTemp == -999.0 {
		return 0, fmt.Errorf("no data")
	}
	
	return int(math.Round(maxTemp)), nil
}

func printStatus() {
	fmt.Println()
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("📊 Status: %d trades today | %d open positions\n",
		state.TotalTrades, len(state.OpenPositions))
	
	if len(state.OpenPositions) > 0 {
		fmt.Println("Open positions:")
		for event, trade := range state.OpenPositions {
			fmt.Printf("  • %s: %s %d @ %d¢ ($%.2f)\n",
				trade.City, trade.Bracket, trade.Quantity, trade.Price, trade.Cost)
			_ = event
		}
	}
	
	fmt.Println(strings.Repeat("─", 80))
}

