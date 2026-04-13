package server

import (
	"log/slog"
	"sync"
)

const subBufferSize = 1000

// EventBus is an in-process pub/sub bus keyed by run ID.
// Agents publish events; SSE handlers subscribe per run.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan []byte
	log         *slog.Logger
}

// NewEventBus creates a new EventBus.
func NewEventBus(log *slog.Logger) *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan []byte),
		log:         log,
	}
}

// Publish sends payload to all subscribers for the given run ID.
// Drops the message (with a warning log) if a subscriber's buffer is full — never blocks.
func (b *EventBus) Publish(runID string, payload []byte) {
	b.mu.RLock()
	subs := make([]chan []byte, len(b.subscribers[runID]))
	copy(subs, b.subscribers[runID])
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- payload:
		default:
			b.log.Warn("event bus: subscriber channel full, dropping event",
				slog.String("run_id", runID),
			)
		}
	}
}

// Subscribe registers a new subscriber for the given run ID.
// Returns a receive-only channel and an unsubscribe function.
// Calling unsubscribe removes the subscriber and closes the channel.
func (b *EventBus) Subscribe(runID string) (<-chan []byte, func()) {
	ch := make(chan []byte, subBufferSize)

	b.mu.Lock()
	b.subscribers[runID] = append(b.subscribers[runID], ch)
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		subs := b.subscribers[runID]
		for i, s := range subs {
			if s == ch {
				b.subscribers[runID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
		if len(b.subscribers[runID]) == 0 {
			delete(b.subscribers, runID)
		}
	}

	return ch, unsub
}

// subscriberCount returns the number of active subscribers for runID.
// Used in tests to assert cleanup.
func (b *EventBus) subscriberCount(runID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers[runID])
}
