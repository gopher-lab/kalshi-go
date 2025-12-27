// Package main tests following the market's highest-priced bracket
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
	Date     time.Time
	Brackets []BracketData
	Winner   int
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
	outputFile, err = os.Create("market_follow_results.txt")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer outputFile.Close()

	log("=" + strings.Repeat("=", 79))
	log("MARKET-FOLLOWING STRATEGY ANALYSIS")
	log("Strategy: Bet on bracket with HIGHEST first trade price")
	log("Started: " + time.Now().Format("2006-01-02 15:04:05"))
	log("=" + strings.Repeat("=", 79))
	log("")

	data := fetchAllData(21)
	log(fmt.Sprintf("Fetched %d days of data", len(data)))
	log("")

	// Strategy 1: Always bet on highest priced bracket
	testHighestPrice(data)

	// Strategy 2: Bet on highest priced bracket only when price > threshold
	testThresholdStrategies(data)

	// Strategy 3: Bet on top 2 brackets (hedge)
	testTop2Hedge(data)

	// Strategy 4: Wait for high confidence (>70¢)
	testHighConfidence(data)

	// Strategy 5: Spread across top 3
	testTop3Spread(data)

	log("")
	log("=" + strings.Repeat("=", 79))
	log("COMPLETE")
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
		fmt.Printf("✅ %d brackets\n", len(dayData.Brackets))
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

	dayData.Winner = winner.FloorStrike

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

	// Sort by price descending
	sort.Slice(dayData.Brackets, func(i, j int) bool {
		return dayData.Brackets[i].FirstPrice > dayData.Brackets[j].FirstPrice
	})

	return dayData, nil
}

func testHighestPrice(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("STRATEGY 1: Bet on Highest-Priced Bracket")
	log("=" + strings.Repeat("=", 60))
	log("")

	wins := 0
	var profits []float64

	for _, d := range data {
		if len(d.Brackets) == 0 {
			continue
		}

		best := d.Brackets[0] // Highest priced
		contracts := 1400 / best.FirstPrice

		if best.Won {
			wins++
			profits = append(profits, float64(contracts)-14.0)
		} else {
			profits = append(profits, -14.0)
		}
	}

	totalProfit := sum(profits)
	log(fmt.Sprintf("Win rate: %.1f%% (%d/%d)", float64(wins)/float64(len(profits))*100, wins, len(profits)))
	log(fmt.Sprintf("Total profit: $%.2f", totalProfit))
	log(fmt.Sprintf("Avg profit/day: $%.2f", totalProfit/float64(len(profits))))
	log("")
}

func testThresholdStrategies(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("STRATEGY 2: Bet on Highest ONLY When Price > Threshold")
	log("=" + strings.Repeat("=", 60))
	log("")

	thresholds := []int{40, 50, 60, 70, 80}

	for _, threshold := range thresholds {
		wins := 0
		trades := 0
		var profits []float64

		for _, d := range data {
			if len(d.Brackets) == 0 {
				continue
			}

			best := d.Brackets[0]
			if best.FirstPrice < threshold {
				continue // Skip low confidence days
			}

			trades++
			contracts := 1400 / best.FirstPrice

			if best.Won {
				wins++
				profits = append(profits, float64(contracts)-14.0)
			} else {
				profits = append(profits, -14.0)
			}
		}

		if trades > 0 {
			totalProfit := sum(profits)
			log(fmt.Sprintf("Threshold >%d¢: Win=%.1f%% (%d/%d), Profit=$%.2f, Avg=$%.2f",
				threshold, float64(wins)/float64(trades)*100, wins, trades, totalProfit, totalProfit/float64(trades)))
		}
	}
	log("")
}

func testTop2Hedge(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("STRATEGY 3: Hedge on Top 2 Highest-Priced Brackets")
	log("=" + strings.Repeat("=", 60))
	log("")

	splits := [][2]int{{70, 30}, {60, 40}, {50, 50}}

	for _, split := range splits {
		wins := 0
		var profits []float64

		for _, d := range data {
			if len(d.Brackets) < 2 {
				continue
			}

			first := d.Brackets[0]
			second := d.Brackets[1]

			budget1 := float64(split[0]) / 100.0 * 14.0
			budget2 := float64(split[1]) / 100.0 * 14.0

			contracts1 := int(budget1 * 100) / first.FirstPrice
			contracts2 := int(budget2 * 100) / second.FirstPrice

			totalCost := float64(contracts1*first.FirstPrice+contracts2*second.FirstPrice) / 100

			var profit float64
			if first.Won {
				profit = float64(contracts1) - totalCost
				wins++
			} else if second.Won {
				profit = float64(contracts2) - totalCost
				wins++
			} else {
				profit = -totalCost
			}
			profits = append(profits, profit)
		}

		totalProfit := sum(profits)
		log(fmt.Sprintf("Split %d/%d: Win=%.1f%% (%d/%d), Profit=$%.2f, Avg=$%.2f",
			split[0], split[1], float64(wins)/float64(len(profits))*100, wins, len(profits), totalProfit, totalProfit/float64(len(profits))))
	}
	log("")
}

func testHighConfidence(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("STRATEGY 4: Only Trade When Top Bracket > 70¢")
	log("=" + strings.Repeat("=", 60))
	log("")

	wins := 0
	trades := 0
	var profits []float64

	for _, d := range data {
		if len(d.Brackets) == 0 {
			continue
		}

		best := d.Brackets[0]
		if best.FirstPrice < 70 {
			continue
		}

		trades++
		contracts := 1400 / best.FirstPrice

		if best.Won {
			wins++
			profits = append(profits, float64(contracts)-14.0)
		} else {
			profits = append(profits, -14.0)
		}
	}

	if trades > 0 {
		totalProfit := sum(profits)
		log(fmt.Sprintf("Trades: %d/%d days (%.1f%%)", trades, len(data), float64(trades)/float64(len(data))*100))
		log(fmt.Sprintf("Win rate: %.1f%% (%d/%d)", float64(wins)/float64(trades)*100, wins, trades))
		log(fmt.Sprintf("Total profit: $%.2f", totalProfit))
		log(fmt.Sprintf("Avg profit/trade: $%.2f", totalProfit/float64(trades)))
	} else {
		log("No trades met criteria")
	}
	log("")
}

func testTop3Spread(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("STRATEGY 5: Spread Across Top 3 Brackets")
	log("=" + strings.Repeat("=", 60))
	log("")

	// Weight by price (higher price = more allocation)
	wins := 0
	var profits []float64

	for _, d := range data {
		if len(d.Brackets) < 3 {
			continue
		}

		top3 := d.Brackets[:3]

		// Proportional allocation based on price
		totalPrice := 0
		for _, b := range top3 {
			totalPrice += b.FirstPrice
		}

		var contracts []int
		var costs []float64

		for _, b := range top3 {
			allocation := float64(b.FirstPrice) / float64(totalPrice) * 14.0
			c := int(allocation * 100) / b.FirstPrice
			contracts = append(contracts, c)
			costs = append(costs, float64(c*b.FirstPrice)/100)
		}

		totalCost := 0.0
		for _, c := range costs {
			totalCost += c
		}

		var profit float64
		won := false
		for i, b := range top3 {
			if b.Won {
				profit = float64(contracts[i]) - totalCost
				won = true
				wins++
				break
			}
		}
		if !won {
			profit = -totalCost
		}
		profits = append(profits, profit)
	}

	totalProfit := sum(profits)
	log(fmt.Sprintf("Win rate: %.1f%% (%d/%d)", float64(wins)/float64(len(profits))*100, wins, len(profits)))
	log(fmt.Sprintf("Total profit: $%.2f", totalProfit))
	log(fmt.Sprintf("Avg profit/day: $%.2f", totalProfit/float64(len(profits))))
	log("")
}

func sum(nums []float64) float64 {
	total := 0.0
	for _, n := range nums {
		total += n
	}
	return total
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

