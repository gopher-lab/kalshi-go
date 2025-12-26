// Package ws provides a WebSocket client for the Kalshi trading API.
package ws

// Channel represents a WebSocket subscription channel.
type Channel string

// Available WebSocket channels.
const (
	// Public channels (no authentication required)
	ChannelOrderbookDelta Channel = "orderbook_delta"
	ChannelTicker         Channel = "ticker"
	ChannelTrade          Channel = "trade"
	ChannelLifecycle      Channel = "lifecycle"

	// Authenticated channels (require API key)
	ChannelFill      Channel = "fill"
	ChannelPositions Channel = "positions"
)

// String returns the string representation of the channel.
func (c Channel) String() string {
	return string(c)
}

// RequiresAuth returns true if the channel requires authentication.
func (c Channel) RequiresAuth() bool {
	switch c {
	case ChannelFill, ChannelPositions:
		return true
	default:
		return false
	}
}

// IsValid returns true if the channel is a known valid channel.
func (c Channel) IsValid() bool {
	switch c {
	case ChannelOrderbookDelta, ChannelTicker, ChannelTrade,
		ChannelLifecycle, ChannelFill, ChannelPositions:
		return true
	default:
		return false
	}
}
