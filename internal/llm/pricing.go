package llm

import "fmt"

// Update this file when provider pricing changes. Verify against provider docs.
// Prices are in USD per 1 million tokens.

// Pricing is the v1 model pricing table. Keyed by model ID.
var Pricing = map[string]Model{
	"claude-opus-4-6": {
		ID:                 "claude-opus-4-6",
		DisplayName:        "Claude Opus 4.6",
		InputPer1MUSD:      15.00,
		OutputPer1MUSD:     75.00,
		CacheReadPer1MUSD:  1.50,
		CacheWritePer1MUSD: 3.75,
		ContextWindow:      200_000,
		MaxOutputTokens:    32_000,
		SupportsTools:      true,
		SupportsStreaming:   true,
		SupportsVision:     true,
	},
	"claude-sonnet-4-6": {
		ID:                 "claude-sonnet-4-6",
		DisplayName:        "Claude Sonnet 4.6",
		InputPer1MUSD:      3.00,
		OutputPer1MUSD:     15.00,
		CacheReadPer1MUSD:  0.30,
		CacheWritePer1MUSD: 3.75,
		ContextWindow:      200_000,
		MaxOutputTokens:    64_000,
		SupportsTools:      true,
		SupportsStreaming:   true,
		SupportsVision:     true,
	},
	"claude-haiku-4-5-20251001": {
		ID:                 "claude-haiku-4-5-20251001",
		DisplayName:        "Claude Haiku 4.5",
		InputPer1MUSD:      0.80,
		OutputPer1MUSD:     4.00,
		CacheReadPer1MUSD:  0.08,
		CacheWritePer1MUSD: 0.80,
		ContextWindow:      200_000,
		MaxOutputTokens:    16_000,
		SupportsTools:      true,
		SupportsStreaming:   true,
		SupportsVision:     true,
	},
	"gpt-4.1": {
		ID:                 "gpt-4.1",
		DisplayName:        "GPT-4.1",
		InputPer1MUSD:      2.00,
		OutputPer1MUSD:     8.00,
		CacheReadPer1MUSD:  0.50,
		CacheWritePer1MUSD: 0.00,
		ContextWindow:      1_048_576,
		MaxOutputTokens:    32_768,
		SupportsTools:      true,
		SupportsStreaming:   true,
		SupportsVision:     false,
	},
	"gpt-4.1-mini": {
		ID:                 "gpt-4.1-mini",
		DisplayName:        "GPT-4.1 Mini",
		InputPer1MUSD:      0.40,
		OutputPer1MUSD:     1.60,
		CacheReadPer1MUSD:  0.10,
		CacheWritePer1MUSD: 0.00,
		ContextWindow:      1_048_576,
		MaxOutputTokens:    16_384,
		SupportsTools:      true,
		SupportsStreaming:   true,
		SupportsVision:     false,
	},
	"gpt-4.1-nano": {
		ID:                 "gpt-4.1-nano",
		DisplayName:        "GPT-4.1 Nano",
		InputPer1MUSD:      0.10,
		OutputPer1MUSD:     0.40,
		CacheReadPer1MUSD:  0.025,
		CacheWritePer1MUSD: 0.00,
		ContextWindow:      1_048_576,
		MaxOutputTokens:    16_384,
		SupportsTools:      true,
		SupportsStreaming:   true,
		SupportsVision:     false,
	},
	"o3": {
		ID:                 "o3",
		DisplayName:        "O3",
		InputPer1MUSD:      10.00,
		OutputPer1MUSD:     40.00,
		CacheReadPer1MUSD:  2.50,
		CacheWritePer1MUSD: 0.00,
		ContextWindow:      200_000,
		MaxOutputTokens:    100_000,
		SupportsTools:      true,
		SupportsStreaming:   false,
		SupportsVision:     false,
	},
	"o4-mini": {
		ID:                 "o4-mini",
		DisplayName:        "O4 Mini",
		InputPer1MUSD:      1.10,
		OutputPer1MUSD:     4.40,
		CacheReadPer1MUSD:  0.275,
		CacheWritePer1MUSD: 0.00,
		ContextWindow:      200_000,
		MaxOutputTokens:    100_000,
		SupportsTools:      true,
		SupportsStreaming:   false,
		SupportsVision:     false,
	},
}

// ComputeCost calculates the total USD cost for a call given token counts.
// Returns an error if the modelID is not in the pricing table.
func ComputeCost(modelID string, inputTokens, outputTokens, cacheRead, cacheWrite int) (float64, error) {
	m, ok := Pricing[modelID]
	if !ok {
		return 0, fmt.Errorf("ComputeCost: unknown model %q", modelID)
	}
	const perMillion = 1_000_000.0
	cost := float64(inputTokens)/perMillion*m.InputPer1MUSD +
		float64(outputTokens)/perMillion*m.OutputPer1MUSD +
		float64(cacheRead)/perMillion*m.CacheReadPer1MUSD +
		float64(cacheWrite)/perMillion*m.CacheWritePer1MUSD
	return cost, nil
}
