package a2a

import (
	"testing"
)

func TestPrefersV2(t *testing.T) {
	t.Parallel()
	if prefersV2(nil) {
		t.Fatal("nil card should not prefer v2")
	}
	legacy := map[string]any{
		"name":            "WeatherAgent",
		"protocolVersion": "0.2.5",
		"url":             "http://agents:8101/a2a",
	}
	if prefersV2(legacy) {
		t.Fatal("0.2.5 card should use legacy")
	}
	takeout := map[string]any{
		"name": "TakeoutMerchantAgent",
		"supportedInterfaces": []any{
			map[string]any{
				"url":             "http://host.docker.internal:13120/a2a",
				"protocolBinding": "JSONRPC",
				"protocolVersion": "1.0",
			},
		},
		"securitySchemes": map[string]any{
			"bearer": map[string]any{},
		},
	}
	if !prefersV2(takeout) {
		t.Fatal("protocol 1.0 + securitySchemes should prefer v2")
	}
}

func TestInferTakeoutIntent(t *testing.T) {
	t.Parallel()
	cardMap := map[string]any{
		"skills": []any{
			map[string]any{"id": "takeout.merchant", "name": "外卖商家"},
		},
	}
	intent, payload, ok := inferTakeoutIntent("今天有什么菜", nil, cardMap)
	if !ok || intent != "menu.search" {
		t.Fatalf("got ok=%v intent=%q want menu.search", ok, intent)
	}
	if q, _ := payload["query"].(string); q != "" {
		t.Fatalf("browse query should be empty, got %q", q)
	}

	intent, payload, ok = inferTakeoutIntent("有没有水煮鱼（地点：北京）", nil, cardMap)
	if !ok || intent != "menu.search" {
		t.Fatalf("dish search: ok=%v intent=%q", ok, intent)
	}
	if q, _ := payload["query"].(string); q != "有没有水煮鱼" {
		t.Fatalf("payload.query=%q", q)
	}

	intent, _, ok = inferTakeoutIntent("intent=handshake", nil, cardMap)
	if !ok || intent != "handshake" {
		t.Fatalf("explicit intent: ok=%v intent=%q", ok, intent)
	}

	_, _, ok = inferTakeoutIntent("今天有什么菜", nil, map[string]any{
		"skills": []any{map[string]any{"id": "weather-skill"}},
	})
	if ok {
		t.Fatal("non-takeout card should not infer takeout intent")
	}
}

func TestLooksLikeProtocolMismatch(t *testing.T) {
	t.Parallel()
	if !looksLikeProtocolMismatch(errString("failed to send message: method not found")) {
		t.Fatal("expected method not found match")
	}
	if looksLikeProtocolMismatch(errString("connection refused")) {
		t.Fatal("connection refused is not a protocol mismatch")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
