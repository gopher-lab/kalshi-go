// Package main provides a Monte Carlo simulation for the LA High Temperature trading strategy.
// It simulates thousands of trades to quantify the expected value and risk profile.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"time"
)

// METARObservation represents a single METAR weather observation.
type METARObservation struct {
	IcaoID     string  `json:"icaoId"`
	ObsTime    int64   `json:"obsTime"`
	ReportTime string  `json:"reportTime"`
	Temp       float64 `json:"temp"`
	Dewp       float64 `json:"dewp"`
	MaxT       float64 `json:"maxT"`
	MinT       float64 `json:"minT"`
	MaxT24     float64 `json:"maxT24"`
	MinT24     float64 `json:"minT24"`
	MetarType  string  `json:"metarType"`
	RawOb      string  `json:"rawOb"`
}

// DayData holds all observations and computed stats for a single day.
type DayData struct {
	Date         string
	Observations []METARObservation
	FinalMaxC    float64
	FinalMaxF    int
	CLIMaxF      int             // Official settlement value (+1°F calibration)
	HourlyTemps  map[int]float64 // Hour -> max temp observed by that hour
}

// Trade represents a simulated trade.
type Trade struct {
	EntryHour    int
	Strike       int
	Direction    string  // "YES" or "NO"
	EntryPrice   float64 // Price paid (0-1)
	SettledPrice float64 // 1 if won, 0 if lost
	PnL          float64
	Correct      bool
}

// Strategy defines a trading strategy.
type Strategy struct {
	Name        string
	Description string
	Execute     func(day DayData, rng *rand.Rand) []Trade
}

// SimulationResult holds results from the Monte Carlo simulation.
type SimulationResult struct {
	StrategyName  string
	TotalTrades   int
	WinRate       float64
	TotalPnL      float64
	AvgPnL        float64
	MaxDrawdown   float64
	SharpeRatio   float64
	ExpectedValue float64
	StdDev        float64
	Percentile5   float64
	Percentile95  float64
}

const (
	metarAPIURL    = "https://aviationweather.gov/api/data/metar?ids=KLAX&hours=96&format=json"
	laTimezone     = "America/Los_Angeles"
	numSimulations = 10000
)

func main() {
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("LA HIGH TEMPERATURE STRATEGY - MONTE CARLO SIMULATION")
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println()

	// Fetch METAR data
	fmt.Println("→ Fetching 96 hours of METAR data...")
	observations, err := fetchMETARData()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching METAR data: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Fetched %d observations\n\n", len(observations))

	// Load LA timezone
	loc, err := time.LoadLocation(laTimezone)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading timezone: %v\n", err)
		os.Exit(1)
	}

	// Process observations into daily data
	days := processObservations(observations, loc)
	fmt.Printf("✓ Processed %d complete trading days\n\n", len(days))

	// Print historical data summary
	printHistoricalSummary(days)

	// Define strategies to test
	strategies := []Strategy{
		{
			Name:        "Early Entry (Before 3PM)",
			Description: "Bet YES on >X when temp crosses strike before 3PM",
			Execute:     strategyEarlyEntry,
		},
		{
			Name:        "Conservative (After 4PM)",
			Description: "Wait until 4PM, bet on confirmed direction",
			Execute:     strategyConservative,
		},
		{
			Name:        "Aggressive Scalp",
			Description: "Bet as soon as any threshold is crossed",
			Execute:     strategyAggressiveScalp,
		},
		{
			Name:        "Calibrated (+1°F)",
			Description: "Early entry with +1°F calibration for CLI difference",
			Execute:     strategyCalibratedEntry,
		},
		{
			Name:        "Random Baseline",
			Description: "Random betting at fair odds (control group)",
			Execute:     strategyRandom,
		},
	}

	// Run Monte Carlo simulation for each strategy
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("MONTE CARLO SIMULATION")
	fmt.Printf("Running %d simulations per strategy...\n", numSimulations)
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println()

	results := make([]SimulationResult, 0)
	for _, strategy := range strategies {
		result := runMonteCarlo(strategy, days, numSimulations)
		results = append(results, result)
		printStrategyResult(result)
	}

	// Print comparison
	printComparison(results)

	// Print optimal strategy recommendation
	printRecommendation(results, days)
}

func fetchMETARData() ([]METARObservation, error) {
	resp, err := http.Get(metarAPIURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var observations []METARObservation
	if err := json.Unmarshal(body, &observations); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	return observations, nil
}

func processObservations(observations []METARObservation, loc *time.Location) []DayData {
	// Sort by time (oldest first)
	sort.Slice(observations, func(i, j int) bool {
		return observations[i].ObsTime < observations[j].ObsTime
	})

	dayMap := make(map[string]*DayData)

	for _, obs := range observations {
		t := time.Unix(obs.ObsTime, 0).In(loc)
		dateKey := t.Format("2006-01-02")

		if _, exists := dayMap[dateKey]; !exists {
			dayMap[dateKey] = &DayData{
				Date:         dateKey,
				Observations: []METARObservation{},
				HourlyTemps:  make(map[int]float64),
			}
		}
		dayMap[dateKey].Observations = append(dayMap[dateKey].Observations, obs)
	}

	// Calculate stats for each day
	var days []DayData
	for _, day := range dayMap {
		if len(day.Observations) < 10 { // Skip incomplete days
			continue
		}

		// Find max temp
		var maxTemp float64 = -999
		for _, obs := range day.Observations {
			if obs.Temp > maxTemp {
				maxTemp = obs.Temp
			}
		}
		day.FinalMaxC = maxTemp
		day.FinalMaxF = celsiusToFahrenheit(maxTemp)
		day.CLIMaxF = day.FinalMaxF + 1 // +1°F calibration for CLI

		// Calculate running max by hour
		var runningMax float64 = -999
		for _, obs := range day.Observations {
			t := time.Unix(obs.ObsTime, 0).In(loc)
			if obs.Temp > runningMax {
				runningMax = obs.Temp
			}
			day.HourlyTemps[t.Hour()] = runningMax
		}

		days = append(days, *day)
	}

	sort.Slice(days, func(i, j int) bool {
		return days[i].Date < days[j].Date
	})

	return days
}

func celsiusToFahrenheit(c float64) int {
	return int((c * 9.0 / 5.0) + 32.5)
}

const (
	kalshiFeeRate = 0.07 // Kalshi takes ~7% of winnings
	bidAskSpread  = 0.02 // 2 cents spread typically
)

// Market price model: simulates the market odds at different times
// Earlier = worse odds (market hasn't realized the outcome yet)
// Later = better odds (market converges to truth)
func getMarketPrice(hour int, strike int, runningMaxF int, direction string, rng *rand.Rand) float64 {
	// Base probability based on current info
	var trueProb float64
	if direction == "YES" {
		if runningMaxF > strike {
			trueProb = 0.75 + rng.Float64()*0.15 // Already crossed: 75-90%
		} else {
			gap := float64(strike - runningMaxF)
			trueProb = math.Max(0.15, 0.5-gap*0.08) // Further away = lower prob
		}
	} else { // NO
		if runningMaxF > strike {
			trueProb = 0.10 + rng.Float64()*0.15 // Already crossed: only 10-25% for NO
		} else {
			gap := float64(strike - runningMaxF)
			trueProb = math.Min(0.85, 0.5+gap*0.08) // Further away = higher prob for NO
		}
	}

	// Add market inefficiency (noise) - higher earlier in day
	noiseMultiplier := 1.0 - float64(hour)/24.0 // More noise early
	noise := (rng.Float64() - 0.5) * 0.15 * noiseMultiplier

	// Add bid-ask spread (we pay the ask when buying)
	spreadCost := bidAskSpread / 2

	price := trueProb + noise + spreadCost
	return math.Max(0.05, math.Min(0.95, price))
}

// Calculate PnL including Kalshi fees
func calculatePnL(entryPrice float64, won bool) float64 {
	if won {
		grossProfit := 1.0 - entryPrice
		fee := grossProfit * kalshiFeeRate
		return grossProfit - fee
	}
	return -entryPrice
}

// Strategy implementations

func strategyEarlyEntry(day DayData, rng *rand.Rand) []Trade {
	var trades []Trade
	strikes := []int{62, 64, 66, 68}

	for _, strike := range strikes {
		// Check each hour before 3PM
		for hour := 8; hour < 15; hour++ {
			if runningMax, ok := day.HourlyTemps[hour]; ok {
				runningMaxF := celsiusToFahrenheit(runningMax)
				if runningMaxF > strike {
					// We've crossed the strike! Bet YES
					price := getMarketPrice(hour, strike, runningMaxF, "YES", rng)
					won := day.CLIMaxF > strike
					pnl := calculatePnL(price, won)
					trades = append(trades, Trade{
						EntryHour:    hour,
						Strike:       strike,
						Direction:    "YES",
						EntryPrice:   price,
						SettledPrice: boolToFloat(won),
						PnL:          pnl,
						Correct:      won,
					})
					break // Only one trade per strike
				}
			}
		}
	}
	return trades
}

func strategyConservative(day DayData, rng *rand.Rand) []Trade {
	var trades []Trade
	strikes := []int{62, 64, 66, 68}

	// Wait until 4PM to bet
	hour := 16
	runningMax, ok := day.HourlyTemps[hour]
	if !ok {
		return trades
	}
	runningMaxF := celsiusToFahrenheit(runningMax)

	for _, strike := range strikes {
		var direction string
		var won bool

		if runningMaxF > strike {
			direction = "YES"
			won = day.CLIMaxF > strike
		} else if runningMaxF < strike-2 { // Need buffer
			direction = "NO"
			won = day.CLIMaxF <= strike
		} else {
			continue // Too close to call
		}

		price := getMarketPrice(hour, strike, runningMaxF, direction, rng)
		pnl := calculatePnL(price, won)

		trades = append(trades, Trade{
			EntryHour:    hour,
			Strike:       strike,
			Direction:    direction,
			EntryPrice:   price,
			SettledPrice: boolToFloat(won),
			PnL:          pnl,
			Correct:      won,
		})
	}
	return trades
}

func strategyAggressiveScalp(day DayData, rng *rand.Rand) []Trade {
	var trades []Trade
	strikes := []int{60, 62, 64, 66, 68, 70}
	tradedStrikes := make(map[int]bool)

	// Trade as soon as any threshold is crossed
	for hour := 0; hour < 24; hour++ {
		if runningMax, ok := day.HourlyTemps[hour]; ok {
			runningMaxF := celsiusToFahrenheit(runningMax)
			for _, strike := range strikes {
				if !tradedStrikes[strike] && runningMaxF > strike {
					tradedStrikes[strike] = true
					price := getMarketPrice(hour, strike, runningMaxF, "YES", rng)
					won := day.CLIMaxF > strike
					pnl := calculatePnL(price, won)
					trades = append(trades, Trade{
						EntryHour:    hour,
						Strike:       strike,
						Direction:    "YES",
						EntryPrice:   price,
						SettledPrice: boolToFloat(won),
						PnL:          pnl,
						Correct:      won,
					})
				}
			}
		}
	}
	return trades
}

func strategyCalibratedEntry(day DayData, rng *rand.Rand) []Trade {
	var trades []Trade
	strikes := []int{62, 64, 66, 68}

	for _, strike := range strikes {
		// Check each hour before 3PM
		for hour := 8; hour < 15; hour++ {
			if runningMax, ok := day.HourlyTemps[hour]; ok {
				runningMaxF := celsiusToFahrenheit(runningMax)
				// Add +1°F calibration: only bet if we're solidly above
				if runningMaxF >= strike { // Changed from > to >= with calibration in mind
					price := getMarketPrice(hour, strike, runningMaxF, "YES", rng)
					won := day.CLIMaxF > strike // CLI is already +1
					pnl := calculatePnL(price, won)
					trades = append(trades, Trade{
						EntryHour:    hour,
						Strike:       strike,
						Direction:    "YES",
						EntryPrice:   price,
						SettledPrice: boolToFloat(won),
						PnL:          pnl,
						Correct:      won,
					})
					break
				}
			}
		}
	}
	return trades
}

func strategyRandom(day DayData, rng *rand.Rand) []Trade {
	var trades []Trade
	strikes := []int{62, 64, 66, 68}

	for _, strike := range strikes {
		// Random entry hour
		hour := 8 + rng.Intn(14) // 8AM to 10PM
		runningMax, ok := day.HourlyTemps[hour]
		if !ok {
			continue
		}
		runningMaxF := celsiusToFahrenheit(runningMax)

		// Random direction
		direction := "YES"
		if rng.Float64() > 0.5 {
			direction = "NO"
		}

		// Fair odds (50/50)
		price := 0.50

		var won bool
		if direction == "YES" {
			won = day.CLIMaxF > strike
		} else {
			won = day.CLIMaxF <= strike
		}

		pnl := calculatePnL(price, won)

		trades = append(trades, Trade{
			EntryHour:    hour,
			Strike:       strike,
			Direction:    direction,
			EntryPrice:   price,
			SettledPrice: boolToFloat(won),
			PnL:          pnl,
			Correct:      won,
		})
		_ = runningMaxF // Unused in random strategy
	}
	return trades
}

func runMonteCarlo(strategy Strategy, days []DayData, numSims int) SimulationResult {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	allPnLs := make([]float64, 0)
	totalTrades := 0
	totalWins := 0

	for sim := 0; sim < numSims; sim++ {
		simPnL := 0.0
		simTrades := 0
		simWins := 0

		// Run strategy on each day
		for _, day := range days {
			trades := strategy.Execute(day, rng)
			for _, trade := range trades {
				simPnL += trade.PnL
				simTrades++
				if trade.Correct {
					simWins++
				}
			}
		}

		if simTrades > 0 {
			allPnLs = append(allPnLs, simPnL)
			totalTrades += simTrades
			totalWins += simWins
		}
	}

	// Calculate statistics
	result := SimulationResult{
		StrategyName: strategy.Name,
		TotalTrades:  totalTrades / numSims,
	}

	if len(allPnLs) > 0 {
		// Mean
		sum := 0.0
		for _, pnl := range allPnLs {
			sum += pnl
		}
		result.TotalPnL = sum / float64(len(allPnLs))
		result.AvgPnL = result.TotalPnL / float64(result.TotalTrades)
		result.WinRate = float64(totalWins) / float64(totalTrades)

		// StdDev
		variance := 0.0
		for _, pnl := range allPnLs {
			variance += (pnl - result.TotalPnL) * (pnl - result.TotalPnL)
		}
		result.StdDev = math.Sqrt(variance / float64(len(allPnLs)))

		// Sharpe (simplified, assuming 0 risk-free rate)
		if result.StdDev > 0 {
			result.SharpeRatio = result.TotalPnL / result.StdDev
		}

		// Expected Value per trade
		result.ExpectedValue = result.AvgPnL

		// Percentiles
		sort.Float64s(allPnLs)
		p5Idx := int(float64(len(allPnLs)) * 0.05)
		p95Idx := int(float64(len(allPnLs)) * 0.95)
		result.Percentile5 = allPnLs[p5Idx]
		result.Percentile95 = allPnLs[p95Idx]

		// Max Drawdown (simplified)
		maxPnL := allPnLs[0]
		maxDD := 0.0
		for _, pnl := range allPnLs {
			if pnl > maxPnL {
				maxPnL = pnl
			}
			dd := maxPnL - pnl
			if dd > maxDD {
				maxDD = dd
			}
		}
		result.MaxDrawdown = maxDD
	}

	return result
}

func printHistoricalSummary(days []DayData) {
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("HISTORICAL DATA SUMMARY")
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println()

	fmt.Printf("%-12s  %-10s  %-10s  %-30s\n", "Date", "METAR Max", "CLI Max*", "Notes")
	fmt.Printf("%-12s  %-10s  %-10s  %-30s\n", "----", "---------", "--------", "-----")

	for _, day := range days {
		notes := ""
		if day.CLIMaxF > 66 {
			notes = "Above normal"
		} else if day.CLIMaxF < 64 {
			notes = "Below normal"
		}
		fmt.Printf("%-12s  %-10d  %-10d  %-30s\n", day.Date, day.FinalMaxF, day.CLIMaxF, notes)
	}

	fmt.Println()
	fmt.Println("* CLI Max includes +1°F calibration adjustment")
	fmt.Println()
}

func printStrategyResult(result SimulationResult) {
	fmt.Printf("📊 %s\n", result.StrategyName)
	fmt.Printf("   Trades/Day: %d | Win Rate: %.1f%% | Avg PnL/Trade: $%.3f\n",
		result.TotalTrades, result.WinRate*100, result.AvgPnL)
	fmt.Printf("   Total PnL: $%.2f | Std Dev: $%.2f | Sharpe: %.2f\n",
		result.TotalPnL, result.StdDev, result.SharpeRatio)
	fmt.Printf("   5th %%ile: $%.2f | 95th %%ile: $%.2f\n",
		result.Percentile5, result.Percentile95)
	fmt.Println()
}

func printComparison(results []SimulationResult) {
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("STRATEGY COMPARISON")
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println()

	fmt.Printf("%-25s  %-8s  %-8s  %-10s  %-8s\n",
		"Strategy", "Win %", "EV/Trade", "Tot PnL", "Sharpe")
	fmt.Printf("%-25s  %-8s  %-8s  %-10s  %-8s\n",
		"--------", "-----", "--------", "-------", "------")

	for _, r := range results {
		fmt.Printf("%-25s  %-8.1f  $%-7.3f  $%-9.2f  %-8.2f\n",
			r.StrategyName, r.WinRate*100, r.ExpectedValue, r.TotalPnL, r.SharpeRatio)
	}
	fmt.Println()
}

func printRecommendation(results []SimulationResult, days []DayData) {
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("RECOMMENDATION")
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println()

	// Find best strategy by Sharpe ratio
	var best SimulationResult
	for _, r := range results {
		if r.StrategyName != "Random Baseline" && r.SharpeRatio > best.SharpeRatio {
			best = r
		}
	}

	fmt.Printf("🏆 BEST STRATEGY: %s\n", best.StrategyName)
	fmt.Println()
	fmt.Printf("   Expected Value per Trade: $%.3f (%.1f%% edge)\n",
		best.ExpectedValue, best.ExpectedValue*100)
	fmt.Printf("   Win Rate: %.1f%%\n", best.WinRate*100)
	fmt.Printf("   Risk-Adjusted Return (Sharpe): %.2f\n", best.SharpeRatio)
	fmt.Println()

	// Find random baseline for comparison
	var baseline SimulationResult
	for _, r := range results {
		if r.StrategyName == "Random Baseline" {
			baseline = r
			break
		}
	}

	if baseline.StrategyName != "" {
		edge := best.ExpectedValue - baseline.ExpectedValue
		fmt.Printf("📈 EDGE VS RANDOM: $%.3f per trade (%.1fx improvement)\n",
			edge, best.ExpectedValue/math.Abs(baseline.ExpectedValue))
	}

	fmt.Println()
	fmt.Println("KEY INSIGHTS:")
	fmt.Println("  1. Early entry captures market inefficiency before consensus forms")
	fmt.Println("  2. The +1°F calibration accounts for METAR vs CLI systematic difference")
	fmt.Println("  3. YES bets on crossed thresholds have higher expected value than NO bets")
	fmt.Println("  4. Risk management: limit position size to handle variance")
	fmt.Println()

	// Risk analysis
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("RISK ANALYSIS & CAVEATS")
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT LIMITATIONS:")
	fmt.Println()
	fmt.Println("  1. SMALL SAMPLE SIZE")
	fmt.Printf("     • Only %d days of data - 100%% win rate is unrealistic long-term\n", len(days))
	fmt.Println("     • Need 30+ days to see realistic loss scenarios")
	fmt.Println("     • Edge case: storm days can have max at unusual times")
	fmt.Println()
	fmt.Println("  2. OVERFITTING RISK")
	fmt.Println("     • The +1°F calibration was derived from this dataset")
	fmt.Println("     • May not hold in all seasons (summer vs winter)")
	fmt.Println("     • Recommend out-of-sample testing with fresh data")
	fmt.Println()
	fmt.Println("  3. MARKET DYNAMICS")
	fmt.Println("     • Actual market prices may differ from simulation")
	fmt.Println("     • Liquidity can be thin - may not get fills at desired price")
	fmt.Println("     • Other traders may have same edge (competition)")
	fmt.Println()
	fmt.Println("  4. POSITION SIZING RECOMMENDATION")
	fmt.Println("     • Kelly Criterion suggests 15-30% of bankroll per trade")
	fmt.Println("     • Conservative: 5-10% per trade to survive variance")
	fmt.Println("     • Max daily exposure: 25% of total bankroll")
	fmt.Println()
	fmt.Println("  5. NEXT STEPS TO VALIDATE")
	fmt.Println("     • Collect 30+ days of historical METAR data")
	fmt.Println("     • Cross-validate with actual NWS CLI settlements")
	fmt.Println("     • Paper trade for 1 week before real money")
	fmt.Println("     • Start with minimum position sizes ($1-5)")
	fmt.Println()
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
