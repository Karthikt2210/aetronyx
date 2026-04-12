// Package llm defines the frozen adapter interface for all LLM providers.
// All provider adapters must implement Adapter exactly — do not change this interface
// without a major version bump (see §6.5 of the master architecture doc).
package llm

import (
	"context"
	"encoding/json"
)

// Adapter is the provider-agnostic interface every LLM backend must implement.
// Frozen for v1: all future providers implement this contract unchanged.
type Adapter interface {
	// Name returns the canonical provider identifier (e.g. "anthropic").
	Name() string
	// Models returns the list of models this adapter supports.
	Models() []Model
	// Complete sends a blocking inference request and returns the full response.
	Complete(ctx context.Context, req Request) (*Response, error)
	// StreamComplete returns a channel of incremental events. The channel is
	// closed after a Kind="end" or Kind="error" event.
	StreamComplete(ctx context.Context, req Request) (<-chan StreamEvent, error)
}

// Request is the provider-agnostic inference request. All provider adapters
// translate this to their native request type.
type Request struct {
	Model       string            `json:"model"`
	System      string            `json:"system,omitempty"`
	Messages    []Message         `json:"messages"`
	Tools       []Tool            `json:"tools,omitempty"`
	MaxTokens   int               `json:"max_tokens"`
	Temperature float32           `json:"temperature,omitempty"`
	StopSeq     []string          `json:"stop_sequences,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"` // for tracing
}

// Response is the provider-agnostic inference response.
type Response struct {
	ID               string          `json:"id"`
	Model            string          `json:"model"`
	Provider         string          `json:"provider"`
	Content          []ContentBlock  `json:"content"`
	InputTokens      int             `json:"input_tokens"`
	OutputTokens     int             `json:"output_tokens"`
	CacheReadTokens  int             `json:"cache_read_tokens"`
	CacheWriteTokens int             `json:"cache_write_tokens"`
	StopReason       string          `json:"stop_reason"`
	CostUSD          float64         `json:"cost_usd"` // computed from pricing table
	Raw              json.RawMessage `json:"raw"`      // full provider response for audit
}

// StreamEvent is one chunk delivered over the streaming channel.
type StreamEvent struct {
	// Kind is one of: "delta", "tool_call_delta", "end", "error".
	Kind  string         `json:"kind"`
	Delta string         `json:"delta,omitempty"`
	Tool  *ToolCallEvent `json:"tool,omitempty"`
	Final *Response      `json:"final,omitempty"` // populated on Kind=="end"
	Err   error          `json:"-"`
}

// Model describes a single model variant and its pricing.
type Model struct {
	ID                 string  `json:"id"`           // e.g. "claude-opus-4-6"
	DisplayName        string  `json:"display_name"`
	InputPer1MUSD      float64 `json:"input_per_1m_usd"`
	OutputPer1MUSD     float64 `json:"output_per_1m_usd"`
	CacheReadPer1MUSD  float64 `json:"cache_read_per_1m_usd"`  // 0 if unsupported
	CacheWritePer1MUSD float64 `json:"cache_write_per_1m_usd"` // 0 if unsupported
	ContextWindow      int     `json:"context_window"`
	MaxOutputTokens    int     `json:"max_output_tokens"`
	SupportsTools      bool    `json:"supports_tools"`
	SupportsStreaming   bool    `json:"supports_streaming"`
	SupportsVision     bool    `json:"supports_vision"`
}

// Message is a single turn in the conversation.
type Message struct {
	Role    string         `json:"role"` // "user" or "assistant"
	Content []ContentBlock `json:"content"`
}

// ContentBlock is a typed piece of content within a message.
type ContentBlock struct {
	Type    string        `json:"type"` // "text" or "tool_use" or "tool_result"
	Text    string        `json:"text,omitempty"`
	ToolUse *ToolUseBlock `json:"tool_use,omitempty"`
}

// ToolUseBlock carries a tool invocation produced by the model.
type ToolUseBlock struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolCallEvent is a partial tool-use delta emitted during streaming.
type ToolCallEvent struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Delta string `json:"delta"` // partial JSON of the tool input
}

// Tool defines a tool the model may invoke.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"` // JSON Schema object
}
