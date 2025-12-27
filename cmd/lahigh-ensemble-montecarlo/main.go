// Package main runs Monte Carlo simulation on the validated ENSEMBLE strategy
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// Observed statistics from 21-day backtest
const (
	// Strategy triggers on 52% of days (11/21)
	triggerRate = 0.524

	// When triggered, win rate is 81.8% (9/11)
	winRate = 0.818

	// Average prices observed
	avgWinPrice  = 68 // cents - average price when we win
	avgLossPrice = 56 // cents - average price when we lose

	// Fixed bet size
	betSize = 14.0 // dollars per trade
)

type SimResult struct {
	TotalProfit    float64
	NumTrades      int
	NumWins        int
	MaxDrawdown    float64
	FinalBankroll  float64
	SharpeRatio    float64
	WinRate        float64
	AvgProfitTrade float64
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("MONTE CARLO SIMULATION: ENSEMBLE STRATEGY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Print observed statistics
	printObservedStats()

	// Run Monte Carlo
	numSimulations := 10000
	tradingDays := 252 // 1 year

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("RUNNING %d SIMULATIONS (%d trading days each)\n", numSimulations, tradingDays)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	results := runMonteCarlo(numSimulations, tradingDays)

	// Analyze results
	analyzeResults(results)

	// Run with different position sizing
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("POSITION SIZING COMPARISON")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	testPositionSizing(numSimulations, tradingDays)

	// Calculate probability of various outcomes
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("PROBABILITY ANALYSIS")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	calculateProbabilities(results)

	// Risk of ruin analysis
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("RISK OF RUIN ANALYSIS")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	riskOfRuin(1000) // Starting with $1000
}

func printObservedStats() {
	fmt.Println("OBSERVED STATISTICS (21-day backtest):")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("  Trigger rate (days with consensus): %.1f%%\n", triggerRate*100)
	fmt.Printf("  Win rate (when triggered): %.1f%%\n", winRate*100)
	fmt.Printf("  Average winning price: %d¢\n", avgWinPrice)
	fmt.Printf("  Average losing price: %d¢\n", avgLossPrice)
	fmt.Println()

	// Calculate expected values
	avgWinProfit := float64(1400/avgWinPrice) - betSize
	avgLossProfit := -betSize

	fmt.Println("CALCULATED METRICS:")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("  Avg profit on WIN: $%.2f\n", avgWinProfit)
	fmt.Printf("  Avg profit on LOSS: $%.2f\n", avgLossProfit)

	// Expected value per triggered day
	evPerTrade := winRate*avgWinProfit + (1-winRate)*avgLossProfit
	fmt.Printf("  Expected value per TRADE: $%.2f\n", evPerTrade)

	// Expected value per calendar day
	evPerDay := triggerRate * evPerTrade
	fmt.Printf("  Expected value per DAY: $%.2f\n", evPerDay)
	fmt.Printf("  Expected monthly (22 days): $%.2f\n", evPerDay*22)
	fmt.Printf("  Expected yearly (252 days): $%.2f\n", evPerDay*252)
}

func runMonteCarlo(numSims, numDays int) []SimResult {
	results := make([]SimResult, numSims)

	for i := 0; i < numSims; i++ {
		results[i] = simulateYear(numDays)
	}

	return results
}

func simulateYear(numDays int) SimResult {
	var result SimResult
	bankroll := 0.0
	peak := 0.0
	maxDD := 0.0

	var dailyReturns []float64

	for day := 0; day < numDays; day++ {
		// Does strategy trigger today?
		if rand.Float64() > triggerRate {
			dailyReturns = append(dailyReturns, 0)
			continue
		}

		result.NumTrades++

		// Simulate price (random within observed range)
		price := 50 + rand.Intn(40) // 50-90 cents

		var profit float64
		if rand.Float64() < winRate {
			// WIN
			result.NumWins++
			contracts := int(betSize*100) / price
			profit = float64(contracts) - betSize
		} else {
			// LOSS
			profit = -betSize
		}

		bankroll += profit
		dailyReturns = append(dailyReturns, profit)

		// Track drawdown
		if bankroll > peak {
			peak = bankroll
		}
		dd := peak - bankroll
		if dd > maxDD {
			maxDD = dd
		}
	}

	result.TotalProfit = bankroll
	result.MaxDrawdown = maxDD
	result.FinalBankroll = bankroll

	if result.NumTrades > 0 {
		result.WinRate = float64(result.NumWins) / float64(result.NumTrades)
		result.AvgProfitTrade = bankroll / float64(result.NumTrades)
	}

	// Calculate Sharpe ratio
	if len(dailyReturns) > 1 {
		mean := bankroll / float64(len(dailyReturns))
		variance := 0.0
		for _, r := range dailyReturns {
			variance += (r - mean) * (r - mean)
		}
		stdDev := math.Sqrt(variance / float64(len(dailyReturns)))
		if stdDev > 0 {
			result.SharpeRatio = (mean / stdDev) * math.Sqrt(252)
		}
	}

	return result
}

func analyzeResults(results []SimResult) {
	n := len(results)

	// Extract metrics
	var profits, drawdowns, sharpes, winRates []float64
	for _, r := range results {
		profits = append(profits, r.TotalProfit)
		drawdowns = append(drawdowns, r.MaxDrawdown)
		sharpes = append(sharpes, r.SharpeRatio)
		winRates = append(winRates, r.WinRate)
	}

	// Sort for percentiles
	sort.Float64s(profits)
	sort.Float64s(drawdowns)
	sort.Float64s(sharpes)

	fmt.Println("PROFIT DISTRIBUTION (1 year):")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("  Mean: $%.2f\n", mean(profits))
	fmt.Printf("  Median: $%.2f\n", percentile(profits, 50))
	fmt.Printf("  Std Dev: $%.2f\n", stdDev(profits))
	fmt.Printf("  5th percentile: $%.2f (worst case)\n", percentile(profits, 5))
	fmt.Printf("  25th percentile: $%.2f\n", percentile(profits, 25))
	fmt.Printf("  75th percentile: $%.2f\n", percentile(profits, 75))
	fmt.Printf("  95th percentile: $%.2f (best case)\n", percentile(profits, 95))
	fmt.Println()

	fmt.Println("DRAWDOWN DISTRIBUTION:")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("  Mean max drawdown: $%.2f\n", mean(drawdowns))
	fmt.Printf("  Median max drawdown: $%.2f\n", percentile(drawdowns, 50))
	fmt.Printf("  95th percentile: $%.2f (worst case)\n", percentile(drawdowns, 95))
	fmt.Println()

	fmt.Println("SHARPE RATIO DISTRIBUTION:")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("  Mean: %.2f\n", mean(sharpes))
	fmt.Printf("  Median: %.2f\n", percentile(sharpes, 50))
	fmt.Printf("  5th percentile: %.2f\n", percentile(sharpes, 5))
	fmt.Printf("  95th percentile: %.2f\n", percentile(sharpes, 95))
	fmt.Println()

	// Profitable simulations
	profitable := 0
	for _, p := range profits {
		if p > 0 {
			profitable++
		}
	}
	fmt.Printf("PROFITABLE YEARS: %d/%d (%.1f%%)\n", profitable, n, float64(profitable)/float64(n)*100)
}

func testPositionSizing(numSims, numDays int) {
	sizes := []float64{7, 14, 28, 50, 100}

	fmt.Printf("%-12s %12s %12s %12s %12s\n", "Bet Size", "Mean Profit", "Median", "95% Worst", "Max DD")
	fmt.Println(strings.Repeat("-", 60))

	for _, size := range sizes {
		var profits, drawdowns []float64

		for i := 0; i < numSims; i++ {
			result := simulateYearWithSize(numDays, size)
			profits = append(profits, result.TotalProfit)
			drawdowns = append(drawdowns, result.MaxDrawdown)
		}

		sort.Float64s(profits)
		sort.Float64s(drawdowns)

		fmt.Printf("$%-11.0f $%11.2f $%11.2f $%11.2f $%11.2f\n",
			size, mean(profits), percentile(profits, 50), percentile(profits, 5), percentile(drawdowns, 95))
	}
}

func simulateYearWithSize(numDays int, betSize float64) SimResult {
	var result SimResult
	bankroll := 0.0
	peak := 0.0
	maxDD := 0.0

	for day := 0; day < numDays; day++ {
		if rand.Float64() > triggerRate {
			continue
		}

		result.NumTrades++
		price := 50 + rand.Intn(40)

		var profit float64
		if rand.Float64() < winRate {
			result.NumWins++
			contracts := int(betSize*100) / price
			profit = float64(contracts) - betSize
		} else {
			profit = -betSize
		}

		bankroll += profit

		if bankroll > peak {
			peak = bankroll
		}
		dd := peak - bankroll
		if dd > maxDD {
			maxDD = dd
		}
	}

	result.TotalProfit = bankroll
	result.MaxDrawdown = maxDD
	return result
}

func calculateProbabilities(results []SimResult) {
	n := float64(len(results))

	thresholds := []float64{0, 100, 250, 500, 1000}

	fmt.Println("Probability of achieving profit targets (1 year):")
	fmt.Println(strings.Repeat("-", 50))

	for _, t := range thresholds {
		count := 0
		for _, r := range results {
			if r.TotalProfit >= t {
				count++
			}
		}
		fmt.Printf("  P(Profit >= $%.0f): %.1f%%\n", t, float64(count)/n*100)
	}

	fmt.Println()
	fmt.Println("Probability of losses:")
	fmt.Println(strings.Repeat("-", 50))

	lossThresholds := []float64{0, -50, -100, -200}
	for _, t := range lossThresholds {
		count := 0
		for _, r := range results {
			if r.TotalProfit < t {
				count++
			}
		}
		fmt.Printf("  P(Profit < $%.0f): %.1f%%\n", t, float64(count)/n*100)
	}
}

func riskOfRuin(startingBankroll float64) {
	fmt.Printf("Starting bankroll: $%.0f\n", startingBankroll)
	fmt.Printf("Bet size: $%.0f\n", betSize)
	fmt.Println()

	// Simulate until ruin or 5 years
	numSims := 10000
	maxDays := 252 * 5 // 5 years

	ruinCount := 0
	var ruinDays []int

	for i := 0; i < numSims; i++ {
		bankroll := startingBankroll
		ruined := false

		for day := 0; day < maxDays; day++ {
			if rand.Float64() > triggerRate {
				continue
			}

			price := 50 + rand.Intn(40)

			var profit float64
			if rand.Float64() < winRate {
				contracts := int(betSize*100) / price
				profit = float64(contracts) - betSize
			} else {
				profit = -betSize
			}

			bankroll += profit

			if bankroll <= 0 {
				ruined = true
				ruinCount++
				ruinDays = append(ruinDays, day)
				break
			}
		}

		_ = ruined
	}

	fmt.Printf("Risk of ruin (5 years): %.2f%% (%d/%d simulations)\n",
		float64(ruinCount)/float64(numSims)*100, ruinCount, numSims)

	if len(ruinDays) > 0 {
		sort.Ints(ruinDays)
		fmt.Printf("Median days to ruin (if ruined): %d\n", ruinDays[len(ruinDays)/2])
	}

	// Calculate theoretical risk of ruin
	// Using simplified formula: R = ((1-p)/p)^n where n = bankroll/bet
	p := winRate * triggerRate // Overall daily win probability when trading
	q := 1 - p
	if p > q {
		units := startingBankroll / betSize
		theoreticalRoR := math.Pow(q/p, units)
		fmt.Printf("Theoretical risk of ruin: %.6f%%\n", theoreticalRoR*100)
	}
}

func mean(nums []float64) float64 {
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	return sum / float64(len(nums))
}

func stdDev(nums []float64) float64 {
	m := mean(nums)
	variance := 0.0
	for _, n := range nums {
		variance += (n - m) * (n - m)
	}
	return math.Sqrt(variance / float64(len(nums)))
}

func percentile(sorted []float64, p float64) float64 {
	idx := int(float64(len(sorted)-1) * p / 100)
	return sorted[idx]
}
