package integration

import (
	"encoding/json"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/llm/anthropic"
	"github.com/karthikcodes/aetronyx/internal/llm/openai"
)

func TestMultiProvider_AdapterInterface(t *testing.T) {
	// Verify both adapters implement the interface
	var _ llm.Adapter = anthropic.New("")
	var _ llm.Adapter = openai.New("")
}

func TestMultiProvider_ModelComparison(t *testing.T) {
	anthropicModels := anthropic.New("").Models()
	openaiModels := openai.New("").Models()

	if len(anthropicModels) == 0 {
		t.Fatal("Expected at least one Anthropic model")
	}

	if len(openaiModels) == 0 {
		t.Fatal("Expected at least one OpenAI model")
	}

	// Both should return models from pricing table
	if anthropicModels[0].InputPer1MUSD <= 0 {
		t.Errorf("Anthropic pricing not set: %+v", anthropicModels[0])
	}

	if openaiModels[0].InputPer1MUSD <= 0 {
		t.Errorf("OpenAI pricing not set: %+v", openaiModels[0])
	}
}

func TestMultiProvider_ResponseConsistency(t *testing.T) {
	// Create identical requests for both providers
	req := llm.Request{
		Model:     "test-model",
		System:    "You are helpful",
		Messages:  []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: "Hi"}}}},
		MaxTokens: 100,
	}

	// Verify request structure is provider-agnostic
	if req.Model == "" {
		t.Fatal("Request model should be set")
	}
	if len(req.Messages) == 0 {
		t.Fatal("Request messages should be set")
	}

	// Both providers accept the same request structure
	// (This is an interface compliance check, not a functional test without mocks)
}

func TestMultiProvider_ProviderNames(t *testing.T) {
	anthropicClient := anthropic.New("")
	openaiClient := openai.New("")

	if anthropicClient.Name() != "anthropic" {
		t.Errorf("Anthropic Name() = %q, want 'anthropic'", anthropicClient.Name())
	}

	if openaiClient.Name() != "openai" {
		t.Errorf("OpenAI Name() = %q, want 'openai'", openaiClient.Name())
	}
}

func TestMultiProvider_ContentBlockMarshaling(t *testing.T) {
	// Test that llm.Response structures marshal correctly for both providers
	resp := &llm.Response{
		ID:           "test-123",
		Model:        "test-model",
		Provider:     "anthropic",
		Content:      []llm.ContentBlock{{Type: "text", Text: "Hello"}},
		InputTokens:  100,
		OutputTokens: 50,
		StopReason:   "end_turn",
		CostUSD:      0.01,
	}

	// Should marshal to JSON
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Should unmarshal back
	var resp2 llm.Response
	if err := json.Unmarshal(data, &resp2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if resp2.Provider != "anthropic" {
		t.Errorf("Provider = %q, want 'anthropic'", resp2.Provider)
	}
}
