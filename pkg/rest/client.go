// Package rest provides a REST API client for the Kalshi trading platform.
package rest

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/brendanplayford/kalshi-go/pkg/ws"
)

const (
	// ProdBaseURL is the production API base URL.
	ProdBaseURL = "https://api.elections.kalshi.com/trade-api/v2"

	// DemoBaseURL is the demo/sandbox API base URL.
	DemoBaseURL = "https://demo-api.kalshi.co/trade-api/v2"
)

// Client is a REST API client for Kalshi.
type Client struct {
	baseURL    string
	apiKey     string
	privateKey *rsa.PrivateKey
	httpClient *http.Client
	debug      bool
}

// Option configures the client.
type Option func(*Client)

// WithBaseURL sets a custom base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithDemo configures the client to use the demo environment.
func WithDemo() Option {
	return func(c *Client) {
		c.baseURL = DemoBaseURL
	}
}

// WithDebug enables debug logging.
func WithDebug() Option {
	return func(c *Client) {
		c.debug = true
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// New creates a new REST API client.
func New(apiKey string, privateKey *rsa.PrivateKey, opts ...Option) *Client {
	c := &Client{
		baseURL:    ProdBaseURL,
		apiKey:     apiKey,
		privateKey: privateKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// request makes an authenticated API request.
func (c *Client) request(method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add authentication headers
	// The signature must include the full path: /trade-api/v2/...
	fullPath := "/trade-api/v2" + path
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signature, err := ws.GenerateSignature(c.privateKey, timestamp, method, fullPath)
	if err != nil {
		return nil, fmt.Errorf("generate signature: %w", err)
	}

	req.Header.Set("KALSHI-ACCESS-KEY", c.apiKey)
	req.Header.Set("KALSHI-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("KALSHI-ACCESS-SIGNATURE", signature)

	if c.debug {
		fmt.Printf("[DEBUG] %s %s\n", method, url)
		fmt.Printf("[DEBUG] Sign path: %s\n", fullPath)
		fmt.Printf("[DEBUG] Headers: KEY=%s, TS=%s\n", c.apiKey, timestamp)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if c.debug {
		fmt.Printf("[DEBUG] Response: %d %s\n", resp.StatusCode, string(respBody))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Code:       errResp.Error.Code,
				Message:    errResp.Error.Message,
			}
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
	}

	return respBody, nil
}

// Get makes a GET request.
func (c *Client) Get(path string) ([]byte, error) {
	return c.request("GET", path, nil)
}

// Post makes a POST request.
func (c *Client) Post(path string, body any) ([]byte, error) {
	return c.request("POST", path, body)
}

// Delete makes a DELETE request.
func (c *Client) Delete(path string) ([]byte, error) {
	return c.request("DELETE", path, nil)
}

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// APIError represents an API error.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("kalshi api error %d: [%s] %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("kalshi api error %d: %s", e.StatusCode, e.Message)
}
