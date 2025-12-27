// Package main provides an improved prediction model that incorporates NWS forecasts.
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

// METARObservation represents a single METAR weather observation.
type METARObservation struct {
	IcaoID   string  `json:"icaoId"`
	ObsTime  int64   `json:"obsTime"`
	Temp     float64 `json:"temp"`
	WxString string  `json:"wxString"`
}

// NWSForecast represents the NWS forecast data
type NWSForecast struct {
	Properties struct {
		Periods []struct {
			Name          string `json:"name"`
			Temperature   int    `json:"temperature"`
			ShortForecast string `json:"shortForecast"`
		} `json:"periods"`
	} `json:"properties"`
}

// KalshiMarket represents the market prices
type KalshiMarket struct {
	Strike    string
	LowBound  int
	HighBound int
	YesPrice  float64
}

const (
	metarAPIURL    = "https://aviationweather.gov/api/data/metar?ids=KLAX&hours=96&format=json"
	nwsForecastURL = "https://api.weather.gov/gridpoints/LOX/154,44/forecast"
	laTimezone     = "America/Los_Angeles"
)

func main() {
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("LA HIGH TEMPERATURE - IMPROVED PREDICTION MODEL v2")
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println()

	// Fetch NWS forecast (primary source)
	fmt.Println("→ Fetching NWS official forecast...")
	nwsForecast, err := fetchNWSForecast()
	if err != nil {
		fmt.Printf("⚠ Warning: Could not fetch NWS forecast: %v\n", err)
	}

	// Fetch METAR data (for calibration)
	fmt.Println("→ Fetching METAR observations...")
	observations, err := fetchMETARData()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching METAR data: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Fetched %d METAR observations\n\n", len(observations))

	loc, _ := time.LoadLocation(laTimezone)

	// Display forecast
	printForecast(nwsForecast)

	// Calculate calibration from recent data
	calibration := calculateCalibration(observations, loc)
	fmt.Printf("📊 METAR→CLI Calibration: +%.1f°F\n\n", calibration)

	// Get Saturday's forecast
	saturdayForecast := getSaturdayForecast(nwsForecast)
	if saturdayForecast == 0 {
		fmt.Println("⚠ Could not find Saturday forecast, using default")
		saturdayForecast = 61
	}

	// Apply calibration
	expectedCLI := float64(saturdayForecast) + calibration

	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("IMPROVED PREDICTION FOR DECEMBER 27, 2025")
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println()
	fmt.Printf("NWS Forecast High: %d°F\n", saturdayForecast)
	fmt.Printf("METAR→CLI Calibration: +%.1f°F\n", calibration)
	fmt.Printf("Expected CLI Settlement: %.0f°F\n", expectedCLI)
	fmt.Println()

	// Model uncertainty (NWS forecasts have ~2°F error for next day)
	stdDev := 2.0 // NWS next-day forecast error

	// Kalshi markets
	markets := []KalshiMarket{
		{Strike: "55 or below", LowBound: 0, HighBound: 55, YesPrice: 0.04},
		{Strike: "56-57", LowBound: 56, HighBound: 57, YesPrice: 0.07},
		{Strike: "58-59", LowBound: 58, HighBound: 59, YesPrice: 0.26},
		{Strike: "60-61", LowBound: 60, HighBound: 61, YesPrice: 0.37},
		{Strike: "62-63", LowBound: 62, HighBound: 63, YesPrice: 0.30},
		{Strike: "64 or above", LowBound: 64, HighBound: 999, YesPrice: 0.13},
	}

	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("MARKET ANALYSIS")
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println()
	fmt.Printf("%-15s %-10s %-12s %-10s %-15s\n",
		"Strike", "Mkt Price", "Our Prob", "Edge", "Action")
	fmt.Printf("%-15s %-10s %-12s %-10s %-15s\n",
		"------", "---------", "--------", "----", "------")

	var bestBet *KalshiMarket
	var bestEdge float64
	var bestProb float64

	for i := range markets {
		m := &markets[i]

		// Calculate probability using normal distribution
		var prob float64
		if m.HighBound == 999 {
			prob = 1 - normalCDF(float64(m.LowBound)-0.5, expectedCLI, stdDev)
		} else if m.LowBound == 0 {
			prob = normalCDF(float64(m.HighBound)+0.5, expectedCLI, stdDev)
		} else {
			prob = normalCDF(float64(m.HighBound)+0.5, expectedCLI, stdDev) -
				normalCDF(float64(m.LowBound)-0.5, expectedCLI, stdDev)
		}

		edge := prob - m.YesPrice

		action := "PASS"
		if edge > 0.10 {
			action = "🟢 BUY YES"
		} else if edge < -0.10 {
			action = "🔴 BUY NO"
		}

		fmt.Printf("%-15s %-10.0f¢ %-12.0f%% %-+10.0f%% %-15s\n",
			m.Strike, m.YesPrice*100, prob*100, edge*100, action)

		if edge > bestEdge {
			bestEdge = edge
			bestBet = m
			bestProb = prob
		}
	}

	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("🎯 FINAL RECOMMENDATION")
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println()

	if bestBet != nil && bestEdge > 0.05 {
		fmt.Printf("✅ BUY YES on \"%s\"\n", bestBet.Strike)
		fmt.Printf("   Expected Probability: %.0f%%\n", bestProb*100)
		fmt.Printf("   Market Price: %.0f¢\n", bestBet.YesPrice*100)
		fmt.Printf("   Edge: +%.0f%%\n", bestEdge*100)
		fmt.Println()

		// Expected value calculation
		ev := bestProb*(1-bestBet.YesPrice)*0.93 - (1-bestProb)*bestBet.YesPrice
		fmt.Printf("   Expected Value: $%.3f per $1 risked\n", ev)
		fmt.Printf("   (after 7%% Kalshi fee)\n")
	}

	fmt.Println()
	fmt.Println("📋 WHY THIS PREDICTION:")
	fmt.Printf("   1. NWS forecasts %d°F for Saturday (post-storm cooling)\n", saturdayForecast)
	fmt.Printf("   2. Historical CLI readings are ~%.0f°F higher than forecasts\n", calibration)
	fmt.Printf("   3. Standard deviation of ~%.0f°F for next-day forecasts\n", stdDev)
	fmt.Println("   4. Using physics-based NWS models, not just historical averages")
	fmt.Println()

	fmt.Println("⚠️  REMAINING UNCERTAINTY:")
	fmt.Println("   • NWS forecast could shift (check updates tomorrow AM)")
	fmt.Println("   • CLI calibration is based on limited data")
	fmt.Println("   • Unusual weather events can cause surprises")
	fmt.Println()
}

func fetchNWSForecast() (*NWSForecast, error) {
	resp, err := http.Get(nwsForecastURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var forecast NWSForecast
	if err := json.Unmarshal(body, &forecast); err != nil {
		return nil, err
	}

	return &forecast, nil
}

func fetchMETARData() ([]METARObservation, error) {
	resp, err := http.Get(metarAPIURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var observations []METARObservation
	if err := json.Unmarshal(body, &observations); err != nil {
		return nil, err
	}

	return observations, nil
}

func printForecast(forecast *NWSForecast) {
	if forecast == nil {
		return
	}

	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("NWS OFFICIAL FORECAST (api.weather.gov)")
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println()

	for i, period := range forecast.Properties.Periods {
		if i >= 8 {
			break
		}
		marker := "  "
		if strings.Contains(period.Name, "Saturday") && !strings.Contains(period.Name, "Night") {
			marker = "→ "
		}
		fmt.Printf("%s%-20s %3d°F  %s\n",
			marker, period.Name, period.Temperature, period.ShortForecast)
	}
	fmt.Println()
}

func getSaturdayForecast(forecast *NWSForecast) int {
	if forecast == nil {
		return 0
	}

	for _, period := range forecast.Properties.Periods {
		if strings.Contains(period.Name, "Saturday") && !strings.Contains(period.Name, "Night") {
			return period.Temperature
		}
	}
	return 0
}

func calculateCalibration(observations []METARObservation, loc *time.Location) float64 {
	// Known CLI values vs our METAR readings
	// From our backtest:
	// Dec 25: METAR 66°F, CLI 67°F → +1
	// Dec 24: METAR 64°F, CLI 64°F → 0
	// Dec 23: METAR 63°F, CLI 64°F → +1

	// Average calibration is about +1°F
	// But let's calculate from actual data

	sort.Slice(observations, func(i, j int) bool {
		return observations[i].ObsTime < observations[j].ObsTime
	})

	// Group by date and find max
	dayMaxes := make(map[string]float64)
	for _, obs := range observations {
		t := time.Unix(obs.ObsTime, 0).In(loc)
		dateKey := t.Format("2006-01-02")
		if obs.Temp > dayMaxes[dateKey] {
			dayMaxes[dateKey] = obs.Temp
		}
	}

	// Known CLI values (from our earlier scraping)
	cliValues := map[string]int{
		"2025-12-25": 67,
		"2025-12-24": 64,
		"2025-12-23": 64,
	}

	var totalDiff float64
	var count int
	for date, cli := range cliValues {
		if metarMax, ok := dayMaxes[date]; ok {
			metarMaxF := int((metarMax * 9.0 / 5.0) + 32.5)
			diff := float64(cli - metarMaxF)
			totalDiff += diff
			count++
		}
	}

	if count > 0 {
		return totalDiff / float64(count)
	}
	return 1.0 // Default calibration
}

func normalCDF(x, mean, stdDev float64) float64 {
	return 0.5 * (1 + math.Erf((x-mean)/(stdDev*math.Sqrt2)))
}

func celsiusToFahrenheit(c float64) int {
	return int((c * 9.0 / 5.0) + 32.5)
}
