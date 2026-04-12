package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

// Complete sends a blocking inference request to Anthropic and returns the full response.
func (c *Client) Complete(ctx context.Context, req llm.Request) (*llm.Response, error) {
	params, err := buildParams(req)
	if err != nil {
		return nil, fmt.Errorf("Complete build params: %w", err)
	}

	msg, err := c.sdk.Messages.New(ctx, params)
	if err != nil {
		return nil, classifySDKError(err)
	}

	return convertMessage(msg, req.Model)
}

// buildParams translates a llm.Request into the SDK's MessageNewParams.
func buildParams(req llm.Request) (anthropicsdk.MessageNewParams, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // safe default
	}

	params := anthropicsdk.MessageNewParams{
		Model:     req.Model,
		MaxTokens: int64(maxTokens),
	}

	// System prompt.
	if req.System != "" {
		params.System = []anthropicsdk.TextBlockParam{{Text: req.System}}
	}

	// Messages.
	for _, m := range req.Messages {
		sdkMsg, err := convertMessage_(m)
		if err != nil {
			return params, fmt.Errorf("buildParams message: %w", err)
		}
		params.Messages = append(params.Messages, sdkMsg)
	}

	// Temperature.
	if req.Temperature != 0 {
		params.Temperature = param.NewOpt(float64(req.Temperature))
	}

	// Stop sequences.
	if len(req.StopSeq) > 0 {
		params.StopSequences = req.StopSeq
	}

	// Tools.
	for _, t := range req.Tools {
		toolParam, err := convertTool(t)
		if err != nil {
			return params, fmt.Errorf("buildParams tool %q: %w", t.Name, err)
		}
		params.Tools = append(params.Tools, toolParam)
	}

	return params, nil
}

// convertMessage_ converts a llm.Message to the SDK MessageParam type.
func convertMessage_(m llm.Message) (anthropicsdk.MessageParam, error) {
	var blocks []anthropicsdk.ContentBlockParamUnion
	for _, cb := range m.Content {
		switch cb.Type {
		case "text":
			blocks = append(blocks, anthropicsdk.NewTextBlock(cb.Text))
		case "tool_result":
			// Tool results are sent as plain text for now.
			blocks = append(blocks, anthropicsdk.NewTextBlock(cb.Text))
		default:
			return anthropicsdk.MessageParam{}, fmt.Errorf("unsupported content block type %q", cb.Type)
		}
	}
	switch m.Role {
	case "user":
		return anthropicsdk.NewUserMessage(blocks...), nil
	case "assistant":
		return anthropicsdk.NewAssistantMessage(blocks...), nil
	default:
		return anthropicsdk.MessageParam{}, fmt.Errorf("unsupported role %q", m.Role)
	}
}

// convertTool converts a llm.Tool to the SDK ToolUnionParam type.
func convertTool(t llm.Tool) (anthropicsdk.ToolUnionParam, error) {
	var schema anthropicsdk.ToolInputSchemaParam
	if len(t.InputSchema) > 0 {
		if err := json.Unmarshal(t.InputSchema, &schema); err != nil {
			return anthropicsdk.ToolUnionParam{}, fmt.Errorf("unmarshal input schema: %w", err)
		}
	}
	toolParam := anthropicsdk.ToolParam{
		Name:        t.Name,
		InputSchema: schema,
	}
	if t.Description != "" {
		toolParam.Description = param.NewOpt(t.Description)
	}
	return anthropicsdk.ToolUnionParam{OfTool: &toolParam}, nil
}

// convertMessage converts an SDK Message to our llm.Response.
func convertMessage(msg *anthropicsdk.Message, modelID string) (*llm.Response, error) {
	if msg == nil {
		return nil, fmt.Errorf("convertMessage: nil message")
	}

	var blocks []llm.ContentBlock
	for _, cb := range msg.Content {
		switch cb.Type {
		case "text":
			t := cb.AsText()
			blocks = append(blocks, llm.ContentBlock{Type: "text", Text: t.Text})
		case "tool_use":
			tu := cb.AsToolUse()
			blocks = append(blocks, llm.ContentBlock{
				Type: "tool_use",
				ToolUse: &llm.ToolUseBlock{
					ID:    tu.ID,
					Name:  tu.Name,
					Input: tu.Input,
				},
			})
		}
	}

	inputTokens := int(msg.Usage.InputTokens)
	outputTokens := int(msg.Usage.OutputTokens)
	cacheRead := int(msg.Usage.CacheReadInputTokens)
	cacheWrite := int(msg.Usage.CacheCreationInputTokens)

	cost, err := llm.ComputeCost(modelID, inputTokens, outputTokens, cacheRead, cacheWrite)
	if err != nil {
		// Non-fatal: unknown model in pricing table. Log the zero cost.
		cost = 0
	}

	raw, _ := json.Marshal(msg)

	return &llm.Response{
		ID:               msg.ID,
		Model:            msg.Model,
		Provider:         "anthropic",
		Content:          blocks,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		CacheReadTokens:  cacheRead,
		CacheWriteTokens: cacheWrite,
		StopReason:       string(msg.StopReason),
		CostUSD:          cost,
		Raw:              raw,
	}, nil
}

// classifySDKError translates an SDK error into a typed *llm.ProviderError.
func classifySDKError(err error) error {
	if err == nil {
		return nil
	}
	// The Anthropic SDK wraps HTTP errors with status codes accessible via the
	// error message. We use a best-effort classification.
	return &llm.ProviderError{
		Code:       llm.ErrUnknown,
		StatusHTTP: 0,
		Retryable:  false,
		Err:        err,
	}
}
