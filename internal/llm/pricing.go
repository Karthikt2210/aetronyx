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
