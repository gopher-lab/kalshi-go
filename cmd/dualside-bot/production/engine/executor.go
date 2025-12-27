package engine

import (
	"crypto/rsa"
	"fmt"
	"log"
	"time"

	"github.com/brendanplayford/kalshi-go/pkg/rest"
)

// ExecuteOrderRequest represents an order to execute
type ExecuteOrderRequest struct {
	Ticker   string
	Side     string // "yes" or "no"
	Action   string // "buy" or "sell"
	Price    int    // in cents
	Quantity int
}

// Executor handles order execution with retry logic
type Executor struct {
	client     *rest.Client
	dryRun     bool
	maxRetries int
	retryDelay time.Duration
}

// NewExecutor creates a new order executor
func NewExecutor(apiKey string, privateKey *rsa.PrivateKey, dryRun bool) (*Executor, error) {
	client := rest.New(apiKey, privateKey)

	// Verify connection
	_, err := client.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to verify API connection: %w", err)
	}

	return &Executor{
		client:     client,
		dryRun:     dryRun,
		maxRetries: 3,
		retryDelay: 2 * time.Second,
	}, nil
}

// GetBalance returns current account balance
func (e *Executor) GetBalance() (float64, error) {
	balance, err := e.client.GetBalance()
	if err != nil {
		return 0, err
	}
	return float64(balance.Balance) / 100.0, nil
}

// ExecuteOrder executes an order with retry logic
func (e *Executor) ExecuteOrder(req ExecuteOrderRequest) (string, error) {
	if e.dryRun {
		orderID := fmt.Sprintf("DRY-%d", time.Now().UnixNano())
		log.Printf("[Executor] DRY RUN: %s %s %d @ %d¢ → %s",
			req.Action, req.Side, req.Quantity, req.Price, orderID)
		return orderID, nil
	}

	var lastErr error
	for attempt := 1; attempt <= e.maxRetries; attempt++ {
		orderID, err := e.executeOnce(req)
		if err == nil {
			return orderID, nil
		}

		lastErr = err
		log.Printf("[Executor] Attempt %d/%d failed: %v", attempt, e.maxRetries, err)

		if attempt < e.maxRetries {
			time.Sleep(e.retryDelay * time.Duration(attempt)) // Exponential backoff
		}
	}

	return "", fmt.Errorf("all %d attempts failed: %w", e.maxRetries, lastErr)
}

func (e *Executor) executeOnce(req ExecuteOrderRequest) (string, error) {
	// Convert string action/side to rest types
	var action rest.OrderAction
	if req.Action == "buy" {
		action = rest.OrderActionBuy
	} else {
		action = rest.OrderActionSell
	}

	var side rest.Side
	if req.Side == "yes" {
		side = rest.SideYes
	} else {
		side = rest.SideNo
	}

	order := &rest.CreateOrderRequest{
		Ticker: req.Ticker,
		Action: action,
		Side:   side,
		Type:   rest.OrderTypeLimit,
		Count:  req.Quantity,
	}

	if req.Side == "yes" {
		order.YesPrice = req.Price
	} else {
		order.NoPrice = req.Price
	}

	resp, err := e.client.CreateOrder(order)
	if err != nil {
		return "", err
	}

	log.Printf("[Executor] Order placed: %s %s %d @ %d¢ → %s",
		req.Action, req.Side, req.Quantity, req.Price, resp.OrderID)

	return resp.OrderID, nil
}

// CancelOrder cancels an order
func (e *Executor) CancelOrder(orderID string) error {
	if e.dryRun {
		log.Printf("[Executor] DRY RUN: Cancel order %s", orderID)
		return nil
	}

	_, err := e.client.CancelOrder(orderID)
	return err
}

// IsDryRun returns true if in dry run mode
func (e *Executor) IsDryRun() bool {
	return e.dryRun
}

