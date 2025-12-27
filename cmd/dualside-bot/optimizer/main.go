// Package main optimizes dual-side strategy parameters through backtesting
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Market struct {
	Ticker      string `json:"ticker"`
	FloorStrike int    `json:"floor_strike"`
	CapStrike   int    `json:"cap_strike"`
	Result      string `json:"result"`
	Status      string `json:"status"`
}

type MarketsResponse struct {
	Markets []Market `json:"markets"`
}

type Trade struct {
	CreatedTime time.Time `json:"created_time"`
	YesPrice    int       `json:"yes_price"`
}

type TradesResponse struct {
	Trades []Trade `json:"trades"`
}

type Station struct {
	Code        string
	City        string
	METAR       string
	EventPrefix string
	Timezone    string
}

var Stations = []Station{
	{"LAX", "Los Angeles", "LAX", "KXHIGHLAX", "America/Los_Angeles"},
	{"NYC", "New York", "JFK", "KXHIGHNY", "America/New_York"},
	{"CHI", "Chicago", "ORD", "KXHIGHCHI", "America/Chicago"},
	{"MIA", "Miami", "MIA", "KXHIGHMIA", "America/New_York"},
	{"AUS", "Austin", "AUS", "KXHIGHAUS", "America/Chicago"},
	{"PHIL", "Philadelphia", "PHL", "KXHIGHPHIL", "America/New_York"},
	{"DEN", "Denver", "DEN", "KXHIGHDEN", "America/Denver"},
}

type DayData struct {
	Date           time.Time
	City           string
	WinningBracket string
	METARMax       int
	METARBracket   string
	BracketPrices  map[string]struct{ Yes, No int }
	FavBracket     string
	FavPrice       int
}

type Parameters struct {
	BetYes      float64
	BetNo       float64
	MinYesPrice int
	MaxYesPrice int
	MinNoPrice  int
	MaxNoPrice  int
	MaxNoTrades int
}

type Result struct {
	Params      Parameters
	Trades      int
	Wins        int
	WinRate     float64
	TotalProfit float64
	AvgProfit   float64
	Sharpe      float64
	MaxDrawdown float64
	YesProfit   float64
	NoProfit    float64
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           DUAL-SIDE STRATEGY PARAMETER OPTIMIZER                            ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Collect historical data first
	fmt.Println("📊 Collecting historical data (21 days, 7 cities)...")
	data := collectData(21)
	fmt.Printf("   Collected %d tradable days\n\n", len(data))

	if len(data) == 0 {
		fmt.Println("No data collected!")
		return
	}

	// Parameter grid to test
	betYesSizes := []float64{100, 200, 300, 400, 500}
	betNoSizes := []float64{50, 75, 100, 150}
	minYesPrices := []int{20, 30, 40, 50}
	maxYesPrices := []int{85, 90, 95}
	minNoPrices := []int{40, 50, 60, 70}
	maxNoPrices := []int{85, 90, 95}
	maxNoTradesCounts := []int{1, 2, 3, 4}

	var results []Result
	totalTests := len(betYesSizes) * len(betNoSizes) * len(minYesPrices) * len(maxYesPrices) * len(minNoPrices) * len(maxNoPrices) * len(maxNoTradesCounts)

	fmt.Printf("🔬 Testing %d parameter combinations...\n\n", totalTests)

	tested := 0
	for _, betYes := range betYesSizes {
		for _, betNo := range betNoSizes {
			for _, minYes := range minYesPrices {
				for _, maxYes := range maxYesPrices {
					if minYes >= maxYes {
						continue
					}
					for _, minNo := range minNoPrices {
						for _, maxNo := range maxNoPrices {
							if minNo >= maxNo {
								continue
							}
							for _, maxNoTrades := range maxNoTradesCounts {
								params := Parameters{
									BetYes:      betYes,
									BetNo:       betNo,
									MinYesPrice: minYes,
									MaxYesPrice: maxYes,
									MinNoPrice:  minNo,
									MaxNoPrice:  maxNo,
									MaxNoTrades: maxNoTrades,
								}

								result := backtest(data, params)
								if result.Trades > 0 {
									results = append(results, result)
								}
								tested++
							}
						}
					}
				}
			}
		}
		fmt.Printf("   Progress: %d/%d...\n", tested, totalTests)
	}

	// Sort by profit
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalProfit > results[j].TotalProfit
	})

	// Print top results
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  TOP 10 PARAMETER COMBINATIONS (by Total Profit)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("  ┌─────┬────────┬────────┬─────────┬─────────┬─────────┬─────────┬───────┬─────────┬────────┬─────────┐")
	fmt.Println("  │ Rank│ BetYes │ BetNo  │ YesMin  │ YesMax  │ NoMin   │ NoMax   │ MaxNo │ Trades  │ WinRate│ Profit  │")
	fmt.Println("  ├─────┼────────┼────────┼─────────┼─────────┼─────────┼─────────┼───────┼─────────┼────────┼─────────┤")

	for i, r := range results {
		if i >= 10 {
			break
		}
		fmt.Printf("  │ %3d │ $%4.0f  │ $%4.0f  │  %2d¢    │  %2d¢    │  %2d¢    │  %2d¢    │  %d    │  %3d    │ %5.1f%% │ $%6.0f  │\n",
			i+1, r.Params.BetYes, r.Params.BetNo,
			r.Params.MinYesPrice, r.Params.MaxYesPrice,
			r.Params.MinNoPrice, r.Params.MaxNoPrice,
			r.Params.MaxNoTrades, r.Trades, r.WinRate, r.TotalProfit)
	}
	fmt.Println("  └─────┴────────┴────────┴─────────┴─────────┴─────────┴─────────┴───────┴─────────┴────────┴─────────┘")

	// Sort by Sharpe ratio
	sort.Slice(results, func(i, j int) bool {
		return results[i].Sharpe > results[j].Sharpe
	})

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  TOP 10 PARAMETER COMBINATIONS (by Sharpe Ratio)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("  ┌─────┬────────┬────────┬─────────┬─────────┬─────────┬─────────┬───────┬─────────┬────────┬─────────┐")
	fmt.Println("  │ Rank│ BetYes │ BetNo  │ YesMin  │ YesMax  │ NoMin   │ NoMax   │ MaxNo │ Trades  │ Sharpe │ Profit  │")
	fmt.Println("  ├─────┼────────┼────────┼─────────┼─────────┼─────────┼─────────┼───────┼─────────┼────────┼─────────┤")

	for i, r := range results {
		if i >= 10 {
			break
		}
		fmt.Printf("  │ %3d │ $%4.0f  │ $%4.0f  │  %2d¢    │  %2d¢    │  %2d¢    │  %2d¢    │  %d    │  %3d    │ %5.2f  │ $%6.0f  │\n",
			i+1, r.Params.BetYes, r.Params.BetNo,
			r.Params.MinYesPrice, r.Params.MaxYesPrice,
			r.Params.MinNoPrice, r.Params.MaxNoPrice,
			r.Params.MaxNoTrades, r.Trades, r.Sharpe, r.TotalProfit)
	}
	fmt.Println("  └─────┴────────┴────────┴─────────┴─────────┴─────────┴─────────┴───────┴─────────┴────────┴─────────┘")

	// Best balanced result
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  RECOMMENDED PARAMETERS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")

	// Find best with good trade count and profit
	var best Result
	for _, r := range results {
		if r.Trades >= 30 && r.WinRate >= 60 && r.TotalProfit > best.TotalProfit {
			best = r
		}
	}

	if best.Trades > 0 {
		fmt.Println()
		fmt.Printf("  BetYes:      $%.0f\n", best.Params.BetYes)
		fmt.Printf("  BetNo:       $%.0f\n", best.Params.BetNo)
		fmt.Printf("  YES Range:   %d¢ - %d¢\n", best.Params.MinYesPrice, best.Params.MaxYesPrice)
		fmt.Printf("  NO Range:    %d¢ - %d¢\n", best.Params.MinNoPrice, best.Params.MaxNoPrice)
		fmt.Printf("  Max NO:      %d trades per event\n", best.Params.MaxNoTrades)
		fmt.Println()
		fmt.Printf("  📊 Results over %d days:\n", len(data))
		fmt.Printf("     Trades:    %d\n", best.Trades)
		fmt.Printf("     Win Rate:  %.1f%%\n", best.WinRate)
		fmt.Printf("     Profit:    $%.2f\n", best.TotalProfit)
		fmt.Printf("     Sharpe:    %.2f\n", best.Sharpe)
		fmt.Printf("     YES P/L:   $%.2f\n", best.YesProfit)
		fmt.Printf("     NO P/L:    $%.2f\n", best.NoProfit)

		// Annual projection
		annual := best.TotalProfit / 21.0 * 365.0
		fmt.Println()
		fmt.Printf("  💰 Annual Projection: $%.0f\n", annual)
	}

	fmt.Println()
}

func collectData(days int) []DayData {
	var data []DayData

	for _, station := range Stations {
		loc, _ := time.LoadLocation(station.Timezone)
		today := time.Now().In(loc)

		for i := 1; i <= days; i++ {
			date := today.AddDate(0, 0, -i)
			dayData := fetchDayData(station, date)
			if dayData != nil && dayData.FavPrice > 0 {
				data = append(data, *dayData)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	return data
}

func fetchDayData(station Station, date time.Time) *DayData {
	loc, _ := time.LoadLocation(station.Timezone)
	dateCode := strings.ToUpper(date.In(loc).Format("06Jan02"))
	eventTicker := fmt.Sprintf("%s-%s", station.EventPrefix, dateCode)

	// Fetch markets
	markets, err := fetchMarkets(eventTicker)
	if err != nil || len(markets) == 0 {
		return nil
	}

	// Find winner
	var winningBracket string
	for _, m := range markets {
		if m.Result == "yes" {
			winningBracket = formatBracket(&m)
			break
		}
	}
	if winningBracket == "" {
		return nil
	}

	// Get METAR
	metarMax, err := getMETARMax(station, date)
	if err != nil {
		return nil
	}

	// Find METAR bracket
	var metarBracket string
	for _, m := range markets {
		if m.FloorStrike <= metarMax && m.CapStrike >= metarMax {
			metarBracket = formatBracket(&m)
			break
		}
	}

	// Get first trade prices
	bracketPrices := make(map[string]struct{ Yes, No int })
	for _, m := range markets {
		yesPrice, noPrice := getFirstTradePrices(m.Ticker)
		if yesPrice > 0 {
			bracketPrices[formatBracket(&m)] = struct{ Yes, No int }{yesPrice, noPrice}
		}
	}

	// Find favorite
	var favBracket string
	var favPrice int
	for bracket, prices := range bracketPrices {
		if prices.Yes > favPrice {
			favPrice = prices.Yes
			favBracket = bracket
		}
	}

	return &DayData{
		Date:           date,
		City:           station.City,
		WinningBracket: winningBracket,
		METARMax:       metarMax,
		METARBracket:   metarBracket,
		BracketPrices:  bracketPrices,
		FavBracket:     favBracket,
		FavPrice:       favPrice,
	}
}

func backtest(data []DayData, params Parameters) Result {
	result := Result{Params: params}
	var profits []float64

	for _, day := range data {
		// Check signal agreement
		if day.FavBracket != day.METARBracket {
			continue
		}

		// Check YES price range
		if day.FavPrice < params.MinYesPrice || day.FavPrice > params.MaxYesPrice {
			continue
		}

		result.Trades++
		dayProfit := 0.0

		// YES trade
		yesContracts := params.BetYes / float64(day.FavPrice) * 100
		if day.WinningBracket == day.FavBracket {
			result.Wins++
			yesProfit := yesContracts - params.BetYes
			result.YesProfit += yesProfit
			dayProfit += yesProfit
		} else {
			result.YesProfit -= params.BetYes
			dayProfit -= params.BetYes
		}

		// NO trades
		noCount := 0
		for bracket, prices := range day.BracketPrices {
			if bracket == day.FavBracket {
				continue
			}
			if noCount >= params.MaxNoTrades {
				break
			}
			if prices.No < params.MinNoPrice || prices.No > params.MaxNoPrice {
				continue
			}

			noContracts := params.BetNo / float64(prices.No) * 100
			if day.WinningBracket != bracket {
				noProfit := noContracts - params.BetNo
				result.NoProfit += noProfit
				dayProfit += noProfit
			} else {
				result.NoProfit -= params.BetNo
				dayProfit -= params.BetNo
			}
			noCount++
		}

		profits = append(profits, dayProfit)
		result.TotalProfit += dayProfit
	}

	if result.Trades > 0 {
		result.WinRate = float64(result.Wins) / float64(result.Trades) * 100
		result.AvgProfit = result.TotalProfit / float64(result.Trades)

		// Calculate Sharpe ratio
		if len(profits) > 1 {
			mean := result.AvgProfit
			variance := 0.0
			for _, p := range profits {
				variance += (p - mean) * (p - mean)
			}
			stdDev := math.Sqrt(variance / float64(len(profits)-1))
			if stdDev > 0 {
				result.Sharpe = mean / stdDev * math.Sqrt(252) // Annualized
			}
		}

		// Calculate max drawdown
		cumProfit := 0.0
		peak := 0.0
		for _, p := range profits {
			cumProfit += p
			if cumProfit > peak {
				peak = cumProfit
			}
			drawdown := peak - cumProfit
			if drawdown > result.MaxDrawdown {
				result.MaxDrawdown = drawdown
			}
		}
	}

	return result
}

func fetchMarkets(eventTicker string) ([]Market, error) {
	url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets?event_ticker=%s&limit=100", eventTicker)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result MarketsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var brackets []Market
	for _, m := range result.Markets {
		parts := strings.Split(m.Ticker, "-")
		if len(parts) >= 3 && strings.HasPrefix(parts[len(parts)-1], "B") {
			brackets = append(brackets, m)
		}
	}

	return brackets, nil
}

func getFirstTradePrices(ticker string) (yesPrice, noPrice int) {
	url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets/trades?ticker=%s&limit=100", ticker)

	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, 0
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result TradesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, 0
	}

	if len(result.Trades) == 0 {
		return 0, 0
	}

	earliest := result.Trades[0]
	for _, t := range result.Trades {
		if t.CreatedTime.Before(earliest.CreatedTime) {
			earliest = t
		}
	}

	yesPrice = earliest.YesPrice
	noPrice = 100 - yesPrice

	return yesPrice, noPrice
}

func getMETARMax(station Station, date time.Time) (int, error) {
	url := fmt.Sprintf(
		"https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py?station=%s&data=tmpf&year1=%d&month1=%d&day1=%d&year2=%d&month2=%d&day2=%d&tz=%s&format=onlycomma&latlon=no&elev=no&missing=M&trace=T&direct=no&report_type=3",
		station.METAR,
		date.Year(), int(date.Month()), date.Day(),
		date.Year(), int(date.Month()), date.Day()+1,
		station.Timezone,
	)

	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(string(body), "\n")
	maxTemp := -999.0

	for _, line := range lines {
		if strings.HasPrefix(line, station.METAR+",") {
			parts := strings.Split(line, ",")
			if len(parts) >= 3 {
				var temp float64
				fmt.Sscanf(parts[2], "%f", &temp)
				if temp > maxTemp {
					maxTemp = temp
				}
			}
		}
	}

	if maxTemp == -999.0 {
		return 0, fmt.Errorf("no data")
	}

	return int(math.Round(maxTemp)), nil
}

func formatBracket(m *Market) string {
	return fmt.Sprintf("%d-%d°", m.FloorStrike, m.CapStrike)
}
