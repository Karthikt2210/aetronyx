package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

// StreamComplete initiates a streaming inference request and returns a channel of events.
// The channel is closed after a Kind="end" or Kind="error" event is emitted.
// Callers must drain the channel or cancel the context to avoid goroutine leaks.
func (c *Client) StreamComplete(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
	params, err := buildParams(req)
	if err != nil {
		return nil, fmt.Errorf("StreamComplete build params: %w", err)
	}

	stream := c.sdk.Messages.NewStreaming(ctx, params)
	if stream == nil {
		return nil, fmt.Errorf("StreamComplete: nil stream returned by SDK")
	}

	ch := make(chan llm.StreamEvent, 32)

	go func() {
		defer close(ch)
		if err := c.drainStream(ctx, stream, req.Model, ch); err != nil {
			send(ctx, ch, llm.StreamEvent{Kind: "error", Err: err})
		}
	}()

	return ch, nil
}

// drainStream reads events from the SDK stream and emits them on ch.
func (c *Client) drainStream(
	ctx context.Context,
	stream interface {
		Next() bool
		Current() anthropicsdk.MessageStreamEventUnion
		Err() error
		Close() error
	},
	modelID string,
	ch chan<- llm.StreamEvent,
) error {
	defer stream.Close()

	// Accumulate state for the final Response.
	var (
		msgID        string
		msgModel     string
		stopReason   string
		inputTokens  int
		outputTokens int
		cacheRead    int
		cacheWrite   int
		allContent   []llm.ContentBlock
	)

	for stream.Next() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ev := stream.Current()
		switch ev.Type {
		case "message_start":
			ms := ev.AsMessageStart()
			msgID = ms.Message.ID
			msgModel = string(ms.Message.Model)
			inputTokens = int(ms.Message.Usage.InputTokens)
			cacheRead = int(ms.Message.Usage.CacheReadInputTokens)
			cacheWrite = int(ms.Message.Usage.CacheCreationInputTokens)

		case "content_block_delta":
			cbd := ev.AsContentBlockDelta()
			delta := cbd.Delta
			switch delta.Type {
			case "text_delta":
				text := delta.AsTextDelta().Text
				send(ctx, ch, llm.StreamEvent{Kind: "delta", Delta: text})
				// Accumulate for final response.
				if len(allContent) == 0 || allContent[len(allContent)-1].Type != "text" {
					allContent = append(allContent, llm.ContentBlock{Type: "text"})
				}
				allContent[len(allContent)-1].Text += text

			case "input_json_delta":
				jsonDelta := delta.AsInputJSONDelta().PartialJSON
				send(ctx, ch, llm.StreamEvent{
					Kind: "tool_call_delta",
					Tool: &llm.ToolCallEvent{Delta: jsonDelta},
				})
			}

		case "message_delta":
			md := ev.AsMessageDelta()
			stopReason = string(md.Delta.StopReason)
			outputTokens = int(md.Usage.OutputTokens)

		case "message_stop":
			// Build the final response.
			cost, _ := llm.ComputeCost(modelID, inputTokens, outputTokens, cacheRead, cacheWrite)
			rawBytes, _ := json.Marshal(map[string]any{
				"id":            msgID,
				"model":         msgModel,
				"stop_reason":   stopReason,
				"input_tokens":  inputTokens,
				"output_tokens": outputTokens,
			})
			final := &llm.Response{
				ID:               msgID,
				Model:            msgModel,
				Provider:         "anthropic",
				Content:          allContent,
				InputTokens:      inputTokens,
				OutputTokens:     outputTokens,
				CacheReadTokens:  cacheRead,
				CacheWriteTokens: cacheWrite,
				StopReason:       stopReason,
				CostUSD:          cost,
				Raw:              rawBytes,
			}
			send(ctx, ch, llm.StreamEvent{Kind: "end", Final: final})
			return nil
		}
	}

	if err := stream.Err(); err != nil {
		return err
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
