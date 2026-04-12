// Package anthropic implements the llm.Adapter interface using the Anthropic SDK.
package anthropic

import (
	"strings"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

// Client wraps the Anthropic SDK and implements llm.Adapter.
type Client struct {
	sdk anthropicsdk.Client
}

// New creates an Anthropic Client using the provided API key.
// If apiKey is empty, the SDK falls back to the ANTHROPIC_API_KEY env var.
// Additional options (e.g. option.WithBaseURL for testing) can be passed.
func New(apiKey string, opts ...option.RequestOption) *Client {
	allOpts := make([]option.RequestOption, 0, len(opts)+1)
	if apiKey != "" {
		allOpts = append(allOpts, option.WithAPIKey(apiKey))
	}
	allOpts = append(allOpts, opts...)
	return &Client{sdk: anthropicsdk.NewClient(allOpts...)}
}

// Name returns the canonical provider identifier.
func (c *Client) Name() string { return "anthropic" }

// Models returns all Claude models currently in the pricing table.
func (c *Client) Models() []llm.Model {
	var out []llm.Model
	for _, m := range llm.Pricing {
		if strings.HasPrefix(m.ID, "claude-") {
			out = append(out, m)
		}
	}
	return out
}
