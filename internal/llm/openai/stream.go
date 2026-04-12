package openai

import (
	"context"
	"encoding/json"
	"fmt"

	openaisdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/ssestream"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

// StreamComplete initiates a streaming inference request and returns a channel of events.
// The channel is closed after a Kind="end" or Kind="error" event is emitted.
// Callers must drain the channel or cancel the context to avoid goroutine leaks.
func (c *Client) StreamComplete(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
	params := buildParams(req)

	stream := c.sdk.Chat.Completions.NewStreaming(ctx, params)
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

// drainStream reads events from the OpenAI stream and emits them on ch.
func (c *Client) drainStream(
	ctx context.Context,
	stream *ssestream.Stream[openaisdk.ChatCompletionChunk],
	modelID string,
	ch chan<- llm.StreamEvent,
) error {
	defer stream.Close()

	// Accumulate state for the final Response.
	var (
		id            string
		model         string
		stopReason    string
		inputTokens   int
		outputTokens  int
		allContent    []llm.ContentBlock
		toolCallStack map[string]*llm.ToolUseBlock // indexed by tool call ID
	)
	toolCallStack = make(map[string]*llm.ToolUseBlock)

	for stream.Next() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		event := stream.Current()

		if event.ID != "" {
			id = event.ID
		}
		if event.Model != "" {
			model = event.Model
		}

		// Process choices.
		for _, choice := range event.Choices {
			delta := choice.Delta

			// Accumulate text content.
			if delta.Content != "" {
				send(ctx, ch, llm.StreamEvent{Kind: "delta", Delta: delta.Content})
				// Accumulate for final response.
				if len(allContent) == 0 || allContent[len(allContent)-1].Type != "text" {
					allContent = append(allContent, llm.ContentBlock{Type: "text"})
				}
				allContent[len(allContent)-1].Text += delta.Content
			}

			// Handle tool calls.
			for _, toolCall := range delta.ToolCalls {
				if toolCall.ID != "" {
					// New tool call.
					if _, exists := toolCallStack[toolCall.ID]; !exists {
						toolCallStack[toolCall.ID] = &llm.ToolUseBlock{
							ID:    toolCall.ID,
							Name:  toolCall.Function.Name,
							Input: json.RawMessage("{}"),
						}
					}
				}

				// Accumulate arguments.
				if toolCall.Function.Arguments != "" {
					send(ctx, ch, llm.StreamEvent{
						Kind: "tool_call_delta",
						Tool: &llm.ToolCallEvent{
							ID:    toolCall.ID,
							Name:  toolCall.Function.Name,
							Delta: toolCall.Function.Arguments,
						},
					})

					// Store the accumulated argument for final response.
					if tu, ok := toolCallStack[toolCall.ID]; ok {
						tu.Input = json.RawMessage(toolCall.Function.Arguments)
					}
				}
			}

			// Capture finish reason.
			if choice.FinishReason != "" {
				switch choice.FinishReason {
				case "stop":
					stopReason = "end_turn"
				case "tool_calls":
					stopReason = "tool_use"
				case "length":
					stopReason = "max_tokens"
				default:
					stopReason = choice.FinishReason
				}
			}
		}

		// Handle usage - OpenAI SDK returns zero values if no usage in chunk
		if event.Usage.PromptTokens > 0 || event.Usage.CompletionTokens > 0 {
			inputTokens = int(event.Usage.PromptTokens)
			outputTokens = int(event.Usage.CompletionTokens)
		}
	}

	// Build tool use blocks from accumulated stack.
	for _, tu := range toolCallStack {
		allContent = append(allContent, llm.ContentBlock{
			Type:    "tool_use",
			ToolUse: tu,
		})
	}

	// Emit final response.
	cost, _ := llm.ComputeCost(modelID, inputTokens, outputTokens, 0, 0)
	rawBytes, _ := json.Marshal(map[string]any{
		"id":          id,
		"model":       model,
		"stop_reason": stopReason,
		"usage": map[string]int{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
		},
	})

	final := &llm.Response{
		ID:               id,
		Model:            model,
		Provider:         "openai",
		Content:          allContent,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		CacheReadTokens:  0,
		CacheWriteTokens: 0,
		StopReason:       stopReason,
		CostUSD:          cost,
		Raw:              rawBytes,
	}

	send(ctx, ch, llm.StreamEvent{Kind: "end", Final: final})
	return stream.Err()
}

// send emits an event on ch, respecting context cancellation.
func send(ctx context.Context, ch chan<- llm.StreamEvent, ev llm.StreamEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}
