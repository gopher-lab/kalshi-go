package notify

import (
	"log"
)

// Notifier provides a unified interface for notifications
type Notifier struct {
	slack   *SlackNotifier
	discord *DiscordNotifier
}

// NewNotifier creates a new unified notifier
func NewNotifier(slackWebhookURL, discordWebhookURL string) *Notifier {
	n := &Notifier{
		slack:   NewSlackNotifier(slackWebhookURL),
		discord: NewDiscordNotifier(discordWebhookURL),
	}

	if n.slack.IsEnabled() {
		log.Println("[Notify] Slack notifications enabled")
	}
	if n.discord.IsEnabled() {
		log.Println("[Notify] Discord notifications enabled")
	}

	return n
}

// IsEnabled returns true if any notification channel is enabled
func (n *Notifier) IsEnabled() bool {
	return n.slack.IsEnabled() || n.discord.IsEnabled()
}

// Send sends a simple text message to all channels
func (n *Notifier) Send(text string) {
	if n.slack.IsEnabled() {
		if err := n.slack.Send(text); err != nil {
			log.Printf("[Notify] Slack error: %v", err)
		}
	}
	if n.discord.IsEnabled() {
		if err := n.discord.Send(text); err != nil {
			log.Printf("[Notify] Discord error: %v", err)
		}
	}
}

// TradeAlert sends a trade execution alert
func (n *Notifier) TradeAlert(city, bracket, side string, price int, quantity int, cost float64, orderID string) {
	if n.slack.IsEnabled() {
		if err := n.slack.SendTradeAlert(city, bracket, side, price, quantity, cost, orderID); err != nil {
			log.Printf("[Notify] Slack error: %v", err)
		}
	}
	if n.discord.IsEnabled() {
		if err := n.discord.SendTradeAlert(city, bracket, side, price, quantity, cost, orderID); err != nil {
			log.Printf("[Notify] Discord error: %v", err)
		}
	}
}

// DailySummary sends the daily P&L summary
func (n *Notifier) DailySummary(trades, wins int, totalCost, totalProfit, netPnL, winRate float64) {
	if n.slack.IsEnabled() {
		if err := n.slack.SendDailySummary(trades, wins, totalCost, totalProfit, netPnL, winRate); err != nil {
			log.Printf("[Notify] Slack error: %v", err)
		}
	}
	if n.discord.IsEnabled() {
		if err := n.discord.SendDailySummary(trades, wins, totalCost, totalProfit, netPnL, winRate); err != nil {
			log.Printf("[Notify] Discord error: %v", err)
		}
	}
}

// Error sends an error alert
func (n *Notifier) Error(component, message string) {
	if n.slack.IsEnabled() {
		if err := n.slack.SendError(component, message); err != nil {
			log.Printf("[Notify] Slack error: %v", err)
		}
	}
	if n.discord.IsEnabled() {
		if err := n.discord.SendError(component, message); err != nil {
			log.Printf("[Notify] Discord error: %v", err)
		}
	}
}

// Startup sends a startup notification
func (n *Notifier) Startup(balance float64, config string) {
	if n.slack.IsEnabled() {
		if err := n.slack.SendStartup(balance, config); err != nil {
			log.Printf("[Notify] Slack error: %v", err)
		}
	}
	if n.discord.IsEnabled() {
		if err := n.discord.SendStartup(balance, config); err != nil {
			log.Printf("[Notify] Discord error: %v", err)
		}
	}
}

// Shutdown sends a shutdown notification
func (n *Notifier) Shutdown(reason string, stats map[string]interface{}) {
	if n.slack.IsEnabled() {
		if err := n.slack.SendShutdown(reason, stats); err != nil {
			log.Printf("[Notify] Slack error: %v", err)
		}
	}
	if n.discord.IsEnabled() {
		if err := n.discord.SendShutdown(reason, stats); err != nil {
			log.Printf("[Notify] Discord error: %v", err)
		}
	}
}

