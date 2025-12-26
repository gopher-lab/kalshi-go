package ws

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
)

func TestClient_New(t *testing.T) {
	client := New()

	if client == nil {
		t.Fatal("New() returned nil")
	}
	if client.opts.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %s, want %s", client.opts.BaseURL, DefaultBaseURL)
	}
}

func TestClient_IsConnected_WhenNotConnected(t *testing.T) {
	client := New()

	if client.IsConnected() {
		t.Error("IsConnected() should return false when not connected")
	}
}

func TestClient_Subscribe_NotConnected(t *testing.T) {
	client := New()

	_, err := client.Subscribe(context.Background(), "TEST-MARKET", ChannelTicker)
	if err != ErrNotConnected {
		t.Errorf("Subscribe() error = %v, want ErrNotConnected", err)
	}
}

func TestClient_Subscribe_InvalidChannel(t *testing.T) {
	// Create a mock connected client by setting conn to non-nil
	// This is a bit hacky but allows testing validation without network.
	client := New()

	_, err := client.Subscribe(context.Background(), "TEST", Channel("invalid"))
	if err != ErrInvalidChannel {
		t.Errorf("Subscribe() error = %v, want ErrInvalidChannel", err)
	}
}

func TestClient_Subscribe_AuthRequired(t *testing.T) {
	client := New() // No auth configured

	_, err := client.Subscribe(context.Background(), "TEST", ChannelFill)
	if err != ErrAuthRequired {
		t.Errorf("Subscribe() error = %v, want ErrAuthRequired", err)
	}
}

func TestClient_Subscribe_AuthChannel_WithAuth(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	client := New(WithAPIKeyOption("key", privateKey))

	// Should fail with ErrNotConnected, not ErrAuthRequired.
	_, err := client.Subscribe(context.Background(), "TEST", ChannelFill)
	if err != ErrNotConnected {
		t.Errorf("Subscribe() error = %v, want ErrNotConnected", err)
	}
}

func TestClient_Unsubscribe_NotConnected(t *testing.T) {
	client := New()

	_, err := client.Unsubscribe(context.Background(), 1, 2, 3)
	if err != ErrNotConnected {
		t.Errorf("Unsubscribe() error = %v, want ErrNotConnected", err)
	}
}

func TestClient_ListSubscriptions_NotConnected(t *testing.T) {
	client := New()

	_, err := client.ListSubscriptions(context.Background())
	if err != ErrNotConnected {
		t.Errorf("ListSubscriptions() error = %v, want ErrNotConnected", err)
	}
}

func TestClient_AddMarkets_NotConnected(t *testing.T) {
	client := New()

	_, err := client.AddMarkets(context.Background(), []int64{1}, []string{"MARKET"})
	if err != ErrNotConnected {
		t.Errorf("AddMarkets() error = %v, want ErrNotConnected", err)
	}
}

func TestClient_RemoveMarkets_NotConnected(t *testing.T) {
	client := New()

	_, err := client.RemoveMarkets(context.Background(), []int64{1}, []string{"MARKET"})
	if err != ErrNotConnected {
		t.Errorf("RemoveMarkets() error = %v, want ErrNotConnected", err)
	}
}

func TestClient_Close_WhenNotConnected(t *testing.T) {
	client := New()

	err := client.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestClient_SetMessageHandler(t *testing.T) {
	client := New()

	client.SetMessageHandler(func(msg *Response) {
		// Handler would be called on message receive.
		_ = msg
	})

	// Handler is set but we can't easily test it without a connection.
	if client.handler == nil {
		t.Error("handler should be set")
	}
}

func TestClient_SetDataHandler(t *testing.T) {
	client := New()

	client.SetDataHandler(func(sid int64, data json.RawMessage) {
		// Handler set.
	})

	if client.dataHandler == nil {
		t.Error("dataHandler should be set")
	}
}

func TestClient_GetActiveSubscriptions_Empty(t *testing.T) {
	client := New()

	subs := client.GetActiveSubscriptions()
	if len(subs) != 0 {
		t.Errorf("GetActiveSubscriptions() len = %d, want 0", len(subs))
	}
}

func TestClient_Connect_AlreadyConnected(t *testing.T) {
	// This test would require mocking the websocket connection.
	// For now, we just test that the error type exists.
	if ErrAlreadyConnected.Error() != "websocket: already connected" {
		t.Errorf("ErrAlreadyConnected = %v", ErrAlreadyConnected)
	}
}

func TestErrors(t *testing.T) {
	errors := []struct {
		err  error
		want string
	}{
		{ErrNotConnected, "websocket: not connected"},
		{ErrAlreadyConnected, "websocket: already connected"},
		{ErrAuthRequired, "websocket: authentication required for this channel"},
		{ErrInvalidChannel, "websocket: invalid channel"},
		{ErrConnectionClosed, "websocket: connection closed"},
	}

	for _, tt := range errors {
		t.Run(tt.want, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("error = %v, want %v", tt.err.Error(), tt.want)
			}
		})
	}
}
