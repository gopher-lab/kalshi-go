// Package strategy provides trading strategy abstractions
package strategy

import (
	"fmt"
	"time"

	"github.com/brendanplayford/kalshi-go/pkg/market"
	"github.com/brendanplayford/kalshi-go/pkg/weather"
)

// Signal represents a trading signal for a specific bracket
type Signal struct {
	Name        string  // Signal source name (e.g., "MarketFavorite", "NWSForecast")
	Bracket     string  // Recommended bracket description (e.g., "60-61°F")
	Ticker      string  // Bracket ticker
	Temperature float64 // Predicted temperature (if applicable)
	Confidence  float64 // Confidence level 0-1
}

// SignalSource is the interface for signal generators
type SignalSource interface {
	Name() string
	Generate(station *weather.Station, marketType weather.MarketType, date time.Time, tempMarket *market.TempMarket) (*Signal, error)
}

// MarketFavoriteSignal generates signals based on the market favorite bracket
type MarketFavoriteSignal struct{}

func (s *MarketFavoriteSignal) Name() string { return "MarketFavorite" }

func (s *MarketFavoriteSignal) Generate(station *weather.Station, marketType weather.MarketType, date time.Time, tm *market.TempMarket) (*Signal, error) {
	fav := tm.GetFavorite()
	if fav == nil {
		return nil, fmt.Errorf("no favorite bracket found")
	}

	return &Signal{
		Name:        s.Name(),
		Bracket:     fav.Description,
		Ticker:      fav.Ticker,
		Temperature: (fav.LowerBound + fav.UpperBound) / 2,
		Confidence:  float64(fav.YesPrice) / 100,
	}, nil
}

// SecondBestSignal generates signals based on the 2nd highest priced bracket
type SecondBestSignal struct{}

func (s *SecondBestSignal) Name() string { return "2ndBest" }

func (s *SecondBestSignal) Generate(station *weather.Station, marketType weather.MarketType, date time.Time, tm *market.TempMarket) (*Signal, error) {
	second := tm.Get2ndBest()
	if second == nil {
		return nil, fmt.Errorf("no 2nd best bracket found")
	}

	return &Signal{
		Name:        s.Name(),
		Bracket:     second.Description,
		Ticker:      second.Ticker,
		Temperature: (second.LowerBound + second.UpperBound) / 2,
		Confidence:  float64(second.YesPrice) / 100,
	}, nil
}

// NWSForecastSignal generates signals based on NWS forecast
type NWSForecastSignal struct{}

func (s *NWSForecastSignal) Name() string { return "NWSForecast" }

func (s *NWSForecastSignal) Generate(station *weather.Station, marketType weather.MarketType, date time.Time, tm *market.TempMarket) (*Signal, error) {
	var temp float64
	var err error

	if marketType == weather.MarketTypeHigh {
		temp, err = weather.FetchTomorrowHigh(station)
	} else {
		// For low temp, fetch tomorrow's low (simplified - using climatology for now)
		temp = station.GetClimatologyLow(date.Month())
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch forecast: %w", err)
	}

	bracket := tm.GetBracketForTemp(temp)
	if bracket == nil {
		return nil, fmt.Errorf("no bracket found for forecast temp %.0f°F", temp)
	}

	return &Signal{
		Name:        s.Name(),
		Bracket:     bracket.Description,
		Ticker:      bracket.Ticker,
		Temperature: temp,
		Confidence:  0.7, // NWS forecast has moderate confidence
	}, nil
}

// ClimatologySignal generates signals based on historical averages
type ClimatologySignal struct{}

func (s *ClimatologySignal) Name() string { return "Climatology" }

func (s *ClimatologySignal) Generate(station *weather.Station, marketType weather.MarketType, date time.Time, tm *market.TempMarket) (*Signal, error) {
	var temp float64

	if marketType == weather.MarketTypeHigh {
		temp = station.GetClimatologyHigh(date.Month())
	} else {
		temp = station.GetClimatologyLow(date.Month())
	}

	bracket := tm.GetBracketForTemp(temp)
	if bracket == nil {
		return nil, fmt.Errorf("no bracket found for climatology temp %.0f°F", temp)
	}

	return &Signal{
		Name:        s.Name(),
		Bracket:     bracket.Description,
		Ticker:      bracket.Ticker,
		Temperature: temp,
		Confidence:  0.3, // Climatology is a weak signal
	}, nil
}

// METARCurrentSignal generates signals based on current METAR observations
type METARCurrentSignal struct{}

func (s *METARCurrentSignal) Name() string { return "METARCurrent" }

func (s *METARCurrentSignal) Generate(station *weather.Station, marketType weather.MarketType, date time.Time, tm *market.TempMarket) (*Signal, error) {
	obs, err := weather.FetchCurrentMETAR(station)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current METAR: %w", err)
	}

	// For high temp markets, the current temp is a lower bound
	// For low temp markets, the current temp is an upper bound
	temp := obs.Temp

	bracket := tm.GetBracketForTemp(temp)
	if bracket == nil {
		// If current temp doesn't match a bracket, find the closest
		if marketType == weather.MarketTypeHigh {
			// Current is a lower bound - find first bracket that could still win
			for i := range tm.Brackets {
				if tm.Brackets[i].UpperBound >= temp {
					bracket = &tm.Brackets[i]
					break
				}
			}
		} else {
			// Current is an upper bound - find last bracket that could still win
			for i := len(tm.Brackets) - 1; i >= 0; i-- {
				if tm.Brackets[i].LowerBound <= temp {
					bracket = &tm.Brackets[i]
					break
				}
			}
		}
	}

	if bracket == nil {
		return nil, fmt.Errorf("no bracket found for METAR temp %.0f°F", temp)
	}

	return &Signal{
		Name:        s.Name(),
		Bracket:     bracket.Description,
		Ticker:      bracket.Ticker,
		Temperature: temp,
		Confidence:  0.5,
	}, nil
}

// AllSignalSources returns all available signal sources
func AllSignalSources() []SignalSource {
	return []SignalSource{
		&MarketFavoriteSignal{},
		&SecondBestSignal{},
		&NWSForecastSignal{},
		&ClimatologySignal{},
		&METARCurrentSignal{},
	}
}

// DefaultSignalSources returns the 3-signal ensemble sources
func DefaultSignalSources() []SignalSource {
	return []SignalSource{
		&MarketFavoriteSignal{},
		&SecondBestSignal{},
		&NWSForecastSignal{},
	}
}

