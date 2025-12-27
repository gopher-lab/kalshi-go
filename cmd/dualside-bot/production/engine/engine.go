package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Station represents a weather station for trading
type Station struct {
	Code        string
	City        string
	METAR       string
	EventPrefix string
	Timezone    string
}

// DefaultStations returns all supported HIGH temperature markets
var DefaultStations = []Station{
	{"LAX", "Los Angeles", "LAX", "KXHIGHLAX", "America/Los_Angeles"},
	{"NYC", "New York", "JFK", "KXHIGHNY", "America/New_York"},
	{"CHI", "Chicago", "ORD", "KXHIGHCHI", "America/Chicago"},
	{"MIA", "Miami", "MIA", "KXHIGHMIA", "America/New_York"},
	{"AUS", "Austin", "AUS", "KXHIGHAUS", "America/Chicago"},
	{"PHIL", "Philadelphia", "PHL", "KXHIGHPHIL", "America/New_York"},
	{"DEN", "Denver", "DEN", "KXHIGHDEN", "America/Denver"},
}

// TradingConfig holds trading parameters
type TradingConfig struct {
	BetYes           float64
	BetNo            float64
	MinYesPrice      int
	MaxYesPrice      int
	MinNoPrice       int
	MaxNoPrice       int
	MaxNoTrades      int
	TradingStartHour int
	TradingEndHour   int
}

// Engine is the core trading engine
type Engine struct {
	config     TradingConfig
	executor   *Executor
	httpClient *http.Client

	// State
	mu            sync.RWMutex
	positions     map[string][]Trade // EventTicker -> trades
	dailyPnL      float64
	totalTrades   int
	totalYesTrades int
	totalNoTrades  int

	// Channels
	tradeChan chan Trade
	errorChan chan error
	stopChan  chan struct{}

	// Callbacks
	onTrade func(Trade)
	onError func(error)
}

// Trade represents a executed trade
type Trade struct {
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
	Status      string // "pending", "filled", "error"
}

// Market data types
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
}

type MarketsResponse struct {
	Markets []Market `json:"markets"`
}

// NewEngine creates a new trading engine
func NewEngine(config TradingConfig, executor *Executor) *Engine {
	return &Engine{
		config:     config,
		executor:   executor,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		positions:  make(map[string][]Trade),
		tradeChan:  make(chan Trade, 100),
		errorChan:  make(chan error, 100),
		stopChan:   make(chan struct{}),
	}
}

// SetTradeCallback sets callback for trade events
func (e *Engine) SetTradeCallback(fn func(Trade)) {
	e.onTrade = fn
}

// SetErrorCallback sets callback for error events
func (e *Engine) SetErrorCallback(fn func(error)) {
	e.onError = fn
}

// Run starts the trading engine
func (e *Engine) Run(ctx context.Context, pollInterval time.Duration) {
	log.Println("[Engine] Starting trading engine...")
	log.Printf("[Engine] Config: BetYes=$%.0f, BetNo=$%.0f, Window=%d-%d",
		e.config.BetYes, e.config.BetNo,
		e.config.TradingStartHour, e.config.TradingEndHour)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Run immediately
	e.tick()

	for {
		select {
		case <-ctx.Done():
			log.Println("[Engine] Shutting down...")
			return
		case <-e.stopChan:
			log.Println("[Engine] Stop signal received")
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

// Stop gracefully stops the engine
func (e *Engine) Stop() {
	close(e.stopChan)
}

// GetStats returns current statistics
func (e *Engine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"total_trades":     e.totalTrades,
		"yes_trades":       e.totalYesTrades,
		"no_trades":        e.totalNoTrades,
		"daily_pnl":        e.dailyPnL,
		"open_positions":   len(e.positions),
		"positions":        e.positions,
	}
}

func (e *Engine) tick() {
	now := time.Now()
	log.Printf("[Engine] Tick at %s", now.Format("15:04:05"))

	for _, station := range DefaultStations {
		e.analyzeStation(station, now)
	}
}

func (e *Engine) analyzeStation(station Station, now time.Time) {
	loc, err := time.LoadLocation(station.Timezone)
	if err != nil {
		log.Printf("[Engine] %s: Failed to load timezone: %v", station.City, err)
		return
	}

	localTime := now.In(loc)
	localHour := localTime.Hour()

	// Check trading window
	if localHour < e.config.TradingStartHour || localHour >= e.config.TradingEndHour {
		log.Printf("[Engine] %s: Outside trading window (%d:00 local)", station.City, localHour)
		return
	}

	// Build event ticker
	dateCode := strings.ToUpper(localTime.Format("06Jan02"))
	eventTicker := fmt.Sprintf("%s-%s", station.EventPrefix, dateCode)

	// Check existing positions
	e.mu.RLock()
	_, hasPosition := e.positions[eventTicker]
	e.mu.RUnlock()

	if hasPosition {
		log.Printf("[Engine] %s: Already have position in %s", station.City, eventTicker)
		return
	}

	// Fetch markets
	markets, err := e.fetchMarkets(eventTicker)
	if err != nil {
		log.Printf("[Engine] %s: Failed to fetch markets: %v", station.City, err)
		return
	}

	if len(markets) == 0 {
		log.Printf("[Engine] %s: No active markets", station.City)
		return
	}

	// Get bracket info
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
		noPrice := 100 - yesPrice
		if m.NoBid > 0 {
			noPrice = int(m.NoBid * 100)
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
		log.Printf("[Engine] %s: No priced brackets", station.City)
		return
	}

	// Sort by YES price (favorite first)
	sort.Slice(brackets, func(i, j int) bool {
		return brackets[i].YesPrice > brackets[j].YesPrice
	})

	favorite := brackets[0]

	// Get METAR
	metarMax, err := e.getMETARMax(station, localTime)
	if err != nil {
		log.Printf("[Engine] %s: Failed to get METAR: %v", station.City, err)
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

	log.Printf("[Engine] %s: Fav=%s@%d¢ METAR=%d°→%s Agree=%v",
		station.City, favorite.Bracket, favorite.YesPrice, metarMax, metarBracket, signalsAgree)

	if !signalsAgree {
		log.Printf("[Engine] %s: Signals don't agree, skipping", station.City)
		return
	}

	// Check YES price range
	if favorite.YesPrice < e.config.MinYesPrice || favorite.YesPrice > e.config.MaxYesPrice {
		log.Printf("[Engine] %s: YES price %d¢ out of range [%d-%d]",
			station.City, favorite.YesPrice, e.config.MinYesPrice, e.config.MaxYesPrice)
		return
	}

	// Execute trades
	var trades []Trade

	// 1. BUY YES on favorite
	yesTrade, err := e.executeYesTrade(station, eventTicker, favorite.Market, favorite.Bracket, favorite.YesPrice)
	if err != nil {
		log.Printf("[Engine] %s: YES trade failed: %v", station.City, err)
		if e.onError != nil {
			e.onError(err)
		}
	} else if yesTrade != nil {
		trades = append(trades, *yesTrade)
		if e.onTrade != nil {
			e.onTrade(*yesTrade)
		}
	}

	// 2. BUY NO on losing brackets
	noCount := 0
	for _, b := range brackets {
		if b.Bracket == favorite.Bracket {
			continue
		}
		if noCount >= e.config.MaxNoTrades {
			break
		}
		if b.NoPrice < e.config.MinNoPrice || b.NoPrice > e.config.MaxNoPrice {
			continue
		}

		noTrade, err := e.executeNoTrade(station, eventTicker, b.Market, b.Bracket, b.NoPrice)
		if err != nil {
			log.Printf("[Engine] %s: NO trade failed: %v", station.City, err)
			if e.onError != nil {
				e.onError(err)
			}
		} else if noTrade != nil {
			trades = append(trades, *noTrade)
			noCount++
			if e.onTrade != nil {
				e.onTrade(*noTrade)
			}
		}
	}

	// Record positions
	if len(trades) > 0 {
		e.mu.Lock()
		e.positions[eventTicker] = trades
		e.mu.Unlock()
	}
}

func (e *Engine) executeYesTrade(station Station, eventTicker string, market Market, bracket string, price int) (*Trade, error) {
	contracts := int(e.config.BetYes * 100 / float64(price))
	if contracts < 1 {
		contracts = 1
	}
	cost := float64(contracts*price) / 100.0

	log.Printf("[Engine] %s: Executing YES BUY %d @ %d¢ ($%.2f)",
		station.City, contracts, price, cost)

	orderID, err := e.executor.ExecuteOrder(ExecuteOrderRequest{
		Ticker:   market.Ticker,
		Side:     "yes",
		Action:   "buy",
		Price:    price,
		Quantity: contracts,
	})

	if err != nil {
		return nil, fmt.Errorf("order failed: %w", err)
	}

	trade := &Trade{
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
		OrderID:     orderID,
		Status:      "filled",
	}

	e.mu.Lock()
	e.totalTrades++
	e.totalYesTrades++
	e.mu.Unlock()

	return trade, nil
}

func (e *Engine) executeNoTrade(station Station, eventTicker string, market Market, bracket string, price int) (*Trade, error) {
	contracts := int(e.config.BetNo * 100 / float64(price))
	if contracts < 1 {
		contracts = 1
	}
	cost := float64(contracts*price) / 100.0

	log.Printf("[Engine] %s: Executing NO BUY %d @ %d¢ ($%.2f)",
		station.City, contracts, price, cost)

	orderID, err := e.executor.ExecuteOrder(ExecuteOrderRequest{
		Ticker:   market.Ticker,
		Side:     "no",
		Action:   "buy",
		Price:    price,
		Quantity: contracts,
	})

	if err != nil {
		return nil, fmt.Errorf("order failed: %w", err)
	}

	trade := &Trade{
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
		OrderID:     orderID,
		Status:      "filled",
	}

	e.mu.Lock()
	e.totalTrades++
	e.totalNoTrades++
	e.mu.Unlock()

	return trade, nil
}

func (e *Engine) fetchMarkets(eventTicker string) ([]Market, error) {
	url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets?event_ticker=%s&limit=100", eventTicker)

	resp, err := e.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result MarketsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var brackets []Market
	for _, m := range result.Markets {
		parts := strings.Split(m.Ticker, "-")
		if len(parts) >= 3 && strings.HasPrefix(parts[len(parts)-1], "B") {
			brackets = append(brackets, m)
		}
	}

	sort.Slice(brackets, func(i, j int) bool {
		return brackets[i].FloorStrike < brackets[j].FloorStrike
	})

	return brackets, nil
}

func (e *Engine) getMETARMax(station Station, date time.Time) (int, error) {
	url := fmt.Sprintf(
		"https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py?station=%s&data=tmpf&year1=%d&month1=%d&day1=%d&year2=%d&month2=%d&day2=%d&tz=%s&format=onlycomma&latlon=no&elev=no&missing=M&trace=T&direct=no&report_type=3",
		station.METAR,
		date.Year(), int(date.Month()), date.Day(),
		date.Year(), int(date.Month()), date.Day()+1,
		station.Timezone,
	)

	resp, err := e.httpClient.Get(url)
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
		return 0, fmt.Errorf("no METAR data")
	}

	return int(math.Round(maxTemp)), nil
}

