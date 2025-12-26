package ws

import "testing"

func TestChannel_String(t *testing.T) {
	tests := []struct {
		channel Channel
		want    string
	}{
		{ChannelOrderbookDelta, "orderbook_delta"},
		{ChannelTicker, "ticker"},
		{ChannelTrade, "trade"},
		{ChannelLifecycle, "lifecycle"},
		{ChannelFill, "fill"},
		{ChannelPositions, "positions"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.channel.String(); got != tt.want {
				t.Errorf("Channel.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChannel_RequiresAuth(t *testing.T) {
	tests := []struct {
		channel  Channel
		wantAuth bool
	}{
		{ChannelOrderbookDelta, false},
		{ChannelTicker, false},
		{ChannelTrade, false},
		{ChannelLifecycle, false},
		{ChannelFill, true},
		{ChannelPositions, true},
		{Channel("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.channel), func(t *testing.T) {
			if got := tt.channel.RequiresAuth(); got != tt.wantAuth {
				t.Errorf("Channel.RequiresAuth() = %v, want %v", got, tt.wantAuth)
			}
		})
	}
}

func TestChannel_IsValid(t *testing.T) {
	tests := []struct {
		channel Channel
		want    bool
	}{
		{ChannelOrderbookDelta, true},
		{ChannelTicker, true},
		{ChannelTrade, true},
		{ChannelLifecycle, true},
		{ChannelFill, true},
		{ChannelPositions, true},
		{Channel("unknown"), false},
		{Channel(""), false},
		{Channel("TICKER"), false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.channel), func(t *testing.T) {
			if got := tt.channel.IsValid(); got != tt.want {
				t.Errorf("Channel.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllPublicChannels(t *testing.T) {
	publicChannels := []Channel{
		ChannelOrderbookDelta,
		ChannelTicker,
		ChannelTrade,
		ChannelLifecycle,
	}

	for _, ch := range publicChannels {
		if ch.RequiresAuth() {
			t.Errorf("channel %s should not require auth", ch)
		}
		if !ch.IsValid() {
			t.Errorf("channel %s should be valid", ch)
		}
	}
}

func TestAllAuthChannels(t *testing.T) {
	authChannels := []Channel{
		ChannelFill,
		ChannelPositions,
	}

	for _, ch := range authChannels {
		if !ch.RequiresAuth() {
			t.Errorf("channel %s should require auth", ch)
		}
		if !ch.IsValid() {
			t.Errorf("channel %s should be valid", ch)
		}
	}
}
