// Package market provides Kalshi market abstractions for temperature trading
package market

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brendanplayford/kalshi-go/pkg/rest"
	"github.com/brendanplayford/kalshi-go/pkg/weather"
)

// TempMarket represents a temperature market (high or low) for a specific date
type TempMarket struct {
	Station    *weather.Station
	MarketType weather.MarketType
	Date       time.Time
	EventTicker string
	Brackets   []Bracket
	
	// Market state
	IsOpen     bool
	ClosesAt   time.Time
}

// Bracket represents a single temperature bracket in a market
type Bracket struct {
	Ticker      string
	LowerBound  float64 // Lower temperature bound (inclusive), -999 for "less than"
	UpperBound  float64 // Upper temperature bound (inclusive), 999 for "greater than"
	YesPrice    int     // Current yes price in cents
	NoPrice     int     // Current no price in cents
	Volume      int     // Trading volume
	Description string  // Human-readable description (e.g., "60-61°F")
}

// FetchTempMarket fetches market data for a station, market type, and date
func FetchTempMarket(client *rest.Client, station *weather.Station, marketType weather.MarketType, date time.Time) (*TempMarket, error) {
	eventTicker := station.EventTickerForType(date, marketType)
	
	// Fetch all markets for this event
	markets, err := client.GetMarkets(eventTicker)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch markets for %s: %w", eventTicker, err)
	}

	if len(markets) == 0 {
		return nil, fmt.Errorf("no markets found for %s", eventTicker)
	}

	tm := &TempMarket{
		Station:     station,
		MarketType:  marketType,
		Date:        date,
		EventTicker: eventTicker,
		IsOpen:      markets[0].Status == "active",
	}

	// Parse brackets from markets
	for _, m := range markets {
		bracket := parseBracket(m)
		if bracket != nil {
			tm.Brackets = append(tm.Brackets, *bracket)
		}
	}

	// Sort brackets by lower bound
	sort.Slice(tm.Brackets, func(i, j int) bool {
		return tm.Brackets[i].LowerBound < tm.Brackets[j].LowerBound
	})

	return tm, nil
}

// parseBracket parses a Kalshi market into a Bracket
func parseBracket(m rest.Market) *Bracket {
	ticker := m.Ticker
	
	b := &Bracket{
		Ticker:      ticker,
		YesPrice:    int(m.YesBid * 100),
		NoPrice:     int(m.NoBid * 100),
		Volume:      m.Volume,
		Description: m.Title,
	}

	// Parse temperature bounds from ticker
	// Format: KXHIGHLAX-25DEC27-B60.5 (bracket 60-61)
	// Format: KXHIGHLAX-25DEC27-T63 (threshold >63)
	// Format: KXHIGHLAX-25DEC27-T56 (threshold <56)
	
	parts := strings.Split(ticker, "-")
	if len(parts) < 3 {
		return nil
	}

	spec := parts[len(parts)-1]
	
	if strings.HasPrefix(spec, "B") {
		// Bracket format: B60.5 means 60-61
		var mid float64
		if _, err := fmt.Sscanf(spec, "B%f", &mid); err == nil {
			b.LowerBound = mid - 0.5
			b.UpperBound = mid + 0.5
			b.Description = fmt.Sprintf("%.0f-%.0f°F", b.LowerBound, b.UpperBound)
		}
	} else if strings.HasPrefix(spec, "T") {
		// Threshold format: T63 could be >63 or <56
		var threshold float64
		if _, err := fmt.Sscanf(spec, "T%f", &threshold); err == nil {
			// Determine if it's greater than or less than based on title
			title := strings.ToLower(m.Title)
			if strings.Contains(title, ">") || strings.Contains(title, "above") || strings.Contains(title, "over") {
				b.LowerBound = threshold + 1
				b.UpperBound = 999
				b.Description = fmt.Sprintf(">%.0f°F", threshold)
			} else {
				b.LowerBound = -999
				b.UpperBound = threshold - 1
				b.Description = fmt.Sprintf("<%.0f°F", threshold)
			}
		}
	}

	return b
}

// GetFavorite returns the bracket with the highest yes price
func (tm *TempMarket) GetFavorite() *Bracket {
	if len(tm.Brackets) == 0 {
		return nil
	}

	favorite := &tm.Brackets[0]
	for i := range tm.Brackets {
		if tm.Brackets[i].YesPrice > favorite.YesPrice {
			favorite = &tm.Brackets[i]
		}
	}
	return favorite
}

// Get2ndBest returns the bracket with the second highest yes price
func (tm *TempMarket) Get2ndBest() *Bracket {
	if len(tm.Brackets) < 2 {
		return nil
	}

	// Sort by price descending
	sorted := make([]Bracket, len(tm.Brackets))
	copy(sorted, tm.Brackets)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].YesPrice > sorted[j].YesPrice
	})

	return &sorted[1]
}

// GetBracketForTemp returns the bracket that would win for a given temperature
func (tm *TempMarket) GetBracketForTemp(temp float64) *Bracket {
	for i := range tm.Brackets {
		b := &tm.Brackets[i]
		if temp >= b.LowerBound && temp <= b.UpperBound {
			return b
		}
	}
	return nil
}

// GetBracketByTicker returns a bracket by its ticker
func (tm *TempMarket) GetBracketByTicker(ticker string) *Bracket {
	for i := range tm.Brackets {
		if tm.Brackets[i].Ticker == ticker {
			return &tm.Brackets[i]
		}
	}
	return nil
}

// TotalVolume returns the total trading volume across all brackets
func (tm *TempMarket) TotalVolume() int {
	total := 0
	for _, b := range tm.Brackets {
		total += b.Volume
	}
	return total
}

// Uncertainty returns a measure of market uncertainty (lower = more certain)
// Calculated as the price spread between top brackets
func (tm *TempMarket) Uncertainty() float64 {
	fav := tm.GetFavorite()
	second := tm.Get2ndBest()
	if fav == nil || second == nil {
		return 100
	}
	return float64(fav.YesPrice - second.YesPrice)
}

