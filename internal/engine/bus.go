package engine

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/glnreddy421/klew/internal/model"
)

// Bus is the internal evidence event channel.
type Bus struct {
	ch     chan model.EvidenceEvent
	closed bool
	mu     sync.Mutex
	// queued counts events accepted; handled counts those applied or dropped.
	queued, handled atomic.Int64
}

func NewBus(buffer int) *Bus {
	if buffer <= 0 {
		buffer = 512
	}
	return &Bus{ch: make(chan model.EvidenceEvent, buffer)}
}

func (b *Bus) Publish(e model.EvidenceEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	select {
	case b.ch <- e:
		b.queued.Add(1)
	default:
		// drop oldest by receiving one and retrying
		select {
		case <-b.ch:
			b.handled.Add(1)
		default:
		}
		select {
		case b.ch <- e:
			b.queued.Add(1)
		default:
		}
	}
}

// Idle reports whether every published event has been applied or dropped,
// including the one a consumer may be working on.
func (b *Bus) Idle() bool {
	return b.handled.Load() == b.queued.Load()
}

func (b *Bus) Events() <-chan model.EvidenceEvent {
	return b.ch
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.closed = true
		close(b.ch)
	}
}

// RunConsumer drains the bus until ctx cancelled or channel closed.
func (b *Bus) RunConsumer(ctx context.Context, fn func(model.EvidenceEvent)) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-b.ch:
			if !ok {
				return
			}
			fn(e)
			b.handled.Add(1)
		}
	}
}
