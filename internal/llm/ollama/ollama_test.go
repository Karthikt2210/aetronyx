package ollama_test

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/llm/ollama"
)

const cannedChatResponse = `{
  "model": "llama2",
  "created_at": "2023-12-12T02:50:37.660608Z",
  "message": {
    "role": "assistant",
    "content": "This is a test response."
  },
  "done": true,
  "total_duration": 5000000000,
  "load_duration": 100000000,
  "prompt_eval_count": 100,
  "eval_count": 50
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*ollama.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	client := ollama.New(srv.URL)
	return client, srv
}

func TestName(t *testing.T) {
	c := ollama.New("http://localhost:11434")
	if c.Name() != "ollama" {
		t.Errorf("Name() = %q, want %q", c.Name(), "ollama")
	}
}

func TestOllamaComplete_MockHTTP(t *testing.T) {
	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, cannedChatResponse)
	})
	defer srv.Close()

	req := llm.Request{
		Model:     "llama2",
		System:    "You are helpful.",
		Messages:  []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "Hi"}}}},
		MaxTokens: 100,
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Provider != "ollama" {
		t.Errorf("Provider = %q, want ollama", resp.Provider)
	}
	if resp.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", resp.InputTokens)
	}
	if resp.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", resp.OutputTokens)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want end_turn", resp.StopReason)
	}
	// Ollama local model cost is always zero.
	if resp.CostUSD != 0.0 {
		t.Errorf("CostUSD = %.10f, want 0.0", resp.CostUSD)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text != "This is a test response." {
		t.Errorf("unexpected content: %+v", resp.Content)
	}
	if len(resp.Raw) == 0 {
		t.Error("Raw should be populated")
	}
}

func TestOllamaModels_MockHTTP(t *testing.T) {
	tagsResponse := `{
  "models": [
    {"name": "llama2", "size": 3825922038},
    {"name": "mistral", "size": 4109056640}
  ]
}`

	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, tagsResponse)
		}
	})
	defer srv.Close()

	models := client.Models()
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}

	for _, m := range models {
		if m.InputPer1MUSD != 0.0 || m.OutputPer1MUSD != 0.0 {
			t.Errorf("local model %q should have zero pricing", m.ID)
		}
		if !m.SupportsStreaming {
			t.Errorf("model %q should support streaming", m.ID)
		}
	}
}

func TestOllamaStream_MockHTTP(t *testing.T) {
	ndjsonEvents := []string{
		`{"model":"llama2","done":false,"message":{"content":"Hello"}}`,
		`{"model":"llama2","done":false,"message":{"content":" world"}}`,
		`{"model":"llama2","done":true,"message":{"content":""},"prompt_eval_count":50,"eval_count":25}`,
	}

	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		bw := bufio.NewWriter(w)
		for _, e := range ndjsonEvents {
			bw.WriteString(e)
			bw.WriteString("\n")
		}
		bw.Flush()
	})
	defer srv.Close()

	req := llm.Request{
		Model:     "llama2",
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
	if endEvent.Final.Provider != "ollama" {
		t.Errorf("Final.Provider = %q, want ollama", endEvent.Final.Provider)
	}
	if endEvent.Final.InputTokens != 50 {
		t.Errorf("Final.InputTokens = %d, want 50", endEvent.Final.InputTokens)
	}
	if endEvent.Final.OutputTokens != 25 {
		t.Errorf("Final.OutputTokens = %d, want 25", endEvent.Final.OutputTokens)
	}
}

func TestOllamaToolFallback_JSON(t *testing.T) {
	// Ollama response with tool invocation in JSON code fence.
	toolResponse := "{\"model\":\"llama2\",\"message\":{\"role\":\"assistant\",\"content\":\"I'll help you.\\n\\n" +
		"```json\\n{\\n  \\\"name\\\": \\\"get_weather\\\",\\n  \\\"input\\\": {\\\"city\\\": \\\"NYC\\\"}\\n}\\n```" +
		"\"},\"done\":true,\"prompt_eval_count\":50,\"eval_count\":20}"

	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, toolResponse)
	})
	defer srv.Close()

	req := llm.Request{
		Model:    "llama2",
		Messages: []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "What's the weather in NYC?"}}}},
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want tool_use", resp.StopReason)
	}

	found := false
	for _, block := range resp.Content {
		if block.Type == "tool_use" && block.ToolUse != nil {
			if block.ToolUse.Name == "get_weather" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("expected tool_use block for get_weather, got: %+v", resp.Content)
	}
}

func TestOllamaToolFallback_NoMatch(t *testing.T) {
	// Response without JSON code fence should be treated as plain text.
	textResponse := `{
  "model": "llama2",
  "message": {
    "role": "assistant",
    "content": "I can't help with that."
  },
  "done": true,
  "prompt_eval_count": 50,
  "eval_count": 10
}`

	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, textResponse)
	})
	defer srv.Close()

	req := llm.Request{
		Model:    "llama2",
		Messages: []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "Hi"}}}},
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want end_turn", resp.StopReason)
	}

	if len(resp.Content) == 0 || resp.Content[0].Type != "text" {
		t.Errorf("expected text block, got: %+v", resp.Content)
	}
}
