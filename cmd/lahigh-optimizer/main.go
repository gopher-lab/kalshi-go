// Package main runs extensive strategy optimization experiments
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

type DayData struct {
	Date          time.Time
	METARMax      int
	WinningFloor  int
	AllMarkets    []Market
	FirstPrices   map[int]int // floor -> first trade price
}

type StrategyResult struct {
	Name           string
	Description    string
	TotalProfit    float64
	WinRate        float64
	AvgProfit      float64
	MaxDrawdown    float64
	SharpeRatio    float64
	DaysAnalyzed   int
}

var loc *time.Location
var httpClient = &http.Client{Timeout: 15 * time.Second}
var results []StrategyResult
var outputFile *os.File

func init() {
	var err error
	loc, err = time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.UTC
	}
}

func main() {
	// Create output file
	var err error
	outputFile, err = os.Create("optimization_results.txt")
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		return
	}
	defer outputFile.Close()

	log("=" + strings.Repeat("=", 79))
	log("LA HIGH TEMPERATURE STRATEGY OPTIMIZER")
	log("Started: " + time.Now().Format("2006-01-02 15:04:05"))
	log("=" + strings.Repeat("=", 79))
	log("")

	// Step 1: Fetch all historical data
	log("PHASE 1: Fetching historical data...")
	data := fetchAllData(21) // 3 weeks of data
	log(fmt.Sprintf("Fetched %d days of data", len(data)))
	log("")

	// Step 2: Run calibration experiments
	log("PHASE 2: Testing calibration values...")
	testCalibrations(data)
	log("")

	// Step 3: Run hedge ratio experiments
	log("PHASE 3: Testing hedge ratios...")
	testHedgeRatios(data)
	log("")

	// Step 4: Run multi-bracket experiments
	log("PHASE 4: Testing multi-bracket strategies...")
	testMultiBracket(data)
	log("")

	// Step 5: Test market-following strategies
	log("PHASE 5: Testing market-following strategies...")
	testMarketFollowing(data)
	log("")

	// Step 6: Test adaptive strategies
	log("PHASE 6: Testing adaptive strategies...")
	testAdaptiveStrategies(data)
	log("")

	// Print final rankings
	printFinalRankings()

	log("")
	log("=" + strings.Repeat("=", 79))
	log("OPTIMIZATION COMPLETE")
	log("Finished: " + time.Now().Format("2006-01-02 15:04:05"))
	log("Results saved to: optimization_results.txt")
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
		fmt.Printf("  Fetching %s... ", targetDate.Format("Jan 2"))

		dayData, err := fetchDayData(targetDate)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			continue
		}
		fmt.Printf("✅ METAR=%d°, Winner=%d°\n", dayData.METARMax, dayData.WinningFloor)
		data = append(data, dayData)
		time.Sleep(600 * time.Millisecond) // Rate limiting
	}

	return data
}

func fetchDayData(date time.Time) (DayData, error) {
	dayData := DayData{
		Date:        date,
		FirstPrices: make(map[int]int),
	}

	// Get METAR max
	metarMax, err := getMETARMax(date)
	if err != nil {
		return dayData, fmt.Errorf("METAR: %v", err)
	}
	dayData.METARMax = metarMax

	// Get markets
	dateCode := strings.ToUpper(date.Format("06Jan02"))
	eventTicker := fmt.Sprintf("KXHIGHLAX-%s", dateCode)

	winner, markets, err := getWinnerAndMarkets(eventTicker)
	if err != nil {
		return dayData, fmt.Errorf("markets: %v", err)
	}
	if winner == nil {
		return dayData, fmt.Errorf("no winner")
	}

	dayData.WinningFloor = winner.FloorStrike
	dayData.AllMarkets = markets

	// Get first prices for all relevant brackets
	for _, m := range markets {
		if m.FloorStrike >= 55 && m.FloorStrike <= 80 {
			price, err := getFirstTradePrice(m.Ticker)
			if err == nil && price > 0 {
				dayData.FirstPrices[m.FloorStrike] = price
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	return dayData, nil
}

func testCalibrations(data []DayData) {
	calibrations := []int{-1, 0, 1, 2, 3}

	for _, cal := range calibrations {
		result := runCalibrationTest(data, cal)
		results = append(results, result)
		log(fmt.Sprintf("  Calibration %+d°F: Accuracy=%.1f%%, Profit=$%.2f, Sharpe=%.2f",
			cal, result.WinRate*100, result.TotalProfit, result.SharpeRatio))
	}
}

func runCalibrationTest(data []DayData, calibration int) StrategyResult {
	name := fmt.Sprintf("Calibration_%+d", calibration)
	var profits []float64
	wins := 0

	for _, d := range data {
		predictedFloor := ((d.METARMax + calibration) / 2) * 2
		if (d.METARMax+calibration)%2 == 1 {
			predictedFloor = d.METARMax + calibration - 1
		}

		price, ok := d.FirstPrices[predictedFloor]
		if !ok || price == 0 {
			price = 50 // Default if no price found
		}

		contracts := 1400 / price
		if d.WinningFloor == predictedFloor {
			profit := float64(contracts) - 14.0
			profits = append(profits, profit)
			wins++
		} else {
			profits = append(profits, -14.0)
		}
	}

	return calculateStats(name, fmt.Sprintf("METAR + %d°F calibration", calibration), profits, wins)
}

func testHedgeRatios(data []DayData) {
	ratios := [][2]int{{100, 0}, {80, 20}, {70, 30}, {60, 40}, {50, 50}}

	for _, ratio := range ratios {
		result := runHedgeTest(data, ratio[0], ratio[1])
		results = append(results, result)
		log(fmt.Sprintf("  Hedge %d/%d: Accuracy=%.1f%%, Profit=$%.2f, Sharpe=%.2f",
			ratio[0], ratio[1], result.WinRate*100, result.TotalProfit, result.SharpeRatio))
	}
}

func runHedgeTest(data []DayData, thesisPct, protectPct int) StrategyResult {
	name := fmt.Sprintf("Hedge_%d_%d", thesisPct, protectPct)
	var profits []float64
	wins := 0

	thesisBudget := float64(thesisPct) / 100.0 * 14.0
	protectBudget := float64(protectPct) / 100.0 * 14.0

	for _, d := range data {
		// Use +1°F calibration
		predictedFloor := ((d.METARMax + 1) / 2) * 2
		protectFloor := predictedFloor - 2

		thesisPrice, ok1 := d.FirstPrices[predictedFloor]
		protectPrice, ok2 := d.FirstPrices[protectFloor]

		if !ok1 || thesisPrice == 0 {
			thesisPrice = 50
		}
		if !ok2 || protectPrice == 0 {
			protectPrice = 50
		}

		thesisContracts := int(thesisBudget * 100) / thesisPrice
		protectContracts := int(protectBudget * 100) / protectPrice
		totalCost := float64(thesisContracts*thesisPrice+protectContracts*protectPrice) / 100

		var profit float64
		if d.WinningFloor == predictedFloor {
			profit = float64(thesisContracts) - totalCost
			wins++
		} else if d.WinningFloor == protectFloor {
			profit = float64(protectContracts) - totalCost
		} else {
			profit = -totalCost
		}
		profits = append(profits, profit)
	}

	return calculateStats(name, fmt.Sprintf("%d%% thesis, %d%% protection (-2°F)", thesisPct, protectPct), profits, wins)
}

func testMultiBracket(data []DayData) {
	// Test spreading across multiple brackets
	spreads := []int{2, 3, 4, 5}

	for _, spread := range spreads {
		result := runMultiBracketTest(data, spread)
		results = append(results, result)
		log(fmt.Sprintf("  %d-bracket spread: HitRate=%.1f%%, Profit=$%.2f, Sharpe=%.2f",
			spread, result.WinRate*100, result.TotalProfit, result.SharpeRatio))
	}
}

func runMultiBracketTest(data []DayData, numBrackets int) StrategyResult {
	name := fmt.Sprintf("Spread_%d_brackets", numBrackets)
	var profits []float64
	hits := 0

	budgetPerBracket := 14.0 / float64(numBrackets)

	for _, d := range data {
		predictedFloor := ((d.METARMax + 1) / 2) * 2
		startFloor := predictedFloor - 2*(numBrackets/2)

		var totalCost float64
		var payout float64

		for i := 0; i < numBrackets; i++ {
			floor := startFloor + i*2
			price, ok := d.FirstPrices[floor]
			if !ok || price == 0 {
				price = 50
			}

			contracts := int(budgetPerBracket * 100) / price
			totalCost += float64(contracts*price) / 100

			if d.WinningFloor == floor {
				payout = float64(contracts)
				hits++
			}
		}

		profits = append(profits, payout-totalCost)
	}

	return calculateStats(name, fmt.Sprintf("Equal spread across %d brackets centered on prediction", numBrackets), profits, hits)
}

func testMarketFollowing(data []DayData) {
	// Strategy: bet on the bracket with lowest first price (market thinks most likely)
	var profits []float64
	hits := 0

	for _, d := range data {
		// Find cheapest bracket (highest probability according to market)
		lowestPrice := 100
		cheapestFloor := 0
		for floor, price := range d.FirstPrices {
			if price < lowestPrice && price > 0 {
				lowestPrice = price
				cheapestFloor = floor
			}
		}

		if cheapestFloor == 0 {
			continue
		}

		contracts := 1400 / lowestPrice
		if d.WinningFloor == cheapestFloor {
			profits = append(profits, float64(contracts)-14.0)
			hits++
		} else {
			profits = append(profits, -14.0)
		}
	}

	result := calculateStats("Market_Favorite", "Bet on bracket with lowest first price", profits, hits)
	results = append(results, result)
	log(fmt.Sprintf("  Market favorite: Accuracy=%.1f%%, Profit=$%.2f, Sharpe=%.2f",
		result.WinRate*100, result.TotalProfit, result.SharpeRatio))
}

func testAdaptiveStrategies(data []DayData) {
	// Strategy 1: Bet on +2°F when METAR > 68, +1°F when <= 68
	result1 := runAdaptiveCalibration(data)
	results = append(results, result1)
	log(fmt.Sprintf("  Adaptive calibration: Accuracy=%.1f%%, Profit=$%.2f, Sharpe=%.2f",
		result1.WinRate*100, result1.TotalProfit, result1.SharpeRatio))

	// Strategy 2: Conservative - always go 2 brackets wide
	result2 := runConservativeHedge(data)
	results = append(results, result2)
	log(fmt.Sprintf("  Conservative 2-bracket: Accuracy=%.1f%%, Profit=$%.2f, Sharpe=%.2f",
		result2.WinRate*100, result2.TotalProfit, result2.SharpeRatio))

	// Strategy 3: Value betting - only trade when first price < 30
	result3 := runValueBetting(data)
	results = append(results, result3)
	log(fmt.Sprintf("  Value betting (<30¢): Accuracy=%.1f%%, Profit=$%.2f, Sharpe=%.2f",
		result3.WinRate*100, result3.TotalProfit, result3.SharpeRatio))
}

func runAdaptiveCalibration(data []DayData) StrategyResult {
	var profits []float64
	wins := 0

	for _, d := range data {
		// Adaptive: use +2°F for warm days, +1°F for cool days
		calibration := 1
		if d.METARMax >= 68 {
			calibration = 2
		}

		predictedFloor := ((d.METARMax + calibration) / 2) * 2

		price, ok := d.FirstPrices[predictedFloor]
		if !ok || price == 0 {
			price = 50
		}

		contracts := 1400 / price
		if d.WinningFloor == predictedFloor {
			profits = append(profits, float64(contracts)-14.0)
			wins++
		} else {
			profits = append(profits, -14.0)
		}
	}

	return calculateStats("Adaptive_Calibration", "+2°F when METAR>68, +1°F otherwise", profits, wins)
}

func runConservativeHedge(data []DayData) StrategyResult {
	var profits []float64
	hits := 0

	for _, d := range data {
		predictedFloor := ((d.METARMax + 1) / 2) * 2
		adjacentFloor := predictedFloor + 2

		price1, ok1 := d.FirstPrices[predictedFloor]
		price2, ok2 := d.FirstPrices[adjacentFloor]

		if !ok1 || price1 == 0 {
			price1 = 50
		}
		if !ok2 || price2 == 0 {
			price2 = 50
		}

		contracts1 := 700 / price1
		contracts2 := 700 / price2
		totalCost := float64(contracts1*price1+contracts2*price2) / 100

		var profit float64
		if d.WinningFloor == predictedFloor {
			profit = float64(contracts1) - totalCost
			hits++
		} else if d.WinningFloor == adjacentFloor {
			profit = float64(contracts2) - totalCost
			hits++
		} else {
			profit = -totalCost
		}
		profits = append(profits, profit)
	}

	return calculateStats("Conservative_2bracket", "50/50 on predicted and +2°F bracket", profits, hits)
}

func runValueBetting(data []DayData) StrategyResult {
	var profits []float64
	wins := 0
	trades := 0

	for _, d := range data {
		predictedFloor := ((d.METARMax + 1) / 2) * 2

		price, ok := d.FirstPrices[predictedFloor]
		if !ok || price == 0 || price >= 30 {
			// Skip - not enough value
			continue
		}

		trades++
		contracts := 1400 / price
		if d.WinningFloor == predictedFloor {
			profits = append(profits, float64(contracts)-14.0)
			wins++
		} else {
			profits = append(profits, -14.0)
		}
	}

	result := calculateStats("Value_Betting", "Only trade when first price < 30¢", profits, wins)
	result.DaysAnalyzed = trades
	return result
}

func calculateStats(name, desc string, profits []float64, wins int) StrategyResult {
	if len(profits) == 0 {
		return StrategyResult{Name: name, Description: desc}
	}

	var total float64
	var maxDD float64
	var peak float64
	var running float64

	for _, p := range profits {
		total += p
		running += p
		if running > peak {
			peak = running
		}
		dd := peak - running
		if dd > maxDD {
			maxDD = dd
		}
	}

	avg := total / float64(len(profits))

	// Calculate Sharpe ratio (simplified)
	var variance float64
	for _, p := range profits {
		variance += (p - avg) * (p - avg)
	}
	stdDev := math.Sqrt(variance / float64(len(profits)))
	sharpe := 0.0
	if stdDev > 0 {
		sharpe = avg / stdDev
	}

	return StrategyResult{
		Name:         name,
		Description:  desc,
		TotalProfit:  total,
		WinRate:      float64(wins) / float64(len(profits)),
		AvgProfit:    avg,
		MaxDrawdown:  maxDD,
		SharpeRatio:  sharpe,
		DaysAnalyzed: len(profits),
	}
}

func printFinalRankings() {
	log("")
	log(strings.Repeat("=", 80))
	log("FINAL STRATEGY RANKINGS (by Sharpe Ratio)")
	log(strings.Repeat("=", 80))
	log("")

	// Sort by Sharpe ratio
	sort.Slice(results, func(i, j int) bool {
		return results[i].SharpeRatio > results[j].SharpeRatio
	})

	log(fmt.Sprintf("%-25s %8s %8s %10s %10s", "Strategy", "WinRate", "Sharpe", "Profit", "MaxDD"))
	log(strings.Repeat("-", 80))

	for i, r := range results {
		marker := ""
		if i == 0 {
			marker = " 🏆 BEST"
		} else if i == 1 {
			marker = " 🥈"
		} else if i == 2 {
			marker = " 🥉"
		}

		log(fmt.Sprintf("%-25s %7.1f%% %8.2f $%8.2f $%8.2f%s",
			r.Name, r.WinRate*100, r.SharpeRatio, r.TotalProfit, r.MaxDrawdown, marker))
	}

	log("")
	log(strings.Repeat("=", 80))
	log("TOP 3 STRATEGIES - DETAILED")
	log(strings.Repeat("=", 80))

	for i := 0; i < 3 && i < len(results); i++ {
		r := results[i]
		log("")
		log(fmt.Sprintf("#%d: %s", i+1, r.Name))
		log(fmt.Sprintf("    Description: %s", r.Description))
		log(fmt.Sprintf("    Win Rate: %.1f%% (%d days)", r.WinRate*100, r.DaysAnalyzed))
		log(fmt.Sprintf("    Total Profit: $%.2f", r.TotalProfit))
		log(fmt.Sprintf("    Avg Profit/Day: $%.2f", r.AvgProfit))
		log(fmt.Sprintf("    Max Drawdown: $%.2f", r.MaxDrawdown))
		log(fmt.Sprintf("    Sharpe Ratio: %.2f", r.SharpeRatio))
	}
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

