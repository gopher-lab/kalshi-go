// Package main provides backtesting for the dual-side (YES + NO) strategy
// When we're confident bracket X wins, we can also BUY NO on losing brackets
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Market struct {
	Ticker      string  `json:"ticker"`
	FloorStrike int     `json:"floor_strike"`
	CapStrike   int     `json:"cap_strike"`
	Result      string  `json:"result"`
	Status      string  `json:"status"`
	YesBid      float64 `json:"yes_bid"`
	YesAsk      float64 `json:"yes_ask"`
}

type MarketsResponse struct {
	Markets []Market `json:"markets"`
}

type Trade struct {
	CreatedTime time.Time `json:"created_time"`
	YesPrice    int       `json:"yes_price"`
	NoPrice     int       `json:"no_price"`
}

type TradesResponse struct {
	Trades []Trade `json:"trades"`
}

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

type DayResult struct {
	Date           time.Time
	City           string
	EventTicker    string
	WinningBracket string
	METARMax       int
	METARBracket   string
	
	// YES trade on favorite
	YesBracket     string
	YesPrice       int
	YesWin         bool
	YesProfit      float64
	
	// NO trades on other brackets
	NoTrades       []NoTrade
	TotalNoProfit  float64
	
	// Combined
	TotalProfit    float64
	SignalsAgree   bool
}

type NoTrade struct {
	Bracket  string
	Price    int
	Win      bool  // NO wins if this bracket LOSES
	Profit   float64
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     DUAL-SIDE BACKTEST (YES + NO Strategy)                                  ║")
	fmt.Println("║     Maximizing liquidity by trading both sides                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	lookbackDays := 21
	betSizeYes := 300.0   // Primary YES bet
	betSizeNo := 100.0    // Each NO bet on losing brackets
	
	fmt.Printf("📅 Lookback: %d days\n", lookbackDays)
	fmt.Printf("💰 YES bet: $%.0f | NO bets: $%.0f each\n", betSizeYes, betSizeNo)
	fmt.Println()

	var allResults []DayResult
	
	for _, station := range Stations {
		fmt.Printf("\n🏙️  %s\n", station.City)
		fmt.Println(strings.Repeat("─", 70))
		
		loc, _ := time.LoadLocation(station.Timezone)
		today := time.Now().In(loc)
		
		for i := 1; i <= lookbackDays; i++ {
			date := today.AddDate(0, 0, -i)
			result := analyzeDay(station, date, betSizeYes, betSizeNo)
			
			if result.SignalsAgree && result.YesPrice > 0 {
				allResults = append(allResults, result)
				
				status := "❌"
				if result.YesWin {
					status = "✅"
				}
				
				fmt.Printf("  %s %s: YES %s@%d¢=$%.0f, NO=$%.0f, Total=$%.0f\n",
					status, date.Format("Jan02"),
					result.YesBracket, result.YesPrice, result.YesProfit,
					result.TotalNoProfit, result.TotalProfit)
			}
			
			time.Sleep(150 * time.Millisecond)
		}
	}

	// Print summary
	printSummary(allResults, betSizeYes, betSizeNo)
}

func analyzeDay(station Station, date time.Time, betSizeYes, betSizeNo float64) DayResult {
	result := DayResult{
		Date: date,
		City: station.City,
	}
	
	loc, _ := time.LoadLocation(station.Timezone)
	dateCode := strings.ToUpper(date.In(loc).Format("06Jan02"))
	result.EventTicker = fmt.Sprintf("%s-%s", station.EventPrefix, dateCode)
	
	// Fetch markets
	markets, err := fetchMarkets(result.EventTicker)
	if err != nil || len(markets) == 0 {
		return result
	}
	
	// Find winner
	var winner *Market
	for i := range markets {
		if markets[i].Result == "yes" {
			winner = &markets[i]
			break
		}
	}
	if winner == nil {
		return result
	}
	
	result.WinningBracket = formatBracket(winner)
	
	// Get METAR
	metarMax, err := getMETARMax(station, date)
	if err != nil {
		return result
	}
	result.METARMax = metarMax
	
	// Find METAR bracket
	for _, m := range markets {
		if m.FloorStrike <= metarMax && m.CapStrike >= metarMax {
			result.METARBracket = formatBracket(&m)
			break
		}
	}
	
	// Get first trade prices for all brackets
	bracketPrices := make(map[string]struct{ yes, no int })
	for _, m := range markets {
		yesPrice, noPrice := getFirstTradePrices(m.Ticker)
		if yesPrice > 0 {
			bracketPrices[formatBracket(&m)] = struct{ yes, no int }{yesPrice, noPrice}
		}
	}
	
	// Find market favorite (highest YES price)
	var favBracket string
	var favPrice int
	for bracket, prices := range bracketPrices {
		if prices.yes > favPrice {
			favPrice = prices.yes
			favBracket = bracket
		}
	}
	
	result.YesBracket = favBracket
	result.YesPrice = favPrice
	
	// Check signal agreement
	result.SignalsAgree = favBracket == result.METARBracket
	
	if !result.SignalsAgree || favPrice < 20 || favPrice > 95 {
		return result
	}
	
	// Calculate YES trade profit
	yesContracts := betSizeYes / float64(favPrice) * 100
	if result.WinningBracket == favBracket {
		result.YesWin = true
		result.YesProfit = yesContracts - betSizeYes
	} else {
		result.YesWin = false
		result.YesProfit = -betSizeYes
	}
	
	// Calculate NO trades on other brackets
	for bracket, prices := range bracketPrices {
		if bracket == favBracket {
			continue // Skip the favorite - we're buying YES there
		}
		
		noPrice := prices.no
		if noPrice < 50 || noPrice > 95 {
			continue // Skip if NO price is too extreme
		}
		
		noContracts := betSizeNo / float64(noPrice) * 100
		
		noTrade := NoTrade{
			Bracket: bracket,
			Price:   noPrice,
		}
		
		// NO wins if this bracket LOSES
		if result.WinningBracket != bracket {
			noTrade.Win = true
			noTrade.Profit = noContracts - betSizeNo
		} else {
			noTrade.Win = false
			noTrade.Profit = -betSizeNo
		}
		
		result.NoTrades = append(result.NoTrades, noTrade)
		result.TotalNoProfit += noTrade.Profit
	}
	
	result.TotalProfit = result.YesProfit + result.TotalNoProfit
	
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

func getFirstTradePrices(ticker string) (yesPrice, noPrice int) {
	url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets/trades?ticker=%s&limit=100", ticker)
	
	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, 0
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	var result TradesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, 0
	}
	
	if len(result.Trades) == 0 {
		return 0, 0
	}
	
	// Find earliest trade
	earliest := result.Trades[0]
	for _, t := range result.Trades {
		if t.CreatedTime.Before(earliest.CreatedTime) {
			earliest = t
		}
	}
	
	yesPrice = earliest.YesPrice
	noPrice = 100 - yesPrice  // NO price = 100 - YES price
	
	return yesPrice, noPrice
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

func formatBracket(m *Market) string {
	return fmt.Sprintf("%d-%d°", m.FloorStrike, m.CapStrike)
}

func printSummary(results []DayResult, betSizeYes, betSizeNo float64) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                          DUAL-SIDE BACKTEST SUMMARY                         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	
	// Calculate stats
	totalTrades := len(results)
	yesWins := 0
	noWins := 0
	noTrades := 0
	
	totalYesProfit := 0.0
	totalNoProfit := 0.0
	totalProfit := 0.0
	
	for _, r := range results {
		if r.YesWin {
			yesWins++
		}
		totalYesProfit += r.YesProfit
		
		for _, nt := range r.NoTrades {
			noTrades++
			if nt.Win {
				noWins++
			}
		}
		totalNoProfit += r.TotalNoProfit
		totalProfit += r.TotalProfit
	}
	
	if totalTrades == 0 {
		fmt.Println("No tradable days found!")
		return
	}
	
	yesWinRate := float64(yesWins) / float64(totalTrades) * 100
	noWinRate := 0.0
	if noTrades > 0 {
		noWinRate = float64(noWins) / float64(noTrades) * 100
	}
	
	avgNoTradesPerDay := float64(noTrades) / float64(totalTrades)
	
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  YES TRADES (Primary)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  Trades:      %d\n", totalTrades)
	fmt.Printf("  Wins:        %d (%.1f%%)\n", yesWins, yesWinRate)
	fmt.Printf("  Bet Size:    $%.0f per trade\n", betSizeYes)
	fmt.Printf("  Total P/L:   $%.2f\n", totalYesProfit)
	fmt.Printf("  Avg P/L:     $%.2f per trade\n", totalYesProfit/float64(totalTrades))
	
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  NO TRADES (Additional Liquidity)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  Total Trades: %d (%.1f per day)\n", noTrades, avgNoTradesPerDay)
	fmt.Printf("  Wins:         %d (%.1f%%)\n", noWins, noWinRate)
	fmt.Printf("  Bet Size:     $%.0f per trade\n", betSizeNo)
	fmt.Printf("  Total P/L:    $%.2f\n", totalNoProfit)
	if noTrades > 0 {
		fmt.Printf("  Avg P/L:      $%.2f per trade\n", totalNoProfit/float64(noTrades))
	}
	
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  COMBINED RESULTS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	
	avgCapitalPerDay := betSizeYes + betSizeNo*avgNoTradesPerDay
	
	fmt.Printf("  Total Profit:     $%.2f\n", totalProfit)
	fmt.Printf("  Avg per day:      $%.2f\n", totalProfit/float64(totalTrades))
	fmt.Printf("  Capital per day:  $%.0f avg\n", avgCapitalPerDay)
	fmt.Printf("  Daily ROI:        %.1f%%\n", (totalProfit/float64(totalTrades))/avgCapitalPerDay*100)
	
	// Project annual
	tradableDaysPerMonth := float64(totalTrades) / 3.0 * (30.0 / 21.0)  // Scale to 30 days
	monthlyProfit := totalProfit / 3.0 * (30.0 / 21.0)  // 3 weeks of data → 1 month
	annualProfit := monthlyProfit * 12
	
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  ANNUAL PROJECTION (Conservative)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  Tradable days/month: %.0f\n", tradableDaysPerMonth)
	fmt.Printf("  Monthly profit:      $%.0f\n", monthlyProfit)
	fmt.Printf("  Annual profit:       $%.0f\n", annualProfit)
	
	// Compare to YES-only
	yesOnlyAnnual := totalYesProfit / 3.0 * (30.0 / 21.0) * 12
	improvement := (annualProfit - yesOnlyAnnual) / yesOnlyAnnual * 100
	
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  COMPARISON: YES-only vs YES+NO")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  YES-only annual:  $%.0f\n", yesOnlyAnnual)
	fmt.Printf("  YES+NO annual:    $%.0f\n", annualProfit)
	fmt.Printf("  Improvement:      +%.1f%%\n", improvement)
	fmt.Println()
}

