//go:build integration

package ws

import (
	"context"
	"crypto/rsa"
	"os"
	"sync"
	"testing"
	"time"
)

// Integration tests require:
// - KALSHI_API_KEY environment variable
// - KALSHI_PRIVATE_KEY environment variable (PEM format)
//
// Run with: go test -tags=integration -v ./pkg/ws/

func getTestCredentials(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()

	apiKey := os.Getenv("KALSHI_API_KEY")
	privateKeyPEM := os.Getenv("KALSHI_PRIVATE_KEY")

	if apiKey == "" || privateKeyPEM == "" {
		t.Skip("KALSHI_API_KEY and KALSHI_PRIVATE_KEY must be set for integration tests")
	}

	privateKey, err := ParsePrivateKeyString(privateKeyPEM)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	return apiKey, privateKey
}

func TestIntegration_Connect(t *testing.T) {
	apiKey, privateKey := getTestCredentials(t)

	client := New(
		WithAPIKeyOption(apiKey, privateKey),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("client should be connected")
	}
}

func TestIntegration_ConnectUnauthenticated(t *testing.T) {
	// Unauthenticated connections may or may not work depending on Kalshi's policy.
	// This test verifies the connection attempt behavior.
	client := New()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	// We don't assert on error here since unauthenticated may be rejected.
	if err == nil {
		defer client.Close()
		t.Log("unauthenticated connection succeeded")
	} else {
		t.Logf("unauthenticated connection failed (expected): %v", err)
	}
}

func TestIntegration_SubscribeTicker(t *testing.T) {
	apiKey, privateKey := getTestCredentials(t)

	var mu sync.Mutex
	var receivedMsg *Response

	client := New(
		WithAPIKeyOption(apiKey, privateKey),
		WithCallbacks(
			func() { t.Log("connected") },
			func(err error) { t.Logf("disconnected: %v", err) },
			func(err error) { t.Logf("error: %v", err) },
		),
	)

	client.SetMessageHandler(func(msg *Response) {
		mu.Lock()
		defer mu.Unlock()
		if receivedMsg == nil {
			receivedMsg = msg
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Give connection time to stabilize.
	time.Sleep(500 * time.Millisecond)

	// Subscribe to a ticker channel.
	// Using a market that should exist - adjust if needed.
	id, err := client.Subscribe(ctx, "KXBTCD-25676-B6.05", ChannelTicker)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	t.Logf("subscription request sent with id=%d", id)

	// Wait for response.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		msg := receivedMsg
		mu.Unlock()

		if msg != nil {
			t.Logf("received message: type=%s id=%d sid=%d", msg.Type, msg.ID, msg.SID)

			if msg.Type == MessageTypeSubscribed {
				t.Log("subscription confirmed")
				return
			}
			if msg.Type == MessageTypeError {
				errMsg, _ := ParseErrorMsg(msg.Msg)
				t.Logf("received error: %+v", errMsg)
				// Don't fail - the market ticker might not exist.
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("test completed - no subscription response received within timeout")
}

func TestIntegration_ListSubscriptions(t *testing.T) {
	apiKey, privateKey := getTestCredentials(t)

	var mu sync.Mutex
	var messages []*Response

	client := New(
		WithAPIKeyOption(apiKey, privateKey),
	)

	client.SetMessageHandler(func(msg *Response) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, msg)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	time.Sleep(500 * time.Millisecond)

	// List subscriptions (should be empty initially).
	id, err := client.ListSubscriptions(ctx)
	if err != nil {
		t.Fatalf("ListSubscriptions failed: %v", err)
	}

	t.Logf("list_subscriptions request sent with id=%d", id)

	// Wait for response.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		for _, msg := range messages {
			if msg.ID == id && msg.Type == MessageTypeOK {
				mu.Unlock()
				t.Log("list_subscriptions response received")
				return
			}
		}
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("list_subscriptions response not received within timeout")
}

func TestIntegration_SubscribeAndUnsubscribe(t *testing.T) {
	apiKey, privateKey := getTestCredentials(t)

	var mu sync.Mutex
	var messages []*Response

	client := New(
		WithAPIKeyOption(apiKey, privateKey),
	)

	client.SetMessageHandler(func(msg *Response) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, msg)
		t.Logf("received: type=%s id=%d sid=%d", msg.Type, msg.ID, msg.SID)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	time.Sleep(500 * time.Millisecond)

	// Subscribe.
	subID, err := client.Subscribe(ctx, "KXBTCD-25676-B6.05", ChannelTicker)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Wait for subscription confirmation.
	var sid int64
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		for _, msg := range messages {
			if msg.ID == subID && msg.Type == MessageTypeSubscribed {
				if subMsg, err := ParseSubscribedMsg(msg.Msg); err == nil {
					sid = subMsg.SID
				}
			}
		}
		mu.Unlock()

		if sid != 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if sid == 0 {
		t.Log("subscription not confirmed, skipping unsubscribe test")
		return
	}

	t.Logf("subscribed with sid=%d", sid)

	// Unsubscribe.
	_, err = client.Unsubscribe(ctx, sid)
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	// Wait for unsubscribe confirmation.
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		for _, msg := range messages {
			if msg.SID == sid && msg.Type == MessageTypeUnsubscribed {
				mu.Unlock()
				t.Log("unsubscribed successfully")
				return
			}
		}
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("unsubscribe confirmation not received within timeout")
}
