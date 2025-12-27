// Package main tests whether adding a 4th signal improves the ENSEMBLE strategy
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

type Trade struct {
	CreatedTime time.Time `json:"created_time"`
	YesPrice    int       `json:"yes_price"`
}

type TradesResponse struct {
	Trades []Trade `json:"trades"`
}

type Market struct {
	Ticker      string `json:"ticker"`
	FloorStrike int    `json:"floor_strike"`
	CapStrike   int    `json:"cap_strike"`
	Result      string `json:"result"`
}

type MarketsResponse struct {
	Markets []Market `json:"markets"`
}

type BracketData struct {
	Floor      int
	FirstPrice int
	Won        bool
}

type DayData struct {
	Date         time.Time
	METARMax     int
	PrevDayMax   int
	Brackets     []BracketData
	WinnerFloor  int
	Climatology  int // Historical average for this date
}

type StrategyResult struct {
	Name        string
	Signals     []string
	Trades      int
	Wins        int
	WinRate     float64
	TotalProfit float64
	AvgProfit   float64
}

var loc *time.Location
var httpClient = &http.Client{Timeout: 15 * time.Second}

func init() {
	var err error
	loc, err = time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.UTC
	}
}

func main() {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("4-SIGNAL ENSEMBLE TEST")
	fmt.Println("Testing if adding a 4th signal improves accuracy")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Fetch data
	data := fetchAllData(21)
	fmt.Printf("Fetched %d days of data\n\n", len(data))

	// Potential signals:
	// 1. Market Favorite (highest price)
	// 2. METAR prediction (today's actual max)
	// 3. 2nd Best (2nd highest price)
	// 4. Previous Day (yesterday's winner/max)
	// 5. Climatology (historical average ~65°F for Dec)
	// 6. 3rd Best (3rd highest price)

	fmt.Println("AVAILABLE SIGNALS:")
	fmt.Println("  1. Market   - Bracket with highest first trade price")
	fmt.Println("  2. METAR    - Bracket containing today's METAR max")
	fmt.Println("  3. 2nd      - Bracket with 2nd highest price")
	fmt.Println("  4. PrevDay  - Previous day's METAR max bracket")
	fmt.Println("  5. Clima    - Climatological average (65°F for Dec)")
	fmt.Println("  6. 3rd      - Bracket with 3rd highest price")
	fmt.Println()

	// Test different combinations
	results := []StrategyResult{}

	// Original 3-signal
	r := testStrategy(data, []string{"Market", "METAR", "2nd"}, 2)
	results = append(results, r)

	// 4-signal combinations (require 2 to agree)
	r = testStrategy(data, []string{"Market", "METAR", "2nd", "PrevDay"}, 2)
	results = append(results, r)

	r = testStrategy(data, []string{"Market", "METAR", "2nd", "Clima"}, 2)
	results = append(results, r)

	r = testStrategy(data, []string{"Market", "METAR", "2nd", "3rd"}, 2)
	results = append(results, r)

	// 4-signal with higher threshold (require 3 to agree)
	r = testStrategy(data, []string{"Market", "METAR", "2nd", "PrevDay"}, 3)
	results = append(results, r)

	r = testStrategy(data, []string{"Market", "METAR", "2nd", "Clima"}, 3)
	results = append(results, r)

	// 5-signal combinations
	r = testStrategy(data, []string{"Market", "METAR", "2nd", "PrevDay", "Clima"}, 2)
	results = append(results, r)

	r = testStrategy(data, []string{"Market", "METAR", "2nd", "PrevDay", "Clima"}, 3)
	results = append(results, r)

	// Print results
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("RESULTS COMPARISON")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	fmt.Printf("%-40s %6s %6s %8s %10s %10s\n", "Strategy", "Trades", "Wins", "WinRate", "Profit", "Avg/Trade")
	fmt.Println(strings.Repeat("-", 90))

	// Sort by profit
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalProfit > results[j].TotalProfit
	})

	for i, r := range results {
		marker := ""
		if i == 0 {
			marker = " 🏆 BEST"
		}
		fmt.Printf("%-40s %6d %6d %7.1f%% $%8.2f $%8.2f%s\n",
			r.Name, r.Trades, r.Wins, r.WinRate, r.TotalProfit, r.AvgProfit, marker)
	}

	fmt.Println()

	// Detailed analysis of top 3
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("TOP 3 STRATEGIES - DETAILED")
	fmt.Println(strings.Repeat("=", 80))

	for i := 0; i < 3 && i < len(results); i++ {
		r := results[i]
		fmt.Println()
		fmt.Printf("#%d: %s\n", i+1, r.Name)
		fmt.Printf("    Signals: %s\n", strings.Join(r.Signals, " + "))
		fmt.Printf("    Trades: %d/%d days (%.1f%%)\n", r.Trades, len(data), float64(r.Trades)/float64(len(data))*100)
		fmt.Printf("    Win Rate: %.1f%%\n", r.WinRate)
		fmt.Printf("    Total Profit: $%.2f\n", r.TotalProfit)
		fmt.Printf("    Avg Profit/Trade: $%.2f\n", r.AvgProfit)

		// Risk metrics
		if r.WinRate > 0 && r.Trades > 0 {
			avgWin := 6.0   // Approximate
			avgLoss := 14.0 // Fixed
			ev := r.WinRate/100*avgWin - (100-r.WinRate)/100*avgLoss
			fmt.Printf("    Expected Value/Trade: $%.2f\n", ev)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("CONCLUSION")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	best := results[0]
	original := StrategyResult{}
	for _, r := range results {
		if r.Name == "3sig(Market+METAR+2nd)≥2" {
			original = r
			break
		}
	}

	if best.TotalProfit > original.TotalProfit {
		improvement := best.TotalProfit - original.TotalProfit
		fmt.Printf("✅ IMPROVEMENT FOUND!\n")
		fmt.Printf("   Best: %s\n", best.Name)
		fmt.Printf("   Profit improvement: +$%.2f (%.1f%% better)\n", improvement, improvement/original.TotalProfit*100)
	} else {
		fmt.Printf("❌ Original 3-signal strategy is still the best.\n")
		fmt.Printf("   Adding more signals doesn't improve results with this data.\n")
	}
}

func testStrategy(data []DayData, signals []string, minAgree int) StrategyResult {
	name := fmt.Sprintf("%dsig(%s)≥%d", len(signals), strings.Join(signals, "+"), minAgree)

	result := StrategyResult{
		Name:    name,
		Signals: signals,
	}

	for _, d := range data {
		if len(d.Brackets) < 3 {
			continue
		}

		// Get signal values
		votes := make(map[int]int)

		for _, sig := range signals {
			bracket := getSignalBracket(d, sig)
			if bracket > 0 {
				votes[bracket]++
			}
		}

		// Find best bracket
		bestBracket := 0
		maxVotes := 0
		for floor, v := range votes {
			if v > maxVotes {
				maxVotes = v
				bestBracket = floor
			}
		}

		// Check if meets threshold
		if maxVotes < minAgree {
			continue
		}

		result.Trades++

		// Find price
		var price int
		var won bool
		for _, b := range d.Brackets {
			if b.Floor == bestBracket {
				price = b.FirstPrice
				won = b.Won
				break
			}
		}

		if price == 0 {
			price = 50
		}

		contracts := 1400 / price
		if won {
			result.Wins++
			result.TotalProfit += float64(contracts) - 14.0
		} else {
			result.TotalProfit -= 14.0
		}
	}

	if result.Trades > 0 {
		result.WinRate = float64(result.Wins) / float64(result.Trades) * 100
		result.AvgProfit = result.TotalProfit / float64(result.Trades)
	}

	return result
}

func getSignalBracket(d DayData, signal string) int {
	switch signal {
	case "Market":
		if len(d.Brackets) > 0 {
			return d.Brackets[0].Floor
		}
	case "METAR":
		return (d.METARMax / 2) * 2
	case "2nd":
		if len(d.Brackets) > 1 {
			return d.Brackets[1].Floor
		}
	case "3rd":
		if len(d.Brackets) > 2 {
			return d.Brackets[2].Floor
		}
	case "PrevDay":
		return (d.PrevDayMax / 2) * 2
	case "Clima":
		return 64 // December average ~65°F, bracket 64-65
	}
	return 0
}

func fetchAllData(days int) []DayData {
	var data []DayData
	today := time.Now().In(loc)

	var prevMax int

	for i := days; i >= 1; i-- { // Go backwards to get prev day data
		targetDate := today.AddDate(0, 0, -i)
		fmt.Printf("Fetching %s... ", targetDate.Format("Jan 2"))

		dayData, err := fetchDayData(targetDate)
		if err != nil {
			fmt.Printf("❌\n")
			continue
		}

		dayData.PrevDayMax = prevMax
		prevMax = dayData.METARMax

		fmt.Printf("✅ METAR=%d°, Prev=%d°\n", dayData.METARMax, dayData.PrevDayMax)
		data = append(data, dayData)
		time.Sleep(500 * time.Millisecond)
	}

	return data
}

func fetchDayData(date time.Time) (DayData, error) {
	dayData := DayData{Date: date}

	metar, _ := getMETARMax(date)
	dayData.METARMax = metar

	dateCode := strings.ToUpper(date.Format("06Jan02"))
	eventTicker := fmt.Sprintf("KXHIGHLAX-%s", dateCode)

	winner, markets, err := getWinnerAndMarkets(eventTicker)
	if err != nil {
		return dayData, err
	}
	if winner == nil {
		return dayData, fmt.Errorf("no winner")
	}

	dayData.WinnerFloor = winner.FloorStrike

	for _, m := range markets {
		if m.FloorStrike >= 55 && m.FloorStrike <= 80 {
			price, err := getFirstTradePrice(m.Ticker)
			if err == nil && price > 0 {
				dayData.Brackets = append(dayData.Brackets, BracketData{
					Floor:      m.FloorStrike,
					FirstPrice: price,
					Won:        m.FloorStrike == winner.FloorStrike,
				})
			}
			time.Sleep(150 * time.Millisecond)
		}
	}

	sort.Slice(dayData.Brackets, func(i, j int) bool {
		return dayData.Brackets[i].FirstPrice > dayData.Brackets[j].FirstPrice
	})

	return dayData, nil
}

func getMETARMax(date time.Time) (int, error) {
	url := fmt.Sprintf(
		"https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py?station=LAX&data=tmpf&year1=%d&month1=%d&day1=%d&year2=%d&month2=%d&day2=%d&tz=America/Los_Angeles&format=onlycomma&latlon=no&elev=no&missing=M&trace=T&direct=no&report_type=3",
		date.Year(), int(date.Month()), date.Day(),
		date.Year(), int(date.Month()), date.Day()+1,
	)

	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(string(body), "\n")
	maxTemp := 0.0

	for _, line := range lines {
		if strings.HasPrefix(line, "LAX,") {
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

	if maxTemp == 0 {
		return 0, fmt.Errorf("no data")
	}

	return int(math.Round(maxTemp)), nil
}

func getWinnerAndMarkets(eventTicker string) (*Market, []Market, error) {
	url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets?event_ticker=%s&limit=100", eventTicker)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result MarketsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, err
	}

	var winner *Market
	for i := range result.Markets {
		if result.Markets[i].Result == "yes" {
			winner = &result.Markets[i]
			break
		}
	}

	return winner, result.Markets, nil
}

func getFirstTradePrice(ticker string) (int, error) {
	url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets/trades?ticker=%s&limit=500", ticker)

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

	sort.Slice(result.Trades, func(i, j int) bool {
		return result.Trades[i].CreatedTime.Before(result.Trades[j].CreatedTime)
	})

	return result.Trades[0].YesPrice, nil
}

