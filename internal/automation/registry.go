package automation

import (
	"context"
	"strings"
	"sync"
)

type routed struct {
	Integration
	config Config
}

func (r routed) HandleEvent(ctx context.Context, e Event) error {
	if strings.HasPrefix(e.Event, "internal.") {
		return nil
	}
	if !r.Enabled() || e.Data.User.ID != r.config.UserID {
		return nil
	}
	matched := false
	for _, pattern := range r.SubscribedEvents() {
		if Matches(pattern, e.Event) {
			matched = true
			break
		}
	}
	if !matched {
		return nil
	}
	for _, filter := range r.config.Filters {
		if !filter.Match(e) {
			return nil
		}
	}
	return r.Integration.HandleEvent(ctx, e)
}

type Registry struct {
	mu    sync.Mutex
	bus   EventBus
	stops map[string]func()
}

func NewRegistry(bus EventBus) *Registry { return &Registry{bus: bus, stops: make(map[string]func())} }
func (r *Registry) Register(i Integration, c Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if stop := r.stops[i.ID()]; stop != nil {
		stop()
	}
	unsubscribe := r.bus.Subscribe(routed{Integration: i, config: c})
	r.stops[i.ID()] = func() {
		unsubscribe()
		if closer, ok := i.(interface{ Close() }); ok {
			closer.Close()
		}
	}
}
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if stop := r.stops[id]; stop != nil {
		stop()
		delete(r.stops, id)
	}
}
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, stop := range r.stops {
		stop()
		delete(r.stops, id)
	}
}
