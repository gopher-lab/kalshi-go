package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackNotifier sends notifications to Slack
type SlackNotifier struct {
	webhookURL string
	httpClient *http.Client
	enabled    bool
}

// SlackMessage represents a Slack message payload
type SlackMessage struct {
	Text        string       `json:"text,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment represents a Slack attachment
type Attachment struct {
	Color      string  `json:"color,omitempty"`
	Title      string  `json:"title,omitempty"`
	Text       string  `json:"text,omitempty"`
	Footer     string  `json:"footer,omitempty"`
	Fields     []Field `json:"fields,omitempty"`
	Timestamp  int64   `json:"ts,omitempty"`
}

// Field represents an attachment field
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		enabled:    webhookURL != "",
	}
}

// IsEnabled returns true if Slack notifications are enabled
func (s *SlackNotifier) IsEnabled() bool {
	return s.enabled
}

// Send sends a simple text message
func (s *SlackNotifier) Send(text string) error {
	if !s.enabled {
		return nil
	}

	msg := SlackMessage{Text: text}
	return s.sendMessage(msg)
}

// SendTradeAlert sends a trade execution alert
func (s *SlackNotifier) SendTradeAlert(city, bracket, side string, price int, quantity int, cost float64, orderID string) error {
	if !s.enabled {
		return nil
	}

	emoji := "📈"
	color := "#36a64f" // green
	if side == "no" {
		emoji = "📉"
		color = "#3498db" // blue
	}

	msg := SlackMessage{
		Attachments: []Attachment{
			{
				Color: color,
				Title: fmt.Sprintf("%s Trade Executed: %s", emoji, city),
				Fields: []Field{
					{Title: "Bracket", Value: bracket, Short: true},
					{Title: "Side", Value: side, Short: true},
					{Title: "Price", Value: fmt.Sprintf("%d¢", price), Short: true},
					{Title: "Quantity", Value: fmt.Sprintf("%d", quantity), Short: true},
					{Title: "Cost", Value: fmt.Sprintf("$%.2f", cost), Short: true},
					{Title: "Order ID", Value: orderID, Short: true},
				},
				Footer:    "Trading Bot",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return s.sendMessage(msg)
}

// SendDailySummary sends the daily P&L summary
func (s *SlackNotifier) SendDailySummary(trades, wins int, totalCost, totalProfit, netPnL, winRate float64) error {
	if !s.enabled {
		return nil
	}

	emoji := "📊"
	color := "#36a64f" // green
	if netPnL < 0 {
		color = "#e74c3c" // red
		emoji = "⚠️"
	}

	msg := SlackMessage{
		Attachments: []Attachment{
			{
				Color: color,
				Title: fmt.Sprintf("%s Daily Trading Summary", emoji),
				Fields: []Field{
					{Title: "Total Trades", Value: fmt.Sprintf("%d", trades), Short: true},
					{Title: "Wins", Value: fmt.Sprintf("%d", wins), Short: true},
					{Title: "Win Rate", Value: fmt.Sprintf("%.1f%%", winRate), Short: true},
					{Title: "Total Cost", Value: fmt.Sprintf("$%.2f", totalCost), Short: true},
					{Title: "Total Profit", Value: fmt.Sprintf("$%.2f", totalProfit), Short: true},
					{Title: "Net P&L", Value: fmt.Sprintf("$%.2f", netPnL), Short: true},
				},
				Footer:    "Trading Bot - Daily Summary",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return s.sendMessage(msg)
}

// SendError sends an error alert
func (s *SlackNotifier) SendError(component, message string) error {
	if !s.enabled {
		return nil
	}

	msg := SlackMessage{
		Attachments: []Attachment{
			{
				Color: "#e74c3c", // red
				Title: "🚨 Error Alert",
				Fields: []Field{
					{Title: "Component", Value: component, Short: true},
					{Title: "Message", Value: message, Short: false},
				},
				Footer:    "Trading Bot - Error",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return s.sendMessage(msg)
}

// SendStartup sends a startup notification
func (s *SlackNotifier) SendStartup(balance float64, config string) error {
	if !s.enabled {
		return nil
	}

	msg := SlackMessage{
		Attachments: []Attachment{
			{
				Color: "#36a64f", // green
				Title: "🚀 Trading Bot Started",
				Fields: []Field{
					{Title: "Balance", Value: fmt.Sprintf("$%.2f", balance), Short: true},
					{Title: "Config", Value: config, Short: false},
				},
				Footer:    "Trading Bot - Startup",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return s.sendMessage(msg)
}

// SendShutdown sends a shutdown notification
func (s *SlackNotifier) SendShutdown(reason string, stats map[string]interface{}) error {
	if !s.enabled {
		return nil
	}

	fields := []Field{
		{Title: "Reason", Value: reason, Short: false},
	}

	for k, v := range stats {
		fields = append(fields, Field{
			Title: k,
			Value: fmt.Sprintf("%v", v),
			Short: true,
		})
	}

	msg := SlackMessage{
		Attachments: []Attachment{
			{
				Color:     "#95a5a6", // gray
				Title:     "⏹️ Trading Bot Shutdown",
				Fields:    fields,
				Footer:    "Trading Bot - Shutdown",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return s.sendMessage(msg)
}

func (s *SlackNotifier) sendMessage(msg SlackMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	resp, err := s.httpClient.Post(s.webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	return nil
}

