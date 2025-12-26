package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var (
	// ErrNotConnected is returned when an operation requires an active connection.
	ErrNotConnected = errors.New("websocket: not connected")

	// ErrAlreadyConnected is returned when Connect is called on an active connection.
	ErrAlreadyConnected = errors.New("websocket: already connected")

	// ErrAuthRequired is returned when subscribing to an authenticated channel without credentials.
	ErrAuthRequired = errors.New("websocket: authentication required for this channel")

	// ErrInvalidChannel is returned when an invalid channel is specified.
	ErrInvalidChannel = errors.New("websocket: invalid channel")

	// ErrConnectionClosed is returned when the connection is closed.
	ErrConnectionClosed = errors.New("websocket: connection closed")
)

// MessageHandler is a callback for handling incoming messages.
type MessageHandler func(msg *Response)

// DataHandler is a callback for handling data messages from subscriptions.
type DataHandler func(sid int64, data json.RawMessage)

// Client is a WebSocket client for the Kalshi API.
type Client struct {
	opts        Options
	conn        *websocket.Conn
	mu          sync.RWMutex
	done        chan struct{}
	msgID       atomic.Int64
	handler     MessageHandler
	dataHandler DataHandler

	// subscriptions tracks active subscriptions by SID.
	subscriptions sync.Map
}

// New creates a new WebSocket client with the given options.
func New(opts ...Option) *Client {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return &Client{
		opts: options,
	}
}

// NewWithOptions creates a new WebSocket client with explicit options.
func NewWithOptions(opts Options) *Client {
	return &Client{
		opts: opts,
	}
}

// Connect establishes a WebSocket connection to the Kalshi API.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return ErrAlreadyConnected
	}

	header := http.Header{}
	if c.opts.Headers != nil {
		for k, v := range c.opts.Headers {
			header[k] = v
		}
	}

	// Add authentication headers if credentials are provided.
	if c.opts.IsAuthenticated() {
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sig, err := GenerateSignature(c.opts.PrivateKey, ts, "GET", "/trade-api/ws/v2")
		if err != nil {
			return fmt.Errorf("generate signature: %w", err)
		}
		header.Set("KALSHI-ACCESS-KEY", c.opts.APIKey)
		header.Set("KALSHI-ACCESS-SIGNATURE", sig)
		header.Set("KALSHI-ACCESS-TIMESTAMP", ts)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.opts.BaseURL, header)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	c.conn = conn
	c.done = make(chan struct{})

	// Start the read loop.
	go c.readLoop()

	// Start the ping loop.
	go c.pingLoop()

	if c.opts.OnConnect != nil {
		c.opts.OnConnect()
	}

	return nil
}

// Close closes the WebSocket connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	// Signal the read and ping loops to stop.
	close(c.done)

	// Send close message.
	err := c.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	if err != nil {
		// Ignore write errors on close.
		_ = err
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("websocket close: %w", err)
	}

	c.conn = nil
	return nil
}

// IsConnected returns true if the client is connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil
}

// SetMessageHandler sets the handler for incoming messages.
func (c *Client) SetMessageHandler(handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// SetDataHandler sets the handler for data messages.
func (c *Client) SetDataHandler(handler DataHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dataHandler = handler
}

// Subscribe subscribes to one or more channels for a market.
func (c *Client) Subscribe(ctx context.Context, marketTicker string, channels ...Channel) (int64, error) {
	// Validate channels.
	for _, ch := range channels {
		if !ch.IsValid() {
			return 0, ErrInvalidChannel
		}
		if ch.RequiresAuth() && !c.opts.IsAuthenticated() {
			return 0, ErrAuthRequired
		}
	}

	params := SubscribeParams{
		Channels:     channels,
		MarketTicker: marketTicker,
	}

	id, err := c.sendCommand(CommandSubscribe, params)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// Unsubscribe cancels subscriptions by their SIDs.
func (c *Client) Unsubscribe(ctx context.Context, sids ...int64) (int64, error) {
	params := UnsubscribeParams{
		SIDs: sids,
	}

	return c.sendCommand(CommandUnsubscribe, params)
}

// ListSubscriptions requests a list of all active subscriptions.
func (c *Client) ListSubscriptions(ctx context.Context) (int64, error) {
	return c.sendCommand(CommandListSubscriptions, nil)
}

// AddMarkets adds markets to existing subscriptions.
func (c *Client) AddMarkets(ctx context.Context, sids []int64, marketTickers []string) (int64, error) {
	params := UpdateSubscriptionParams{
		SIDs:          sids,
		MarketTickers: marketTickers,
		Action:        ActionAddMarkets,
	}

	return c.sendCommand(CommandUpdateSubscription, params)
}

// RemoveMarkets removes markets from existing subscriptions.
func (c *Client) RemoveMarkets(ctx context.Context, sids []int64, marketTickers []string) (int64, error) {
	params := UpdateSubscriptionParams{
		SIDs:          sids,
		MarketTickers: marketTickers,
		Action:        ActionDeleteMarkets,
	}

	return c.sendCommand(CommandUpdateSubscription, params)
}

// sendCommand sends a command to the WebSocket server.
func (c *Client) sendCommand(cmd Command, params any) (int64, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return 0, ErrNotConnected
	}

	id := c.msgID.Add(1)

	req := Request{
		ID:  id,
		Cmd: cmd,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return 0, fmt.Errorf("marshal params: %w", err)
		}
		req.Params = data
	}

	c.mu.Lock()
	err := c.conn.WriteJSON(req)
	c.mu.Unlock()

	if err != nil {
		return 0, fmt.Errorf("write message: %w", err)
	}

	return id, nil
}

// readLoop reads messages from the WebSocket connection.
func (c *Client) readLoop() {
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()

		if c.opts.OnDisconnect != nil {
			c.opts.OnDisconnect(nil)
		}
	}()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return
			}
			if c.opts.OnError != nil {
				c.opts.OnError(err)
			}
			return
		}

		resp, err := ParseResponse(message)
		if err != nil {
			if c.opts.OnError != nil {
				c.opts.OnError(fmt.Errorf("parse response: %w", err))
			}
			continue
		}

		// Track subscriptions.
		if resp.Type == MessageTypeSubscribed {
			if subMsg, err := ParseSubscribedMsg(resp.Msg); err == nil {
				c.subscriptions.Store(subMsg.SID, subMsg.Channel)
			}
		} else if resp.Type == MessageTypeUnsubscribed {
			c.subscriptions.Delete(resp.SID)
		}

		c.mu.RLock()
		handler := c.handler
		c.mu.RUnlock()

		if handler != nil {
			handler(resp)
		}
	}
}

// pingLoop sends periodic ping frames to keep the connection alive.
func (c *Client) pingLoop() {
	ticker := time.NewTicker(c.opts.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				return
			}

			c.mu.Lock()
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			c.mu.Unlock()

			if err != nil {
				if c.opts.OnError != nil {
					c.opts.OnError(fmt.Errorf("ping: %w", err))
				}
				return
			}
		}
	}
}

// GetActiveSubscriptions returns a map of active subscription SIDs to channels.
func (c *Client) GetActiveSubscriptions() map[int64]Channel {
	result := make(map[int64]Channel)
	c.subscriptions.Range(func(key, value any) bool {
		if sid, ok := key.(int64); ok {
			if ch, ok := value.(Channel); ok {
				result[sid] = ch
			}
		}
		return true
	})
	return result
}
