package weather

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// METARObservation represents a single METAR temperature observation
type METARObservation struct {
	Time time.Time
	Temp float64 // Temperature in Fahrenheit
}

// METARData holds METAR data for a station/date
type METARData struct {
	Station      *Station
	Date         time.Time
	Observations []METARObservation
	MaxTemp      float64 // Maximum temperature in Fahrenheit
	MaxTempTime  time.Time
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

// FetchMETARMax fetches the maximum METAR temperature for a station on a given date
func FetchMETARMax(station *Station, date time.Time) (*METARData, error) {
	url := station.METARHistoryURL(date)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch METAR: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read METAR response: %w", err)
	}

	return parseMETARData(station, date, string(body))
}

func parseMETARData(station *Station, date time.Time, data string) (*METARData, error) {
	result := &METARData{
		Station: station,
		Date:    date,
	}

	// Remove 'K' prefix for matching
	stationCode := station.ID
	if len(stationCode) > 1 && stationCode[0] == 'K' {
		stationCode = stationCode[1:]
	}

	lines := strings.Split(data, "\n")
	maxTemp := -999.0
	var maxTime time.Time

	loc := station.Location()

	for _, line := range lines {
		if !strings.HasPrefix(line, stationCode+",") {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}

		// Parse timestamp (format: 2025-12-26 14:53)
		timeStr := parts[1]
		t, err := time.ParseInLocation("2006-01-02 15:04", timeStr, loc)
		if err != nil {
			continue
		}

		// Parse temperature
		var temp float64
		if _, err := fmt.Sscanf(parts[2], "%f", &temp); err != nil {
			continue
		}

		obs := METARObservation{
			Time: t,
			Temp: temp,
		}
		result.Observations = append(result.Observations, obs)

		if temp > maxTemp {
			maxTemp = temp
			maxTime = t
		}
	}

	if maxTemp == -999.0 {
		return nil, fmt.Errorf("no METAR data found for %s on %s", station.ID, date.Format("2006-01-02"))
	}

	result.MaxTemp = math.Round(maxTemp)
	result.MaxTempTime = maxTime

	return result, nil
}

// FetchCurrentMETAR fetches the current METAR observation for a station
func FetchCurrentMETAR(station *Station) (*METARObservation, error) {
	url := "https://aviationweather.gov/api/data/metar?ids=" + station.ID + "&format=json"

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current METAR: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read METAR response: %w", err)
	}

	// Parse JSON manually (simplified)
	data := string(body)

	// Extract temperature (look for "temp": value)
	tempIdx := strings.Index(data, `"temp":`)
	if tempIdx == -1 {
		return nil, fmt.Errorf("no temperature in METAR response")
	}

	var tempC float64
	if _, err := fmt.Sscanf(data[tempIdx+7:], "%f", &tempC); err != nil {
		return nil, fmt.Errorf("failed to parse temperature: %w", err)
	}

	// Convert Celsius to Fahrenheit
	tempF := tempC*9/5 + 32

	return &METARObservation{
		Time: time.Now().In(station.Location()),
		Temp: math.Round(tempF),
	}, nil
}

