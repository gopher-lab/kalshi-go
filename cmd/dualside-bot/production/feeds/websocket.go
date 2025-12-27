package feeds

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/brendanplayford/kalshi-go/pkg/ws"
)

// Orderbook represents an orderbook for a market
type Orderbook struct {
	Ticker   string
	YesBid   int // Best YES bid price (cents)
	YesAsk   int // Best YES ask price (cents)
	NoBid    int // Best NO bid price (cents)
	NoAsk    int // Best NO ask price (cents)
	Updated  time.Time
}

// TickerData represents ticker data for a market
type TickerData struct {
	Ticker  string
	YesBid  int
	YesAsk  int
	Volume  int
	Updated time.Time
}

// KalshiFeed provides real-time market data via WebSocket
type KalshiFeed struct {
	client  *ws.Client
	apiKey  string
	privKey *rsa.PrivateKey

	mu         sync.RWMutex
	tickers    map[string]*TickerData
	subscribed map[string]int64 // ticker -> SID

	stopChan  chan struct{}
	connected bool
}

// NewKalshiFeed creates a new WebSocket feed
func NewKalshiFeed(apiKey string, privateKey *rsa.PrivateKey) *KalshiFeed {
	return &KalshiFeed{
		apiKey:     apiKey,
		privKey:    privateKey,
		tickers:    make(map[string]*TickerData),
		subscribed: make(map[string]int64),
		stopChan:   make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection
func (f *KalshiFeed) Connect(ctx context.Context) error {
	f.client = ws.New(
		ws.WithAPIKeyOption(f.apiKey, f.privKey),
		ws.WithCallbacks(
			func() {
				log.Println("[WSFeed] Connected")
				f.connected = true
			},
			func(err error) {
				log.Printf("[WSFeed] Disconnected: %v", err)
				f.connected = false
			},
			func(err error) {
				log.Printf("[WSFeed] Error: %v", err)
			},
		),
	)

	// Set up message handler
	f.client.SetMessageHandler(f.handleMessage)

	// Connect
	if err := f.client.Connect(ctx); err != nil {
		return err
	}

	log.Println("[WSFeed] Connected to Kalshi WebSocket")

	// Start reconnection monitor
	go f.monitorConnection(ctx)

	return nil
}

// Subscribe subscribes to ticker updates for a market
func (f *KalshiFeed) Subscribe(ctx context.Context, ticker string) error {
	f.mu.Lock()
	if _, exists := f.subscribed[ticker]; exists {
		f.mu.Unlock()
		return nil
	}
	f.mu.Unlock()

	// Subscribe to ticker channel for this market
	sid, err := f.client.Subscribe(ctx, ticker, ws.ChannelTicker)
	if err != nil {
		return err
	}

	f.mu.Lock()
	f.subscribed[ticker] = sid
	f.mu.Unlock()

	log.Printf("[WSFeed] Subscribed to %s (SID: %d)", ticker, sid)
	return nil
}

// Unsubscribe unsubscribes from ticker updates
func (f *KalshiFeed) Unsubscribe(ctx context.Context, ticker string) error {
	f.mu.Lock()
	sid, exists := f.subscribed[ticker]
	if !exists {
		f.mu.Unlock()
		return nil
	}
	delete(f.subscribed, ticker)
	delete(f.tickers, ticker)
	f.mu.Unlock()

	_, err := f.client.Unsubscribe(ctx, sid)
	if err != nil {
		return err
	}

	log.Printf("[WSFeed] Unsubscribed from %s", ticker)
	return nil
}

// GetTicker returns the current ticker data for a market
func (f *KalshiFeed) GetTicker(ticker string) *TickerData {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.tickers[ticker]
}

// GetBestYesBid returns the best YES bid for a ticker
func (f *KalshiFeed) GetBestYesBid(ticker string) int {
	data := f.GetTicker(ticker)
	if data == nil {
		return 0
	}
	return data.YesBid
}

// IsConnected returns true if WebSocket is connected
func (f *KalshiFeed) IsConnected() bool {
	return f.connected && f.client.IsConnected()
}

// Close closes the WebSocket connection
func (f *KalshiFeed) Close() error {
	close(f.stopChan)
	f.connected = false
	if f.client != nil {
		return f.client.Close()
	}
	return nil
}

func (f *KalshiFeed) handleMessage(resp *ws.Response) {
	// Handle different message types
	switch resp.Type {
	case ws.MessageTypeData:
		f.handleDataMessage(resp)
	case ws.MessageTypeSubscribed:
		log.Printf("[WSFeed] Subscription confirmed: SID=%d", resp.SID)
	case ws.MessageTypeError:
		if errMsg, err := ws.ParseErrorMsg(resp.Msg); err == nil {
			log.Printf("[WSFeed] Error message: %s", errMsg.Msg)
		}
	}
}

func (f *KalshiFeed) handleDataMessage(resp *ws.Response) {
	// Parse data field
	if resp.Msg == nil {
		return
	}

	data, err := json.Marshal(resp.Msg)
	if err != nil {
		return
	}

	// Try to parse as ticker update
	var tickerUpdate struct {
		MarketTicker string `json:"market_ticker"`
		YesBid       int    `json:"yes_bid"`
		YesAsk       int    `json:"yes_ask"`
		NoBid        int    `json:"no_bid"`
		NoAsk        int    `json:"no_ask"`
		Volume       int    `json:"volume"`
	}

	if err := json.Unmarshal(data, &tickerUpdate); err != nil {
		return
	}

	if tickerUpdate.MarketTicker == "" {
		return
	}

	f.mu.Lock()
	f.tickers[tickerUpdate.MarketTicker] = &TickerData{
		Ticker:  tickerUpdate.MarketTicker,
		YesBid:  tickerUpdate.YesBid,
		YesAsk:  tickerUpdate.YesAsk,
		Volume:  tickerUpdate.Volume,
		Updated: time.Now(),
	}
	f.mu.Unlock()

	log.Printf("[WSFeed] Ticker update: %s YesBid=%d¢ YesAsk=%d¢",
		tickerUpdate.MarketTicker, tickerUpdate.YesBid, tickerUpdate.YesAsk)
}

func (f *KalshiFeed) monitorConnection(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-f.stopChan:
			return
		case <-ticker.C:
			if !f.IsConnected() {
				log.Println("[WSFeed] Connection lost, attempting reconnect...")
				if err := f.Connect(ctx); err != nil {
					log.Printf("[WSFeed] Reconnect failed: %v", err)
				} else {
					// Resubscribe to all markets
					f.mu.RLock()
					tickers := make([]string, 0, len(f.subscribed))
					for ticker := range f.subscribed {
						tickers = append(tickers, ticker)
					}
					f.mu.RUnlock()

					for _, t := range tickers {
						if err := f.Subscribe(ctx, t); err != nil {
							log.Printf("[WSFeed] Resubscribe failed for %s: %v", t, err)
						}
					}
				}
			}
		}
	}
}
