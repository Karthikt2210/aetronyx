package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

// Complete sends a blocking inference request to Ollama and returns the full response.
func (c *Client) Complete(ctx context.Context, req llm.Request) (*llm.Response, error) {
	params := buildParams(req)

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("Complete: marshal params: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("Complete: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Complete: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Complete: HTTP %d", httpResp.StatusCode)
	}

	var chatResp struct {
		Model   string `json:"model"`
		Message struct {
			Role      string          `json:"role"`
			Content   string          `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		PromptEvalCount int `json:"prompt_eval_count"`
		EvalCount       int `json:"eval_count"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("Complete: decode: %w", err)
	}

	return convertChatResponse(chatResp, req.Model)
}

// buildParams constructs the Ollama chat API request body.
func buildParams(req llm.Request) map[string]interface{} {
	messages := []map[string]interface{}{}

	// Add system message if present.
	if req.System != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": req.System,
		})
	}

	// Add conversation messages.
	for _, m := range req.Messages {
		msg := map[string]interface{}{
			"role": m.Role,
		}

		// Extract content from blocks.
		var content string
		for _, cb := range m.Content {
			if cb.Type == "text" {
				content = cb.Text
				break
			}
		}
		msg["content"] = content

		messages = append(messages, msg)
	}

	params := map[string]interface{}{
		"model":    req.Model,
		"messages": messages,
		"stream":   false,
	}

	// Add tools if present.
	if len(req.Tools) > 0 {
		var tools []map[string]interface{}
		for _, t := range req.Tools {
			var schema map[string]interface{}
			if len(t.InputSchema) > 0 {
				_ = json.Unmarshal(t.InputSchema, &schema)
			}
			tools = append(tools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  schema,
				},
			})
		}
		params["tools"] = tools
	}

	// Add max tokens via options.
	if req.MaxTokens > 0 {
		params["options"] = map[string]interface{}{
			"num_predict": req.MaxTokens,
		}
	}

	return params
}

// convertChatResponse transforms an Ollama chat response to llm.Response.
func convertChatResponse(resp interface{}, modelID string) (*llm.Response, error) {
	// Handle the response type.
	chatResp, ok := resp.(struct {
		Model   string `json:"model"`
		Message struct {
			Role      string          `json:"role"`
			Content   string          `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		PromptEvalCount int `json:"prompt_eval_count"`
		EvalCount       int `json:"eval_count"`
	})
	if !ok {
		return nil, fmt.Errorf("convertChatResponse: invalid response type")
	}

	var blocks []llm.ContentBlock
	stopReason := "end_turn"

	// Handle native tool calls.
	if len(chatResp.Message.ToolCalls) > 0 {
		for _, tc := range chatResp.Message.ToolCalls {
			blocks = append(blocks, llm.ContentBlock{
				Type: "tool_use",
				ToolUse: &llm.ToolUseBlock{
					ID:    tc.Function.Name,
					Name:  tc.Function.Name,
					Input: tc.Function.Arguments,
				},
			})
		}
		stopReason = "tool_use"
	} else if chatResp.Message.Content != "" {
		// Check for JSON tool fallback in content.
		if tu := extractToolFromJSON(chatResp.Message.Content); tu != nil {
			blocks = append(blocks, llm.ContentBlock{
				Type:    "tool_use",
				ToolUse: tu,
			})
			stopReason = "tool_use"
		} else {
			// Regular text content.
			blocks = append(blocks, llm.ContentBlock{
				Type: "text",
				Text: chatResp.Message.Content,
			})
		}
	}

	raw, _ := json.Marshal(resp)

	return &llm.Response{
		ID:               modelID,
		Model:            chatResp.Model,
		Provider:         "ollama",
		Content:          blocks,
		InputTokens:      chatResp.PromptEvalCount,
		OutputTokens:     chatResp.EvalCount,
		CacheReadTokens:  0,
		CacheWriteTokens: 0,
		StopReason:       stopReason,
		CostUSD:          0.0, // Local model, no cost
		Raw:              raw,
	}, nil
}

// extractToolFromJSON attempts to parse a tool invocation from a JSON code fence in the content.
func extractToolFromJSON(content string) *llm.ToolUseBlock {
	// Look for ```json ... ``` or ``` ... ``` block.
	re := regexp.MustCompile("```(?:json)?\\s*\\n([\\s\\S]*?)\\n```")
	matches := re.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil
	}

	jsonStr := strings.TrimSpace(matches[1])
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
		return nil
	}

	// Expect {"name": "...", "input": {...}}.
	name, ok := obj["name"].(string)
	if !ok {
		return nil
	}

	input, _ := json.Marshal(obj["input"])
	return &llm.ToolUseBlock{
		ID:    name,
		Name:  name,
		Input: input,
	}
}
