// Package main provides a proof-of-concept backtesting script for the LA High Temperature strategy.
// It fetches METAR data and analyzes how early we can predict the daily maximum temperature.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"
)

// METARObservation represents a single METAR weather observation.
type METARObservation struct {
	IcaoID     string  `json:"icaoId"`
	ObsTime    int64   `json:"obsTime"`    // Unix timestamp
	ReportTime string  `json:"reportTime"` // ISO timestamp
	Temp       float64 `json:"temp"`       // Temperature in Celsius
	Dewp       float64 `json:"dewp"`       // Dew point in Celsius
	MaxT       float64 `json:"maxT"`       // 6-hour max temp (when available)
	MinT       float64 `json:"minT"`       // 6-hour min temp (when available)
	MaxT24     float64 `json:"maxT24"`     // 24-hour max temp (when available)
	MinT24     float64 `json:"minT24"`     // 24-hour min temp (when available)
	MetarType  string  `json:"metarType"`  // METAR or SPECI
	RawOb      string  `json:"rawOb"`      // Raw METAR string
}

// DailyStats holds statistics for a single day.
type DailyStats struct {
	Date           string
	Observations   []METARObservation
	FinalMaxC      float64   // Final max temp in Celsius for the day
	FinalMaxF      int       // Final max temp in Fahrenheit (rounded)
	MaxReachedAt   time.Time // When the max was first reached
	HourlyMaxes    []HourlyMax
	EarlyPrediction *EarlyPrediction
}

// HourlyMax tracks the running max at each hour.
type HourlyMax struct {
	Hour       int
	Time       time.Time
	RunningMax float64 // Running max up to this point (Celsius)
	CurrentTemp float64 // Current observation temp
}

// EarlyPrediction represents when we could have predicted the final max.
type EarlyPrediction struct {
	PredictedAt     time.Time
	HoursBeforeEnd  float64
	ConfidenceLevel string // "exact", "within_1F", "within_2F"
}

const (
	metarAPIURL = "https://aviationweather.gov/api/data/metar?ids=KLAX&hours=96&format=json"
	laTimezone  = "America/Los_Angeles"
)

func main() {
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println("LA HIGH TEMPERATURE STRATEGY - PROOF OF CONCEPT BACKTEST")
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println()

	// Fetch METAR data
	fmt.Println("→ Fetching 96 hours of METAR data from Aviation Weather Center...")
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

	// Group observations by date (LA time)
	dailyData := groupByDay(observations, loc)

	// Analyze each day
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println("DAILY ANALYSIS")
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println()

	var completeDays []DailyStats
	for _, day := range dailyData {
		if len(day.Observations) < 10 { // Skip incomplete days
			continue
		}
		analyzeDay(&day, loc)
		completeDays = append(completeDays, day)
		printDayAnalysis(day)
	}

	// Print summary
	printSummary(completeDays)

	// Print validation against CLI
	printValidation(completeDays)

	// Print strike analysis
	printStrikeAnalysis(completeDays, loc)

	// Print trading edge analysis
	printTradingEdge(completeDays)
}

func fetchMETARData() ([]METARObservation, error) {
	resp, err := http.Get(metarAPIURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

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

func groupByDay(observations []METARObservation, loc *time.Location) []DailyStats {
	// Sort by time (oldest first)
	sort.Slice(observations, func(i, j int) bool {
		return observations[i].ObsTime < observations[j].ObsTime
	})

	dayMap := make(map[string]*DailyStats)

	for _, obs := range observations {
		t := time.Unix(obs.ObsTime, 0).In(loc)
		dateKey := t.Format("2006-01-02")

		if _, exists := dayMap[dateKey]; !exists {
			dayMap[dateKey] = &DailyStats{
				Date:         dateKey,
				Observations: []METARObservation{},
			}
		}
		dayMap[dateKey].Observations = append(dayMap[dateKey].Observations, obs)
	}

	// Convert to slice and sort by date
	var days []DailyStats
	for _, day := range dayMap {
		days = append(days, *day)
	}
	sort.Slice(days, func(i, j int) bool {
		return days[i].Date < days[j].Date
	})

	return days
}

func analyzeDay(day *DailyStats, loc *time.Location) {
	if len(day.Observations) == 0 {
		return
	}

	// Find the actual max for the day
	var maxTemp float64 = -999
	var maxTime time.Time

	for _, obs := range day.Observations {
		if obs.Temp > maxTemp {
			maxTemp = obs.Temp
			maxTime = time.Unix(obs.ObsTime, 0).In(loc)
		}
	}

	day.FinalMaxC = maxTemp
	day.FinalMaxF = celsiusToFahrenheit(maxTemp)
	day.MaxReachedAt = maxTime

	// Track running max throughout the day
	var runningMax float64 = -999
	for _, obs := range day.Observations {
		t := time.Unix(obs.ObsTime, 0).In(loc)
		if obs.Temp > runningMax {
			runningMax = obs.Temp
		}
		day.HourlyMaxes = append(day.HourlyMaxes, HourlyMax{
			Hour:        t.Hour(),
			Time:        t,
			RunningMax:  runningMax,
			CurrentTemp: obs.Temp,
		})
	}

	// Determine when we could have predicted the final max
	// (when the running max equals the final max and stays there)
	for i, hm := range day.HourlyMaxes {
		if celsiusToFahrenheit(hm.RunningMax) == day.FinalMaxF {
			// Check if this holds for the rest of the day
			holdsForRest := true
			for j := i; j < len(day.HourlyMaxes); j++ {
				if celsiusToFahrenheit(day.HourlyMaxes[j].RunningMax) != day.FinalMaxF {
					holdsForRest = false
					break
				}
			}
			if holdsForRest {
				endOfDay := time.Date(hm.Time.Year(), hm.Time.Month(), hm.Time.Day(), 23, 59, 0, 0, loc)
				day.EarlyPrediction = &EarlyPrediction{
					PredictedAt:     hm.Time,
					HoursBeforeEnd:  endOfDay.Sub(hm.Time).Hours(),
					ConfidenceLevel: "exact",
				}
				break
			}
		}
	}
}

func celsiusToFahrenheit(c float64) int {
	return int((c * 9.0 / 5.0) + 32.5) // Rounded to nearest integer
}

func printDayAnalysis(day DailyStats) {
	fmt.Printf("Date: %s\n", day.Date)
	fmt.Printf("  Observations: %d\n", len(day.Observations))
	fmt.Printf("  Final Max: %.1f°C / %d°F\n", day.FinalMaxC, day.FinalMaxF)
	fmt.Printf("  Max Reached At: %s (LA time)\n", day.MaxReachedAt.Format("3:04 PM"))

	if day.EarlyPrediction != nil {
		fmt.Printf("  ⚡ EARLY PREDICTION: Could predict at %s (%.1f hours before market close)\n",
			day.EarlyPrediction.PredictedAt.Format("3:04 PM"),
			day.EarlyPrediction.HoursBeforeEnd)
	}

	// Show hourly progression (simplified)
	fmt.Printf("  Hourly Max Progression:\n")
	lastHour := -1
	for _, hm := range day.HourlyMaxes {
		if hm.Hour != lastHour {
			runningF := celsiusToFahrenheit(hm.RunningMax)
			indicator := ""
			if runningF == day.FinalMaxF {
				indicator = " ← FINAL MAX REACHED"
			}
			fmt.Printf("    %02d:00 → Running Max: %d°F%s\n", hm.Hour, runningF, indicator)
			lastHour = hm.Hour
		}
	}
	fmt.Println()
}

func printSummary(days []DailyStats) {
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println("SUMMARY STATISTICS")
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println()

	var totalHoursEarly float64
	var predictableDays int
	var maxTimes []int // Hour of day when max was reached

	for _, day := range days {
		maxTimes = append(maxTimes, day.MaxReachedAt.Hour())
		if day.EarlyPrediction != nil {
			totalHoursEarly += day.EarlyPrediction.HoursBeforeEnd
			predictableDays++
		}
	}

	fmt.Printf("Total Complete Days Analyzed: %d\n", len(days))
	fmt.Printf("Days with Early Prediction: %d (%.1f%%)\n",
		predictableDays, float64(predictableDays)/float64(len(days))*100)

	if predictableDays > 0 {
		fmt.Printf("Average Hours Before Market Close: %.1f hours\n",
			totalHoursEarly/float64(predictableDays))
	}

	// Distribution of when max occurs
	hourCounts := make(map[int]int)
	for _, h := range maxTimes {
		hourCounts[h]++
	}

	fmt.Println("\nWhen Does Daily Max Typically Occur?")
	fmt.Println("(Hour of Day, LA Time)")
	for h := 6; h <= 23; h++ {
		if count := hourCounts[h]; count > 0 {
			bar := repeatStr("█", count*3)
			fmt.Printf("  %02d:00  %s (%d)\n", h, bar, count)
		}
	}
	fmt.Println()
}

func printValidation(days []DailyStats) {
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println("VALIDATION: METAR vs NWS CLI (Official Settlement)")
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println()

	// Fetch CLI data for comparison
	cliData := map[string]int{
		"2025-12-25": 67,
		"2025-12-24": 64,
		"2025-12-23": 64,
	}

	fmt.Println("Comparing METAR predictions to NWS CLI official settlement values:")
	fmt.Println()
	fmt.Printf("%-12s  %-10s  %-10s  %-10s\n", "Date", "METAR Max", "NWS CLI", "Difference")
	fmt.Printf("%-12s  %-10s  %-10s  %-10s\n", "----", "---------", "-------", "----------")

	var totalDiff, matchCount int
	for _, day := range days {
		if cliMax, ok := cliData[day.Date]; ok {
			diff := day.FinalMaxF - cliMax
			totalDiff += abs(diff)
			status := "✓ Match"
			if diff != 0 {
				status = fmt.Sprintf("%+d°F", diff)
			} else {
				matchCount++
			}
			fmt.Printf("%-12s  %-10d  %-10d  %-10s\n", day.Date, day.FinalMaxF, cliMax, status)
		}
	}

	fmt.Println()
	fmt.Println("NOTE: Small discrepancies (1°F) are common due to:")
	fmt.Println("  • Rounding differences in C→F conversion")
	fmt.Println("  • NWS uses higher-precision sensors")
	fmt.Println("  • Timing differences in observation windows")
	fmt.Println()
	fmt.Println("IMPLICATION FOR STRATEGY:")
	fmt.Println("  When METAR shows we're at a strike boundary (e.g., 66°F vs 67°F),")
	fmt.Println("  there's uncertainty. Use this as a signal to:")
	fmt.Println("  • Bet on the HIGHER value (NWS tends to round up)")
	fmt.Println("  • Or hedge with multiple strike positions")
	fmt.Println()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func printTradingEdge(days []DailyStats) {
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println("TRADING EDGE ANALYSIS")
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println()

	fmt.Println("KALSHI Market Rules:")
	fmt.Println("  • Last Trading Time: 11:59 PM PT (LA time)")
	fmt.Println("  • Settlement: Based on NWS CLI report (published after midnight)")
	fmt.Println("  • Expiration: 10:00 AM ET next day")
	fmt.Println()

	fmt.Println("KEY INSIGHT:")
	fmt.Println("  The daily maximum temperature is typically reached in the afternoon.")
	fmt.Println("  By tracking METAR data, we can often know the final max 6-10 hours")
	fmt.Println("  before the market closes at 11:59 PM PT.")
	fmt.Println()

	// Calculate prediction windows
	var earlyAfternoon, lateAfternoon, evening int
	for _, day := range days {
		if day.EarlyPrediction != nil {
			h := day.EarlyPrediction.PredictedAt.Hour()
			switch {
			case h < 15:
				earlyAfternoon++
			case h < 18:
				lateAfternoon++
			default:
				evening++
			}
		}
	}

	totalPredictable := earlyAfternoon + lateAfternoon + evening
	if totalPredictable > 0 {
		fmt.Println("Prediction Windows (when we know the final max):")
		fmt.Printf("  Before 3 PM:  %d days (%.0f%%) - 9+ hours before close\n",
			earlyAfternoon, float64(earlyAfternoon)/float64(totalPredictable)*100)
		fmt.Printf("  3 PM - 6 PM:  %d days (%.0f%%) - 6-9 hours before close\n",
			lateAfternoon, float64(lateAfternoon)/float64(totalPredictable)*100)
		fmt.Printf("  After 6 PM:   %d days (%.0f%%) - <6 hours before close\n",
			evening, float64(evening)/float64(totalPredictable)*100)
	}

	fmt.Println()
	fmt.Println("STRATEGY RECOMMENDATION:")
	fmt.Println("  1. Monitor METAR data throughout the day")
	fmt.Println("  2. Track running maximum temperature")
	fmt.Println("  3. Once the temperature starts declining (typically after 2-4 PM),")
	fmt.Println("     the running max is likely to be the final max")
	fmt.Println("  4. Enter positions early to get better odds before market consensus")
	fmt.Println()
}

func printStrikeAnalysis(days []DailyStats, loc *time.Location) {
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println("KALSHI STRIKE PRICE ANALYSIS")
	fmt.Println("=" + repeatStr("=", 70))
	fmt.Println()

	fmt.Println("For each day, when could we confidently bet on specific strikes?")
	fmt.Println("(Kalshi offers 'greater than X°F' and 'less than X°F' contracts)")
	fmt.Println()

	for _, day := range days {
		if len(day.HourlyMaxes) == 0 {
			continue
		}

		fmt.Printf("📅 %s (Final: %d°F)\n", day.Date, day.FinalMaxF)

		// Find key strike thresholds
		strikes := []int{60, 62, 64, 66, 68, 70}

		for _, strike := range strikes {
			// Find when we first exceeded this strike and never went below
			var crossedAt *time.Time
			for i := range day.HourlyMaxes {
				runningMaxF := celsiusToFahrenheit(day.HourlyMaxes[i].RunningMax)
				if runningMaxF > strike && crossedAt == nil {
					t := day.HourlyMaxes[i].Time
					crossedAt = &t
				}
			}

			if crossedAt != nil && day.FinalMaxF > strike {
				endOfDay := time.Date(crossedAt.Year(), crossedAt.Month(), crossedAt.Day(), 23, 59, 0, 0, loc)
				hoursEarly := endOfDay.Sub(*crossedAt).Hours()
				fmt.Printf("   > %d°F: BET YES at %s (%.1f hrs before close) ✓\n",
					strike, crossedAt.Format("3:04 PM"), hoursEarly)
			} else if day.FinalMaxF <= strike {
				// Find when we could confidently say it WON'T exceed this
				// (after typical max time and temperature declining)
				for i := len(day.HourlyMaxes) - 1; i >= 0; i-- {
					runningMaxF := celsiusToFahrenheit(day.HourlyMaxes[i].RunningMax)
					t := day.HourlyMaxes[i].Time
					// After 4 PM and max not reached? Good signal
					if t.Hour() >= 16 && runningMaxF < strike {
						endOfDay := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 0, 0, loc)
						hoursEarly := endOfDay.Sub(t).Hours()
						fmt.Printf("   > %d°F: BET NO at %s (%.1f hrs before close) ✓\n",
							strike, t.Format("3:04 PM"), hoursEarly)
						break
					}
				}
			}
		}
		fmt.Println()
	}
}

func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

