package ws

import "encoding/json"

// MessageType represents the type of WebSocket message.
type MessageType string

// Message types received from the server.
const (
	MessageTypeSubscribed   MessageType = "subscribed"
	MessageTypeUnsubscribed MessageType = "unsubscribed"
	MessageTypeOK           MessageType = "ok"
	MessageTypeError        MessageType = "error"
	MessageTypeData         MessageType = "data"
)

// Command represents a WebSocket command.
type Command string

// Available commands.
const (
	CommandSubscribe          Command = "subscribe"
	CommandUnsubscribe        Command = "unsubscribe"
	CommandListSubscriptions  Command = "list_subscriptions"
	CommandUpdateSubscription Command = "update_subscription"
)

// UpdateAction represents an action for update_subscription.
type UpdateAction string

// Update actions.
const (
	ActionAddMarkets    UpdateAction = "add_markets"
	ActionDeleteMarkets UpdateAction = "delete_markets"
)

// Request represents a WebSocket request message.
type Request struct {
	ID     int64           `json:"id"`
	Cmd    Command         `json:"cmd"`
	Params json.RawMessage `json:"params,omitempty"`
}

// SubscribeParams represents parameters for a subscribe command.
type SubscribeParams struct {
	Channels     []Channel `json:"channels"`
	MarketTicker string    `json:"market_ticker,omitempty"`
}

// UnsubscribeParams represents parameters for an unsubscribe command.
type UnsubscribeParams struct {
	SIDs []int64 `json:"sids"`
}

// UpdateSubscriptionParams represents parameters for update_subscription.
type UpdateSubscriptionParams struct {
	SIDs          []int64      `json:"sids,omitempty"`
	SID           int64        `json:"sid,omitempty"`
	MarketTickers []string     `json:"market_tickers"`
	Action        UpdateAction `json:"action"`
}

// Response represents a generic WebSocket response.
type Response struct {
	ID   int64       `json:"id,omitempty"`
	SID  int64       `json:"sid,omitempty"`
	Seq  int64       `json:"seq,omitempty"`
	Type MessageType `json:"type"`
	Msg  any         `json:"msg,omitempty"`
}

// SubscribedMsg represents the message payload for a subscribed response.
type SubscribedMsg struct {
	Channel Channel `json:"channel"`
	SID     int64   `json:"sid"`
}

// ErrorMsg represents the message payload for an error response.
type ErrorMsg struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// Subscription represents an active subscription.
type Subscription struct {
	Channel Channel `json:"channel"`
	SID     int64   `json:"sid"`
}

// ListSubscriptionsResponse represents a list_subscriptions response.
type ListSubscriptionsResponse struct {
	ID            int64          `json:"id"`
	Type          MessageType    `json:"type"`
	Subscriptions []Subscription `json:"subscriptions"`
}

// OKResponse represents a successful operation response.
type OKResponse struct {
	ID            int64       `json:"id"`
	SID           int64       `json:"sid,omitempty"`
	Seq           int64       `json:"seq,omitempty"`
	Type          MessageType `json:"type"`
	MarketTickers []string    `json:"market_tickers,omitempty"`
}

// DataMessage represents a data message from a subscription.
type DataMessage struct {
	SID  int64           `json:"sid"`
	Seq  int64           `json:"seq"`
	Type MessageType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

// ParseResponse attempts to parse a raw message into a Response.
func ParseResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ParseSubscribedMsg parses the Msg field of a subscribed response.
func ParseSubscribedMsg(msg any) (*SubscribedMsg, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var result SubscribedMsg
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ParseErrorMsg parses the Msg field of an error response.
func ParseErrorMsg(msg any) (*ErrorMsg, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var result ErrorMsg
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

