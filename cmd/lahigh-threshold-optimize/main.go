// Package main optimizes the threshold strategy in fine detail
package main

import (
	"encoding/json"
	"fmt"
	"io"
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
	Date     time.Time
	Brackets []BracketData
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
	outputFile, err = os.Create("threshold_optimize_results.txt")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer outputFile.Close()

	log("=" + strings.Repeat("=", 79))
	log("THRESHOLD STRATEGY OPTIMIZATION")
	log("Finding the optimal threshold and position sizing")
	log("Started: " + time.Now().Format("2006-01-02 15:04:05"))
	log("=" + strings.Repeat("=", 79))
	log("")

	data := fetchAllData(21)
	log(fmt.Sprintf("Fetched %d days of data", len(data)))
	log("")

	// Test fine-grained thresholds
	log("=" + strings.Repeat("=", 60))
	log("FINE-GRAINED THRESHOLD ANALYSIS (5¢ increments)")
	log("=" + strings.Repeat("=", 60))
	log("")

	log(fmt.Sprintf("%-12s %8s %8s %10s %10s %10s", "Threshold", "Trades", "Wins", "WinRate", "Profit", "Avg/Trade"))
	log(strings.Repeat("-", 70))

	bestThreshold := 0
	bestProfit := -1000.0

	for threshold := 30; threshold <= 85; threshold += 5 {
		wins, trades, profit := testThreshold(data, threshold)
		if trades == 0 {
			continue
		}

		winRate := float64(wins) / float64(trades) * 100
		avgProfit := profit / float64(trades)

		marker := ""
		if profit > bestProfit {
			bestProfit = profit
			bestThreshold = threshold
			marker = " ⭐"
		}

		log(fmt.Sprintf(">=%3d¢       %8d %8d %9.1f%% $%8.2f $%8.2f%s",
			threshold, trades, wins, winRate, profit, avgProfit, marker))
	}

	log("")
	log(fmt.Sprintf("BEST THRESHOLD: >=%d¢ with profit $%.2f", bestThreshold, bestProfit))
	log("")

	// Now test Kelly criterion position sizing
	log("=" + strings.Repeat("=", 60))
	log("POSITION SIZING ANALYSIS (at optimal threshold)")
	log("=" + strings.Repeat("=", 60))
	log("")

	wins, trades, _ := testThreshold(data, bestThreshold)
	winRate := float64(wins) / float64(trades)

	// Get average odds at this threshold
	avgOdds := getAverageOdds(data, bestThreshold)

	// Kelly criterion: f* = (bp - q) / b where b = odds, p = win prob, q = lose prob
	b := (100.0 - float64(avgOdds)) / float64(avgOdds) // payout ratio
	p := winRate
	q := 1 - p
	kellyFraction := (b*p - q) / b

	log(fmt.Sprintf("Win rate at threshold: %.1f%%", winRate*100))
	log(fmt.Sprintf("Average first price: %d¢", avgOdds))
	log(fmt.Sprintf("Payout ratio (b): %.2f", b))
	log(fmt.Sprintf("Kelly fraction: %.2f%% of bankroll", kellyFraction*100))
	log("")

	if kellyFraction > 0 {
		log("KELLY SUGGESTS: Positive edge exists!")
		log(fmt.Sprintf("  Bet %.1f%% of bankroll per trade", kellyFraction*100))
		log(fmt.Sprintf("  For $1000 bankroll: bet $%.0f per trade", kellyFraction*1000))
	} else {
		log("KELLY SUGGESTS: No edge (negative Kelly)")
		log("  Either don't trade or model is wrong")
	}
	log("")

	// Day by day breakdown of best strategy
	log("=" + strings.Repeat("=", 60))
	log("DAY-BY-DAY BREAKDOWN (Best Strategy)")
	log("=" + strings.Repeat("=", 60))
	log("")

	showDayByDay(data, bestThreshold)

	// Calculate Sharpe ratio
	log("")
	log("=" + strings.Repeat("=", 60))
	log("RISK METRICS")
	log("=" + strings.Repeat("=", 60))
	log("")

	calculateRiskMetrics(data, bestThreshold)

	log("")
	log("=" + strings.Repeat("=", 79))
	log("OPTIMIZATION COMPLETE")
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

	dateCode := strings.ToUpper(date.Format("06Jan02"))
	eventTicker := fmt.Sprintf("KXHIGHLAX-%s", dateCode)

	winner, markets, err := getWinnerAndMarkets(eventTicker)
	if err != nil {
		return dayData, err
	}
	if winner == nil {
		return dayData, fmt.Errorf("no winner")
	}

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

func testThreshold(data []DayData, threshold int) (wins, trades int, profit float64) {
	for _, d := range data {
		if len(d.Brackets) == 0 {
			continue
		}

		best := d.Brackets[0]
		if best.FirstPrice < threshold {
			continue
		}

		trades++
		contracts := 1400 / best.FirstPrice

		if best.Won {
			wins++
			profit += float64(contracts) - 14.0
		} else {
			profit -= 14.0
		}
	}
	return
}

func getAverageOdds(data []DayData, threshold int) int {
	total := 0
	count := 0

	for _, d := range data {
		if len(d.Brackets) == 0 {
			continue
		}
		best := d.Brackets[0]
		if best.FirstPrice >= threshold {
			total += best.FirstPrice
			count++
		}
	}

	if count == 0 {
		return 50
	}
	return total / count
}

func showDayByDay(data []DayData, threshold int) {
	log(fmt.Sprintf("%-12s %10s %8s %10s %10s", "Date", "Top Price", "Won?", "Contracts", "Profit"))
	log(strings.Repeat("-", 60))

	var running float64
	for _, d := range data {
		if len(d.Brackets) == 0 {
			continue
		}

		best := d.Brackets[0]
		if best.FirstPrice < threshold {
			log(fmt.Sprintf("%-12s %9d¢ %8s %10s %10s (skipped)",
				d.Date.Format("Jan 2"), best.FirstPrice, "-", "-", "-"))
			continue
		}

		contracts := 1400 / best.FirstPrice
		var profit float64
		result := "❌"

		if best.Won {
			profit = float64(contracts) - 14.0
			result = "✅"
		} else {
			profit = -14.0
		}

		running += profit

		log(fmt.Sprintf("%-12s %9d¢ %8s %10d $%8.2f (running: $%.2f)",
			d.Date.Format("Jan 2"), best.FirstPrice, result, contracts, profit, running))
	}
}

func calculateRiskMetrics(data []DayData, threshold int) {
	var profits []float64

	for _, d := range data {
		if len(d.Brackets) == 0 {
			continue
		}

		best := d.Brackets[0]
		if best.FirstPrice < threshold {
			continue
		}

		contracts := 1400 / best.FirstPrice
		if best.Won {
			profits = append(profits, float64(contracts)-14.0)
		} else {
			profits = append(profits, -14.0)
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
	stdDev := 0.0
	if len(profits) > 1 {
		stdDev = sqrt(variance / float64(len(profits)-1))
	}

	// Sharpe ratio (annualized, assuming 250 trading days)
	sharpe := 0.0
	if stdDev > 0 {
		sharpe = (mean / stdDev) * sqrt(250)
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

	// Win streak and loss streak
	maxWinStreak := 0
	maxLossStreak := 0
	currentStreak := 0
	lastWin := true

	for _, p := range profits {
		if p > 0 {
			if lastWin {
				currentStreak++
			} else {
				currentStreak = 1
				lastWin = true
			}
			if currentStreak > maxWinStreak {
				maxWinStreak = currentStreak
			}
		} else {
			if !lastWin {
				currentStreak++
			} else {
				currentStreak = 1
				lastWin = false
			}
			if currentStreak > maxLossStreak {
				maxLossStreak = currentStreak
			}
		}
	}

	log(fmt.Sprintf("Number of trades: %d", len(profits)))
	log(fmt.Sprintf("Mean profit/trade: $%.2f", mean))
	log(fmt.Sprintf("Std deviation: $%.2f", stdDev))
	log(fmt.Sprintf("Sharpe ratio (annualized): %.2f", sharpe))
	log(fmt.Sprintf("Max drawdown: $%.2f", maxDD))
	log(fmt.Sprintf("Max win streak: %d", maxWinStreak))
	log(fmt.Sprintf("Max loss streak: %d", maxLossStreak))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton's method
	z := x
	for i := 0; i < 100; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
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

