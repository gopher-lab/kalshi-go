// Package main provides a real-time trading monitor for the LA High Temperature market.
// Run this on market day to track the developing maximum and get trading signals.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/brendanplayford/kalshi-go/internal/config"
	"github.com/brendanplayford/kalshi-go/pkg/ws"
)

// METAR observation from Aviation Weather Center
type METARObservation struct {
	IcaoID     string  `json:"icaoId"`
	ObsTime    int64   `json:"obsTime"`
	ReportTime string  `json:"reportTime"`
	Temp       float64 `json:"temp"`
	Dewp       float64 `json:"dewp"`
	WxString   string  `json:"wxString"`
	RawOb      string  `json:"rawOb"`
}

// NWSForecast from api.weather.gov
type NWSForecast struct {
	Properties struct {
		Periods []struct {
			Name          string `json:"name"`
			Temperature   int    `json:"temperature"`
			ShortForecast string `json:"shortForecast"`
			IsDaytime     bool   `json:"isDaytime"`
		} `json:"periods"`
	} `json:"properties"`
}

// TradingState tracks the current trading state
type TradingState struct {
	// Weather data
	CurrentTempF      int
	RunningMaxF       int
	ExpectedMaxF      int
	NWSForecastF      int
	LastUpdate        time.Time
	WeatherConditions string

	// Market state
	Strikes map[string]*StrikeState

	// Signals
	Alerts []string
}

// StrikeState tracks state for each strike
type StrikeState struct {
	Strike      string
	LowBound    int
	HighBound   int
	Crossed     bool
	CrossedAt   time.Time
	Probability float64
	MarketPrice float64
	Edge        float64
	Recommended string
}

const (
	metarAPIURL    = "https://aviationweather.gov/api/data/metar?ids=KLAX&hours=3&format=json"
	nwsForecastURL = "https://api.weather.gov/gridpoints/LOX/154,44/forecast"
	pollInterval   = 5 * time.Minute
	cliCalibration = 1.0 // METAR→CLI adjustment
)

var (
	strikes = []StrikeState{
		{Strike: "55 or below", LowBound: 0, HighBound: 55},
		{Strike: "56-57", LowBound: 56, HighBound: 57},
		{Strike: "58-59", LowBound: 58, HighBound: 59},
		{Strike: "60-61", LowBound: 60, HighBound: 61},
		{Strike: "62-63", LowBound: 62, HighBound: 63},
		{Strike: "64 or above", LowBound: 64, HighBound: 999},
	}
)

func main() {
	// Parse flags
	marketTicker := flag.String("market", "KXHIGHLAX-25DEC27", "Market ticker (e.g., KXHIGHLAX-25DEC27)")
	useWebSocket := flag.Bool("ws", false, "Connect to Kalshi WebSocket for live prices")
	flag.Parse()

	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("🌡️  LA HIGH TEMPERATURE - LIVE TRADING MONITOR")
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println()
	fmt.Printf("Market: %s\n", *marketTicker)
	fmt.Printf("Poll Interval: %v\n", pollInterval)
	fmt.Printf("CLI Calibration: +%.1f°F\n", cliCalibration)
	fmt.Println()

	// Initialize state
	state := &TradingState{
		Strikes: make(map[string]*StrikeState),
	}
	for i := range strikes {
		s := strikes[i] // Copy
		state.Strikes[s.Strike] = &s
	}

	// Initial data fetch
	updateWeatherData(state)
	printStatus(state)

	// Optional: Connect to Kalshi WebSocket
	var client *ws.Client
	if *useWebSocket {
		var err error
		client, err = connectKalshi(*marketTicker)
		if err != nil {
			fmt.Printf("⚠ Warning: Could not connect to Kalshi: %v\n", err)
			fmt.Println("  Continuing without live market data...")
		} else {
			defer client.Close()
		}
	}

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start polling loop
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	fmt.Println()
	fmt.Println("📡 Monitoring started. Press Ctrl+C to stop.")
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println()

	for {
		select {
		case <-ticker.C:
			prevMax := state.RunningMaxF
			updateWeatherData(state)

			// Check for new threshold crossings
			checkThresholds(state, prevMax)

			// Print update
			printUpdate(state)

		case <-sigCh:
			fmt.Println("\n→ Shutting down...")
			printFinalSummary(state)
			return
		}
	}
}

func updateWeatherData(state *TradingState) {
	loc, _ := time.LoadLocation("America/Los_Angeles")

	// Fetch latest METAR
	metar, err := fetchLatestMETAR()
	if err != nil {
		fmt.Printf("⚠ Error fetching METAR: %v\n", err)
		return
	}

	if metar != nil {
		tempF := celsiusToFahrenheit(metar.Temp)
		state.CurrentTempF = tempF
		state.LastUpdate = time.Unix(metar.ObsTime, 0).In(loc)
		state.WeatherConditions = metar.WxString

		// Update running max
		if tempF > state.RunningMaxF {
			state.RunningMaxF = tempF
		}
	}

	// Fetch NWS forecast (less frequently would be fine, but keeping simple)
	forecast, err := fetchNWSForecast()
	if err == nil && forecast != nil {
		for _, period := range forecast.Properties.Periods {
			if period.IsDaytime && strings.Contains(strings.ToLower(period.Name), "today") {
				state.NWSForecastF = period.Temperature
				break
			}
			// Fallback to first daytime period
			if period.IsDaytime && state.NWSForecastF == 0 {
				state.NWSForecastF = period.Temperature
			}
		}
	}

	// Calculate expected CLI max
	state.ExpectedMaxF = int(math.Max(float64(state.RunningMaxF), float64(state.NWSForecastF)) + cliCalibration)

	// Update strike probabilities
	updateProbabilities(state)
}

func updateProbabilities(state *TradingState) {
	expectedCLI := float64(state.ExpectedMaxF)
	stdDev := 2.0 // Typical forecast uncertainty

	// Adjust stdDev based on time of day
	loc, _ := time.LoadLocation("America/Los_Angeles")
	hour := time.Now().In(loc).Hour()

	if hour >= 16 { // After 4PM, less uncertainty
		stdDev = 1.5
	}
	if hour >= 18 { // After 6PM, even less
		stdDev = 1.0
	}
	if hour >= 20 { // After 8PM, pretty certain
		stdDev = 0.5
	}

	for _, s := range state.Strikes {
		var prob float64
		if s.HighBound == 999 {
			prob = 1 - normalCDF(float64(s.LowBound)-0.5, expectedCLI, stdDev)
		} else if s.LowBound == 0 {
			prob = normalCDF(float64(s.HighBound)+0.5, expectedCLI, stdDev)
		} else {
			prob = normalCDF(float64(s.HighBound)+0.5, expectedCLI, stdDev) -
				normalCDF(float64(s.LowBound)-0.5, expectedCLI, stdDev)
		}
		s.Probability = prob

		// Check if threshold crossed (for YES bets)
		cliMax := state.RunningMaxF + int(cliCalibration)
		if !s.Crossed && cliMax > s.LowBound {
			s.Crossed = true
			s.CrossedAt = time.Now()
		}
	}
}

func checkThresholds(state *TradingState, prevMax int) {
	cliMax := state.RunningMaxF + int(cliCalibration)
	prevCLI := prevMax + int(cliCalibration)

	for _, s := range state.Strikes {
		// Check if we just crossed a threshold
		if prevCLI <= s.LowBound && cliMax > s.LowBound {
			alert := fmt.Sprintf("🚨 THRESHOLD CROSSED: %d°F (CLI) > %s strike!", cliMax, s.Strike)
			state.Alerts = append(state.Alerts, alert)
			fmt.Println()
			fmt.Println(strings.Repeat("!", 78))
			fmt.Println(alert)
			fmt.Printf("   → Consider: BUY YES on \"%s\"\n", s.Strike)
			fmt.Println(strings.Repeat("!", 78))
			fmt.Println()
		}
	}
}

func printStatus(state *TradingState) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	now := time.Now().In(loc)

	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("INITIAL STATUS")
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println()
	fmt.Printf("📅 Date: %s\n", now.Format("Monday, January 2, 2006"))
	fmt.Printf("🕐 Time: %s\n", now.Format("3:04 PM MST"))
	fmt.Println()
	fmt.Printf("🌡️  Current Temp: %d°F\n", state.CurrentTempF)
	fmt.Printf("📈 Running Max: %d°F (METAR) → %d°F (Est. CLI)\n",
		state.RunningMaxF, state.RunningMaxF+int(cliCalibration))
	fmt.Printf("🌤️  NWS Forecast: %d°F\n", state.NWSForecastF)
	fmt.Printf("🎯 Expected CLI: %d°F\n", state.ExpectedMaxF)
	if state.WeatherConditions != "" {
		fmt.Printf("☁️  Conditions: %s\n", state.WeatherConditions)
	}
	fmt.Println()

	// Print strike analysis
	fmt.Println("STRIKE ANALYSIS:")
	fmt.Printf("%-15s %-12s %-12s %-15s\n", "Strike", "Probability", "Crossed?", "Signal")
	fmt.Printf("%-15s %-12s %-12s %-15s\n", "------", "-----------", "--------", "------")

	for _, s := range getSortedStrikes(state) {
		crossed := "No"
		if s.Crossed {
			crossed = fmt.Sprintf("Yes @ %s", s.CrossedAt.Format("3:04 PM"))
		}

		signal := ""
		if s.Probability > 0.5 {
			signal = "🟢 Likely"
		} else if s.Probability > 0.3 {
			signal = "🟡 Possible"
		} else {
			signal = "🔴 Unlikely"
		}

		fmt.Printf("%-15s %-12.0f%% %-12s %-15s\n",
			s.Strike, s.Probability*100, crossed, signal)
	}
	fmt.Println()
}

func printUpdate(state *TradingState) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	now := time.Now().In(loc)

	fmt.Printf("[%s] Temp: %d°F | Max: %d°F (CLI: %d°F) | Expected: %d°F",
		now.Format("15:04"),
		state.CurrentTempF,
		state.RunningMaxF,
		state.RunningMaxF+int(cliCalibration),
		state.ExpectedMaxF)

	// Find most likely bracket
	var maxProb float64
	var maxStrike string
	for _, s := range state.Strikes {
		if s.Probability > maxProb {
			maxProb = s.Probability
			maxStrike = s.Strike
		}
	}
	fmt.Printf(" | Best: %s (%.0f%%)\n", maxStrike, maxProb*100)
}

func printFinalSummary(state *TradingState) {
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("FINAL SUMMARY")
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println()
	fmt.Printf("🌡️  Final Running Max: %d°F (METAR)\n", state.RunningMaxF)
	fmt.Printf("📊 Estimated CLI: %d°F\n", state.RunningMaxF+int(cliCalibration))
	fmt.Println()

	fmt.Println("THRESHOLDS CROSSED:")
	for _, s := range getSortedStrikes(state) {
		if s.Crossed {
			fmt.Printf("  ✅ %s (at %s)\n", s.Strike, s.CrossedAt.Format("3:04 PM"))
		}
	}

	fmt.Println()
	fmt.Println("ALERTS GENERATED:")
	if len(state.Alerts) == 0 {
		fmt.Println("  (none)")
	}
	for _, alert := range state.Alerts {
		fmt.Printf("  %s\n", alert)
	}
	fmt.Println()
}

func getSortedStrikes(state *TradingState) []*StrikeState {
	result := make([]*StrikeState, 0, len(state.Strikes))
	for _, s := range state.Strikes {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LowBound < result[j].LowBound
	})
	return result
}

func connectKalshi(marketTicker string) (*ws.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if !cfg.IsAuthenticated() {
		return nil, fmt.Errorf("no Kalshi credentials configured")
	}

	opts := []ws.Option{
		ws.WithAPIKeyOption(cfg.APIKey, cfg.PrivateKey),
		ws.WithCallbacks(
			func() { fmt.Println("✓ Connected to Kalshi WebSocket") },
			func(err error) { fmt.Printf("✗ Kalshi disconnected: %v\n", err) },
			func(err error) { fmt.Printf("⚠ Kalshi error: %v\n", err) },
		),
	}

	if cfg.BaseURL != "" {
		opts = append(opts, ws.WithBaseURLOption(cfg.BaseURL))
	}

	client := ws.New(opts...)

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	// Subscribe to ticker for market prices
	_, err = client.Subscribe(ctx, marketTicker, ws.ChannelTicker)
	if err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}

func fetchLatestMETAR() (*METARObservation, error) {
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

	if len(observations) == 0 {
		return nil, fmt.Errorf("no observations returned")
	}

	// Return most recent
	return &observations[0], nil
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

func celsiusToFahrenheit(c float64) int {
	return int((c * 9.0 / 5.0) + 32.5)
}

func normalCDF(x, mean, stdDev float64) float64 {
	return 0.5 * (1 + math.Erf((x-mean)/(stdDev*math.Sqrt2)))
}
