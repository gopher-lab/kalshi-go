package ws

import (
	"encoding/json"
	"testing"
)

func TestParseResponse_Subscribed(t *testing.T) {
	data := []byte(`{
		"id": 1,
		"type": "subscribed",
		"msg": {
			"channel": "orderbook_delta",
			"sid": 123
		}
	}`)

	resp, err := ParseResponse(data)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.ID != 1 {
		t.Errorf("ID = %d, want 1", resp.ID)
	}
	if resp.Type != MessageTypeSubscribed {
		t.Errorf("Type = %s, want %s", resp.Type, MessageTypeSubscribed)
	}
}

func TestParseResponse_Unsubscribed(t *testing.T) {
	data := []byte(`{
		"sid": 2,
		"type": "unsubscribed"
	}`)

	resp, err := ParseResponse(data)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.SID != 2 {
		t.Errorf("SID = %d, want 2", resp.SID)
	}
	if resp.Type != MessageTypeUnsubscribed {
		t.Errorf("Type = %s, want %s", resp.Type, MessageTypeUnsubscribed)
	}
}

func TestParseResponse_OK(t *testing.T) {
	data := []byte(`{
		"id": 123,
		"sid": 456,
		"seq": 222,
		"type": "ok",
		"market_tickers": ["MARKET-1", "MARKET-2"]
	}`)

	resp, err := ParseResponse(data)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.ID != 123 {
		t.Errorf("ID = %d, want 123", resp.ID)
	}
	if resp.SID != 456 {
		t.Errorf("SID = %d, want 456", resp.SID)
	}
	if resp.Seq != 222 {
		t.Errorf("Seq = %d, want 222", resp.Seq)
	}
	if resp.Type != MessageTypeOK {
		t.Errorf("Type = %s, want %s", resp.Type, MessageTypeOK)
	}
}

func TestParseResponse_Error(t *testing.T) {
	data := []byte(`{
		"id": 123,
		"type": "error",
		"msg": {
			"code": 6,
			"msg": "Already subscribed"
		}
	}`)

	resp, err := ParseResponse(data)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.Type != MessageTypeError {
		t.Errorf("Type = %s, want %s", resp.Type, MessageTypeError)
	}

	errMsg, err := ParseErrorMsg(resp.Msg)
	if err != nil {
		t.Fatalf("ParseErrorMsg failed: %v", err)
	}

	if errMsg.Code != 6 {
		t.Errorf("Code = %d, want 6", errMsg.Code)
	}
	if errMsg.Msg != "Already subscribed" {
		t.Errorf("Msg = %s, want 'Already subscribed'", errMsg.Msg)
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	data := []byte(`not valid json`)

	_, err := ParseResponse(data)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseSubscribedMsg(t *testing.T) {
	msg := map[string]any{
		"channel": "ticker",
		"sid":     float64(42), // JSON numbers are float64
	}

	result, err := ParseSubscribedMsg(msg)
	if err != nil {
		t.Fatalf("ParseSubscribedMsg failed: %v", err)
	}

	if result.Channel != ChannelTicker {
		t.Errorf("Channel = %s, want %s", result.Channel, ChannelTicker)
	}
	if result.SID != 42 {
		t.Errorf("SID = %d, want 42", result.SID)
	}
}

func TestParseErrorMsg(t *testing.T) {
	msg := map[string]any{
		"code": float64(404),
		"msg":  "Not found",
	}

	result, err := ParseErrorMsg(msg)
	if err != nil {
		t.Fatalf("ParseErrorMsg failed: %v", err)
	}

	if result.Code != 404 {
		t.Errorf("Code = %d, want 404", result.Code)
	}
	if result.Msg != "Not found" {
		t.Errorf("Msg = %s, want 'Not found'", result.Msg)
	}
}

func TestSubscribeParams_JSON(t *testing.T) {
	params := SubscribeParams{
		Channels:     []Channel{ChannelOrderbookDelta, ChannelTicker},
		MarketTicker: "TEST-MARKET",
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `{"channels":["orderbook_delta","ticker"],"market_ticker":"TEST-MARKET"}`
	if string(data) != expected {
		t.Errorf("JSON = %s, want %s", string(data), expected)
	}
}

func TestUnsubscribeParams_JSON(t *testing.T) {
	params := UnsubscribeParams{
		SIDs: []int64{1, 2, 3},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `{"sids":[1,2,3]}`
	if string(data) != expected {
		t.Errorf("JSON = %s, want %s", string(data), expected)
	}
}

func TestUpdateSubscriptionParams_JSON(t *testing.T) {
	params := UpdateSubscriptionParams{
		SIDs:          []int64{456},
		MarketTickers: []string{"MARKET-1", "MARKET-2"},
		Action:        ActionAddMarkets,
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify the JSON contains expected fields.
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result["action"] != "add_markets" {
		t.Errorf("action = %v, want add_markets", result["action"])
	}
}

func TestRequest_JSON(t *testing.T) {
	params := SubscribeParams{
		Channels:     []Channel{ChannelTicker},
		MarketTicker: "TEST",
	}
	paramsData, _ := json.Marshal(params)

	req := Request{
		ID:     1,
		Cmd:    CommandSubscribe,
		Params: paramsData,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result["id"] != float64(1) {
		t.Errorf("id = %v, want 1", result["id"])
	}
	if result["cmd"] != "subscribe" {
		t.Errorf("cmd = %v, want subscribe", result["cmd"])
	}
}

