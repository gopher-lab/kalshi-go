package ws

import (
	"crypto/rsa"
	"net/http"
	"time"
)

const (
	// DefaultBaseURL is the default Kalshi WebSocket endpoint.
	DefaultBaseURL = "wss://api.elections.kalshi.com/trade-api/ws/v2"

	// DefaultPingInterval is the default interval for sending ping frames.
	DefaultPingInterval = 10 * time.Second

	// DefaultPongTimeout is the default timeout for receiving pong responses.
	DefaultPongTimeout = 30 * time.Second

	// DefaultReconnectDelay is the default delay before attempting to reconnect.
	DefaultReconnectDelay = 5 * time.Second

	// DefaultMaxReconnectAttempts is the default maximum number of reconnection attempts.
	DefaultMaxReconnectAttempts = 10
)

// Options configures the WebSocket client.
type Options struct {
	// BaseURL is the WebSocket server URL.
	BaseURL string

	// APIKey is the API key ID for authenticated connections.
	// Leave empty for unauthenticated connections.
	APIKey string

	// PrivateKey is the parsed RSA private key for signing requests.
	// Required for authenticated connections along with APIKey.
	PrivateKey *rsa.PrivateKey

	// Headers are additional HTTP headers to include in the handshake.
	Headers http.Header

	// PingInterval is the interval for sending ping frames.
	PingInterval time.Duration

	// PongTimeout is the timeout for receiving pong responses.
	PongTimeout time.Duration

	// ReconnectDelay is the delay before attempting to reconnect.
	ReconnectDelay time.Duration

	// MaxReconnectAttempts is the maximum number of reconnection attempts.
	// Set to 0 to disable auto-reconnect, -1 for unlimited attempts.
	MaxReconnectAttempts int

	// AutoReconnect enables automatic reconnection on disconnect.
	AutoReconnect bool

	// OnConnect is called when the connection is established.
	OnConnect func()

	// OnDisconnect is called when the connection is lost.
	OnDisconnect func(err error)

	// OnError is called when an error occurs.
	OnError func(err error)
}

// DefaultOptions returns Options with default values.
func DefaultOptions() Options {
	return Options{
		BaseURL:              DefaultBaseURL,
		PingInterval:         DefaultPingInterval,
		PongTimeout:          DefaultPongTimeout,
		ReconnectDelay:       DefaultReconnectDelay,
		MaxReconnectAttempts: DefaultMaxReconnectAttempts,
		AutoReconnect:        true,
	}
}

// WithAPIKey returns a copy of Options with the API key and private key set.
func (o Options) WithAPIKey(apiKey string, privateKey *rsa.PrivateKey) Options {
	o.APIKey = apiKey
	o.PrivateKey = privateKey
	return o
}

// WithBaseURL returns a copy of Options with the base URL set.
func (o Options) WithBaseURL(url string) Options {
	o.BaseURL = url
	return o
}

// WithAutoReconnect returns a copy of Options with auto-reconnect configured.
func (o Options) WithAutoReconnect(enabled bool, maxAttempts int) Options {
	o.AutoReconnect = enabled
	o.MaxReconnectAttempts = maxAttempts
	return o
}

// IsAuthenticated returns true if API credentials are configured.
func (o Options) IsAuthenticated() bool {
	return o.APIKey != "" && o.PrivateKey != nil
}

// Option is a functional option for configuring the client.
type Option func(*Options)

// WithAPIKeyOption returns an Option that sets the API key and private key.
func WithAPIKeyOption(apiKey string, privateKey *rsa.PrivateKey) Option {
	return func(o *Options) {
		o.APIKey = apiKey
		o.PrivateKey = privateKey
	}
}

// WithBaseURLOption returns an Option that sets the base URL.
func WithBaseURLOption(url string) Option {
	return func(o *Options) {
		o.BaseURL = url
	}
}

// WithAutoReconnectOption returns an Option that configures auto-reconnect.
func WithAutoReconnectOption(enabled bool, maxAttempts int) Option {
	return func(o *Options) {
		o.AutoReconnect = enabled
		o.MaxReconnectAttempts = maxAttempts
	}
}

// WithPingIntervalOption returns an Option that sets the ping interval.
func WithPingIntervalOption(interval time.Duration) Option {
	return func(o *Options) {
		o.PingInterval = interval
	}
}

// WithCallbacks returns an Option that sets the callback functions.
func WithCallbacks(onConnect func(), onDisconnect func(error), onError func(error)) Option {
	return func(o *Options) {
		o.OnConnect = onConnect
		o.OnDisconnect = onDisconnect
		o.OnError = onError
	}
}
