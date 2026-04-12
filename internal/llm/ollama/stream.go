package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

// StreamComplete initiates a streaming inference request and returns a channel of events.
// The channel is closed after a Kind="end" or Kind="error" event is emitted.
// Callers must drain the channel or cancel the context to avoid goroutine leaks.
func (c *Client) StreamComplete(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
	params := buildParams(req)
	params["stream"] = true

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("StreamComplete: marshal params: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("StreamComplete: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("StreamComplete: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close()
		return nil, fmt.Errorf("StreamComplete: HTTP %d", httpResp.StatusCode)
	}

	ch := make(chan llm.StreamEvent, 32)

	go func() {
		defer close(ch)
		defer httpResp.Body.Close()
		if err := c.drainStream(ctx, httpResp, req.Model, ch); err != nil {
			send(ctx, ch, llm.StreamEvent{Kind: "error", Err: err})
		}
	}()

	return ch, nil
}

// drainStream reads NDJSON events from Ollama streaming response.
func (c *Client) drainStream(
	ctx context.Context,
	httpResp *http.Response,
	modelID string,
	ch chan<- llm.StreamEvent,
) error {
	scanner := bufio.NewScanner(httpResp.Body)
	var (
		model         string
		allContent    string
		inputTokens   int
		outputTokens  int
	)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event struct {
			Model   string `json:"model"`
			Done    bool   `json:"done"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			PromptEvalCount int `json:"prompt_eval_count"`
			EvalCount       int `json:"eval_count"`
		}

		if err := json.Unmarshal(line, &event); err != nil {
			return fmt.Errorf("drainStream: decode: %w", err)
		}

		if event.Model != "" {
			model = event.Model
		}

		// Emit delta if content present.
		if event.Message.Content != "" {
			send(ctx, ch, llm.StreamEvent{Kind: "delta", Delta: event.Message.Content})
			allContent += event.Message.Content
		}

		// Accumulate token counts.
		if event.PromptEvalCount > 0 {
			inputTokens = event.PromptEvalCount
		}
		if event.EvalCount > 0 {
			outputTokens = event.EvalCount
		}

		// On done, emit final response.
		if event.Done {
			var blocks []llm.ContentBlock
			blocks = append(blocks, llm.ContentBlock{
				Type: "text",
				Text: allContent,
			})

			final := &llm.Response{
				ID:               modelID,
				Model:            model,
				Provider:         "ollama",
				Content:          blocks,
				InputTokens:      inputTokens,
				OutputTokens:     outputTokens,
				CacheReadTokens:  0,
				CacheWriteTokens: 0,
				StopReason:       "end_turn",
				CostUSD:          0.0,
				Raw:              line,
			}
			send(ctx, ch, llm.StreamEvent{Kind: "end", Final: final})
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("drainStream: scan: %w", err)
	}

	return nil
}

// send emits an event on ch, respecting context cancellation.
func send(ctx context.Context, ch chan<- llm.StreamEvent, ev llm.StreamEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}
