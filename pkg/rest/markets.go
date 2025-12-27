package rest

import (
	"encoding/json"
	"fmt"
)

// Market represents a Kalshi market.
type Market struct {
	Ticker             string  `json:"ticker"`
	EventTicker        string  `json:"event_ticker"`
	MarketType         string  `json:"market_type"`
	Title              string  `json:"title"`
	Subtitle           string  `json:"subtitle"`
	YesSubTitle        string  `json:"yes_sub_title"`
	NoSubTitle         string  `json:"no_sub_title"`
	Status             string  `json:"status"`
	YesBid             int     `json:"yes_bid"`
	YesAsk             int     `json:"yes_ask"`
	NoBid              int     `json:"no_bid"`
	NoAsk              int     `json:"no_ask"`
	LastPrice          int     `json:"last_price"`
	PreviousYesBid     int     `json:"previous_yes_bid"`
	PreviousYesAsk     int     `json:"previous_yes_ask"`
	PreviousPrice      int     `json:"previous_price"`
	Volume             int     `json:"volume"`
	Volume24H          int     `json:"volume_24h"`
	Liquidity          int     `json:"liquidity"`
	OpenInterest       int     `json:"open_interest"`
	Result             string  `json:"result"`
	CapStrike          float64 `json:"cap_strike"`
	FloorStrike        float64 `json:"floor_strike"`
	ExpectedExpiryTime string  `json:"expected_expiration_time"`
	ExpirationTime     string  `json:"expiration_time"`
	LatestExpiryTime   string  `json:"latest_expiration_time"`
	SettlementTimerSec int     `json:"settlement_timer_seconds"`
	CloseTime          string  `json:"close_time"`
	OpenTime           string  `json:"open_time"`
	Category           string  `json:"category"`
}

// Event represents a Kalshi event (contains multiple markets).
type Event struct {
	EventTicker  string `json:"event_ticker"`
	SeriesTicker string `json:"series_ticker"`
	Title        string `json:"title"`
	Mutually     bool   `json:"mutually_exclusive"`
	Category     string `json:"category"`
	SubTitle     string `json:"sub_title"`
	StrikeDate   string `json:"strike_date"`
	StrikePeriod string `json:"strike_period"`
}

// GetMarketsResponse represents a response from getting markets.
type GetMarketsResponse struct {
	Markets []Market `json:"markets"`
	Cursor  string   `json:"cursor"`
}

// GetEventResponse represents a response from getting an event.
type GetEventResponse struct {
	Event   Event    `json:"event"`
	Markets []Market `json:"markets"`
}

// Position represents a position in a market.
type Position struct {
	Ticker             string `json:"ticker"`
	EventTicker        string `json:"event_ticker"`
	EventTitle         string `json:"event_title"`
	MarketTitle        string `json:"market_title"`
	YesPosition        int    `json:"yes_position"`
	NoPosition         int    `json:"no_position"`
	TotalCost          int    `json:"total_cost"`
	RealizedPnl        int    `json:"realized_pnl"`
	RestingOrdersCount int    `json:"resting_orders_count"`
	Fees               int    `json:"fees"`
}

// GetPositionsResponse represents a response from getting positions.
type GetPositionsResponse struct {
	Positions []Position `json:"market_positions"`
	Cursor    string     `json:"cursor"`
}

// Balance represents account balance.
type Balance struct {
	Balance         int `json:"balance"`         // Available balance in cents
	PortfolioValue  int `json:"portfolio_value"` // Value of all positions in cents
	PayoutAvailable int `json:"payout_available"`
}

// GetMarket retrieves a single market by ticker.
func (c *Client) GetMarket(ticker string) (*Market, error) {
	data, err := c.Get(fmt.Sprintf("/markets/%s", ticker))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Market Market `json:"market"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp.Market, nil
}

// GetMarkets retrieves markets for an event.
func (c *Client) GetMarkets(eventTicker string) ([]Market, error) {
	path := "/markets"
	if eventTicker != "" {
		path += "?event_ticker=" + eventTicker
	}

	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	var resp GetMarketsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp.Markets, nil
}

// GetEvent retrieves an event and its markets.
func (c *Client) GetEvent(eventTicker string) (*Event, []Market, error) {
	data, err := c.Get(fmt.Sprintf("/events/%s", eventTicker))
	if err != nil {
		return nil, nil, err
	}

	var resp GetEventResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp.Event, resp.Markets, nil
}

// GetPositions retrieves all positions.
func (c *Client) GetPositions() ([]Position, error) {
	data, err := c.Get("/portfolio/positions")
	if err != nil {
		return nil, err
	}

	var resp GetPositionsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp.Positions, nil
}

// GetPosition retrieves position for a specific ticker.
func (c *Client) GetPosition(ticker string) (*Position, error) {
	data, err := c.Get(fmt.Sprintf("/portfolio/positions/%s", ticker))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Position Position `json:"market_position"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp.Position, nil
}

// GetBalance retrieves account balance.
func (c *Client) GetBalance() (*Balance, error) {
	data, err := c.Get("/portfolio/balance")
	if err != nil {
		return nil, err
	}

	var resp Balance
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}
