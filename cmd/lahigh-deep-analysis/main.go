// Package main runs deep analysis on LA High Temperature patterns
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
	Date           time.Time
	METARMax       int
	WinningFloor   int
	CLI_METAR_Diff int
	AllMarkets     []Market
	FirstPrices    map[int]int
	MarketFavorite int // Bracket with highest first trade price (most confident)
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
	outputFile, err = os.Create("deep_analysis_results.txt")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer outputFile.Close()

	log("=" + strings.Repeat("=", 79))
	log("DEEP ANALYSIS: Finding Hidden Patterns")
	log("Started: " + time.Now().Format("2006-01-02 15:04:05"))
	log("=" + strings.Repeat("=", 79))
	log("")

	// Fetch extended data
	log("Fetching 21 days of historical data...")
	data := fetchAllData(21)
	log(fmt.Sprintf("Fetched %d days", len(data)))
	log("")

	// Analysis 1: CLI vs METAR difference patterns
	analyzeCalibrationVariance(data)

	// Analysis 2: When does cheap actually win?
	analyzeValueOpportunities(data)

	// Analysis 3: Opposite market strategy
	analyzeContrarian(data)

	// Analysis 4: Temperature range patterns
	analyzeTemperatureRanges(data)

	// Analysis 5: Day of week patterns
	analyzeDayOfWeek(data)

	// Analysis 6: Create optimal composite strategy
	createOptimalStrategy(data)

	log("")
	log("=" + strings.Repeat("=", 79))
	log("ANALYSIS COMPLETE")
	log("Finished: " + time.Now().Format("2006-01-02 15:04:05"))
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
		fmt.Printf("  %s... ", targetDate.Format("Jan 2"))

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
	dayData := DayData{
		Date:        date,
		FirstPrices: make(map[int]int),
	}

	metarMax, err := getMETARMax(date)
	if err != nil {
		return dayData, err
	}
	dayData.METARMax = metarMax

	dateCode := strings.ToUpper(date.Format("06Jan02"))
	eventTicker := fmt.Sprintf("KXHIGHLAX-%s", dateCode)

	winner, markets, err := getWinnerAndMarkets(eventTicker)
	if err != nil {
		return dayData, err
	}
	if winner == nil {
		return dayData, fmt.Errorf("no winner")
	}

	dayData.WinningFloor = winner.FloorStrike
	dayData.AllMarkets = markets
	dayData.CLI_METAR_Diff = winner.FloorStrike - metarMax

	// Get first prices and find market favorite
	highestPrice := 0
	for _, m := range markets {
		if m.FloorStrike >= 55 && m.FloorStrike <= 80 {
			price, err := getFirstTradePrice(m.Ticker)
			if err == nil && price > 0 {
				dayData.FirstPrices[m.FloorStrike] = price
				if price > highestPrice {
					highestPrice = price
					dayData.MarketFavorite = m.FloorStrike
				}
			}
			time.Sleep(150 * time.Millisecond)
		}
	}

	return dayData, nil
}

func analyzeCalibrationVariance(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("ANALYSIS 1: CLI vs METAR Calibration Patterns")
	log("=" + strings.Repeat("=", 60))
	log("")

	diffCounts := make(map[int]int)
	var diffs []int

	for _, d := range data {
		if d.WinningFloor > 0 && d.METARMax > 0 {
			diff := d.CLI_METAR_Diff
			diffCounts[diff]++
			diffs = append(diffs, diff)
		}
	}

	log("Distribution of CLI - METAR difference:")
	for diff := -3; diff <= 5; diff++ {
		count := diffCounts[diff]
		bar := strings.Repeat("█", count*3)
		pct := float64(count) / float64(len(diffs)) * 100
		log(fmt.Sprintf("  %+d°F: %s %.1f%% (%d days)", diff, bar, pct, count))
	}

	// Calculate mean and std dev
	sum := 0.0
	for _, d := range diffs {
		sum += float64(d)
	}
	mean := sum / float64(len(diffs))

	variance := 0.0
	for _, d := range diffs {
		variance += (float64(d) - mean) * (float64(d) - mean)
	}
	stdDev := math.Sqrt(variance / float64(len(diffs)))

	log("")
	log(fmt.Sprintf("Mean calibration: %+.2f°F", mean))
	log(fmt.Sprintf("Std deviation: %.2f°F", stdDev))
	log(fmt.Sprintf("Range: %+d to %+d°F", minInt(diffs), maxInt(diffs)))
	log("")

	// Recommendation
	log("INSIGHT:")
	if mean > 0.5 && mean < 1.5 {
		log("  +1°F calibration is reasonable on average, but variance is high")
	} else if mean > 1.5 {
		log(fmt.Sprintf("  Consider using +%.0f°F calibration instead of +1°F", math.Round(mean)))
	} else {
		log(fmt.Sprintf("  Calibration is lower than expected: +%.1f°F average", mean))
	}
	log("")
}

func analyzeValueOpportunities(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("ANALYSIS 2: When Do Cheap Brackets Actually Win?")
	log("=" + strings.Repeat("=", 60))
	log("")

	type priceWin struct {
		price int
		won   bool
	}

	var allTrades []priceWin

	for _, d := range data {
		for floor, price := range d.FirstPrices {
			won := floor == d.WinningFloor
			allTrades = append(allTrades, priceWin{price, won})
		}
	}

	// Group by price ranges
	ranges := [][2]int{{1, 10}, {11, 20}, {21, 30}, {31, 40}, {41, 50}, {51, 60}, {61, 70}, {71, 80}, {81, 100}}

	log("Win rate by first trade price range:")
	log("")
	log(fmt.Sprintf("%-15s %10s %10s %10s", "Price Range", "Total", "Won", "Win Rate"))
	log(strings.Repeat("-", 50))

	for _, r := range ranges {
		total := 0
		won := 0
		for _, t := range allTrades {
			if t.price >= r[0] && t.price <= r[1] {
				total++
				if t.won {
					won++
				}
			}
		}
		if total > 0 {
			rate := float64(won) / float64(total) * 100
			log(fmt.Sprintf("%3d¢ - %3d¢    %10d %10d %9.1f%%", r[0], r[1], total, won, rate))
		}
	}
	log("")
}

func analyzeContrarian(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("ANALYSIS 3: Contrarian Strategy (Bet AGAINST Market)")
	log("=" + strings.Repeat("=", 60))
	log("")

	// Test: bet on bracket with HIGHEST price (market thinks most likely)
	wins := 0
	total := 0
	var profits []float64

	for _, d := range data {
		if d.MarketFavorite == 0 {
			continue
		}
		total++

		price := d.FirstPrices[d.MarketFavorite]
		contracts := 1400 / price

		if d.WinningFloor == d.MarketFavorite {
			wins++
			profits = append(profits, float64(contracts)-14.0)
		} else {
			profits = append(profits, -14.0)
		}
	}

	totalProfit := 0.0
	for _, p := range profits {
		totalProfit += p
	}

	log("Strategy: Bet on bracket with HIGHEST first trade price")
	log(fmt.Sprintf("  (The market's most confident pick)"))
	log("")
	log(fmt.Sprintf("  Win rate: %.1f%% (%d/%d)", float64(wins)/float64(total)*100, wins, total))
	log(fmt.Sprintf("  Total profit: $%.2f", totalProfit))
	log("")

	// Now test betting on 2nd highest
	log("Alternative: Bet on 2nd highest price bracket")
	wins2 := 0
	var profits2 []float64

	for _, d := range data {
		if len(d.FirstPrices) < 2 {
			continue
		}

		// Find 2nd highest
		type fp struct {
			floor int
			price int
		}
		var sorted []fp
		for f, p := range d.FirstPrices {
			sorted = append(sorted, fp{f, p})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].price > sorted[j].price
		})

		if len(sorted) < 2 {
			continue
		}
		secondBest := sorted[1].floor
		price := sorted[1].price
		contracts := 1400 / price

		if d.WinningFloor == secondBest {
			wins2++
			profits2 = append(profits2, float64(contracts)-14.0)
		} else {
			profits2 = append(profits2, -14.0)
		}
	}

	totalProfit2 := 0.0
	for _, p := range profits2 {
		totalProfit2 += p
	}

	log(fmt.Sprintf("  Win rate: %.1f%% (%d/%d)", float64(wins2)/float64(len(profits2))*100, wins2, len(profits2)))
	log(fmt.Sprintf("  Total profit: $%.2f", totalProfit2))
	log("")
}

func analyzeTemperatureRanges(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("ANALYSIS 4: Temperature Range Patterns")
	log("=" + strings.Repeat("=", 60))
	log("")

	// Does calibration vary by temperature?
	lowTemp := []DayData{}  // METAR < 65
	highTemp := []DayData{} // METAR >= 65

	for _, d := range data {
		if d.METARMax < 65 {
			lowTemp = append(lowTemp, d)
		} else {
			highTemp = append(highTemp, d)
		}
	}

	log("Calibration by temperature range:")
	log("")

	// Low temp calibration
	lowSum := 0.0
	for _, d := range lowTemp {
		lowSum += float64(d.CLI_METAR_Diff)
	}
	if len(lowTemp) > 0 {
		log(fmt.Sprintf("  Cool days (METAR < 65°F): Avg calibration = %+.2f°F (%d days)", lowSum/float64(len(lowTemp)), len(lowTemp)))
	}

	// High temp calibration
	highSum := 0.0
	for _, d := range highTemp {
		highSum += float64(d.CLI_METAR_Diff)
	}
	if len(highTemp) > 0 {
		log(fmt.Sprintf("  Warm days (METAR >= 65°F): Avg calibration = %+.2f°F (%d days)", highSum/float64(len(highTemp)), len(highTemp)))
	}

	log("")

	// Test adaptive calibration
	log("Adaptive calibration strategy:")
	wins := 0
	for _, d := range data {
		var cal int
		if d.METARMax < 65 {
			cal = int(math.Round(lowSum / float64(len(lowTemp))))
		} else {
			cal = int(math.Round(highSum / float64(len(highTemp))))
		}

		predictedFloor := ((d.METARMax + cal) / 2) * 2
		if d.WinningFloor == predictedFloor {
			wins++
		}
	}
	log(fmt.Sprintf("  Win rate with adaptive calibration: %.1f%% (%d/%d)", float64(wins)/float64(len(data))*100, wins, len(data)))
	log("")
}

func analyzeDayOfWeek(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("ANALYSIS 5: Day of Week Patterns")
	log("=" + strings.Repeat("=", 60))
	log("")

	dayStats := make(map[time.Weekday][]float64)

	for _, d := range data {
		dayStats[d.Date.Weekday()] = append(dayStats[d.Date.Weekday()], float64(d.CLI_METAR_Diff))
	}

	log("Average calibration by day of week:")
	for day := time.Sunday; day <= time.Saturday; day++ {
		diffs := dayStats[day]
		if len(diffs) == 0 {
			continue
		}
		sum := 0.0
		for _, d := range diffs {
			sum += d
		}
		avg := sum / float64(len(diffs))
		log(fmt.Sprintf("  %-9s: %+.2f°F (%d samples)", day.String(), avg, len(diffs)))
	}
	log("")
}

func createOptimalStrategy(data []DayData) {
	log("=" + strings.Repeat("=", 60))
	log("ANALYSIS 6: Synthesizing Optimal Strategy")
	log("=" + strings.Repeat("=", 60))
	log("")

	// Calculate optimal calibration by temperature range
	lowCalSum, lowCount := 0.0, 0
	highCalSum, highCount := 0.0, 0

	for _, d := range data {
		if d.METARMax < 65 {
			lowCalSum += float64(d.CLI_METAR_Diff)
			lowCount++
		} else {
			highCalSum += float64(d.CLI_METAR_Diff)
			highCount++
		}
	}

	lowCal := 0
	if lowCount > 0 {
		lowCal = int(math.Round(lowCalSum / float64(lowCount)))
	}
	highCal := 0
	if highCount > 0 {
		highCal = int(math.Round(highCalSum / float64(highCount)))
	}

	log("Derived optimal parameters:")
	log(fmt.Sprintf("  Cool days (METAR < 65°F): Use %+d°F calibration", lowCal))
	log(fmt.Sprintf("  Warm days (METAR >= 65°F): Use %+d°F calibration", highCal))
	log("")

	// Test this strategy
	wins := 0
	var profits []float64

	for _, d := range data {
		var cal int
		if d.METARMax < 65 {
			cal = lowCal
		} else {
			cal = highCal
		}

		predictedFloor := ((d.METARMax + cal) / 2) * 2
		price, ok := d.FirstPrices[predictedFloor]
		if !ok || price == 0 {
			price = 50
		}

		contracts := 1400 / price
		if d.WinningFloor == predictedFloor {
			wins++
			profits = append(profits, float64(contracts)-14.0)
		} else {
			profits = append(profits, -14.0)
		}
	}

	totalProfit := 0.0
	for _, p := range profits {
		totalProfit += p
	}

	log("Optimal adaptive strategy results:")
	log(fmt.Sprintf("  Win rate: %.1f%% (%d/%d)", float64(wins)/float64(len(data))*100, wins, len(data)))
	log(fmt.Sprintf("  Total profit: $%.2f", totalProfit))
	log(fmt.Sprintf("  Avg profit/day: $%.2f", totalProfit/float64(len(data))))
	log("")

	// Now test with hedging
	log("Optimal strategy WITH 70/30 hedge:")
	hedgeWins := 0
	var hedgeProfits []float64

	for _, d := range data {
		var cal int
		if d.METARMax < 65 {
			cal = lowCal
		} else {
			cal = highCal
		}

		predictedFloor := ((d.METARMax + cal) / 2) * 2
		protectFloor := predictedFloor - 2

		thesisPrice, ok1 := d.FirstPrices[predictedFloor]
		protectPrice, ok2 := d.FirstPrices[protectFloor]

		if !ok1 || thesisPrice == 0 {
			thesisPrice = 50
		}
		if !ok2 || protectPrice == 0 {
			protectPrice = 50
		}

		thesisContracts := 1000 / thesisPrice
		protectContracts := 400 / protectPrice
		totalCost := float64(thesisContracts*thesisPrice+protectContracts*protectPrice) / 100

		var profit float64
		if d.WinningFloor == predictedFloor {
			profit = float64(thesisContracts) - totalCost
			hedgeWins++
		} else if d.WinningFloor == protectFloor {
			profit = float64(protectContracts) - totalCost
		} else {
			profit = -totalCost
		}
		hedgeProfits = append(hedgeProfits, profit)
	}

	hedgeTotal := 0.0
	for _, p := range hedgeProfits {
		hedgeTotal += p
	}

	log(fmt.Sprintf("  Win rate: %.1f%% (%d/%d)", float64(hedgeWins)/float64(len(data))*100, hedgeWins, len(data)))
	log(fmt.Sprintf("  Total profit: $%.2f", hedgeTotal))
	log(fmt.Sprintf("  Avg profit/day: $%.2f", hedgeTotal/float64(len(data))))
	log("")

	log("=" + strings.Repeat("=", 60))
	log("FINAL RECOMMENDATION")
	log("=" + strings.Repeat("=", 60))
	log("")
	log("Based on this analysis:")
	log("")
	if wins > len(data)/2 {
		log("  ✅ Adaptive calibration shows promise")
		log(fmt.Sprintf("     Use %+d°F for cool days, %+d°F for warm days", lowCal, highCal))
	} else {
		log("  ⚠️  Even adaptive calibration struggles")
		log("     Consider:")
		log("     1. Only trade when price < 30¢ (value betting)")
		log("     2. Use NWS forecast directly instead of METAR calibration")
		log("     3. Wait for more data before trading")
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

func minInt(nums []int) int {
	if len(nums) == 0 {
		return 0
	}
	min := nums[0]
	for _, n := range nums {
		if n < min {
			min = n
		}
	}
	return min
}

func maxInt(nums []int) int {
	if len(nums) == 0 {
		return 0
	}
	max := nums[0]
	for _, n := range nums {
		if n > max {
			max = n
		}
	}
	return max
}

