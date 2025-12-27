// Package main implements and validates the best strategies found
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
	Date        time.Time
	METARMax    int
	Brackets    []BracketData
	WinnerFloor int
}

type TradeResult struct {
	Date      time.Time
	Strategy  string
	Bracket   int
	Price     int
	Contracts int
	Won       bool
	Profit    float64
	Reason    string
}

var loc *time.Location
var httpClient = &http.Client{Timeout: 15 * time.Second}
var outputFile *os.File

func init() {
	var err error
	loc, err = time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.UTC
	}
}

func main() {
	var err error
	outputFile, err = os.Create("final_strategy_results.txt")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer outputFile.Close()

	log("=" + strings.Repeat("=", 79))
	log("FINAL STRATEGY VALIDATION")
	log("Testing the Ensemble strategy in detail")
	log("Started: " + time.Now().Format("2006-01-02 15:04:05"))
	log("=" + strings.Repeat("=", 79))
	log("")

	data := fetchAllData(21)
	log(fmt.Sprintf("Fetched %d days of data", len(data)))
	log("")

	// Run detailed ensemble strategy
	results := runEnsembleDetailed(data)

	// Print detailed results
	printDetailedResults(results)

	// Calculate risk metrics
	calculateRiskMetrics(results)

	// Generate recommendations
	generateRecommendations(results)

	log("")
	log("=" + strings.Repeat("=", 79))
	log("VALIDATION COMPLETE")
	log("=" + strings.Repeat("=", 79))
}

func log(msg string) {
	fmt.Println(msg)
	outputFile.WriteString(msg + "\n")
}

func fetchAllData(days int) []DayData {
	var data []DayData
	today := time.Now().In(loc)

	for i := 1; i <= days; i++ {
		targetDate := today.AddDate(0, 0, -i)
		fmt.Printf("Fetching %s... ", targetDate.Format("Jan 2"))

		dayData, err := fetchDayData(targetDate)
		if err != nil {
			fmt.Printf("❌\n")
			continue
		}
		fmt.Printf("✅\n")
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

func runEnsembleDetailed(data []DayData) []TradeResult {
	var results []TradeResult

	for _, d := range data {
		if len(d.Brackets) < 2 || d.METARMax == 0 {
			results = append(results, TradeResult{
				Date:    d.Date,
				Reason:  "SKIP: insufficient data",
				Profit:  0,
			})
			continue
		}

		// Signal 1: Market favorite
		marketPick := d.Brackets[0].Floor

		// Signal 2: METAR prediction (round to bracket)
		metarPick := ((d.METARMax) / 2) * 2

		// Signal 3: 2nd best bracket
		secondPick := d.Brackets[1].Floor

		// Count votes
		votes := make(map[int]int)
		votes[marketPick]++
		votes[metarPick]++
		votes[secondPick]++

		// Find best bracket
		bestBracket := 0
		maxVotes := 0
		for floor, v := range votes {
			if v > maxVotes {
				maxVotes = v
				bestBracket = floor
			}
		}

		// Generate reason string
		var reasons []string
		if marketPick == bestBracket {
			reasons = append(reasons, "market")
		}
		if metarPick == bestBracket {
			reasons = append(reasons, "METAR")
		}
		if secondPick == bestBracket {
			reasons = append(reasons, "2nd")
		}

		if maxVotes < 2 {
			results = append(results, TradeResult{
				Date:   d.Date,
				Reason: fmt.Sprintf("SKIP: no consensus (market=%d, METAR=%d, 2nd=%d)", marketPick, metarPick, secondPick),
				Profit: 0,
			})
			continue
		}

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
		var profit float64
		if won {
			profit = float64(contracts) - 14.0
		} else {
			profit = -14.0
		}

		results = append(results, TradeResult{
			Date:      d.Date,
			Strategy:  "ENSEMBLE",
			Bracket:   bestBracket,
			Price:     price,
			Contracts: contracts,
			Won:       won,
			Profit:    profit,
			Reason:    fmt.Sprintf("TRADE: %s agree on %d°", strings.Join(reasons, "+"), bestBracket),
		})
	}

	return results
}

func printDetailedResults(results []TradeResult) {
	log("=" + strings.Repeat("=", 70))
	log("DAY-BY-DAY BREAKDOWN")
	log("=" + strings.Repeat("=", 70))
	log("")

	log(fmt.Sprintf("%-12s %-8s %6s %6s %10s %s", "Date", "Result", "Price", "Cont", "Profit", "Reason"))
	log(strings.Repeat("-", 80))

	var running float64
	trades := 0
	wins := 0

	for _, r := range results {
		if r.Strategy == "" {
			log(fmt.Sprintf("%-12s %-8s %6s %6s %10s %s",
				r.Date.Format("Jan 2"), "-", "-", "-", "-", r.Reason))
			continue
		}

		trades++
		result := "❌ LOSS"
		if r.Won {
			result = "✅ WIN"
			wins++
		}
		running += r.Profit

		log(fmt.Sprintf("%-12s %-8s %5d¢ %6d $%8.2f %s (running: $%.2f)",
			r.Date.Format("Jan 2"), result, r.Price, r.Contracts, r.Profit, r.Reason, running))
	}

	log("")
	log(fmt.Sprintf("Total trades: %d", trades))
	log(fmt.Sprintf("Wins: %d (%.1f%%)", wins, float64(wins)/float64(trades)*100))
	log(fmt.Sprintf("Total profit: $%.2f", running))
	log("")
}

func calculateRiskMetrics(results []TradeResult) {
	log("=" + strings.Repeat("=", 70))
	log("RISK METRICS")
	log("=" + strings.Repeat("=", 70))
	log("")

	var profits []float64
	for _, r := range results {
		if r.Strategy != "" {
			profits = append(profits, r.Profit)
		}
	}

	if len(profits) == 0 {
		log("No trades to analyze")
		return
	}

	// Mean
	sum := 0.0
	for _, p := range profits {
		sum += p
	}
	mean := sum / float64(len(profits))

	// Std dev
	variance := 0.0
	for _, p := range profits {
		variance += (p - mean) * (p - mean)
	}
	stdDev := math.Sqrt(variance / float64(len(profits)))

	// Sharpe
	sharpe := 0.0
	if stdDev > 0 {
		sharpe = (mean / stdDev) * math.Sqrt(250)
	}

	// Max drawdown
	peak := 0.0
	maxDD := 0.0
	running := 0.0
	for _, p := range profits {
		running += p
		if running > peak {
			peak = running
		}
		dd := peak - running
		if dd > maxDD {
			maxDD = dd
		}
	}

	// Win/loss stats
	var winProfits, lossProfits []float64
	for _, p := range profits {
		if p > 0 {
			winProfits = append(winProfits, p)
		} else {
			lossProfits = append(lossProfits, p)
		}
	}

	avgWin := 0.0
	if len(winProfits) > 0 {
		for _, w := range winProfits {
			avgWin += w
		}
		avgWin /= float64(len(winProfits))
	}

	avgLoss := 0.0
	if len(lossProfits) > 0 {
		for _, l := range lossProfits {
			avgLoss += l
		}
		avgLoss /= float64(len(lossProfits))
	}

	log(fmt.Sprintf("Mean profit/trade: $%.2f", mean))
	log(fmt.Sprintf("Std deviation: $%.2f", stdDev))
	log(fmt.Sprintf("Sharpe ratio (annualized): %.2f", sharpe))
	log(fmt.Sprintf("Max drawdown: $%.2f", maxDD))
	log(fmt.Sprintf("Avg winning trade: $%.2f", avgWin))
	log(fmt.Sprintf("Avg losing trade: $%.2f", avgLoss))
	log(fmt.Sprintf("Win/Loss ratio: %.2f", avgWin/math.Abs(avgLoss)))
	log("")

	// Kelly criterion
	winRate := float64(len(winProfits)) / float64(len(profits))
	if avgLoss != 0 {
		b := avgWin / math.Abs(avgLoss)
		p := winRate
		q := 1 - p
		kellyFraction := (b*p - q) / b

		log(fmt.Sprintf("Kelly fraction: %.2f%%", kellyFraction*100))
		if kellyFraction > 0 {
			log("✅ POSITIVE KELLY - Strategy has edge!")
			log(fmt.Sprintf("   Suggested bet size: %.1f%% of bankroll", kellyFraction*100))
		} else {
			log("❌ NEGATIVE KELLY - No edge detected")
		}
	}
	log("")
}

func generateRecommendations(results []TradeResult) {
	log("=" + strings.Repeat("=", 70))
	log("FINAL RECOMMENDATIONS")
	log("=" + strings.Repeat("=", 70))
	log("")

	trades := 0
	wins := 0
	var totalProfit float64

	for _, r := range results {
		if r.Strategy != "" {
			trades++
			if r.Won {
				wins++
			}
			totalProfit += r.Profit
		}
	}

	winRate := float64(wins) / float64(trades) * 100
	avgProfit := totalProfit / float64(trades)

	log("ENSEMBLE STRATEGY SUMMARY:")
	log(fmt.Sprintf("  • Trade when 2+ signals agree (market, METAR, 2nd best)"))
	log(fmt.Sprintf("  • Win rate: %.1f%%", winRate))
	log(fmt.Sprintf("  • Avg profit: $%.2f/trade", avgProfit))
	log(fmt.Sprintf("  • Total profit (21 days): $%.2f", totalProfit))
	log("")

	if winRate >= 75 && avgProfit > 0 {
		log("✅ STRATEGY VALIDATED!")
		log("")
		log("IMPLEMENTATION:")
		log("  1. Check if market favorite matches METAR prediction")
		log("  2. Check if 2nd best bracket agrees with either")
		log("  3. If 2+ agree → BUY that bracket")
		log("  4. If no consensus → SKIP the day")
		log("")
		log("POSITION SIZING:")
		log("  • Conservative: $14/day fixed")
		log("  • Moderate: 2-5% of bankroll")
		log("  • Aggressive: Kelly fraction of bankroll")
	} else if avgProfit > 0 {
		log("⚠️ MARGINAL EDGE")
		log("  Strategy shows profit but sample size is small")
		log("  Consider paper trading for 30 more days")
	} else {
		log("❌ NO RELIABLE EDGE FOUND")
		log("  Current strategies don't show consistent profit")
		log("  Need better prediction model or more data")
	}
	log("")
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

