package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Forecast represents a weather forecast for a specific period
type Forecast struct {
	Station     *Station
	Date        time.Time
	HighTemp    float64 // Forecasted high temperature in Fahrenheit
	LowTemp     float64 // Forecasted low temperature in Fahrenheit
	Description string  // Short forecast description
	IsDaytime   bool
}

// NWSForecastResponse represents the NWS API forecast response
type NWSForecastResponse struct {
	Properties struct {
		Periods []struct {
			Number      int    `json:"number"`
			Name        string `json:"name"`
			IsDaytime   bool   `json:"isDaytime"`
			Temperature int    `json:"temperature"`
			ShortForecast string `json:"shortForecast"`
		} `json:"periods"`
	} `json:"properties"`
}

// FetchNWSForecast fetches the NWS forecast for a station
func FetchNWSForecast(station *Station) ([]Forecast, error) {
	url := station.NWSForecastURL()

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NWS forecast: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read NWS response: %w", err)
	}

	var nwsResp NWSForecastResponse
	if err := json.Unmarshal(body, &nwsResp); err != nil {
		return nil, fmt.Errorf("failed to parse NWS response: %w", err)
	}

	var forecasts []Forecast
	loc := station.Location()

	for _, period := range nwsResp.Properties.Periods {
		f := Forecast{
			Station:     station,
			Date:        time.Now().In(loc),
			Description: period.ShortForecast,
			IsDaytime:   period.IsDaytime,
		}

		if period.IsDaytime {
			f.HighTemp = float64(period.Temperature)
		} else {
			f.LowTemp = float64(period.Temperature)
		}

		forecasts = append(forecasts, f)
	}

	return forecasts, nil
}

// FetchTomorrowHigh fetches the forecasted high temperature for tomorrow
func FetchTomorrowHigh(station *Station) (float64, error) {
	forecasts, err := FetchNWSForecast(station)
	if err != nil {
		return 0, err
	}

	// Look for the first daytime forecast (tomorrow's high)
	for _, f := range forecasts {
		if f.IsDaytime && f.HighTemp > 0 {
			return f.HighTemp, nil
		}
	}

	// Fallback to climatology
	return station.GetClimatologyHigh(time.Now().Month()), nil
}

// FetchForecastForDate fetches the forecast for a specific date
func FetchForecastForDate(station *Station, targetDate time.Time) (*Forecast, error) {
	forecasts, err := FetchNWSForecast(station)
	if err != nil {
		return nil, err
	}

	loc := station.Location()
	today := time.Now().In(loc)
	targetDay := targetDate.In(loc)

	// Determine which forecast period to use based on date offset
	dayOffset := int(targetDay.Sub(today).Hours() / 24)
	
	// Each day typically has 2 periods (day + night)
	periodIndex := dayOffset * 2
	if !today.Before(time.Date(today.Year(), today.Month(), today.Day(), 12, 0, 0, 0, loc)) {
		// If it's afternoon, skip today's daytime period
		periodIndex++
	}

	// Find the daytime period for the target date
	for i := periodIndex; i < len(forecasts); i++ {
		if forecasts[i].IsDaytime {
			return &forecasts[i], nil
		}
	}

	return nil, fmt.Errorf("no forecast found for %s", targetDate.Format("2006-01-02"))
}


