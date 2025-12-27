package storage

import "time"

// Trade represents a trade record
type Trade struct {
	ID          int64     `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	City        string    `json:"city"`
	EventTicker string    `json:"event_ticker"`
	Bracket     string    `json:"bracket"`
	Ticker      string    `json:"ticker"`
	Side        string    `json:"side"`   // "yes" or "no"
	Action      string    `json:"action"` // "buy" or "sell"
	Price       int       `json:"price"`  // cents
	Quantity    int       `json:"quantity"`
	Cost        float64   `json:"cost"`
	OrderID     string    `json:"order_id"`
	Status      string    `json:"status"` // "pending", "filled", "error"
	Profit      float64   `json:"profit"` // Realized P&L (0 if not settled)
	Settled     bool      `json:"settled"`
	SettledAt   *time.Time `json:"settled_at,omitempty"`
}

// Position represents an open position
type Position struct {
	ID          int64     `json:"id"`
	EventTicker string    `json:"event_ticker"`
	City        string    `json:"city"`
	Bracket     string    `json:"bracket"`
	Ticker      string    `json:"ticker"`
	Side        string    `json:"side"`
	Quantity    int       `json:"quantity"`
	AvgPrice    float64   `json:"avg_price"`
	Cost        float64   `json:"cost"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DailyPnL represents daily profit/loss summary
type DailyPnL struct {
	ID           int64     `json:"id"`
	Date         time.Time `json:"date"`
	TotalTrades  int       `json:"total_trades"`
	YesTrades    int       `json:"yes_trades"`
	NoTrades     int       `json:"no_trades"`
	Wins         int       `json:"wins"`
	Losses       int       `json:"losses"`
	TotalCost    float64   `json:"total_cost"`
	TotalProfit  float64   `json:"total_profit"`
	NetPnL       float64   `json:"net_pnl"`
	WinRate      float64   `json:"win_rate"`
}

// ErrorLog represents an error event
type ErrorLog struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // "error", "warning", "info"
	Component string    `json:"component"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
}

// BotState represents the bot's runtime state
type BotState struct {
	ID            int64     `json:"id"`
	Key           string    `json:"key"`
	Value         string    `json:"value"`
	UpdatedAt     time.Time `json:"updated_at"`
}

