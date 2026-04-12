package anthropic_test

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/llm/anthropic"
)

// cannedMessageResponse is a minimal Anthropic Messages API JSON response.
const cannedMessageResponse = `{
  "id": "msg_test123",
  "type": "message",
  "role": "assistant",
  "model": "claude-haiku-4-5-20251001",
  "content": [{"type": "text", "text": "Hello, world!"}],
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 1000,
    "output_tokens": 500,
    "cache_read_input_tokens": 0,
    "cache_creation_input_tokens": 0
  }
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*anthropic.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	client := anthropic.New("test-key",
		option.WithBaseURL(srv.URL),
		option.WithMaxRetries(0),
	)
	return client, srv
}

func TestName(t *testing.T) {
	c := anthropic.New("key")
	if c.Name() != "anthropic" {
		t.Errorf("Name() = %q, want %q", c.Name(), "anthropic")
	}
}

func TestModels(t *testing.T) {
	c := anthropic.New("key")
	models := c.Models()
	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}
	for _, m := range models {
		if !strings.HasPrefix(m.ID, "claude-") {
			t.Errorf("unexpected model ID %q", m.ID)
		}
	}
}

func TestComplete_MockHTTP(t *testing.T) {
	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, cannedMessageResponse)
	})
	defer srv.Close()

	req := llm.Request{
		Model:     "claude-haiku-4-5-20251001",
		System:    "You are helpful.",
		Messages:  []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "Hi"}}}},
		MaxTokens: 100,
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.ID != "msg_test123" {
		t.Errorf("ID = %q, want msg_test123", resp.ID)
	}
	if resp.Provider != "anthropic" {
		t.Errorf("Provider = %q, want anthropic", resp.Provider)
	}
	if resp.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", resp.InputTokens)
	}
	if resp.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", resp.OutputTokens)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want end_turn", resp.StopReason)
	}
	// haiku: 1000 input * 0.80/1M + 500 output * 4.00/1M = 0.0008 + 0.002 = 0.0028
	const wantCost = 0.0028
	if !approxEqual(resp.CostUSD, wantCost, 1e-10) {
		t.Errorf("CostUSD = %.10f, want %.10f", resp.CostUSD, wantCost)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text != "Hello, world!" {
		t.Errorf("unexpected content: %+v", resp.Content)
	}
	if len(resp.Raw) == 0 {
		t.Error("Raw should be populated")
	}
}

func TestComplete_WithTools(t *testing.T) {
	toolResponse := `{
  "id": "msg_tool1",
  "type": "message",
  "role": "assistant",
  "model": "claude-haiku-4-5-20251001",
  "content": [{"type": "tool_use", "id": "tu1", "name": "get_weather", "input": {"city": "NYC"}}],
  "stop_reason": "tool_use",
  "stop_sequence": null,
  "usage": {"input_tokens": 50, "output_tokens": 20, "cache_read_input_tokens": 0, "cache_creation_input_tokens": 0}
}`

	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, toolResponse)
	})
	defer srv.Close()

	req := llm.Request{
		Model: "claude-haiku-4-5-20251001",
		Messages: []llm.Message{
			{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "Weather in NYC?"}}},
		},
		MaxTokens: 200,
		Tools: []llm.Tool{{
			Name:        "get_weather",
			Description: "Get weather",
			InputSchema: []byte(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		}},
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete with tools: %v", err)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want tool_use", resp.StopReason)
	}
}

func TestComplete_ErrorResponse(t *testing.T) {
	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"type":"authentication_error","message":"bad key"}}`, http.StatusUnauthorized)
	})
	defer srv.Close()

	req := llm.Request{
		Model:     "claude-haiku-4-5-20251001",
		Messages:  []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "Hi"}}}},
		MaxTokens: 10,
	}

	_, err := client.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestStreamComplete_MockHTTP(t *testing.T) {
	sseEvents := []string{
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_s1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-haiku-4-5-20251001\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":0,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":0}}}",
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}",
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}",
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":5}}",
		"event: message_stop\ndata: {\"type\":\"message_stop\"}",
	}

	var sseBody strings.Builder
	for _, e := range sseEvents {
		sseBody.WriteString(e)
		sseBody.WriteString("\n\n")
	}

	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		bw := bufio.NewWriter(w)
		fmt.Fprint(bw, sseBody.String())
		bw.Flush()
	})
	defer srv.Close()

	req := llm.Request{
		Model:     "claude-haiku-4-5-20251001",
		Messages:  []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "Hi"}}}},
		MaxTokens: 100,
	}

	ch, err := client.StreamComplete(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamComplete: %v", err)
	}

	var deltas []string
	var endEvent *llm.StreamEvent

	for ev := range ch {
		switch ev.Kind {
		case "delta":
			deltas = append(deltas, ev.Delta)
		case "end":
			cp := ev
			endEvent = &cp
		case "error":
			t.Fatalf("stream error: %v", ev.Err)
		}
	}

	combined := strings.Join(deltas, "")
	if combined != "Hello world" {
		t.Errorf("deltas = %q, want %q", combined, "Hello world")
	}
	if endEvent == nil {
		t.Fatal("expected end event")
	}
	if endEvent.Final == nil {
		t.Fatal("end event has nil Final")
	}
	if endEvent.Final.StopReason != "end_turn" {
		t.Errorf("Final.StopReason = %q, want end_turn", endEvent.Final.StopReason)
	}
	if endEvent.Final.Provider != "anthropic" {
		t.Errorf("Final.Provider = %q, want anthropic", endEvent.Final.Provider)
	}
}

func TestStreamComplete_CancelledContext(t *testing.T) {
	// Use an already-cancelled context — the request should fail quickly.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should never be reached.
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := anthropic.New("test-key",
		option.WithBaseURL(srv.URL),
		option.WithMaxRetries(0),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before making the request

	req := llm.Request{
		Model:     "claude-haiku-4-5-20251001",
		Messages:  []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "Hi"}}}},
		MaxTokens: 10,
	}

	// With a cancelled context the channel should either not be returned (error)
	// or close immediately with an error event.
	ch, err := client.StreamComplete(ctx, req)
	if err != nil {
		// Expected: SDK returned error immediately for cancelled context.
		return
	}

	// If we got a channel, drain it — must close promptly.
	var gotError bool
	for ev := range ch {
		if ev.Err != nil || ev.Kind == "error" {
			gotError = true
		}
	}
	if !gotError {
		t.Log("stream completed without error on cancelled context (acceptable)")
	}
}

func approxEqual(a, b, epsilon float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < epsilon
}
