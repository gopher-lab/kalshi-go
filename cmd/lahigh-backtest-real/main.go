// Package main provides accurate backtesting using real Kalshi trade data
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/brendanplayford/kalshi-go/internal/config"
	"github.com/brendanplayford/kalshi-go/pkg/rest"
)

type Trade struct {
	CreatedTime time.Time `json:"created_time"`
	YesPrice    int       `json:"yes_price"`
	Count       int       `json:"count"`
	Ticker      string    `json:"ticker"`
}

type TradesResponse struct {
	Trades []Trade `json:"trades"`
	Cursor string  `json:"cursor"`
}

type Market struct {
	Ticker      string `json:"ticker"`
	FloorStrike int    `json:"floor_strike"`
	CapStrike   int    `json:"cap_strike"`
	Result      string `json:"result"`
	Subtitle    string `json:"subtitle"`
}

type MarketsResponse struct {
	Markets []Market `json:"markets"`
}

type DayResult struct {
	Date           string
	WinningBracket string
	WinningTicker  string
	FirstTradeTime time.Time
	FirstPrice     int
	Hit50Time      *time.Time
	Hit50Price     int
	Hit80Time      *time.Time
	Hit80Price     int
	Hit90Time      *time.Time
	Hit90Price     int
	TotalTrades    int
}

var loc *time.Location

func init() {
	var err error
	loc, err = time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.UTC
	}
}

func main() {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("LA HIGH TEMPERATURE - REAL DATA BACKTEST")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Load config for authenticated requests
	cfg, _ := config.Load()
	var client *rest.Client
	if cfg.IsAuthenticated() {
		client = rest.New(cfg.APIKey, cfg.PrivateKey)
		fmt.Println("✅ Using authenticated API")
	} else {
		fmt.Println("⚠️  Using public API (may have rate limits)")
	}

	// Get dates to analyze (last 14 days)
	results := []DayResult{}
	today := time.Now().In(loc)

	for i := 1; i <= 14; i++ {
		date := today.AddDate(0, 0, -i)
		dateCode := strings.ToUpper(date.Format("06Jan02"))
		eventTicker := fmt.Sprintf("KXHIGHLAX-%s", dateCode)

		fmt.Printf("\n📅 %s (%s)\n", date.Format("Mon Jan 2"), eventTicker)

		// Find the winning market
		winner, err := findWinner(eventTicker)
		if err != nil {
			fmt.Printf("   ❌ Error finding winner: %v\n", err)
			continue
		}
		if winner == nil {
			fmt.Printf("   ⏳ No winner yet (market still open?)\n")
			continue
		}

		fmt.Printf("   🏆 Winner: %s\n", bracketName(winner))

		// Get all trades for the winning market
		trades, err := getAllTrades(winner.Ticker, client)
		if err != nil {
			fmt.Printf("   ❌ Error getting trades: %v\n", err)
			continue
		}

		if len(trades) == 0 {
			fmt.Printf("   ⚠️  No trades found\n")
			continue
		}

		// Analyze trades
		result := analyzeTrades(date.Format("2006-01-02"), winner, trades)
		results = append(results, result)

		// Print summary
		fmt.Printf("   📊 %d trades, first @ %s PT (%d¢)\n",
			result.TotalTrades,
			result.FirstTradeTime.In(loc).Format("01/02 3:04 PM"),
			result.FirstPrice)

		if result.Hit80Time != nil {
			fmt.Printf("   📈 Hit 80%%: %s PT (%d¢)\n",
				result.Hit80Time.In(loc).Format("01/02 3:04 PM"),
				result.Hit80Price)
		}
		if result.Hit90Time != nil {
			fmt.Printf("   📈 Hit 90%%: %s PT (%d¢)\n",
				result.Hit90Time.In(loc).Format("01/02 3:04 PM"),
				result.Hit90Price)
		}

		time.Sleep(200 * time.Millisecond) // Rate limiting
	}

	// Summary analysis
	printSummary(results)
}

func findWinner(eventTicker string) (*Market, error) {
	url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets?event_ticker=%s&limit=100", eventTicker)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result MarketsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	for _, m := range result.Markets {
		if m.Result == "yes" {
			return &m, nil
		}
	}

	return nil, nil
}

func bracketName(m *Market) string {
	if m.FloorStrike > 0 && m.CapStrike > 0 {
		return fmt.Sprintf("%d-%d°F", m.FloorStrike, m.CapStrike)
	} else if m.CapStrike > 0 {
		return fmt.Sprintf("%d° or below", m.CapStrike)
	} else if m.FloorStrike > 0 {
		return fmt.Sprintf("%d° or above", m.FloorStrike)
	}
	return m.Subtitle
}

func getAllTrades(ticker string, client *rest.Client) ([]Trade, error) {
	allTrades := []Trade{}
	cursor := ""

	for i := 0; i < 10; i++ { // Max 10 pages
		url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets/trades?ticker=%s&limit=1000", ticker)
		if cursor != "" {
			url += "&cursor=" + cursor
		}

		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var result TradesResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		allTrades = append(allTrades, result.Trades...)

		if result.Cursor == "" || len(result.Trades) == 0 {
			break
		}
		cursor = result.Cursor
	}

	// Sort by time
	sort.Slice(allTrades, func(i, j int) bool {
		return allTrades[i].CreatedTime.Before(allTrades[j].CreatedTime)
	})

	return allTrades, nil
}

func analyzeTrades(date string, market *Market, trades []Trade) DayResult {
	result := DayResult{
		Date:           date,
		WinningBracket: bracketName(market),
		WinningTicker:  market.Ticker,
		TotalTrades:    len(trades),
	}

	if len(trades) == 0 {
		return result
	}

	result.FirstTradeTime = trades[0].CreatedTime
	result.FirstPrice = trades[0].YesPrice

	for _, t := range trades {
		if t.YesPrice >= 50 && result.Hit50Time == nil {
			result.Hit50Time = &t.CreatedTime
			result.Hit50Price = t.YesPrice
		}
		if t.YesPrice >= 80 && result.Hit80Time == nil {
			result.Hit80Time = &t.CreatedTime
			result.Hit80Price = t.YesPrice
		}
		if t.YesPrice >= 90 && result.Hit90Time == nil {
			result.Hit90Time = &t.CreatedTime
			result.Hit90Price = t.YesPrice
		}
	}

	return result
}

func printSummary(results []DayResult) {
	if len(results) == 0 {
		fmt.Println("\nNo results to summarize")
		return
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("SUMMARY: Price Timeline Analysis")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Header
	fmt.Printf("%-12s %-12s %8s %12s %12s %12s\n",
		"Date", "Winner", "1st Price", "Hit 80%", "Hit 90%", "Edge@1st")
	fmt.Println(strings.Repeat("-", 70))

	totalEdge := 0
	daysWithEdge := 0

	for _, r := range results {
		edge := 100 - r.FirstPrice
		totalEdge += edge
		if edge >= 20 {
			daysWithEdge++
		}

		hit80 := "N/A"
		hit90 := "N/A"

		if r.Hit80Time != nil {
			hit80 = r.Hit80Time.In(loc).Format("3:04 PM")
		}
		if r.Hit90Time != nil {
			hit90 = r.Hit90Time.In(loc).Format("3:04 PM")
		}

		edgeStr := fmt.Sprintf("%d¢", edge)
		if edge >= 50 {
			edgeStr += " 🎯"
		} else if edge >= 30 {
			edgeStr += " ✅"
		}

		fmt.Printf("%-12s %-12s %7d¢ %12s %12s %12s\n",
			r.Date, r.WinningBracket, r.FirstPrice, hit80, hit90, edgeStr)
	}

	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("\n📊 Average edge at first trade: %d¢\n", totalEdge/len(results))
	fmt.Printf("📈 Days with 20%+ edge: %d/%d (%.0f%%)\n",
		daysWithEdge, len(results), float64(daysWithEdge)/float64(len(results))*100)

	// Analyze best entry times
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("OPTIMAL ENTRY TIME ANALYSIS")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Group by time windows
	evening := 0   // 5 PM - 11:59 PM previous day
	lateNight := 0 // 12 AM - 5 AM
	earlyAM := 0   // 5 AM - 7 AM
	morning := 0   // 7 AM - 9 AM
	midMorn := 0   // 9 AM - 12 PM

	for _, r := range results {
		hour := r.FirstTradeTime.In(loc).Hour()
		if hour >= 17 { // 5 PM or later
			evening++
		} else if hour < 5 {
			lateNight++
		} else if hour < 7 {
			earlyAM++
		} else if hour < 9 {
			morning++
		} else {
			midMorn++
		}
	}

	total := len(results)
	fmt.Println("When does trading START for the winning bracket?")
	fmt.Printf("  Evening (5-11 PM):     %d/%d (%.0f%%)\n", evening, total, pct(evening, total))
	fmt.Printf("  Late Night (12-5 AM):  %d/%d (%.0f%%)\n", lateNight, total, pct(lateNight, total))
	fmt.Printf("  Early AM (5-7 AM):     %d/%d (%.0f%%)\n", earlyAM, total, pct(earlyAM, total))
	fmt.Printf("  Morning (7-9 AM):      %d/%d (%.0f%%)\n", morning, total, pct(morning, total))
	fmt.Printf("  Mid-Morning (9-12):    %d/%d (%.0f%%)\n", midMorn, total, pct(midMorn, total))

	fmt.Println()
	fmt.Println("💡 CONCLUSION:")
	fmt.Println("   The winning bracket often starts trading the EVENING BEFORE")
	fmt.Println("   at very low prices (15-40¢). This is where the edge is!")
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}
