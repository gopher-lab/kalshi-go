package rest

import (
	"encoding/json"
	"fmt"
)

// Side represents the order side.
type Side string

const (
	SideYes Side = "yes"
	SideNo  Side = "no"
)

// OrderType represents the order type.
type OrderType string

const (
	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"
)

// OrderAction represents the order action.
type OrderAction string

const (
	OrderActionBuy  OrderAction = "buy"
	OrderActionSell OrderAction = "sell"
)

// OrderStatus represents the order status.
type OrderStatus string

const (
	OrderStatusResting  OrderStatus = "resting"
	OrderStatusCanceled OrderStatus = "canceled"
	OrderStatusExecuted OrderStatus = "executed"
	OrderStatusPending  OrderStatus = "pending"
)

// CreateOrderRequest represents a request to create an order.
type CreateOrderRequest struct {
	Ticker          string      `json:"ticker"`
	Action          OrderAction `json:"action"`
	Side            Side        `json:"side"`
	Type            OrderType   `json:"type"`
	Count           int         `json:"count"`
	YesPrice        int         `json:"yes_price,omitempty"`         // In cents (1-99)
	NoPrice         int         `json:"no_price,omitempty"`          // In cents (1-99)
	ClientOrderID   string      `json:"client_order_id,omitempty"`
	Expiration      string      `json:"expiration_ts,omitempty"`     // RFC3339 timestamp
	SellPositionCap int         `json:"sell_position_floor,omitempty"`
	BuyMaxCost      int         `json:"buy_max_cost,omitempty"`      // Max cost in cents
}

// Order represents an order.
type Order struct {
	OrderID             string      `json:"order_id"`
	Ticker              string      `json:"ticker"`
	Action              OrderAction `json:"action"`
	Side                Side        `json:"side"`
	Type                OrderType   `json:"type"`
	Status              OrderStatus `json:"status"`
	YesPrice            int         `json:"yes_price"`
	NoPrice             int         `json:"no_price"`
	CreatedTime         string      `json:"created_time"`
	ExpirationTime      string      `json:"expiration_time"`
	ClientOrderID       string      `json:"client_order_id"`
	OrderGroupID        string      `json:"order_group_id"`
	PlaceCount          int         `json:"place_count"`
	DecreaseCount       int         `json:"decrease_count"`
	QueuePosition       int         `json:"queue_position"`
	RemainingCount      int         `json:"remaining_count"`
	TakerFillCount      int         `json:"taker_fill_count"`
	MakerFillCount      int         `json:"maker_fill_count"`
	TakerFillCost       int         `json:"taker_fill_cost"`
	MakerFillCost       int         `json:"maker_fill_cost"`
	LastUpdateTime      string      `json:"last_update_time"`
	Ticker2             string      `json:"ticker_2,omitempty"`
}

// CreateOrderResponse represents a response from creating an order.
type CreateOrderResponse struct {
	Order Order `json:"order"`
}

// GetOrdersResponse represents a response from getting orders.
type GetOrdersResponse struct {
	Orders []Order `json:"orders"`
	Cursor string  `json:"cursor"`
}

// CancelOrderResponse represents a response from canceling an order.
type CancelOrderResponse struct {
	Order          Order `json:"order"`
	ReducedBy      int   `json:"reduced_by"`
}

// CreateOrder places a new order.
func (c *Client) CreateOrder(req *CreateOrderRequest) (*Order, error) {
	data, err := c.Post("/portfolio/orders", req)
	if err != nil {
		return nil, err
	}

	var resp CreateOrderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp.Order, nil
}

// GetOrder retrieves an order by ID.
func (c *Client) GetOrder(orderID string) (*Order, error) {
	data, err := c.Get(fmt.Sprintf("/portfolio/orders/%s", orderID))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Order Order `json:"order"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp.Order, nil
}

// GetOrders retrieves all orders for a ticker.
func (c *Client) GetOrders(ticker string, status OrderStatus) ([]Order, error) {
	path := "/portfolio/orders"
	if ticker != "" {
		path += "?ticker=" + ticker
		if status != "" {
			path += "&status=" + string(status)
		}
	} else if status != "" {
		path += "?status=" + string(status)
	}

	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	var resp GetOrdersResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp.Orders, nil
}

// CancelOrder cancels an order.
func (c *Client) CancelOrder(orderID string) (*Order, error) {
	data, err := c.Delete(fmt.Sprintf("/portfolio/orders/%s", orderID))
	if err != nil {
		return nil, err
	}

	var resp CancelOrderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp.Order, nil
}

// BuyYes is a convenience function to buy YES contracts.
func (c *Client) BuyYes(ticker string, count int, maxPriceCents int) (*Order, error) {
	return c.CreateOrder(&CreateOrderRequest{
		Ticker:   ticker,
		Action:   OrderActionBuy,
		Side:     SideYes,
		Type:     OrderTypeLimit,
		Count:    count,
		YesPrice: maxPriceCents,
	})
}

// BuyNo is a convenience function to buy NO contracts.
func (c *Client) BuyNo(ticker string, count int, maxPriceCents int) (*Order, error) {
	return c.CreateOrder(&CreateOrderRequest{
		Ticker:   ticker,
		Action:   OrderActionBuy,
		Side:     SideNo,
		Type:     OrderTypeLimit,
		Count:    count,
		NoPrice:  maxPriceCents,
	})
}

// SellYes is a convenience function to sell YES contracts.
func (c *Client) SellYes(ticker string, count int, minPriceCents int) (*Order, error) {
	return c.CreateOrder(&CreateOrderRequest{
		Ticker:   ticker,
		Action:   OrderActionSell,
		Side:     SideYes,
		Type:     OrderTypeLimit,
		Count:    count,
		YesPrice: minPriceCents,
	})
}

// SellNo is a convenience function to sell NO contracts.
func (c *Client) SellNo(ticker string, count int, minPriceCents int) (*Order, error) {
	return c.CreateOrder(&CreateOrderRequest{
		Ticker:   ticker,
		Action:   OrderActionSell,
		Side:     SideNo,
		Type:     OrderTypeLimit,
		Count:    count,
		NoPrice:  minPriceCents,
	})
}

