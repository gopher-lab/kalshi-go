package strategy

import (
	"fmt"
	"time"

	"github.com/brendanplayford/kalshi-go/pkg/market"
	"github.com/brendanplayford/kalshi-go/pkg/weather"
)

// EnsembleConfig configures the ensemble strategy
type EnsembleConfig struct {
	SignalSources []SignalSource
	MinAgreement  int     // Minimum signals that must agree (default: all)
	MaxBuyPrice   int     // Maximum price to buy at (cents)
	MinBuyPrice   int     // Minimum price to buy at (cents)
	BetSize       float64 // Position size in dollars
}

// DefaultEnsembleConfig returns the default 3-signal ensemble configuration
func DefaultEnsembleConfig() *EnsembleConfig {
	return &EnsembleConfig{
		SignalSources: DefaultSignalSources(),
		MinAgreement:  3, // All 3 signals must agree
		MaxBuyPrice:   60,
		MinBuyPrice:   20,
		BetSize:       10.0,
	}
}

// EnsembleResult contains the result of running the ensemble strategy
type EnsembleResult struct {
	Station       *weather.Station
	MarketType    weather.MarketType
	Date          time.Time
	Signals       []*Signal
	Agreement     map[string]int // Bracket -> count of signals
	Recommendation *TradeRecommendation
}

// TradeRecommendation represents a trade recommendation
type TradeRecommendation struct {
	Action       string  // "BUY", "SELL", or "NO_TRADE"
	Bracket      string  // Bracket description
	Ticker       string  // Bracket ticker
	Price        int     // Price in cents
	Quantity     int     // Number of contracts
	Reason       string  // Explanation
	Confidence   float64 // Confidence level 0-1
	ExpectedEdge float64 // Expected edge percentage
}

// Ensemble represents the ensemble strategy engine
type Ensemble struct {
	Config *EnsembleConfig
}

// NewEnsemble creates a new ensemble strategy with default config
func NewEnsemble() *Ensemble {
	return &Ensemble{Config: DefaultEnsembleConfig()}
}

// NewEnsembleWithConfig creates a new ensemble strategy with custom config
func NewEnsembleWithConfig(config *EnsembleConfig) *Ensemble {
	return &Ensemble{Config: config}
}

// Analyze runs the ensemble analysis on a market
func (e *Ensemble) Analyze(station *weather.Station, marketType weather.MarketType, date time.Time, tm *market.TempMarket) (*EnsembleResult, error) {
	result := &EnsembleResult{
		Station:    station,
		MarketType: marketType,
		Date:       date,
		Agreement:  make(map[string]int),
	}

	// Generate signals from all sources
	for _, source := range e.Config.SignalSources {
		signal, err := source.Generate(station, marketType, date, tm)
		if err != nil {
			// Log but continue - some signals may fail
			continue
		}
		result.Signals = append(result.Signals, signal)
		result.Agreement[signal.Bracket]++
	}

	// Find the bracket with most agreement
	var bestBracket string
	var bestCount int
	for bracket, count := range result.Agreement {
		if count > bestCount {
			bestBracket = bracket
			bestCount = count
		}
	}

	// Check if we have enough agreement
	if bestCount < e.Config.MinAgreement {
		result.Recommendation = &TradeRecommendation{
			Action: "NO_TRADE",
			Reason: fmt.Sprintf("Only %d/%d signals agree on %s (need %d)",
				bestCount, len(e.Config.SignalSources), bestBracket, e.Config.MinAgreement),
		}
		return result, nil
	}

	// Find the bracket in the market
	var targetBracket *market.Bracket
	for i := range tm.Brackets {
		if tm.Brackets[i].Description == bestBracket {
			targetBracket = &tm.Brackets[i]
			break
		}
	}

	if targetBracket == nil {
		result.Recommendation = &TradeRecommendation{
			Action: "NO_TRADE",
			Reason: fmt.Sprintf("Bracket %s not found in market", bestBracket),
		}
		return result, nil
	}

	// Check price constraints
	if targetBracket.YesPrice > e.Config.MaxBuyPrice {
		result.Recommendation = &TradeRecommendation{
			Action: "NO_TRADE",
			Reason: fmt.Sprintf("Price %d¢ exceeds max buy price %d¢",
				targetBracket.YesPrice, e.Config.MaxBuyPrice),
			Bracket: bestBracket,
			Ticker:  targetBracket.Ticker,
			Price:   targetBracket.YesPrice,
		}
		return result, nil
	}

	if targetBracket.YesPrice < e.Config.MinBuyPrice {
		result.Recommendation = &TradeRecommendation{
			Action: "NO_TRADE",
			Reason: fmt.Sprintf("Price %d¢ below min buy price %d¢",
				targetBracket.YesPrice, e.Config.MinBuyPrice),
			Bracket: bestBracket,
			Ticker:  targetBracket.Ticker,
			Price:   targetBracket.YesPrice,
		}
		return result, nil
	}

	// Calculate expected edge
	// With N signals agreeing, our confidence is approximately N/total
	confidence := float64(bestCount) / float64(len(e.Config.SignalSources))
	expectedEdge := (confidence * 100) - float64(targetBracket.YesPrice)

	// Calculate quantity
	quantity := int(e.Config.BetSize * 100 / float64(targetBracket.YesPrice))
	if quantity < 1 {
		quantity = 1
	}

	result.Recommendation = &TradeRecommendation{
		Action:       "BUY",
		Bracket:      bestBracket,
		Ticker:       targetBracket.Ticker,
		Price:        targetBracket.YesPrice,
		Quantity:     quantity,
		Reason:       fmt.Sprintf("%d/%d signals agree on %s", bestCount, len(e.Config.SignalSources), bestBracket),
		Confidence:   confidence,
		ExpectedEdge: expectedEdge,
	}

	return result, nil
}

// AnalyzeAll runs the ensemble analysis on all active markets for a station
func (e *Ensemble) AnalyzeAll(station *weather.Station, date time.Time, tmHigh, tmLow *market.TempMarket) ([]*EnsembleResult, error) {
	var results []*EnsembleResult

	if tmHigh != nil {
		result, err := e.Analyze(station, weather.MarketTypeHigh, date, tmHigh)
		if err == nil {
			results = append(results, result)
		}
	}

	if tmLow != nil {
		result, err := e.Analyze(station, weather.MarketTypeLow, date, tmLow)
		if err == nil {
			results = append(results, result)
		}
	}

	return results, nil
}

