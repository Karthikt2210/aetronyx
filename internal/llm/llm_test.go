package llm_test

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
	anthropicadapter "github.com/karthikcodes/aetronyx/internal/llm/anthropic"
)

// ---------------------------------------------------------------------------
// Pricing tests
// ---------------------------------------------------------------------------

func TestComputeCost(t *testing.T) {
	// haiku: Input=0.80, Output=4.00 per 1M tokens
	// 1000 input tokens → 0.80 * 1000 / 1_000_000 = 0.0008
	// 500 output tokens → 4.00 * 500 / 1_000_000 = 0.002
	// total = 0.0028
	cost, err := llm.ComputeCost("claude-haiku-4-5-20251001", 1000, 500, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const want = 0.0028
	if !approxEqual(cost, want, 1e-10) {
		t.Errorf("cost = %.10f, want %.10f", cost, want)
	}
}

func TestComputeCostCacheTokens(t *testing.T) {
	// sonnet: Input=3.00, Output=15.00, CacheRead=0.30, CacheWrite=3.75 per 1M
	// 0 input, 0 output, 1000 cacheRead, 500 cacheWrite
	// = 0.30*1000/1e6 + 3.75*500/1e6 = 0.0003 + 0.001875 = 0.002175
	cost, err := llm.ComputeCost("claude-sonnet-4-6", 0, 0, 1000, 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const want = 0.002175
	if !approxEqual(cost, want, 1e-10) {
		t.Errorf("cost = %.10f, want %.10f", cost, want)
	}
}

func TestComputeCostUnknownModel(t *testing.T) {
	_, err := llm.ComputeCost("gpt-99-ultra", 1000, 500, 0, 0)
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

// ---------------------------------------------------------------------------
// Error classification tests
// ---------------------------------------------------------------------------

func TestClassifyHTTPError(t *testing.T) {
	tests := []struct {
		code      int
		wantCode  string
		retryable bool
	}{
		{401, llm.ErrAuthentication, false},
		{403, llm.ErrAuthentication, false},
		{429, llm.ErrRateLimit, true},
		{400, llm.ErrBadRequest, false},
		{422, llm.ErrBadRequest, false},
		{500, llm.ErrProviderUnavailable, true},
		{503, llm.ErrProviderUnavailable, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("HTTP_%d", tt.code), func(t *testing.T) {
			e := llm.ClassifyHTTPError(tt.code, []byte("error body"))
			if e.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", e.Code, tt.wantCode)
			}
			if e.Retryable != tt.retryable {
				t.Errorf("Retryable = %v, want %v", e.Retryable, tt.retryable)
			}
			if e.StatusHTTP != tt.code {
				t.Errorf("StatusHTTP = %d, want %d", e.StatusHTTP, tt.code)
			}
			if e.Unwrap() == nil {
				t.Error("expected non-nil inner error")
			}
		})
	}
}

func TestProviderErrorString(t *testing.T) {
	e := &llm.ProviderError{Code: llm.ErrRateLimit, StatusHTTP: 429, Retryable: true}
	s := e.Error()
	if !strings.Contains(s, "provider.rate_limit") {
		t.Errorf("Error() does not contain code: %s", s)
	}
}

// ---------------------------------------------------------------------------
// Anthropic Complete — mock HTTP server
// ---------------------------------------------------------------------------

// cannedMessageResponse returns a JSON response matching the Anthropic Messages API.
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

func TestAnthropicComplete_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, cannedMessageResponse)
	}))
	defer srv.Close()

	client := anthropicadapter.New("test-key",
		option.WithBaseURL(srv.URL),
		option.WithMaxRetries(0),
	)

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
		t.Errorf("ID = %q, want %q", resp.ID, "msg_test123")
	}
	if resp.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", resp.Provider, "anthropic")
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
	// Cost: haiku 1000 input + 500 output = 0.0028 USD
	const wantCost = 0.0028
	if !approxEqual(resp.CostUSD, wantCost, 1e-10) {
		t.Errorf("CostUSD = %.10f, want %.10f", resp.CostUSD, wantCost)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text != "Hello, world!" {
		t.Errorf("unexpected content: %+v", resp.Content)
	}
	if len(resp.Raw) == 0 {
		t.Error("Raw field should be populated")
	}
}

func TestAnthropicName(t *testing.T) {
	client := anthropicadapter.New("test")
	if client.Name() != "anthropic" {
		t.Errorf("Name() = %q, want %q", client.Name(), "anthropic")
	}
}

func TestAnthropicModels(t *testing.T) {
	client := anthropicadapter.New("test")
	models := client.Models()
	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}
	for _, m := range models {
		if !strings.HasPrefix(m.ID, "claude-") {
			t.Errorf("unexpected model ID %q", m.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Anthropic StreamComplete — mock SSE server
// ---------------------------------------------------------------------------

// buildSSEResponse constructs a text/event-stream body with the given events.
func buildSSEResponse(events []string) string {
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString(e)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func TestAnthropicStream_MockHTTP(t *testing.T) {
	sseBody := buildSSEResponse([]string{
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_stream1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-haiku-4-5-20251001\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":0,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":0}}}",
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}",
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}",
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":5}}",
		"event: message_stop\ndata: {\"type\":\"message_stop\"}",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		bw := bufio.NewWriter(w)
		fmt.Fprint(bw, sseBody)
		bw.Flush()
	}))
	defer srv.Close()

	client := anthropicadapter.New("test-key",
		option.WithBaseURL(srv.URL),
		option.WithMaxRetries(0),
	)

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
		if ev.Err != nil {
			t.Fatalf("stream error: %v", ev.Err)
		}
		switch ev.Kind {
		case "delta":
			deltas = append(deltas, ev.Delta)
		case "end":
			cp := ev
			endEvent = &cp
		case "error":
			t.Fatalf("stream error event: %v", ev.Err)
		}
	}

	if len(deltas) < 2 {
		t.Errorf("expected at least 2 delta events, got %d: %v", len(deltas), deltas)
	}
	combined := strings.Join(deltas, "")
	if combined != "Hello world" {
		t.Errorf("combined deltas = %q, want %q", combined, "Hello world")
	}

	if endEvent == nil {
		t.Fatal("expected end event, got none")
	}
	if endEvent.Final == nil {
		t.Fatal("end event has nil Final")
	}
	if endEvent.Final.StopReason != "end_turn" {
		t.Errorf("Final.StopReason = %q, want end_turn", endEvent.Final.StopReason)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func approxEqual(a, b, epsilon float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < epsilon
}
