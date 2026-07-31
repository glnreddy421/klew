package engine

import (
	"context"
	"sync"

	"github.com/glnreddy421/klew/internal/model"
)

// Bus is the internal evidence event channel.
type Bus struct {
	ch     chan model.EvidenceEvent
	closed bool
	mu     sync.Mutex
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
	default:
		// drop oldest by receiving one and retrying
		select {
		case <-b.ch:
		default:
		}
		select {
		case b.ch <- e:
		default:
		}
	}
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
		}
	}
}
