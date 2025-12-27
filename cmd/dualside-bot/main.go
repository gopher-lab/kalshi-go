// Package main provides a dual-side automated trading bot
// Trades both YES on favorites and NO on losing brackets for maximum liquidity
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
	betSizeYes    float64
	betSizeNo     float64
	dryRun        bool
	pollInterval  time.Duration
	maxNoTrades   int
	minNoPrice    int
	maxNoPrice    int
)

// Station configuration
type Station struct {
	Code        string
	City        string
	METAR       string
	EventPrefix string
	Timezone    string
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

// Market data
type Market struct {
	Ticker      string  `json:"ticker"`
	EventTicker string  `json:"event_ticker"`
	FloorStrike int     `json:"floor_strike"`
	CapStrike   int     `json:"cap_strike"`
	Status      string  `json:"status"`
	YesBid      float64 `json:"yes_bid"`
	YesAsk      float64 `json:"yes_ask"`
	NoBid       float64 `json:"no_bid"`
	NoAsk       float64 `json:"no_ask"`
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
	Side        string // "yes" or "no"
	Action      string // "buy"
	Price       int
	Quantity    int
	Cost        float64
	OrderID     string
}

type BotState struct {
	StartTime      time.Time
	YesTrades      int
	NoTrades       int
	OpenPositions  map[string][]TradeRecord // EventTicker -> trades
	TotalCost      float64
	CurrentBalance float64
}

var (
	client     *rest.Client
	httpClient = &http.Client{Timeout: 15 * time.Second}
	state      BotState
)

func init() {
	flag.Float64Var(&betSizeYes, "bet-yes", 300, "Bet size for YES trades (dollars)")
	flag.Float64Var(&betSizeNo, "bet-no", 100, "Bet size for each NO trade (dollars)")
	flag.BoolVar(&dryRun, "dry-run", false, "Simulate trades without executing")
	flag.DurationVar(&pollInterval, "interval", 5*time.Minute, "Polling interval")
	flag.IntVar(&maxNoTrades, "max-no", 3, "Maximum NO trades per event")
	flag.IntVar(&minNoPrice, "min-no-price", 50, "Minimum NO price to trade (cents)")
	flag.IntVar(&maxNoPrice, "max-no-price", 90, "Maximum NO price to trade (cents)")
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
		OpenPositions: make(map[string][]TradeRecord),
	}

	// Get initial balance
	balance, err := client.GetBalance()
	if err != nil {
		log.Fatalf("Failed to get balance: %v", err)
	}
	state.CurrentBalance = float64(balance.Balance) / 100.0

	fmt.Printf("\n💰 Starting Balance: $%.2f\n", state.CurrentBalance)
	fmt.Printf("📊 YES Bet: $%.0f | NO Bet: $%.0f (max %d per event)\n", betSizeYes, betSizeNo, maxNoTrades)
	fmt.Printf("📊 NO Price Range: %d¢ - %d¢\n", minNoPrice, maxNoPrice)
	fmt.Printf("🔄 Poll Interval: %v\n", pollInterval)

	if dryRun {
		fmt.Println("\n⚠️  DRY RUN MODE - No real trades will be executed")
	}

	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("Starting dual-side trading loop...")
	fmt.Println(strings.Repeat("═", 80))

	// Main trading loop
	runTradingLoop()
}

func printBanner() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    DUAL-SIDE TEMPERATURE TRADING BOT                        ║")
	fmt.Println("║                    YES + NO Strategy for Maximum Liquidity                  ║")
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

	// Trading window: 7 AM - 2 PM local
	if localHour < 7 || localHour >= 14 {
		fmt.Printf("  %s: Outside trading window (%d:00 local)\n", station.City, localHour)
		return
	}

	// Get today's event
	dateCode := strings.ToUpper(localTime.Format("06Jan02"))
	eventTicker := fmt.Sprintf("%s-%s", station.EventPrefix, dateCode)

	// Check if we already have positions
	if _, exists := state.OpenPositions[eventTicker]; exists {
		fmt.Printf("  %s: Already have positions in %s\n", station.City, eventTicker)
		return
	}

	// Fetch markets
	markets, err := fetchMarkets(eventTicker)
	if err != nil {
		fmt.Printf("  %s: No market (%v)\n", station.City, err)
		return
	}

	// Get bracket prices
	type BracketInfo struct {
		Market   Market
		Bracket  string
		YesPrice int
		NoPrice  int
	}

	var brackets []BracketInfo
	for _, m := range markets {
		if m.Status != "active" {
			continue
		}
		yesPrice := int(m.YesBid * 100)
		noPrice := int(m.NoBid * 100)
		if noPrice == 0 {
			noPrice = 100 - yesPrice // Estimate
		}

		if yesPrice > 0 {
			brackets = append(brackets, BracketInfo{
				Market:   m,
				Bracket:  fmt.Sprintf("%d-%d°", m.FloorStrike, m.CapStrike),
				YesPrice: yesPrice,
				NoPrice:  noPrice,
			})
		}
	}

	if len(brackets) == 0 {
		fmt.Printf("  %s: No active brackets\n", station.City)
		return
	}

	// Sort by YES price (favorite first)
	sort.Slice(brackets, func(i, j int) bool {
		return brackets[i].YesPrice > brackets[j].YesPrice
	})

	favorite := brackets[0]

	// Get METAR
	metarMax, err := getMETARMax(station, localTime)
	if err != nil {
		fmt.Printf("  %s: No METAR data (%v)\n", station.City, err)
		return
	}

	// Find METAR bracket
	var metarBracket string
	for _, b := range brackets {
		if b.Market.FloorStrike <= metarMax && b.Market.CapStrike >= metarMax {
			metarBracket = b.Bracket
			break
		}
	}

	// Check signal agreement
	signalsAgree := favorite.Bracket == metarBracket

	fmt.Printf("  %s: Fav=%s@%d¢ METAR=%d°→%s Agree=%v\n",
		station.City, favorite.Bracket, favorite.YesPrice, metarMax, metarBracket, signalsAgree)

	if !signalsAgree {
		fmt.Printf("    → SKIP: Signals don't agree\n")
		return
	}

	if favorite.YesPrice < 20 || favorite.YesPrice > 95 {
		fmt.Printf("    → SKIP: YES price out of range (%d¢)\n", favorite.YesPrice)
		return
	}

	// Execute trades
	var trades []TradeRecord

	// 1. BUY YES on favorite
	yesTrade := executeYesTrade(station, eventTicker, favorite.Market, favorite.Bracket, favorite.YesPrice)
	if yesTrade != nil {
		trades = append(trades, *yesTrade)
	}

	// 2. BUY NO on losing brackets
	noCount := 0
	for _, b := range brackets {
		if b.Bracket == favorite.Bracket {
			continue // Skip favorite
		}
		if noCount >= maxNoTrades {
			break
		}
		if b.NoPrice < minNoPrice || b.NoPrice > maxNoPrice {
			continue // Skip if NO price out of range
		}

		noTrade := executeNoTrade(station, eventTicker, b.Market, b.Bracket, b.NoPrice)
		if noTrade != nil {
			trades = append(trades, *noTrade)
			noCount++
		}
	}

	// Record positions
	if len(trades) > 0 {
		state.OpenPositions[eventTicker] = trades
	}
}

func executeYesTrade(station Station, eventTicker string, market Market, bracket string, price int) *TradeRecord {
	contracts := int(betSizeYes * 100 / float64(price))
	if contracts < 1 {
		contracts = 1
	}

	cost := float64(contracts*price) / 100.0

	fmt.Printf("    → YES: BUY %d contracts of %s @ %d¢ ($%.2f)\n",
		contracts, bracket, price, cost)

	if dryRun {
		state.YesTrades++
		state.TotalCost += cost
		return &TradeRecord{
			Timestamp:   time.Now(),
			City:        station.City,
			EventTicker: eventTicker,
			Bracket:     bracket,
			Ticker:      market.Ticker,
			Side:        "yes",
			Action:      "buy",
			Price:       price,
			Quantity:    contracts,
			Cost:        cost,
			OrderID:     "DRY-RUN",
		}
	}

	// Place real order
	order := &rest.CreateOrderRequest{
		Ticker:   market.Ticker,
		Action:   "buy",
		Side:     "yes",
		Type:     "limit",
		YesPrice: price,
		Count:    contracts,
	}

	resp, err := client.CreateOrder(order)
	if err != nil {
		fmt.Printf("    → ERROR: %v\n", err)
		return nil
	}

	fmt.Printf("    → SUCCESS: Order %s\n", resp.OrderID)
	state.YesTrades++
	state.TotalCost += cost

	return &TradeRecord{
		Timestamp:   time.Now(),
		City:        station.City,
		EventTicker: eventTicker,
		Bracket:     bracket,
		Ticker:      market.Ticker,
		Side:        "yes",
		Action:      "buy",
		Price:       price,
		Quantity:    contracts,
		Cost:        cost,
		OrderID:     resp.OrderID,
	}
}

func executeNoTrade(station Station, eventTicker string, market Market, bracket string, price int) *TradeRecord {
	contracts := int(betSizeNo * 100 / float64(price))
	if contracts < 1 {
		contracts = 1
	}

	cost := float64(contracts*price) / 100.0

	fmt.Printf("    → NO:  BUY %d contracts of %s @ %d¢ ($%.2f)\n",
		contracts, bracket, price, cost)

	if dryRun {
		state.NoTrades++
		state.TotalCost += cost
		return &TradeRecord{
			Timestamp:   time.Now(),
			City:        station.City,
			EventTicker: eventTicker,
			Bracket:     bracket,
			Ticker:      market.Ticker,
			Side:        "no",
			Action:      "buy",
			Price:       price,
			Quantity:    contracts,
			Cost:        cost,
			OrderID:     "DRY-RUN",
		}
	}

	// Place real order
	order := &rest.CreateOrderRequest{
		Ticker:  market.Ticker,
		Action:  "buy",
		Side:    "no",
		Type:    "limit",
		NoPrice: price,
		Count:   contracts,
	}

	resp, err := client.CreateOrder(order)
	if err != nil {
		fmt.Printf("    → ERROR: %v\n", err)
		return nil
	}

	fmt.Printf("    → SUCCESS: Order %s\n", resp.OrderID)
	state.NoTrades++
	state.TotalCost += cost

	return &TradeRecord{
		Timestamp:   time.Now(),
		City:        station.City,
		EventTicker: eventTicker,
		Bracket:     bracket,
		Ticker:      market.Ticker,
		Side:        "no",
		Action:      "buy",
		Price:       price,
		Quantity:    contracts,
		Cost:        cost,
		OrderID:     resp.OrderID,
	}
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

	// Filter to bracket markets
	var brackets []Market
	for _, m := range result.Markets {
		parts := strings.Split(m.Ticker, "-")
		if len(parts) >= 3 && strings.HasPrefix(parts[len(parts)-1], "B") {
			brackets = append(brackets, m)
		}
	}

	if len(brackets) == 0 {
		return nil, fmt.Errorf("no bracket markets")
	}

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
	fmt.Printf("📊 Status: YES=%d NO=%d | Total positions: %d events | Cost: $%.2f\n",
		state.YesTrades, state.NoTrades, len(state.OpenPositions), state.TotalCost)

	if len(state.OpenPositions) > 0 {
		fmt.Println("Open positions:")
		for event, trades := range state.OpenPositions {
			yesCost := 0.0
			noCost := 0.0
			for _, t := range trades {
				if t.Side == "yes" {
					yesCost += t.Cost
				} else {
					noCost += t.Cost
				}
			}
			fmt.Printf("  • %s: YES=$%.0f NO=$%.0f\n", event, yesCost, noCost)
		}
	}

	fmt.Println(strings.Repeat("─", 80))
}


