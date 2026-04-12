// Package ollama implements the llm.Adapter interface for local Ollama models.
package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

// Client wraps an HTTP connection to a local Ollama instance.
type Client struct {
	baseURL string
	http    *http.Client

	// Cache models list to avoid repeated API calls.
	modelsMu sync.RWMutex
	models   []llm.Model
	modelsErr error
}

// New creates an Ollama Client.
// If baseURL is empty, defaults to http://localhost:11434.
func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

// Name returns the canonical provider identifier.
func (c *Client) Name() string { return "ollama" }

// Models returns the list of models available on the Ollama instance.
// Calls the Ollama API once and caches the result.
func (c *Client) Models() []llm.Model {
	c.modelsMu.RLock()
	if c.models != nil || c.modelsErr != nil {
		defer c.modelsMu.RUnlock()
		return c.models
	}
	c.modelsMu.RUnlock()

	// Fetch from API.
	models, err := c.fetchModels(context.Background())

	c.modelsMu.Lock()
	defer c.modelsMu.Unlock()
	c.models = models
	c.modelsErr = err

	return c.models
}

// fetchModels retrieves the list of available models from /api/tags.
func (c *Client) fetchModels(ctx context.Context) ([]llm.Model, error) {
	url := c.baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchModels: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchModels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchModels: HTTP %d", resp.StatusCode)
	}

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("fetchModels: %w", err)
	}

	var models []llm.Model
	for _, m := range tagsResp.Models {
		models = append(models, llm.Model{
			ID:                 m.Name,
			DisplayName:        m.Name,
			InputPer1MUSD:      0.0, // Local model, no cost
			OutputPer1MUSD:     0.0,
			CacheReadPer1MUSD:  0.0,
			CacheWritePer1MUSD: 0.0,
			ContextWindow:      8192,    // Conservative default
			MaxOutputTokens:    4096,    // Conservative default
			SupportsTools:      true,    // Most modern Ollama models support tools
			SupportsStreaming:   true,   // Ollama supports streaming
			SupportsVision:     false,   // Assume vision unsupported unless stated
		})
	}

	return models, nil
}
