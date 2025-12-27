package storage

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store provides SQLite-based persistence
type Store struct {
	db *sql.DB
}

// NewStore creates a new SQLite store
func NewStore(dataDir string) (*Store, error) {
	dbPath := filepath.Join(dataDir, "bot.db")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	log.Printf("[Store] Initialized SQLite database at %s", dbPath)
	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate creates the database schema
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS trades (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		city TEXT NOT NULL,
		event_ticker TEXT NOT NULL,
		bracket TEXT NOT NULL,
		ticker TEXT NOT NULL,
		side TEXT NOT NULL,
		action TEXT NOT NULL,
		price INTEGER NOT NULL,
		quantity INTEGER NOT NULL,
		cost REAL NOT NULL,
		order_id TEXT NOT NULL,
		status TEXT NOT NULL,
		profit REAL DEFAULT 0,
		settled INTEGER DEFAULT 0,
		settled_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_trades_event ON trades(event_ticker);
	CREATE INDEX IF NOT EXISTS idx_trades_timestamp ON trades(timestamp);
	CREATE INDEX IF NOT EXISTS idx_trades_settled ON trades(settled);

	CREATE TABLE IF NOT EXISTS positions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_ticker TEXT NOT NULL,
		city TEXT NOT NULL,
		bracket TEXT NOT NULL,
		ticker TEXT NOT NULL,
		side TEXT NOT NULL,
		quantity INTEGER NOT NULL,
		avg_price REAL NOT NULL,
		cost REAL NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_positions_ticker ON positions(ticker, side);

	CREATE TABLE IF NOT EXISTS daily_pnl (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date DATE UNIQUE NOT NULL,
		total_trades INTEGER DEFAULT 0,
		yes_trades INTEGER DEFAULT 0,
		no_trades INTEGER DEFAULT 0,
		wins INTEGER DEFAULT 0,
		losses INTEGER DEFAULT 0,
		total_cost REAL DEFAULT 0,
		total_profit REAL DEFAULT 0,
		net_pnl REAL DEFAULT 0,
		win_rate REAL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS error_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		level TEXT NOT NULL,
		component TEXT NOT NULL,
		message TEXT NOT NULL,
		details TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_errors_timestamp ON error_logs(timestamp);

	CREATE TABLE IF NOT EXISTS bot_state (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT UNIQUE NOT NULL,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveTrade saves a trade to the database
func (s *Store) SaveTrade(t *Trade) error {
	result, err := s.db.Exec(`
		INSERT INTO trades (timestamp, city, event_ticker, bracket, ticker, side, action, price, quantity, cost, order_id, status, profit, settled, settled_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Timestamp, t.City, t.EventTicker, t.Bracket, t.Ticker, t.Side, t.Action,
		t.Price, t.Quantity, t.Cost, t.OrderID, t.Status, t.Profit, t.Settled, t.SettledAt,
	)
	if err != nil {
		return err
	}

	id, _ := result.LastInsertId()
	t.ID = id
	return nil
}

// GetTradesByEvent returns all trades for an event
func (s *Store) GetTradesByEvent(eventTicker string) ([]Trade, error) {
	rows, err := s.db.Query(`
		SELECT id, timestamp, city, event_ticker, bracket, ticker, side, action, price, quantity, cost, order_id, status, profit, settled, settled_at
		FROM trades WHERE event_ticker = ? ORDER BY timestamp DESC`,
		eventTicker,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var t Trade
		if err := rows.Scan(&t.ID, &t.Timestamp, &t.City, &t.EventTicker, &t.Bracket, &t.Ticker,
			&t.Side, &t.Action, &t.Price, &t.Quantity, &t.Cost, &t.OrderID, &t.Status, &t.Profit, &t.Settled, &t.SettledAt); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}

// GetUnsettledTrades returns all unsettled trades
func (s *Store) GetUnsettledTrades() ([]Trade, error) {
	rows, err := s.db.Query(`
		SELECT id, timestamp, city, event_ticker, bracket, ticker, side, action, price, quantity, cost, order_id, status, profit, settled, settled_at
		FROM trades WHERE settled = 0 ORDER BY timestamp DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var t Trade
		if err := rows.Scan(&t.ID, &t.Timestamp, &t.City, &t.EventTicker, &t.Bracket, &t.Ticker,
			&t.Side, &t.Action, &t.Price, &t.Quantity, &t.Cost, &t.OrderID, &t.Status, &t.Profit, &t.Settled, &t.SettledAt); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}

// SettleTrade marks a trade as settled with profit
func (s *Store) SettleTrade(id int64, profit float64) error {
	now := time.Now()
	_, err := s.db.Exec(`UPDATE trades SET settled = 1, profit = ?, settled_at = ? WHERE id = ?`,
		profit, now, id)
	return err
}

// GetTodayStats returns trading statistics for today
func (s *Store) GetTodayStats() (*DailyPnL, error) {
	today := time.Now().Truncate(24 * time.Hour)
	
	var stats DailyPnL
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(1), 0), 
			   COALESCE(SUM(CASE WHEN side = 'yes' THEN 1 ELSE 0 END), 0),
			   COALESCE(SUM(CASE WHEN side = 'no' THEN 1 ELSE 0 END), 0),
			   COALESCE(SUM(CASE WHEN profit > 0 THEN 1 ELSE 0 END), 0),
			   COALESCE(SUM(CASE WHEN profit < 0 THEN 1 ELSE 0 END), 0),
			   COALESCE(SUM(cost), 0),
			   COALESCE(SUM(profit), 0)
		FROM trades WHERE DATE(timestamp) = DATE(?)`,
		today,
	).Scan(&stats.TotalTrades, &stats.YesTrades, &stats.NoTrades, &stats.Wins, &stats.Losses, &stats.TotalCost, &stats.TotalProfit)
	
	if err != nil {
		return nil, err
	}

	stats.Date = today
	stats.NetPnL = stats.TotalProfit
	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.TotalTrades) * 100
	}
	
	return &stats, nil
}

// LogError logs an error to the database
func (s *Store) LogError(level, component, message, details string) error {
	_, err := s.db.Exec(`
		INSERT INTO error_logs (timestamp, level, component, message, details)
		VALUES (?, ?, ?, ?, ?)`,
		time.Now(), level, component, message, details,
	)
	return err
}

// GetState retrieves a bot state value
func (s *Store) GetState(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM bot_state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetState sets a bot state value
func (s *Store) SetState(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO bot_state (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = ?`,
		key, value, time.Now(), value, time.Now(),
	)
	return err
}

// GetAllTimeStats returns all-time trading statistics
func (s *Store) GetAllTimeStats() (map[string]interface{}, error) {
	var totalTrades, wins int
	var totalCost, totalProfit float64
	
	err := s.db.QueryRow(`
		SELECT COALESCE(COUNT(*), 0),
			   COALESCE(SUM(CASE WHEN profit > 0 THEN 1 ELSE 0 END), 0),
			   COALESCE(SUM(cost), 0),
			   COALESCE(SUM(profit), 0)
		FROM trades WHERE settled = 1`,
	).Scan(&totalTrades, &wins, &totalCost, &totalProfit)
	
	if err != nil {
		return nil, err
	}

	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(wins) / float64(totalTrades) * 100
	}
	
	return map[string]interface{}{
		"total_trades":  totalTrades,
		"wins":          wins,
		"total_cost":    totalCost,
		"total_profit":  totalProfit,
		"net_pnl":       totalProfit,
		"win_rate":      winRate,
	}, nil
}

