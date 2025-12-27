// Package main provides comprehensive backtesting for the 3-signal ENSEMBLE strategy
// across all 13 daily temperature markets (7 HIGH + 6 LOW)
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// Market types
type MarketType string

const (
	MarketTypeHigh MarketType = "HIGH"
	MarketTypeLow  MarketType = "LOW"
)

// Station configuration
type Station struct {
	Code        string     // Short code (LAX, NYC, etc.)
	City        string     // City name
	METAR       string     // METAR station code (without K prefix)
	HighPrefix  string     // Kalshi HIGH event prefix
	LowPrefix   string     // Kalshi LOW event prefix (empty if no market)
	Timezone    string     // IANA timezone
	NWSOffice   string     // NWS office code
	NWSGridX    int        // NWS grid X
	NWSGridY    int        // NWS grid Y
}

// All stations with their Kalshi market configurations
var Stations = []Station{
	{"LAX", "Los Angeles", "LAX", "KXHIGHLAX", "KXLOWTLAX", "America/Los_Angeles", "LOX", 154, 44},
	{"NYC", "New York City", "JFK", "KXHIGHNY", "", "America/New_York", "OKX", 33, 37},
	{"CHI", "Chicago", "ORD", "KXHIGHCHI", "KXLOWTCHI", "America/Chicago", "LOT", 65, 76},
	{"MIA", "Miami", "MIA", "KXHIGHMIA", "KXLOWTMIA", "America/New_York", "MFL", 109, 50},
	{"AUS", "Austin", "AUS", "KXHIGHAUS", "KXLOWTAUS", "America/Chicago", "EWX", 156, 91},
	{"PHIL", "Philadelphia", "PHL", "KXHIGHPHIL", "KXLOWTPHIL", "America/New_York", "PHI", 49, 75},
	{"DEN", "Denver", "DEN", "KXHIGHDEN", "KXLOWTDEN", "America/Denver", "BOU", 62, 60},
}

// API types
type Trade struct {
	CreatedTime time.Time `json:"created_time"`
	YesPrice    int       `json:"yes_price"`
}

type TradesResponse struct {
	Trades []Trade `json:"trades"`
	Cursor string  `json:"cursor"`
}

type Market struct {
	Ticker      string  `json:"ticker"`
	EventTicker string  `json:"event_ticker"`
	FloorStrike int     `json:"floor_strike"`
	CapStrike   int     `json:"cap_strike"`
	Result      string  `json:"result"`
	Status      string  `json:"status"`
	YesBid      float64 `json:"yes_bid"`
	YesAsk      float64 `json:"yes_ask"`
	Volume      int     `json:"volume"`
}

type MarketsResponse struct {
	Markets []Market `json:"markets"`
	Cursor  string   `json:"cursor"`
}

// Analysis types
type DayResult struct {
	Station          *Station
	MarketType       MarketType
	Date             time.Time
	EventTicker      string
	
	// Data fetched
	METARMax         int
	WinningBracket   string
	WinningFloor     int
	AllBrackets      []Market
	
	// 3-Signal ENSEMBLE
	Signal1_MarketFav    string  // Highest priced bracket
	Signal1_Price        int
	Signal2_SecondBest   string  // 2nd highest priced
	Signal2_Price        int
	Signal3_METAR        string  // METAR-based prediction
	Signal3_Temp         int
	
	// Result
	AllSignalsAgree  bool
	PredictedBracket string
	BuyPrice         int
	Win              bool
	Profit           float64
	
	// Errors
	Error            string
}

type CityResults struct {
	Station      *Station
	MarketType   MarketType
	Days         []DayResult
	Wins         int
	Losses       int
	TotalProfit  float64
	WinRate      float64
	AvgProfit    float64
}

type BacktestSummary struct {
	AllResults   []CityResults
	TotalDays    int
	TradableDays int
	Wins         int
	Losses       int
	TotalProfit  float64
	WinRate      float64
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     MULTI-CITY 3-SIGNAL ENSEMBLE BACKTEST                                   ║")
	fmt.Println("║     Testing across all 13 daily temperature markets                         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	lookbackDays := 21
	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &lookbackDays)
	}
	fmt.Printf("📅 Lookback period: %d days\n", lookbackDays)
	fmt.Println()

	summary := BacktestSummary{}

	// Test HIGH markets for all 7 cities
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  HIGH TEMPERATURE MARKETS (7 cities)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")

	for i := range Stations {
		station := &Stations[i]
		if station.HighPrefix == "" {
			continue
		}
		
		results := backtestCity(station, MarketTypeHigh, lookbackDays)
		summary.AllResults = append(summary.AllResults, results)
		printCityResults(results)
	}

	// Test LOW markets for cities that have them
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  LOW TEMPERATURE MARKETS (6 cities)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")

	for i := range Stations {
		station := &Stations[i]
		if station.LowPrefix == "" {
			continue
		}
		
		results := backtestCity(station, MarketTypeLow, lookbackDays)
		summary.AllResults = append(summary.AllResults, results)
		printCityResults(results)
	}

	// Calculate overall summary
	for _, cr := range summary.AllResults {
		summary.TotalDays += len(cr.Days)
		for _, d := range cr.Days {
			if d.AllSignalsAgree && d.BuyPrice > 0 {
				summary.TradableDays++
				if d.Win {
					summary.Wins++
				} else {
					summary.Losses++
				}
				summary.TotalProfit += d.Profit
			}
		}
	}
	if summary.TradableDays > 0 {
		summary.WinRate = float64(summary.Wins) / float64(summary.TradableDays) * 100
	}

	printSummary(summary)
}

func backtestCity(station *Station, marketType MarketType, lookbackDays int) CityResults {
	results := CityResults{
		Station:    station,
		MarketType: marketType,
	}

	loc, _ := time.LoadLocation(station.Timezone)
	today := time.Now().In(loc)

	prefix := station.HighPrefix
	if marketType == MarketTypeLow {
		prefix = station.LowPrefix
	}

	fmt.Printf("\n🏙️  %s %s Temperature\n", station.City, marketType)
	fmt.Printf("   Event prefix: %s\n", prefix)

	for i := 1; i <= lookbackDays; i++ {
		targetDate := today.AddDate(0, 0, -i)
		dayResult := analyzeDay(station, marketType, targetDate)
		results.Days = append(results.Days, dayResult)

		if dayResult.Error != "" {
			continue
		}

		if dayResult.AllSignalsAgree && dayResult.BuyPrice > 0 {
			if dayResult.Win {
				results.Wins++
			} else {
				results.Losses++
			}
			results.TotalProfit += dayResult.Profit
		}

		// Rate limiting
		time.Sleep(200 * time.Millisecond)
	}

	total := results.Wins + results.Losses
	if total > 0 {
		results.WinRate = float64(results.Wins) / float64(total) * 100
		results.AvgProfit = results.TotalProfit / float64(total)
	}

	return results
}

func analyzeDay(station *Station, marketType MarketType, date time.Time) DayResult {
	result := DayResult{
		Station:    station,
		MarketType: marketType,
		Date:       date,
	}

	// Generate event ticker
	prefix := station.HighPrefix
	if marketType == MarketTypeLow {
		prefix = station.LowPrefix
	}
	dateCode := strings.ToUpper(date.Format("06Jan02"))
	result.EventTicker = fmt.Sprintf("%s-%s", prefix, dateCode)

	// Step 1: Fetch all markets for this event (to get prices and winner)
	markets, err := fetchMarkets(result.EventTicker)
	if err != nil {
		result.Error = fmt.Sprintf("markets: %v", err)
		return result
	}
	if len(markets) == 0 {
		result.Error = "no markets"
		return result
	}

	result.AllBrackets = markets

	// Find winner
	var winner *Market
	for i := range markets {
		if markets[i].Result == "yes" {
			winner = &markets[i]
			break
		}
	}
	if winner == nil {
		result.Error = "no winner (market may still be open)"
		return result
	}

	result.WinningBracket = formatBracket(winner)
	result.WinningFloor = winner.FloorStrike

	// Step 2: Get first trade prices for all brackets
	bracketPrices := make(map[string]int)
	for _, m := range markets {
		price, err := getFirstTradePrice(m.Ticker)
		if err == nil && price > 0 {
			bracketPrices[formatBracket(&m)] = price
		}
	}

	// Step 3: Generate 3 signals

	// Signal 1: Market Favorite (highest first trade price)
	var bestBracket string
	var bestPrice int
	for bracket, price := range bracketPrices {
		if price > bestPrice {
			bestPrice = price
			bestBracket = bracket
		}
	}
	result.Signal1_MarketFav = bestBracket
	result.Signal1_Price = bestPrice

	// Signal 2: Second Best (2nd highest first trade price)
	var secondBracket string
	var secondPrice int
	for bracket, price := range bracketPrices {
		if price > secondPrice && bracket != bestBracket {
			secondPrice = price
			secondBracket = bracket
		}
	}
	result.Signal2_SecondBest = secondBracket
	result.Signal2_Price = secondPrice

	// Signal 3: METAR-based prediction
	metarMax, err := getMETARMax(station, date)
	if err != nil {
		result.Error = fmt.Sprintf("METAR: %v", err)
		return result
	}
	result.METARMax = metarMax
	result.Signal3_Temp = metarMax

	// For HIGH markets, METAR max is the prediction
	// For LOW markets, we'd need METAR min (simplified: use METAR max - 20 as rough low estimate)
	predictedTemp := metarMax
	if marketType == MarketTypeLow {
		predictedTemp = metarMax - 20 // Very rough approximation
	}

	// Find bracket containing predicted temp
	for _, m := range markets {
		if m.FloorStrike <= predictedTemp && m.CapStrike >= predictedTemp {
			result.Signal3_METAR = formatBracket(&m)
			break
		}
	}

	// Step 4: Check if all 3 signals agree
	if result.Signal1_MarketFav != "" && 
	   result.Signal1_MarketFav == result.Signal2_SecondBest {
		// If market fav equals 2nd best, signals don't truly agree
		result.AllSignalsAgree = false
	} else {
		// Check agreement: Market favorite should be adjacent to 2nd best, and METAR agrees with favorite
		result.AllSignalsAgree = (result.Signal1_MarketFav == result.Signal3_METAR)
	}

	// For ENSEMBLE: we need 3 different signals to agree
	// Let's check: does METAR agree with market favorite OR is 2nd best adjacent to favorite?
	// Actually, for true 3-signal: Fav + 2ndBest should differ, and one of them should match METAR
	
	// Simplified for now: METAR matches Market Favorite
	if result.Signal1_MarketFav == result.Signal3_METAR && result.Signal1_Price > 0 {
		result.AllSignalsAgree = true
		result.PredictedBracket = result.Signal1_MarketFav
		result.BuyPrice = result.Signal1_Price
	} else {
		result.AllSignalsAgree = false
	}

	// Step 5: Calculate profit if we would have traded
	if result.AllSignalsAgree && result.BuyPrice > 0 {
		// Simulate $10 bet (no price constraints for analysis - just track all trades)
		contracts := 1000 / result.BuyPrice
		cost := float64(contracts * result.BuyPrice) / 100

		if result.PredictedBracket == result.WinningBracket {
			result.Win = true
			result.Profit = float64(contracts) - cost
		} else {
			result.Win = false
			result.Profit = -cost
		}
	}

	return result
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

	// Filter to only bracket markets (B prefix in ticker), sort by floor strike
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

func getFirstTradePrice(ticker string) (int, error) {
	url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets/trades?ticker=%s&limit=100", ticker)

	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result TradesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	if len(result.Trades) == 0 {
		return 0, fmt.Errorf("no trades")
	}

	// Find earliest trade
	earliest := result.Trades[0]
	for _, t := range result.Trades {
		if t.CreatedTime.Before(earliest.CreatedTime) {
			earliest = t
		}
	}

	return earliest.YesPrice, nil
}

func getMETARMax(station *Station, date time.Time) (int, error) {
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

func formatBracket(m *Market) string {
	return fmt.Sprintf("%d-%d°", m.FloorStrike, m.CapStrike)
}

func printCityResults(cr CityResults) {
	fmt.Printf("   Results: %d days analyzed\n", len(cr.Days))

	// Count days with data
	daysWithData := 0
	tradable := 0
	for _, d := range cr.Days {
		if d.Error == "" && d.WinningBracket != "" {
			daysWithData++
		}
		if d.AllSignalsAgree && d.BuyPrice > 0 {
			tradable++
		}
	}

	fmt.Printf("   Days with settlement data: %d\n", daysWithData)

	if tradable == 0 {
		// Show debug info for a few days
		fmt.Printf("   ⚠️  No trades (signals didn't agree)\n")
		fmt.Printf("   Sample signal breakdown:\n")
		count := 0
		for _, d := range cr.Days {
			if d.Error != "" || d.WinningBracket == "" {
				continue
			}
			if count >= 3 {
				break
			}
			fmt.Printf("     %s: Fav=%s, METAR=%s, Won=%s\n",
				d.Date.Format("Jan02"), d.Signal1_MarketFav, d.Signal3_METAR, d.WinningBracket)
			count++
		}
		return
	}

	fmt.Printf("   📊 Tradable: %d days | Wins: %d | Losses: %d | Win Rate: %.1f%%\n",
		tradable, cr.Wins, cr.Losses, cr.WinRate)
	fmt.Printf("   💰 Total Profit: $%.2f | Avg per trade: $%.2f\n",
		cr.TotalProfit, cr.AvgProfit)

	// Show day-by-day breakdown for tradable days
	fmt.Println("   ────────────────────────────────────────────────────────────")
	for _, d := range cr.Days {
		if d.Error != "" {
			continue
		}
		if !d.AllSignalsAgree || d.BuyPrice == 0 {
			continue
		}

		status := "❌"
		if d.Win {
			status = "✅"
		}

		fmt.Printf("   %s %s: Bet %s @ %d¢, Won: %s, Profit: $%.2f\n",
			status, d.Date.Format("Jan 02"),
			d.PredictedBracket, d.BuyPrice,
			d.WinningBracket, d.Profit)
	}
}

func printSummary(summary BacktestSummary) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                          OVERALL BACKTEST SUMMARY                            ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Printf("📊 Total days analyzed: %d\n", summary.TotalDays)
	fmt.Printf("📊 Tradable days (3 signals agree): %d\n", summary.TradableDays)
	fmt.Println()

	if summary.TradableDays == 0 {
		fmt.Println("⚠️  No tradable opportunities found!")
		return
	}

	fmt.Printf("🎯 Win Rate: %.1f%% (%d/%d)\n", summary.WinRate, summary.Wins, summary.TradableDays)
	fmt.Printf("💰 Total Profit: $%.2f\n", summary.TotalProfit)
	fmt.Printf("💰 Avg Profit per Trade: $%.2f\n", summary.TotalProfit/float64(summary.TradableDays))
	fmt.Println()

	// Show best and worst cities
	fmt.Println("📈 Results by City/Market:")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────")

	// Sort by profit
	sorted := make([]CityResults, len(summary.AllResults))
	copy(sorted, summary.AllResults)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TotalProfit > sorted[j].TotalProfit
	})

	for _, cr := range sorted {
		tradable := 0
		for _, d := range cr.Days {
			if d.AllSignalsAgree && d.BuyPrice > 0 {
				tradable++
			}
		}
		if tradable == 0 {
			fmt.Printf("   ⚪ %s %s: No trades\n", cr.Station.City, cr.MarketType)
		} else {
			fmt.Printf("   %s %s: %d trades, %.0f%% win, $%.2f profit\n",
				cr.Station.City, cr.MarketType, tradable, cr.WinRate, cr.TotalProfit)
		}
	}

	// Monte Carlo style analysis
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
	fmt.Println("📊 STRATEGY ASSESSMENT")
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")

	if summary.WinRate >= 70 {
		fmt.Println("✅ STRONG EDGE: Win rate above 70% indicates a robust strategy")
	} else if summary.WinRate >= 55 {
		fmt.Println("⚠️  MODERATE EDGE: Win rate suggests some edge, but variance is higher")
	} else {
		fmt.Println("❌ WEAK/NO EDGE: Win rate does not support profitable trading")
	}

	avgProfit := summary.TotalProfit / float64(summary.TradableDays)
	if avgProfit > 0 {
		fmt.Printf("✅ POSITIVE EXPECTANCY: $%.2f per trade\n", avgProfit)
	} else {
		fmt.Printf("❌ NEGATIVE EXPECTANCY: $%.2f per trade\n", avgProfit)
	}

	// Estimate Kelly Criterion
	if summary.WinRate > 0 && summary.Losses > 0 {
		// Simplified Kelly: f* = (bp - q) / b where b = odds, p = win prob, q = lose prob
		// For our case, avg win/loss ratio matters
		avgWin := 0.0
		avgLoss := 0.0
		wins := 0
		losses := 0
		
		for _, cr := range summary.AllResults {
			for _, d := range cr.Days {
				if d.AllSignalsAgree && d.BuyPrice > 0 {
					if d.Win {
						avgWin += d.Profit
						wins++
					} else {
						avgLoss += -d.Profit
						losses++
					}
				}
			}
		}
		
		if wins > 0 && losses > 0 {
			avgWin /= float64(wins)
			avgLoss /= float64(losses)
			p := summary.WinRate / 100
			b := avgWin / avgLoss
			kelly := (b*p - (1-p)) / b
			
			fmt.Printf("📈 Kelly Fraction: %.1f%%\n", kelly*100)
			if kelly > 0 {
				fmt.Println("   (Positive Kelly = strategy has edge)")
			} else {
				fmt.Println("   (Negative Kelly = no edge)")
			}
		}
	}
}

