// Package main searches for specific edge conditions
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
	outputFile, err = os.Create("edge_finder_results.txt")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer outputFile.Close()

	log("=" + strings.Repeat("=", 79))
	log("EDGE FINDER: Searching for Profitable Conditions")
	log("Started: " + time.Now().Format("2006-01-02 15:04:05"))
	log("=" + strings.Repeat("=", 79))
	log("")

	data := fetchAllData(21)
	log(fmt.Sprintf("Fetched %d days of data", len(data)))
	log("")

	// Strategy 1: When market is UNCERTAIN (no clear favorite)
	testUncertainMarket(data)

	// Strategy 2: When top price < 50¢ (market unsure)
	testUnsureMarket(data)

	// Strategy 3: Combine METAR signal with market price
	testCombinedSignal(data)

	// Strategy 4: Look for mispricing based on METAR
	testMispricingDetection(data)

	// Strategy 5: Ensemble approach
	testEnsemble(data)

	log("")
	log("=" + strings.Repeat("=", 79))
	log("ANALYSIS COMPLETE")
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

	// Get METAR
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

func testUncertainMarket(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("STRATEGY 1: When Market is UNCERTAIN")
	log("(Gap between 1st and 2nd < 15¢)")
	log("=" + strings.Repeat("=", 60))
	log("")

	wins := 0
	trades := 0
	var profits []float64

	for _, d := range data {
		if len(d.Brackets) < 2 {
			continue
		}

		gap := d.Brackets[0].FirstPrice - d.Brackets[1].FirstPrice

		// Market is uncertain when gap is small
		if gap >= 15 {
			continue // Skip confident markets
		}

		trades++

		// In uncertain markets, bet on both top brackets
		b1 := d.Brackets[0]
		b2 := d.Brackets[1]

		c1 := 700 / b1.FirstPrice
		c2 := 700 / b2.FirstPrice
		cost := float64(c1*b1.FirstPrice+c2*b2.FirstPrice) / 100

		var profit float64
		if b1.Won {
			profit = float64(c1) - cost
			wins++
		} else if b2.Won {
			profit = float64(c2) - cost
			wins++
		} else {
			profit = -cost
		}
		profits = append(profits, profit)
	}

	if trades > 0 {
		total := sum(profits)
		log(fmt.Sprintf("Uncertain days: %d/%d", trades, len(data)))
		log(fmt.Sprintf("Win rate: %.1f%% (%d/%d)", float64(wins)/float64(trades)*100, wins, trades))
		log(fmt.Sprintf("Total profit: $%.2f", total))
		log(fmt.Sprintf("Avg profit: $%.2f", total/float64(trades)))
	}
	log("")
}

func testUnsureMarket(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("STRATEGY 2: When Market is UNSURE (top < 50¢)")
	log("Spread across top 3 brackets")
	log("=" + strings.Repeat("=", 60))
	log("")

	wins := 0
	trades := 0
	var profits []float64

	for _, d := range data {
		if len(d.Brackets) < 3 {
			continue
		}

		// Only trade when market is unsure
		if d.Brackets[0].FirstPrice >= 50 {
			continue
		}

		trades++

		// Spread across top 3
		b1, b2, b3 := d.Brackets[0], d.Brackets[1], d.Brackets[2]
		c1 := 500 / b1.FirstPrice
		c2 := 500 / b2.FirstPrice
		c3 := 400 / b3.FirstPrice
		cost := float64(c1*b1.FirstPrice+c2*b2.FirstPrice+c3*b3.FirstPrice) / 100

		var profit float64
		if b1.Won {
			profit = float64(c1) - cost
			wins++
		} else if b2.Won {
			profit = float64(c2) - cost
			wins++
		} else if b3.Won {
			profit = float64(c3) - cost
			wins++
		} else {
			profit = -cost
		}
		profits = append(profits, profit)
	}

	if trades > 0 {
		total := sum(profits)
		log(fmt.Sprintf("Unsure days: %d/%d", trades, len(data)))
		log(fmt.Sprintf("Win rate: %.1f%% (%d/%d)", float64(wins)/float64(trades)*100, wins, trades))
		log(fmt.Sprintf("Total profit: $%.2f", total))
	}
	log("")
}

func testCombinedSignal(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("STRATEGY 3: Combined Signal")
	log("(Market favorite AND matches METAR prediction)")
	log("=" + strings.Repeat("=", 60))
	log("")

	wins := 0
	trades := 0
	var profits []float64

	for _, d := range data {
		if len(d.Brackets) == 0 || d.METARMax == 0 {
			continue
		}

		marketFav := d.Brackets[0]
		metarPred := ((d.METARMax) / 2) * 2 // Round to bracket

		// Only trade when they agree
		if marketFav.Floor != metarPred && marketFav.Floor != metarPred+2 && marketFav.Floor != metarPred-2 {
			continue
		}

		trades++
		contracts := 1400 / marketFav.FirstPrice

		if marketFav.Won {
			wins++
			profits = append(profits, float64(contracts)-14.0)
		} else {
			profits = append(profits, -14.0)
		}
	}

	if trades > 0 {
		total := sum(profits)
		log(fmt.Sprintf("Agreement days: %d/%d", trades, len(data)))
		log(fmt.Sprintf("Win rate: %.1f%% (%d/%d)", float64(wins)/float64(trades)*100, wins, trades))
		log(fmt.Sprintf("Total profit: $%.2f", total))
	}
	log("")
}

func testMispricingDetection(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("STRATEGY 4: Detect Mispricing")
	log("(METAR bracket is underpriced relative to market)")
	log("=" + strings.Repeat("=", 60))
	log("")

	wins := 0
	trades := 0
	var profits []float64

	for _, d := range data {
		if len(d.Brackets) < 2 || d.METARMax == 0 {
			continue
		}

		metarFloor := ((d.METARMax) / 2) * 2

		// Find METAR bracket's price
		var metarBracket *BracketData
		var metarPrice int
		for i := range d.Brackets {
			if d.Brackets[i].Floor == metarFloor {
				metarBracket = &d.Brackets[i]
				metarPrice = d.Brackets[i].FirstPrice
				break
			}
		}

		if metarBracket == nil {
			continue
		}

		// Is METAR bracket underpriced? (not the favorite but should be)
		topPrice := d.Brackets[0].FirstPrice
		if metarPrice >= topPrice-10 {
			continue // Not underpriced enough
		}

		// METAR bracket is significantly cheaper than favorite - BUY IT
		trades++
		contracts := 1400 / metarPrice

		if metarBracket.Won {
			wins++
			profits = append(profits, float64(contracts)-14.0)
		} else {
			profits = append(profits, -14.0)
		}
	}

	if trades > 0 {
		total := sum(profits)
		log(fmt.Sprintf("Mispriced days: %d/%d", trades, len(data)))
		log(fmt.Sprintf("Win rate: %.1f%% (%d/%d)", float64(wins)/float64(trades)*100, wins, trades))
		log(fmt.Sprintf("Total profit: $%.2f", total))
	} else {
		log("No mispricing opportunities found")
	}
	log("")
}

func testEnsemble(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("STRATEGY 5: Ensemble (Vote-Based)")
	log("(Trade when 2+ signals agree)")
	log("=" + strings.Repeat("=", 60))
	log("")

	wins := 0
	trades := 0
	var profits []float64

	for _, d := range data {
		if len(d.Brackets) < 2 || d.METARMax == 0 {
			continue
		}

		// Signal 1: Market favorite
		marketPick := d.Brackets[0].Floor

		// Signal 2: METAR prediction
		metarPick := ((d.METARMax) / 2) * 2

		// Signal 3: 2nd best bracket
		secondPick := d.Brackets[1].Floor

		// Count votes for each bracket
		votes := make(map[int]int)
		votes[marketPick]++
		votes[metarPick]++
		votes[secondPick]++

		// Find bracket with most votes
		bestBracket := 0
		maxVotes := 0
		for floor, v := range votes {
			if v > maxVotes {
				maxVotes = v
				bestBracket = floor
			}
		}

		// Only trade if 2+ signals agree
		if maxVotes < 2 {
			continue
		}

		// Find price for chosen bracket
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

		trades++
		contracts := 1400 / price

		if won {
			wins++
			profits = append(profits, float64(contracts)-14.0)
		} else {
			profits = append(profits, -14.0)
		}
	}

	if trades > 0 {
		total := sum(profits)
		log(fmt.Sprintf("Consensus days: %d/%d", trades, len(data)))
		log(fmt.Sprintf("Win rate: %.1f%% (%d/%d)", float64(wins)/float64(trades)*100, wins, trades))
		log(fmt.Sprintf("Total profit: $%.2f", total))
	}
	log("")
}

func sum(nums []float64) float64 {
	total := 0.0
	for _, n := range nums {
		total += n
	}
	return total
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

