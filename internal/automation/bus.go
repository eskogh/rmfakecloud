package automation

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type EventBus interface {
	Publish(context.Context, Event)
	Subscribe(EventHandler) func()
}
type subscriber struct {
	queue  chan Event
	cancel context.CancelFunc
}

// Bus has one bounded queue/worker per subscriber. Slow handlers cannot consume
// unbounded goroutines or delay publishers or other integrations. Overflow drops
// the newest event with a warning. Delivery is best-effort, not durable.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[*subscriber]struct{}
	closed      bool
	capacity    int
	timeout     time.Duration
}

func NewBus(capacity int, timeout time.Duration) *Bus {
	if capacity < 1 {
		capacity = 64
	}
	if timeout <= 0 {
		timeout = 4 * time.Minute
	}
	return &Bus{subscribers: make(map[*subscriber]struct{}), capacity: capacity, timeout: timeout}
}
func (b *Bus) Subscribe(handler EventHandler) func() {
	ctx, cancel := context.WithCancel(context.Background())
	s := &subscriber{queue: make(chan Event, b.capacity), cancel: cancel}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		cancel()
		return func() {}
	}
	b.subscribers[s] = struct{}{}
	b.mu.Unlock()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-s.queue:
				if ctx.Err() != nil {
					return
				}
				b.handle(ctx, handler, event)
			}
		}
	}()
	return func() { b.mu.Lock(); delete(b.subscribers, s); cancel(); b.mu.Unlock() }
}
func (b *Bus) handle(parent context.Context, handler EventHandler, event Event) {
	ctx, cancel := context.WithTimeout(parent, b.timeout)
	defer cancel()
	defer func() {
		if recover() != nil {
			log.WithField("event", event.Event).Error("Automation handler panicked")
		}
	}()
	if err := handler.HandleEvent(ctx, event); err != nil {
		// Handler errors can contain credentials. Delivery adapters record safe details.
		log.WithField("event", event.Event).Warn("Automation handler failed")
	}
}
func (b *Bus) Publish(ctx context.Context, event Event) {
	if ctx.Err() != nil {
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for s := range b.subscribers {
		// Detach pointer fields so subscribers cannot alter each other's payloads.
		copy := event
		if event.Data.Document != nil {
			d := *event.Data.Document
			copy.Data.Document = &d
		}
		if event.Data.Page != nil {
			p := *event.Data.Page
			copy.Data.Page = &p
		}
		if event.Data.Content != nil {
			c := *event.Data.Content
			copy.Data.Content = &c
		}
		select {
		case s.queue <- copy:
		default:
			log.WithField("event", event.Event).Warn("Automation queue full; event dropped")
		}
	}
}
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for s := range b.subscribers {
		s.cancel()
		delete(b.subscribers, s)
	}
}
