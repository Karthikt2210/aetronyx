package openai

import (
	"context"
	"encoding/json"
	"fmt"

	openaisdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

// Complete sends a blocking inference request to OpenAI and returns the full response.
func (c *Client) Complete(ctx context.Context, req llm.Request) (*llm.Response, error) {
	params := buildParams(req)

	msg, err := c.sdk.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, classifySDKError(err)
	}

	return convertMessage(msg, req.Model)
}

// buildParams translates a llm.Request into OpenAI ChatCompletionNewParams.
func buildParams(req llm.Request) openaisdk.ChatCompletionNewParams {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // safe default
	}

	// Build messages slice.
	var messages []openaisdk.ChatCompletionMessageParamUnion

	// System prompt prepended as first message.
	if req.System != "" {
		messages = append(messages, openaisdk.SystemMessage(req.System))
	}

	// User messages.
	for _, m := range req.Messages {
		messages = append(messages, convertMessage_(m))
	}

	params := openaisdk.ChatCompletionNewParams{
		Model:     shared.ChatModelGPT4_1, // Will be overridden by model in req
		Messages:  messages,
		MaxTokens: openaisdk.Int(int64(maxTokens)),
	}

	// Set actual model from request.
	params.Model = shared.ChatModel(req.Model)

	// Temperature.
	if req.Temperature != 0 {
		params.Temperature = openaisdk.Float(float64(req.Temperature))
	}

	// Stop sequences.
	if len(req.StopSeq) > 0 && len(req.StopSeq) == 1 {
		params.Stop = openaisdk.ChatCompletionNewParamsStopUnion{
			OfString: openaisdk.String(req.StopSeq[0]),
		}
	}

	// Tools.
	for _, t := range req.Tools {
		toolParam := convertTool(t)
		params.Tools = append(params.Tools, toolParam)
	}

	return params
}

// convertMessage_ converts a llm.Message to OpenAI ChatCompletionMessageParamUnion.
func convertMessage_(m llm.Message) openaisdk.ChatCompletionMessageParamUnion {
	// Extract text content.
	text := ""
	for _, cb := range m.Content {
		if cb.Type == "text" {
			text = cb.Text
			break
		}
	}

	switch m.Role {
	case "user":
		return openaisdk.UserMessage(text)
	case "assistant":
		return openaisdk.AssistantMessage(text)
	default:
		return openaisdk.UserMessage(text)
	}
}

// convertTool converts a llm.Tool to OpenAI ChatCompletionToolParam.
func convertTool(t llm.Tool) openaisdk.ChatCompletionToolParam {
	var params interface{}
	if len(t.InputSchema) > 0 {
		_ = json.Unmarshal(t.InputSchema, &params)
	}

	return openaisdk.ChatCompletionToolParam{
		Function: shared.FunctionDefinitionParam{
			Name:        t.Name,
			Description: openaisdk.String(t.Description),
			Parameters: shared.FunctionParameters{
				"input": params,
			},
		},
	}
}

// convertMessage converts an OpenAI ChatCompletion to our llm.Response.
func convertMessage(msg *openaisdk.ChatCompletion, modelID string) (*llm.Response, error) {
	if msg == nil {
		return nil, fmt.Errorf("convertMessage: nil message")
	}

	var blocks []llm.ContentBlock

	// OpenAI responses typically have one choice; extract its content.
	if len(msg.Choices) > 0 {
		choice := msg.Choices[0]
		if choice.Message.Content != "" {
			blocks = append(blocks, llm.ContentBlock{
				Type: "text",
				Text: choice.Message.Content,
			})
		}

		// Extract tool calls if present.
		for _, tc := range choice.Message.ToolCalls {
			input, _ := json.Marshal(tc.Function.Arguments)
			blocks = append(blocks, llm.ContentBlock{
				Type: "tool_use",
				ToolUse: &llm.ToolUseBlock{
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				},
			})
		}
	}

	// Determine stop reason from finish_reason string.
	stopReason := "end_turn"
	if len(msg.Choices) > 0 {
		choice := msg.Choices[0]
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

	cost, err := llm.ComputeCost(modelID, int(msg.Usage.PromptTokens), int(msg.Usage.CompletionTokens), 0, 0)
	if err != nil {
		// Non-fatal: unknown model in pricing table. Log the zero cost.
		cost = 0
	}

	raw, _ := json.Marshal(msg)

	return &llm.Response{
		ID:               msg.ID,
		Model:            msg.Model,
		Provider:         "openai",
		Content:          blocks,
		InputTokens:      int(msg.Usage.PromptTokens),
		OutputTokens:     int(msg.Usage.CompletionTokens),
		CacheReadTokens:  0,
		CacheWriteTokens: 0,
		StopReason:       stopReason,
		CostUSD:          cost,
		Raw:              raw,
	}, nil
}

// classifySDKError translates an SDK error into a typed *llm.ProviderError.
func classifySDKError(err error) error {
	if err == nil {
		return nil
	}
	// For OpenAI, we'll attempt a best-effort classification.
	// Detailed error classification requires checking the error type.
	return &llm.ProviderError{
		Code:       llm.ErrUnknown,
		StatusHTTP: 0,
		Retryable:  false,
		Err:        err,
	}
}
