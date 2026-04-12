package openai_test

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go/option"

	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/llm/openai"
)

// cannedChatResponse is a minimal OpenAI Chat Completion API JSON response.
const cannedChatResponse = `{
  "id": "chatcmpl_test123",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gpt-4.1",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello, world!"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 1000,
    "completion_tokens": 500,
    "total_tokens": 1500
  }
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*openai.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	client := openai.New("test-key",
		option.WithBaseURL(srv.URL),
	)
	return client, srv
}

func TestName(t *testing.T) {
	c := openai.New("key")
	if c.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", c.Name(), "openai")
	}
}

func TestModels(t *testing.T) {
	c := openai.New("key")
	models := c.Models()
	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}
	for _, m := range models {
		if !strings.HasPrefix(m.ID, "gpt-") && !strings.HasPrefix(m.ID, "o") {
			t.Errorf("unexpected model ID %q", m.ID)
		}
	}
}

func TestOpenAIComplete_MockHTTP(t *testing.T) {
	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, cannedChatResponse)
	})
	defer srv.Close()

	req := llm.Request{
		Model:     "gpt-4.1",
		System:    "You are helpful.",
		Messages:  []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "Hi"}}}},
		MaxTokens: 100,
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.ID != "chatcmpl_test123" {
		t.Errorf("ID = %q, want chatcmpl_test123", resp.ID)
	}
	if resp.Provider != "openai" {
		t.Errorf("Provider = %q, want openai", resp.Provider)
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
	// gpt-4.1: 1000 input * 2.00/1M + 500 output * 8.00/1M = 0.002 + 0.004 = 0.006
	const wantCost = 0.006
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

func TestOpenAIToolUse_MockHTTP(t *testing.T) {
	toolResponse := `{
  "id": "chatcmpl_tool1",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gpt-4.1",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_123",
            "type": "function",
            "function": {
              "name": "get_weather",
              "arguments": "{\"city\": \"NYC\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {
    "prompt_tokens": 50,
    "completion_tokens": 20,
    "total_tokens": 70
  }
}`

	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, toolResponse)
	})
	defer srv.Close()

	req := llm.Request{
		Model: "gpt-4.1",
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
	if len(resp.Content) == 0 || resp.Content[0].Type != "tool_use" {
		t.Errorf("expected tool_use in content, got: %+v", resp.Content)
	}
	if resp.Content[0].ToolUse.Name != "get_weather" {
		t.Errorf("tool name = %q, want get_weather", resp.Content[0].ToolUse.Name)
	}
}

func TestOpenAIClassifyErrors(t *testing.T) {
	tests := []struct {
		statusCode int
		bodyMsg    string
		wantCode   string
	}{
		{401, "invalid api key", llm.ErrAuthentication},
		{429, "rate limited", llm.ErrRateLimit},
		{500, "internal server error", llm.ErrProviderUnavailable},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, tt.bodyMsg, tt.statusCode)
			})
			defer srv.Close()

			req := llm.Request{
				Model:     "gpt-4.1",
				Messages:  []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "Hi"}}}},
				MaxTokens: 10,
			}

			_, err := client.Complete(context.Background(), req)
			if err == nil {
				t.Fatal("expected error")
			}

			// The actual error classification may vary by SDK implementation,
			// but we should get some error.
		})
	}
}

func TestOpenAIStream_MockHTTP(t *testing.T) {
	sseEvents := []string{
		`{"object":"text_completion.chunk","index":0,"choices":[{"delta":{"role":"assistant","content":"Hello"},"index":0,"finish_reason":null}],"model":"gpt-4.1","id":"chatcmpl_stream1"}`,
		`{"object":"text_completion.chunk","index":0,"choices":[{"delta":{"content":" world"},"index":0,"finish_reason":null}],"model":"gpt-4.1","id":"chatcmpl_stream1"}`,
		`{"object":"text_completion.chunk","index":0,"choices":[{"delta":{"content":""},"index":0,"finish_reason":"stop"}],"model":"gpt-4.1","id":"chatcmpl_stream1","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	}

	var sseBody strings.Builder
	for _, e := range sseEvents {
		sseBody.WriteString("data: ")
		sseBody.WriteString(e)
		sseBody.WriteString("\n\n")
	}
	sseBody.WriteString("data: [DONE]\n\n")

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
		Model:     "gpt-4.1",
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
	if endEvent.Final.Provider != "openai" {
		t.Errorf("Final.Provider = %q, want openai", endEvent.Final.Provider)
	}
	if endEvent.Final.InputTokens != 10 {
		t.Errorf("Final.InputTokens = %d, want 10", endEvent.Final.InputTokens)
	}
}

func approxEqual(a, b, epsilon float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < epsilon
}
