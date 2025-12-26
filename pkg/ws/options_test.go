package ws

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %s, want %s", opts.BaseURL, DefaultBaseURL)
	}
	if opts.PingInterval != DefaultPingInterval {
		t.Errorf("PingInterval = %v, want %v", opts.PingInterval, DefaultPingInterval)
	}
	if opts.PongTimeout != DefaultPongTimeout {
		t.Errorf("PongTimeout = %v, want %v", opts.PongTimeout, DefaultPongTimeout)
	}
	if opts.ReconnectDelay != DefaultReconnectDelay {
		t.Errorf("ReconnectDelay = %v, want %v", opts.ReconnectDelay, DefaultReconnectDelay)
	}
	if opts.MaxReconnectAttempts != DefaultMaxReconnectAttempts {
		t.Errorf("MaxReconnectAttempts = %d, want %d", opts.MaxReconnectAttempts, DefaultMaxReconnectAttempts)
	}
	if !opts.AutoReconnect {
		t.Error("AutoReconnect should be true by default")
	}
}

func TestOptions_IsAuthenticated(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	tests := []struct {
		name       string
		apiKey     string
		privateKey *rsa.PrivateKey
		want       bool
	}{
		{"no credentials", "", nil, false},
		{"only api key", "key", nil, false},
		{"only private key", "", privateKey, false},
		{"both credentials", "key", privateKey, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				APIKey:     tt.apiKey,
				PrivateKey: tt.privateKey,
			}
			if got := opts.IsAuthenticated(); got != tt.want {
				t.Errorf("IsAuthenticated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOptions_WithAPIKey(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	opts := DefaultOptions().WithAPIKey("test-key", privateKey)

	if opts.APIKey != "test-key" {
		t.Errorf("APIKey = %s, want test-key", opts.APIKey)
	}
	if opts.PrivateKey != privateKey {
		t.Error("PrivateKey not set correctly")
	}
}

func TestOptions_WithBaseURL(t *testing.T) {
	opts := DefaultOptions().WithBaseURL("wss://custom.url")

	if opts.BaseURL != "wss://custom.url" {
		t.Errorf("BaseURL = %s, want wss://custom.url", opts.BaseURL)
	}
}

func TestOptions_WithAutoReconnect(t *testing.T) {
	opts := DefaultOptions().WithAutoReconnect(false, 5)

	if opts.AutoReconnect {
		t.Error("AutoReconnect should be false")
	}
	if opts.MaxReconnectAttempts != 5 {
		t.Errorf("MaxReconnectAttempts = %d, want 5", opts.MaxReconnectAttempts)
	}
}

func TestWithAPIKeyOption(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	opts := DefaultOptions()
	WithAPIKeyOption("test-key", privateKey)(&opts)

	if opts.APIKey != "test-key" {
		t.Errorf("APIKey = %s, want test-key", opts.APIKey)
	}
}

func TestWithBaseURLOption(t *testing.T) {
	opts := DefaultOptions()
	WithBaseURLOption("wss://custom.url")(&opts)

	if opts.BaseURL != "wss://custom.url" {
		t.Errorf("BaseURL = %s, want wss://custom.url", opts.BaseURL)
	}
}

func TestWithAutoReconnectOption(t *testing.T) {
	opts := DefaultOptions()
	WithAutoReconnectOption(false, 3)(&opts)

	if opts.AutoReconnect {
		t.Error("AutoReconnect should be false")
	}
	if opts.MaxReconnectAttempts != 3 {
		t.Errorf("MaxReconnectAttempts = %d, want 3", opts.MaxReconnectAttempts)
	}
}

func TestWithPingIntervalOption(t *testing.T) {
	opts := DefaultOptions()
	WithPingIntervalOption(5 * time.Second)(&opts)

	if opts.PingInterval != 5*time.Second {
		t.Errorf("PingInterval = %v, want 5s", opts.PingInterval)
	}
}

func TestWithCallbacks(t *testing.T) {
	connectCalled := false
	disconnectCalled := false
	errorCalled := false

	opts := DefaultOptions()
	WithCallbacks(
		func() { connectCalled = true },
		func(error) { disconnectCalled = true },
		func(error) { errorCalled = true },
	)(&opts)

	if opts.OnConnect == nil {
		t.Error("OnConnect should be set")
	}
	if opts.OnDisconnect == nil {
		t.Error("OnDisconnect should be set")
	}
	if opts.OnError == nil {
		t.Error("OnError should be set")
	}

	// Call callbacks to verify they work.
	opts.OnConnect()
	opts.OnDisconnect(nil)
	opts.OnError(nil)

	if !connectCalled {
		t.Error("OnConnect was not called")
	}
	if !disconnectCalled {
		t.Error("OnDisconnect was not called")
	}
	if !errorCalled {
		t.Error("OnError was not called")
	}
}

func TestNew_WithOptions(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	client := New(
		WithAPIKeyOption("test-key", privateKey),
		WithBaseURLOption("wss://custom.url"),
		WithAutoReconnectOption(false, 5),
	)

	if client.opts.APIKey != "test-key" {
		t.Errorf("APIKey = %s, want test-key", client.opts.APIKey)
	}
	if client.opts.BaseURL != "wss://custom.url" {
		t.Errorf("BaseURL = %s, want wss://custom.url", client.opts.BaseURL)
	}
	if client.opts.AutoReconnect {
		t.Error("AutoReconnect should be false")
	}
}

func TestNewWithOptions(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	opts := Options{
		APIKey:     "test-key",
		PrivateKey: privateKey,
		BaseURL:    "wss://custom.url",
	}

	client := NewWithOptions(opts)

	if client.opts.APIKey != "test-key" {
		t.Errorf("APIKey = %s, want test-key", client.opts.APIKey)
	}
}
