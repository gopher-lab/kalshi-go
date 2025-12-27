package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DiscordNotifier sends notifications to Discord
type DiscordNotifier struct {
	webhookURL string
	httpClient *http.Client
	enabled    bool
}

// DiscordMessage represents a Discord webhook message
type DiscordMessage struct {
	Content string         `json:"content,omitempty"`
	Embeds  []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents a Discord embed
type DiscordEmbed struct {
	Title       string               `json:"title,omitempty"`
	Description string               `json:"description,omitempty"`
	Color       int                  `json:"color,omitempty"`
	Fields      []DiscordEmbedField  `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter  `json:"footer,omitempty"`
	Timestamp   string               `json:"timestamp,omitempty"`
}

// DiscordEmbedField represents an embed field
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// DiscordEmbedFooter represents an embed footer
type DiscordEmbedFooter struct {
	Text string `json:"text"`
}

// NewDiscordNotifier creates a new Discord notifier
func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	return &DiscordNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		enabled:    webhookURL != "",
	}
}

// IsEnabled returns true if Discord notifications are enabled
func (d *DiscordNotifier) IsEnabled() bool {
	return d.enabled
}

// Send sends a simple text message
func (d *DiscordNotifier) Send(text string) error {
	if !d.enabled {
		return nil
	}

	msg := DiscordMessage{Content: text}
	return d.sendMessage(msg)
}

// SendTradeAlert sends a trade execution alert
func (d *DiscordNotifier) SendTradeAlert(city, bracket, side string, price int, quantity int, cost float64, orderID string) error {
	if !d.enabled {
		return nil
	}

	emoji := "📈"
	color := 0x36a64f // green
	if side == "no" {
		emoji = "📉"
		color = 0x3498db // blue
	}

	msg := DiscordMessage{
		Embeds: []DiscordEmbed{
			{
				Title: fmt.Sprintf("%s Trade Executed: %s", emoji, city),
				Color: color,
				Fields: []DiscordEmbedField{
					{Name: "Bracket", Value: bracket, Inline: true},
					{Name: "Side", Value: side, Inline: true},
					{Name: "Price", Value: fmt.Sprintf("%d¢", price), Inline: true},
					{Name: "Quantity", Value: fmt.Sprintf("%d", quantity), Inline: true},
					{Name: "Cost", Value: fmt.Sprintf("$%.2f", cost), Inline: true},
					{Name: "Order ID", Value: orderID, Inline: true},
				},
				Footer:    &DiscordEmbedFooter{Text: "Trading Bot"},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}

	return d.sendMessage(msg)
}

// SendDailySummary sends the daily P&L summary
func (d *DiscordNotifier) SendDailySummary(trades, wins int, totalCost, totalProfit, netPnL, winRate float64) error {
	if !d.enabled {
		return nil
	}

	emoji := "📊"
	color := 0x36a64f // green
	if netPnL < 0 {
		color = 0xe74c3c // red
		emoji = "⚠️"
	}

	msg := DiscordMessage{
		Embeds: []DiscordEmbed{
			{
				Title: fmt.Sprintf("%s Daily Trading Summary", emoji),
				Color: color,
				Fields: []DiscordEmbedField{
					{Name: "Total Trades", Value: fmt.Sprintf("%d", trades), Inline: true},
					{Name: "Wins", Value: fmt.Sprintf("%d", wins), Inline: true},
					{Name: "Win Rate", Value: fmt.Sprintf("%.1f%%", winRate), Inline: true},
					{Name: "Total Cost", Value: fmt.Sprintf("$%.2f", totalCost), Inline: true},
					{Name: "Total Profit", Value: fmt.Sprintf("$%.2f", totalProfit), Inline: true},
					{Name: "Net P&L", Value: fmt.Sprintf("$%.2f", netPnL), Inline: true},
				},
				Footer:    &DiscordEmbedFooter{Text: "Trading Bot - Daily Summary"},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}

	return d.sendMessage(msg)
}

// SendError sends an error alert
func (d *DiscordNotifier) SendError(component, message string) error {
	if !d.enabled {
		return nil
	}

	msg := DiscordMessage{
		Embeds: []DiscordEmbed{
			{
				Title: "🚨 Error Alert",
				Color: 0xe74c3c, // red
				Fields: []DiscordEmbedField{
					{Name: "Component", Value: component, Inline: true},
					{Name: "Message", Value: message, Inline: false},
				},
				Footer:    &DiscordEmbedFooter{Text: "Trading Bot - Error"},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}

	return d.sendMessage(msg)
}

// SendStartup sends a startup notification
func (d *DiscordNotifier) SendStartup(balance float64, config string) error {
	if !d.enabled {
		return nil
	}

	msg := DiscordMessage{
		Embeds: []DiscordEmbed{
			{
				Title: "🚀 Trading Bot Started",
				Color: 0x36a64f, // green
				Fields: []DiscordEmbedField{
					{Name: "Balance", Value: fmt.Sprintf("$%.2f", balance), Inline: true},
					{Name: "Config", Value: config, Inline: false},
				},
				Footer:    &DiscordEmbedFooter{Text: "Trading Bot - Startup"},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}

	return d.sendMessage(msg)
}

// SendShutdown sends a shutdown notification
func (d *DiscordNotifier) SendShutdown(reason string, stats map[string]interface{}) error {
	if !d.enabled {
		return nil
	}

	fields := []DiscordEmbedField{
		{Name: "Reason", Value: reason, Inline: false},
	}

	for k, v := range stats {
		fields = append(fields, DiscordEmbedField{
			Name:   k,
			Value:  fmt.Sprintf("%v", v),
			Inline: true,
		})
	}

	msg := DiscordMessage{
		Embeds: []DiscordEmbed{
			{
				Title:     "⏹️ Trading Bot Shutdown",
				Color:     0x95a5a6, // gray
				Fields:    fields,
				Footer:    &DiscordEmbedFooter{Text: "Trading Bot - Shutdown"},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}

	return d.sendMessage(msg)
}

func (d *DiscordNotifier) sendMessage(msg DiscordMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	resp, err := d.httpClient.Post(d.webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}

	return nil
}

