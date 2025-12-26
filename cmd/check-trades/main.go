package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/brendanplayford/kalshi-go/internal/config"
	"github.com/brendanplayford/kalshi-go/pkg/rest"
)

type Trade struct {
	CreatedTime time.Time `json:"created_time"`
	YesPrice    int       `json:"yes_price"`
	Count       int       `json:"count"`
}

type TradesResponse struct {
	Trades []Trade `json:"trades"`
}

func main() {
	cfg, _ := config.Load()
	if !cfg.IsAuthenticated() {
		fmt.Println("No credentials")
		os.Exit(1)
	}

	client := rest.New(cfg.APIKey, cfg.PrivateKey)

	tickers := []string{
		"KXHIGHLAX-25DEC25-B66.5",
		"KXHIGHLAX-25DEC24-B64.5", 
		"KXHIGHLAX-25DEC23-B64.5",
	}

	fmt.Println("=" + string(make([]byte, 60)))
	fmt.Println("WHEN DID MARKET KNOW? (Last 3 Days)")
	fmt.Println("=" + string(make([]byte, 60)))

	for _, ticker := range tickers {
		fmt.Printf("\n%s\n", ticker)
		
		data, err := client.Get("/markets/" + ticker + "/trades?limit=500")
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		var resp TradesResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			fmt.Printf("  Parse error: %v\n", err)
			continue
		}

		if len(resp.Trades) == 0 {
			fmt.Println("  No trades")
			continue
		}

		sort.Slice(resp.Trades, func(i, j int) bool {
			return resp.Trades[i].CreatedTime.Before(resp.Trades[j].CreatedTime)
		})

		fmt.Printf("  Trades: %d\n", len(resp.Trades))

		loc, _ := time.LoadLocation("America/Los_Angeles")
		
		var hit50, hit80, hit90 *Trade
		for i := range resp.Trades {
			t := &resp.Trades[i]
			if t.YesPrice >= 50 && hit50 == nil {
				hit50 = t
			}
			if t.YesPrice >= 80 && hit80 == nil {
				hit80 = t
			}
			if t.YesPrice >= 90 && hit90 == nil {
				hit90 = t
			}
		}

		if hit50 != nil {
			fmt.Printf("  50%%: %s @ %d¢\n", hit50.CreatedTime.In(loc).Format("01/02 3:04 PM PT"), hit50.YesPrice)
		}
		if hit80 != nil {
			fmt.Printf("  80%%: %s @ %d¢\n", hit80.CreatedTime.In(loc).Format("01/02 3:04 PM PT"), hit80.YesPrice)
		}
		if hit90 != nil {
			fmt.Printf("  90%%: %s @ %d¢\n", hit90.CreatedTime.In(loc).Format("01/02 3:04 PM PT"), hit90.YesPrice)
		}
	}
}
