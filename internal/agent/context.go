package agent

import (
	"fmt"
	"strings"
	"time"
)

// contextEntry is one labelled piece of content in the context window.
type contextEntry struct {
	label   string
	content string
	tokens  int
	addedAt time.Time
	pinned  bool // pinned entries (spec, plan) are never evicted
}

// ContextBuilder manages the agent's rolling context window.
// When the total estimated token count exceeds 70% of maxTokens, the oldest
// non-pinned entries are evicted to make room.
type ContextBuilder struct {
	maxTokens int
	total     int
	entries   []contextEntry
}

// NewContextBuilder creates a ContextBuilder with the given token budget.
func NewContextBuilder(maxTokens int) *ContextBuilder {
	return &ContextBuilder{maxTokens: maxTokens}
}

// Add appends a labelled content block. Tokens are estimated as len(content)/4.
// Pinned entries (label prefix "spec" or "plan") are never evicted.
func (cb *ContextBuilder) Add(label, content string) {
	tokens := len(content) / 4
	if tokens == 0 {
		tokens = 1
	}
	pinned := strings.HasPrefix(label, "spec") || strings.HasPrefix(label, "plan")
	cb.entries = append(cb.entries, contextEntry{
		label:   label,
		content: content,
		tokens:  tokens,
		addedAt: time.Now(),
		pinned:  pinned,
	})
	cb.total += tokens
	cb.evictIfNeeded()
}

// evictIfNeeded removes the oldest non-pinned entry when over budget.
func (cb *ContextBuilder) evictIfNeeded() {
	limit := int(float64(cb.maxTokens) * 0.7)
	for cb.total > limit {
		idx := cb.oldestUnpinnedIdx()
		if idx < 0 {
			break // nothing left to evict
		}
		cb.total -= cb.entries[idx].tokens
		cb.entries = append(cb.entries[:idx], cb.entries[idx+1:]...)
	}
}

// oldestUnpinnedIdx returns the index of the oldest non-pinned entry, or -1.
func (cb *ContextBuilder) oldestUnpinnedIdx() int {
	for i, e := range cb.entries {
		if !e.pinned {
			return i
		}
	}
	return -1
}

// Build concatenates all entries into a single context string.
func (cb *ContextBuilder) Build() string {
	var sb strings.Builder
	for _, e := range cb.entries {
		fmt.Fprintf(&sb, "## %s\n%s\n\n", e.label, e.content)
	}
	return sb.String()
}

// TokenCount returns the current estimated token count.
func (cb *ContextBuilder) TokenCount() int {
	return cb.total
}
