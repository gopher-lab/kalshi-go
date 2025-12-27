// Package main provides real-time trading recommendations using the ENSEMBLE strategy
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
	Ticker      string  `json:"ticker"`
	FloorStrike int     `json:"floor_strike"`
	CapStrike   int     `json:"cap_strike"`
	YesBid      float64 `json:"yes_bid"`
	YesAsk      float64 `json:"yes_ask"`
	LastPrice   float64 `json:"last_price"`
	Volume      int     `json:"volume"`
	Status      string  `json:"status"`
}

type MarketsResponse struct {
	Markets []Market `json:"markets"`
}

type BracketInfo struct {
	Floor    int
	Cap      int
	Ticker   string
	BidPrice int
	AskPrice int
	MidPrice int
	Volume   int
}

var loc *time.Location
var httpClient = &http.Client{Timeout: 15 * time.Second}

func init() {
	var err error
	loc, err = time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.UTC
	}
}

func main() {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("🎯 ENSEMBLE STRATEGY - LIVE RECOMMENDATION")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
	fmt.Printf("Time: %s\n", time.Now().In(loc).Format("Mon Jan 2, 2006 3:04 PM MST"))
	fmt.Println()

	// Target: Dec 27, 2025
	targetDate := time.Date(2025, 12, 27, 0, 0, 0, 0, loc)
	fmt.Printf("Target Market: KXHIGHLAX-%s (Dec 27 high temperature)\n", strings.ToUpper(targetDate.Format("06Jan02")))
	fmt.Println()

	// Get current METAR
	fmt.Println("📡 Fetching current METAR data...")
	metar, metarTime := getCurrentMETAR()
	if metar > 0 {
		fmt.Printf("   Current LAX temperature: %d°F (as of %s)\n", metar, metarTime)
	} else {
		fmt.Println("   ⚠️  Could not fetch METAR - using NWS forecast")
		metar = 61 // NWS forecast for Dec 27
	}
	fmt.Println()

	// Get market data
	fmt.Println("📊 Fetching Kalshi market data...")
	brackets := getMarketData("KXHIGHLAX-25DEC27")
	if len(brackets) == 0 {
		fmt.Println("   ❌ Could not fetch market data")
		return
	}
	fmt.Printf("   Found %d brackets\n", len(brackets))
	fmt.Println()

	// Sort by mid price (highest = market favorite)
	sort.Slice(brackets, func(i, j int) bool {
		return brackets[i].MidPrice > brackets[j].MidPrice
	})

	// Display market
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("CURRENT MARKET PRICES")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
	fmt.Printf("%-15s %8s %8s %8s %8s\n", "Bracket", "Bid", "Ask", "Mid", "Volume")
	fmt.Println(strings.Repeat("-", 50))

	for _, b := range brackets {
		marker := ""
		if b == brackets[0] {
			marker = " 👑 MARKET FAV"
		}
		if b == brackets[1] {
			marker = " 🥈 2ND BEST"
		}
		fmt.Printf("%3d-%3d°F      %7d¢ %7d¢ %7d¢ %8d%s\n",
			b.Floor, b.Cap, b.BidPrice, b.AskPrice, b.MidPrice, b.Volume, marker)
	}
	fmt.Println()

	// Apply ENSEMBLE strategy
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("🧠 ENSEMBLE STRATEGY ANALYSIS")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Signal 1: Market favorite
	marketFav := brackets[0].Floor
	fmt.Printf("Signal 1 - MARKET FAVORITE:  %d-%d°F (highest price: %d¢)\n",
		brackets[0].Floor, brackets[0].Cap, brackets[0].MidPrice)

	// Signal 2: METAR prediction
	metarBracket := (metar / 2) * 2 // Round down to even
	if metar%2 == 1 {
		metarBracket = metar - 1
	}
	fmt.Printf("Signal 2 - METAR PREDICTION: %d-%d°F (current temp: %d°F)\n",
		metarBracket, metarBracket+1, metar)

	// Signal 3: 2nd best
	secondBest := brackets[1].Floor
	fmt.Printf("Signal 3 - 2ND BEST:         %d-%d°F (price: %d¢)\n",
		brackets[1].Floor, brackets[1].Cap, brackets[1].MidPrice)

	fmt.Println()

	// Count votes
	votes := make(map[int][]string)
	votes[marketFav] = append(votes[marketFav], "Market")
	votes[metarBracket] = append(votes[metarBracket], "METAR")
	votes[secondBest] = append(votes[secondBest], "2nd")

	fmt.Println("VOTE COUNT:")
	for floor, voters := range votes {
		fmt.Printf("   %d-%d°F: %d votes (%s)\n", floor, floor+1, len(voters), strings.Join(voters, " + "))
	}
	fmt.Println()

	// Find consensus
	bestBracket := 0
	maxVotes := 0
	var reasons []string
	for floor, voters := range votes {
		if len(voters) > maxVotes {
			maxVotes = len(voters)
			bestBracket = floor
			reasons = voters
		}
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("📋 RECOMMENDATION")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	if maxVotes >= 2 {
		// Find the bracket info
		var targetBracket *BracketInfo
		for i := range brackets {
			if brackets[i].Floor == bestBracket {
				targetBracket = &brackets[i]
				break
			}
		}

		if targetBracket == nil {
			fmt.Println("⚠️  Recommended bracket not found in market")
			return
		}

		fmt.Println("✅ CONSENSUS REACHED!")
		fmt.Println()
		fmt.Printf("   Bracket: %d-%d°F\n", bestBracket, bestBracket+1)
		fmt.Printf("   Signals: %s (%d/3 agree)\n", strings.Join(reasons, " + "), maxVotes)
		fmt.Printf("   Current Ask: %d¢\n", targetBracket.AskPrice)
		fmt.Printf("   Ticker: %s\n", targetBracket.Ticker)
		fmt.Println()

		// Calculate position
		betSize := 14.0 // $14
		contracts := int(betSize*100) / targetBracket.AskPrice
		totalCost := float64(contracts*targetBracket.AskPrice) / 100
		potentialPayout := float64(contracts)
		potentialProfit := potentialPayout - totalCost

		fmt.Println("💰 TRADE DETAILS:")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("   Bet size: $%.2f\n", betSize)
		fmt.Printf("   Price: %d¢ per contract\n", targetBracket.AskPrice)
		fmt.Printf("   Contracts: %d\n", contracts)
		fmt.Printf("   Total cost: $%.2f\n", totalCost)
		fmt.Printf("   Potential payout: $%.2f\n", potentialPayout)
		fmt.Printf("   Potential profit: $%.2f (%.0f%% return)\n", potentialProfit, potentialProfit/totalCost*100)
		fmt.Println()

		fmt.Println("🚀 TO EXECUTE:")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("   1. Go to: https://kalshi.com/markets/kxhighlax/highest-temperature-in-los-angeles\n")
		fmt.Printf("   2. Select: %d-%d°F bracket\n", bestBracket, bestBracket+1)
		fmt.Printf("   3. Buy YES at %d¢\n", targetBracket.AskPrice)
		fmt.Printf("   4. Quantity: %d contracts\n", contracts)
		fmt.Printf("   5. Total: $%.2f\n", totalCost)
		fmt.Println()

		// Or via API
		fmt.Println("📡 OR VIA API:")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("   go run ./cmd/lahigh-trader/ -event KXHIGHLAX-25DEC27 -auto\n")

	} else {
		fmt.Println("❌ NO CONSENSUS - SKIP TODAY")
		fmt.Println()
		fmt.Println("   The 3 signals disagree:")
		fmt.Printf("   • Market says: %d-%d°F\n", marketFav, marketFav+1)
		fmt.Printf("   • METAR says: %d-%d°F\n", metarBracket, metarBracket+1)
		fmt.Printf("   • 2nd best: %d-%d°F\n", secondBest, secondBest+1)
		fmt.Println()
		fmt.Println("   Per our strategy: Don't bet when signals disagree.")
		fmt.Println("   Wait for tomorrow's market.")
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("⚠️  DISCLAIMER: This is not financial advice. Trade at your own risk.")
	fmt.Println(strings.Repeat("=", 80))
}

func getCurrentMETAR() (int, string) {
	// Get from Aviation Weather Center
	url := "https://aviationweather.gov/api/data/metar?ids=KLAX&format=json"

	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var data []map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil || len(data) == 0 {
		return 0, ""
	}

	// Extract temperature
	if temp, ok := data[0]["temp"].(float64); ok {
		tempF := int(temp*9/5 + 32)
		obsTime := ""
		if t, ok := data[0]["obsTime"].(string); ok {
			obsTime = t
		}
		return tempF, obsTime
	}

	return 0, ""
}

func getMarketData(eventTicker string) []BracketInfo {
	url := fmt.Sprintf("https://api.elections.kalshi.com/trade-api/v2/markets?event_ticker=%s&limit=100", eventTicker)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result MarketsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil
	}

	var brackets []BracketInfo
	for _, m := range result.Markets {
		if m.Status != "active" {
			continue
		}

		bid := int(m.YesBid * 100)
		ask := int(m.YesAsk * 100)
		if ask == 0 {
			ask = int(m.LastPrice * 100)
		}
		if bid == 0 {
			bid = ask - 5
		}
		mid := (bid + ask) / 2

		brackets = append(brackets, BracketInfo{
			Floor:    m.FloorStrike,
			Cap:      m.CapStrike,
			Ticker:   m.Ticker,
			BidPrice: bid,
			AskPrice: ask,
			MidPrice: mid,
			Volume:   m.Volume,
		})
	}

	return brackets
}

func getMETARMax(date time.Time) (int, error) {
	url := fmt.Sprintf(
		"https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py?station=LAX&data=tmpf&year1=%d&month1=%d&day1=%d&year2=%d&month2=%d&day2=%d&tz=America/Los_Angeles&format=onlycomma&latlon=no&elev=no&missing=M&trace=T&direct=no&report_type=3",
		date.Year(), int(date.Month()), date.Day(),
		date.Year(), int(date.Month()), date.Day()+1,
	)

	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(string(body), "\n")
	maxTemp := 0.0

	for _, line := range lines {
		if strings.HasPrefix(line, "LAX,") {
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

	if maxTemp == 0 {
		return 0, fmt.Errorf("no data")
	}

	return int(math.Round(maxTemp)), nil
}

