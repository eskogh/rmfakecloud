package automation

import (
	"context"
	"strings"
	"sync"
)

type routed struct {
	Integration
	config Config
	rules  func(Event) bool
}

func (r routed) HandleEvent(ctx context.Context, e Event) error {
	if strings.HasPrefix(e.Event, "internal.") {
		return nil
	}
	if e.Data.User.ID != r.config.UserID {
		return nil
	}
	if e.TargetID != "" {
		if e.TargetID != r.ID() {
			return nil
		}
		if e.Event == "integration.test" && e.Data.Test {
			return r.Integration.HandleEvent(ctx, e)
		}
	}
	if !r.Enabled() {
		return nil
	}
	matched := false
	for _, pattern := range r.SubscribedEvents() {
		if Matches(pattern, e.Event) {
			matched = true
			break
		}
	}
	if !matched && (r.rules == nil || !r.rules(e)) {
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

func NewRegistry(bus EventBus) *Registry             { return &Registry{bus: bus, stops: make(map[string]func())} }
func (r *Registry) Register(i Integration, c Config) { r.register(i, c, nil) }
func (r *Registry) register(i Integration, c Config, rules func(Event) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if stop := r.stops[i.ID()]; stop != nil {
		stop()
	}
	unsubscribe := r.bus.Subscribe(routed{Integration: i, config: c, rules: rules})
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
