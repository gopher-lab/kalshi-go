// Package main predicts tomorrow's LA High Temperature for Kalshi trading.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
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
	WxString   string  `json:"wxString"` // Weather conditions
}

// KalshiMarket represents the market prices
type KalshiMarket struct {
	Strike   string
	YesPrice float64
	NoPrice  float64
}

// Prediction represents our model's prediction
type Prediction struct {
	Strike      string
	Probability float64
	Edge        float64 // Our prob - market implied prob
	Recommendation string
	Confidence  string
}

const (
	metarAPIURL = "https://aviationweather.gov/api/data/metar?ids=KLAX&hours=96&format=json"
	laTimezone  = "America/Los_Angeles"
	
	// Historical normals for LA (late December)
	normalHighF = 66
	normalLowF  = 49
)

func main() {
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("LA HIGH TEMPERATURE - PREDICTION FOR DECEMBER 27, 2025")
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println()

	// Fetch METAR data
	fmt.Println("→ Fetching current METAR data...")
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

	// Analyze recent data
	analysis := analyzeRecentData(observations, loc)
	
	// Print current conditions
	printCurrentConditions(observations, loc)
	
	// Print recent history
	printRecentHistory(analysis)
	
	// Define Kalshi market (from user's input)
	markets := []KalshiMarket{
		{Strike: "55 or below", YesPrice: 0.04, NoPrice: 0.98},
		{Strike: "56-57", YesPrice: 0.07, NoPrice: 0.95},
		{Strike: "58-59", YesPrice: 0.26, NoPrice: 0.76},
		{Strike: "60-61", YesPrice: 0.37, NoPrice: 0.65},
		{Strike: "62-63", YesPrice: 0.30, NoPrice: 0.73},
		{Strike: "64 or above", YesPrice: 0.13, NoPrice: 0.92},
	}
	
	// Generate predictions
	predictions := generatePredictions(analysis, markets)
	
	// Print market analysis
	printMarketAnalysis(markets, predictions)
	
	// Print trading recommendation
	printRecommendation(predictions, analysis)
}

type DayAnalysis struct {
	Date     string
	MaxTempF int
	CLIMaxF  int // +1°F calibration
	Weather  string
}

type RecentAnalysis struct {
	Days           []DayAnalysis
	AvgMaxF        float64
	TrendDirection string // "warming", "cooling", "stable"
	HasRain        bool
	CurrentTempF   int
	CurrentTime    time.Time
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

func analyzeRecentData(observations []METARObservation, loc *time.Location) RecentAnalysis {
	// Sort by time (oldest first)
	sort.Slice(observations, func(i, j int) bool {
		return observations[i].ObsTime < observations[j].ObsTime
	})

	// Group by date
	dayMap := make(map[string][]METARObservation)
	for _, obs := range observations {
		t := time.Unix(obs.ObsTime, 0).In(loc)
		dateKey := t.Format("2006-01-02")
		dayMap[dateKey] = append(dayMap[dateKey], obs)
	}

	// Calculate daily stats
	var days []DayAnalysis
	var hasRain bool

	for date, dayObs := range dayMap {
		if len(dayObs) < 5 { // Skip incomplete days
			continue
		}

		var maxTemp float64 = -999
		var weather string
		for _, obs := range dayObs {
			if obs.Temp > maxTemp {
				maxTemp = obs.Temp
			}
			if obs.WxString != "" {
				if containsRain(obs.WxString) {
					weather = "Rain"
					hasRain = true
				} else if weather == "" {
					weather = obs.WxString
				}
			}
		}

		maxF := celsiusToFahrenheit(maxTemp)
		days = append(days, DayAnalysis{
			Date:     date,
			MaxTempF: maxF,
			CLIMaxF:  maxF + 1, // +1°F calibration
			Weather:  weather,
		})
	}

	// Sort days by date
	sort.Slice(days, func(i, j int) bool {
		return days[i].Date < days[j].Date
	})

	// Calculate average and trend
	var sum float64
	for _, d := range days {
		sum += float64(d.CLIMaxF)
	}
	avgMax := sum / float64(len(days))

	// Determine trend (compare first half to second half)
	trend := "stable"
	if len(days) >= 4 {
		firstHalf := (float64(days[0].CLIMaxF) + float64(days[1].CLIMaxF)) / 2
		secondHalf := (float64(days[len(days)-2].CLIMaxF) + float64(days[len(days)-1].CLIMaxF)) / 2
		if secondHalf > firstHalf+1 {
			trend = "warming"
		} else if secondHalf < firstHalf-1 {
			trend = "cooling"
		}
	}

	// Get current conditions
	var currentTempF int
	var currentTime time.Time
	if len(observations) > 0 {
		latest := observations[0] // Most recent
		currentTempF = celsiusToFahrenheit(latest.Temp)
		currentTime = time.Unix(latest.ObsTime, 0).In(loc)
	}

	return RecentAnalysis{
		Days:           days,
		AvgMaxF:        avgMax,
		TrendDirection: trend,
		HasRain:        hasRain,
		CurrentTempF:   currentTempF,
		CurrentTime:    currentTime,
	}
}

func containsRain(wx string) bool {
	rainIndicators := []string{"RA", "SHRA", "TSRA", "DZ", "-RA", "+RA"}
	for _, r := range rainIndicators {
		if len(wx) >= len(r) {
			for i := 0; i <= len(wx)-len(r); i++ {
				if wx[i:i+len(r)] == r {
					return true
				}
			}
		}
	}
	return false
}

func generatePredictions(analysis RecentAnalysis, markets []KalshiMarket) []Prediction {
	// Build probability distribution based on historical data
	// Use the average + trend adjustment
	
	expectedMax := analysis.AvgMaxF
	
	// Trend adjustment
	switch analysis.TrendDirection {
	case "warming":
		expectedMax += 1.5
	case "cooling":
		expectedMax -= 1.5
	}
	
	// Rain adjustment (rain typically means cooler temps)
	if analysis.HasRain {
		expectedMax -= 1.0
	}
	
	// Standard deviation from historical data (~3°F for LA winter)
	stdDev := 3.0
	
	fmt.Printf("📊 MODEL PARAMETERS:\n")
	fmt.Printf("   Expected Max (CLI): %.1f°F\n", expectedMax)
	fmt.Printf("   Std Dev: %.1f°F\n", stdDev)
	fmt.Printf("   Trend: %s\n", analysis.TrendDirection)
	fmt.Printf("   Recent Rain: %v\n\n", analysis.HasRain)
	
	// Calculate probabilities for each bracket
	predictions := make([]Prediction, len(markets))
	
	for i, market := range markets {
		var prob float64
		
		switch market.Strike {
		case "55 or below":
			prob = normalCDF(55.5, expectedMax, stdDev)
		case "56-57":
			prob = normalCDF(57.5, expectedMax, stdDev) - normalCDF(55.5, expectedMax, stdDev)
		case "58-59":
			prob = normalCDF(59.5, expectedMax, stdDev) - normalCDF(57.5, expectedMax, stdDev)
		case "60-61":
			prob = normalCDF(61.5, expectedMax, stdDev) - normalCDF(59.5, expectedMax, stdDev)
		case "62-63":
			prob = normalCDF(63.5, expectedMax, stdDev) - normalCDF(61.5, expectedMax, stdDev)
		case "64 or above":
			prob = 1 - normalCDF(63.5, expectedMax, stdDev)
		}
		
		// Market implied probability
		impliedProb := market.YesPrice
		
		// Edge = our probability - market probability
		edge := prob - impliedProb
		
		// Determine recommendation
		rec := "PASS"
		confidence := "Low"
		
		if edge > 0.10 { // 10%+ edge
			rec = "BUY YES"
			if edge > 0.20 {
				confidence = "High"
			} else {
				confidence = "Medium"
			}
		} else if edge < -0.10 {
			rec = "BUY NO"
			if edge < -0.20 {
				confidence = "High"
			} else {
				confidence = "Medium"
			}
		}
		
		predictions[i] = Prediction{
			Strike:         market.Strike,
			Probability:    prob,
			Edge:           edge,
			Recommendation: rec,
			Confidence:     confidence,
		}
	}
	
	return predictions
}

// Normal CDF using error function approximation
func normalCDF(x, mean, stdDev float64) float64 {
	return 0.5 * (1 + math.Erf((x-mean)/(stdDev*math.Sqrt2)))
}

func printCurrentConditions(observations []METARObservation, loc *time.Location) {
	if len(observations) == 0 {
		return
	}
	
	latest := observations[0]
	t := time.Unix(latest.ObsTime, 0).In(loc)
	
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("CURRENT CONDITIONS AT LAX")
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Printf("Time: %s\n", t.Format("Mon Jan 2, 2006 3:04 PM MST"))
	fmt.Printf("Temperature: %d°F (%.1f°C)\n", celsiusToFahrenheit(latest.Temp), latest.Temp)
	fmt.Printf("Dew Point: %d°F\n", celsiusToFahrenheit(latest.Dewp))
	if latest.WxString != "" {
		fmt.Printf("Weather: %s\n", latest.WxString)
	}
	fmt.Printf("Raw METAR: %s\n", latest.RawOb)
	fmt.Println()
}

func printRecentHistory(analysis RecentAnalysis) {
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("RECENT DAILY HIGHS (with +1°F CLI calibration)")
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Printf("%-12s  %-10s  %-10s  %-15s\n", "Date", "METAR Max", "CLI Est*", "Weather")
	fmt.Printf("%-12s  %-10s  %-10s  %-15s\n", "----", "---------", "--------", "-------")
	
	for _, day := range analysis.Days {
		fmt.Printf("%-12s  %-10d  %-10d  %-15s\n", 
			day.Date, day.MaxTempF, day.CLIMaxF, day.Weather)
	}
	
	fmt.Println()
	fmt.Printf("Average CLI Max: %.1f°F\n", analysis.AvgMaxF)
	fmt.Printf("Trend: %s\n", analysis.TrendDirection)
	fmt.Printf("Normal for Dec 27: %d°F\n", normalHighF)
	fmt.Println()
}

func printMarketAnalysis(markets []KalshiMarket, predictions []Prediction) {
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("MARKET ANALYSIS - December 27, 2025")
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println()
	
	fmt.Printf("%-14s  %-8s  %-10s  %-8s  %-10s  %-12s\n",
		"Strike", "Mkt Yes", "Our Prob", "Edge", "Action", "Confidence")
	fmt.Printf("%-14s  %-8s  %-10s  %-8s  %-10s  %-12s\n",
		"------", "-------", "--------", "----", "------", "----------")
	
	for i, market := range markets {
		pred := predictions[i]
		edgeStr := fmt.Sprintf("%+.0f%%", pred.Edge*100)
		
		actionIcon := "  "
		if pred.Recommendation == "BUY YES" {
			actionIcon = "🟢"
		} else if pred.Recommendation == "BUY NO" {
			actionIcon = "🔴"
		}
		
		fmt.Printf("%-14s  %-8.0f¢  %-10.0f%%  %-8s  %s %-8s  %-12s\n",
			market.Strike,
			market.YesPrice*100,
			pred.Probability*100,
			edgeStr,
			actionIcon,
			pred.Recommendation,
			pred.Confidence)
	}
	fmt.Println()
}

func printRecommendation(predictions []Prediction, analysis RecentAnalysis) {
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println("🎯 TRADING RECOMMENDATION")
	fmt.Println("=" + repeatStr("=", 78))
	fmt.Println()
	
	// Find best opportunities
	var bestYes, bestNo *Prediction
	for i := range predictions {
		p := &predictions[i]
		if p.Edge > 0 && (bestYes == nil || p.Edge > bestYes.Edge) {
			bestYes = p
		}
		if p.Edge < 0 && (bestNo == nil || p.Edge < bestNo.Edge) {
			bestNo = p
		}
	}
	
	if bestYes != nil && bestYes.Edge > 0.05 {
		fmt.Printf("✅ BUY YES on \"%s\"\n", bestYes.Strike)
		fmt.Printf("   Model Probability: %.0f%%\n", bestYes.Probability*100)
		fmt.Printf("   Market Price: Implies %.0f%%\n", (1-math.Abs(bestYes.Edge))*bestYes.Probability*100)
		fmt.Printf("   Edge: %+.1f%%\n", bestYes.Edge*100)
		fmt.Printf("   Confidence: %s\n", bestYes.Confidence)
		fmt.Println()
	}
	
	if bestNo != nil && bestNo.Edge < -0.05 {
		fmt.Printf("✅ BUY NO on \"%s\"\n", bestNo.Strike)
		fmt.Printf("   Model Probability (NO): %.0f%%\n", (1-bestNo.Probability)*100)
		fmt.Printf("   Edge: %+.1f%% (market overpricing YES)\n", -bestNo.Edge*100)
		fmt.Printf("   Confidence: %s\n", bestNo.Confidence)
		fmt.Println()
	}
	
	// Cross-validate with NWS forecast
	fmt.Println("🌤️  NWS OFFICIAL FORECAST (api.weather.gov):")
	fmt.Println("   Saturday Dec 27: 61°F, Mostly Sunny")
	fmt.Println("   With +1°F CLI calibration: ~62°F")
	fmt.Println()
	
	// Overall outlook
	fmt.Println("📈 FORECAST SUMMARY:")
	fmt.Printf("   Model Expected: %.0f°F (based on recent data)\n", analysis.AvgMaxF)
	fmt.Println("   NWS Forecast: 61°F (62°F with CLI calibration)")
	fmt.Printf("   Most Likely Bracket: ")
	
	// Find highest probability bracket
	maxProb := 0.0
	maxBracket := ""
	for _, p := range predictions {
		if p.Probability > maxProb {
			maxProb = p.Probability
			maxBracket = p.Strike
		}
	}
	fmt.Printf("%s (%.0f%% probability)\n", maxBracket, maxProb*100)
	
	fmt.Println()
	fmt.Println("⚠️  RISK FACTORS:")
	fmt.Println("   • Weather systems can shift - monitor updates")
	if analysis.HasRain {
		fmt.Println("   • Recent rain may continue - could suppress temps")
	}
	fmt.Println("   • Model based on limited data (5 days)")
	fmt.Println("   • Recommend small position sizes ($5-10)")
	fmt.Println()
	
	fmt.Println("📋 ACTION PLAN:")
	fmt.Println("   1. Check weather forecast for Dec 27 (NWS, AccuWeather)")
	fmt.Println("   2. Monitor METAR tomorrow morning for early signals")
	fmt.Println("   3. Enter position when confidence is high")
	fmt.Println("   4. Track running max via METAR throughout the day")
	fmt.Println()
}

func celsiusToFahrenheit(c float64) int {
	return int((c * 9.0 / 5.0) + 32.5)
}

func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

