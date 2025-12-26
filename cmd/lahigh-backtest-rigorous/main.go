// Package main provides a rigorous backtest using real data at every step
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
	Cursor string  `json:"cursor"`
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

type DayAnalysis struct {
	Date             string
	METARMax         int
	CLISettlement    int
	WinningBracket   string
	PredictedBracket string
	PredictedCorrect bool
	ThesisPrice      int
	ProtectPrice     int
	Profit           float64
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
	fmt.Println(strings.Repeat("=", 90))
	fmt.Println("RIGOROUS BACKTEST: Using Real Data At Every Step")
	fmt.Println(strings.Repeat("=", 90))
	fmt.Println()
	fmt.Println("Methodology:")
	fmt.Println("1. Fetch METAR max for each historical day")
	fmt.Println("2. Model prediction: METAR max + 1°F calibration")
	fmt.Println("3. Fetch actual settlement (winning bracket) from Kalshi")
	fmt.Println("4. Fetch real first trade prices for predicted & adjacent brackets")
	fmt.Println("5. Simulate 70/30 hedge and calculate profit/loss")
	fmt.Println()

	results := []DayAnalysis{}
	today := time.Now().In(loc)

	for i := 1; i <= 14; i++ {
		targetDate := today.AddDate(0, 0, -i)
		fmt.Printf("Analyzing %s... ", targetDate.Format("Jan 2"))

		analysis, err := analyzeDay(targetDate)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			continue
		}

		if analysis.PredictedCorrect {
			fmt.Printf("✅ Predicted %s correctly\n", analysis.PredictedBracket)
		} else {
			fmt.Printf("❌ Predicted %s, actual %s\n", analysis.PredictedBracket, analysis.WinningBracket)
		}

		results = append(results, analysis)
		time.Sleep(500 * time.Millisecond)
	}

	printResults(results)
}

func analyzeDay(targetDate time.Time) (DayAnalysis, error) {
	analysis := DayAnalysis{
		Date: targetDate.Format("2006-01-02"),
	}

	// Step 1: Get METAR max
	metarMax, err := getMETARMax(targetDate)
	if err != nil {
		return analysis, fmt.Errorf("METAR: %v", err)
	}
	analysis.METARMax = metarMax

	// Step 2: Get winning bracket from Kalshi
	dateCode := strings.ToUpper(targetDate.Format("06Jan02"))
	eventTicker := fmt.Sprintf("KXHIGHLAX-%s", dateCode)

	winner, allMarkets, err := getWinnerAndMarkets(eventTicker)
	if err != nil {
		return analysis, fmt.Errorf("markets: %v", err)
	}
	if winner == nil {
		return analysis, fmt.Errorf("no winner")
	}

	analysis.WinningBracket = formatBracket(winner)
	analysis.CLISettlement = winner.FloorStrike

	// Step 3: What would our model predict?
	// Model: METAR max + 1°F = CLI estimate
	predictedCLI := metarMax + 1
	
	// Find bracket containing predicted CLI
	predictedFloor := 0
	for _, m := range allMarkets {
		if m.FloorStrike <= predictedCLI && m.CapStrike >= predictedCLI {
			predictedFloor = m.FloorStrike
			break
		}
	}
	
	if predictedFloor == 0 {
		// Fallback: round to nearest 2-degree bracket
		predictedFloor = (predictedCLI / 2) * 2
	}

	// Find our prediction market and protection market
	var predictedMarket, protectMarket *Market
	for i := range allMarkets {
		m := &allMarkets[i]
		if m.FloorStrike == predictedFloor {
			predictedMarket = m
		}
		// Protection: bracket below
		if m.FloorStrike == predictedFloor-2 {
			protectMarket = m
		}
	}

	if predictedMarket != nil {
		analysis.PredictedBracket = formatBracket(predictedMarket)
		analysis.PredictedCorrect = predictedMarket.Result == "yes"

		price, err := getFirstTradePrice(predictedMarket.Ticker)
		if err == nil {
			analysis.ThesisPrice = price
		}
	} else {
		analysis.PredictedBracket = fmt.Sprintf("%d-%d°", predictedFloor, predictedFloor+1)
	}

	if protectMarket != nil {
		price, err := getFirstTradePrice(protectMarket.Ticker)
		if err == nil {
			analysis.ProtectPrice = price
		}
	}

	// Step 4: Calculate profit with 70/30 hedge ($10 thesis, $4 protect)
	if analysis.ThesisPrice > 0 {
		thesisContracts := 1000 / analysis.ThesisPrice
		protectContracts := 0
		if analysis.ProtectPrice > 0 {
			protectContracts = 400 / analysis.ProtectPrice
		}

		totalCost := float64(thesisContracts*analysis.ThesisPrice+protectContracts*analysis.ProtectPrice) / 100

		if analysis.PredictedCorrect {
			// Thesis won
			analysis.Profit = float64(thesisContracts) - totalCost
		} else if protectMarket != nil && protectMarket.Result == "yes" {
			// Protection won
			analysis.Profit = float64(protectContracts) - totalCost
		} else {
			// Neither won
			analysis.Profit = -totalCost
		}
	}

	return analysis, nil
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

func formatBracket(m *Market) string {
	if m.FloorStrike > 0 && m.CapStrike > 0 {
		return fmt.Sprintf("%d-%d°", m.FloorStrike, m.CapStrike)
	} else if m.CapStrike > 0 {
		return fmt.Sprintf("≤%d°", m.CapStrike)
	} else if m.FloorStrike > 0 {
		return fmt.Sprintf("≥%d°", m.FloorStrike)
	}
	return "?"
}

func printResults(results []DayAnalysis) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 100))
	fmt.Println("DETAILED RESULTS")
	fmt.Println(strings.Repeat("=", 100))
	fmt.Println()

	fmt.Printf("%-12s %6s %6s %-10s %-10s %6s %8s %8s %10s\n",
		"Date", "METAR", "+1°F", "Winner", "Predicted", "Right?", "Thesis¢", "Prot¢", "Profit")
	fmt.Println(strings.Repeat("-", 100))

	totalProfit := 0.0
	correctPredictions := 0
	protectionWins := 0
	totalDays := 0

	for _, r := range results {
		if r.WinningBracket == "" {
			continue
		}
		totalDays++

		correct := "❌"
		if r.PredictedCorrect {
			correct = "✅"
			correctPredictions++
		}

		profitStr := fmt.Sprintf("$%.2f", r.Profit)
		if r.Profit > 0 {
			profitStr = fmt.Sprintf("+$%.2f", r.Profit)
		}

		protectStr := "-"
		if r.ProtectPrice > 0 {
			protectStr = fmt.Sprintf("%d¢", r.ProtectPrice)
		}

		fmt.Printf("%-12s %4d°F %4d°F %-10s %-10s %6s %7d¢ %8s %10s\n",
			r.Date,
			r.METARMax,
			r.METARMax+1,
			r.WinningBracket,
			r.PredictedBracket,
			correct,
			r.ThesisPrice,
			protectStr,
			profitStr)

		totalProfit += r.Profit
		
		// Check if protection would have won
		if !r.PredictedCorrect && r.Profit > -10 {
			protectionWins++
		}
	}

	fmt.Println(strings.Repeat("-", 100))
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	accuracy := 0.0
	if totalDays > 0 {
		accuracy = float64(correctPredictions) / float64(totalDays) * 100
	}

	fmt.Printf("  📊 Days analyzed: %d\n", totalDays)
	fmt.Printf("  🎯 Thesis correct: %d/%d (%.1f%%)\n", correctPredictions, totalDays, accuracy)
	fmt.Printf("  🛡️  Protection saved: %d days\n", protectionWins)
	fmt.Printf("  💰 Total profit: $%.2f\n", totalProfit)
	fmt.Printf("  📈 Avg profit/day: $%.2f\n", totalProfit/float64(totalDays))
	fmt.Printf("  💵 Total invested: $%.2f\n", float64(totalDays)*14.0)
	
	roi := 0.0
	if totalDays > 0 {
		roi = totalProfit / float64(totalDays*14) * 100
	}
	fmt.Printf("  📊 ROI: %.1f%%\n", roi)
	fmt.Println()

	// Error analysis
	fmt.Println("PREDICTION ERRORS:")
	fmt.Println("------------------")
	hasErrors := false
	for _, r := range results {
		if !r.PredictedCorrect && r.WinningBracket != "" {
			hasErrors = true
			diff := 0
			if r.CLISettlement > 0 && r.METARMax > 0 {
				diff = r.CLISettlement - r.METARMax
			}
			fmt.Printf("  %s: METAR=%d° → Predicted %s, Actual %s (CLI-METAR=%+d°)\n",
				r.Date, r.METARMax, r.PredictedBracket, r.WinningBracket, diff)
		}
	}
	if !hasErrors {
		fmt.Println("  None! All predictions were correct.")
	}
}

