package automation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRulesRoutingPersistenceAndOwnership(t *testing.T) {
	var count atomic.Int32
	got := make(chan struct{}, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count.Add(1); got <- struct{}{} }))
	defer server.Close()
	bus := NewBus(16, time.Second)
	defer bus.Close()
	dir := t.TempDir()
	m, err := OpenManager(dir, bus, Policy{true, true})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	c := testConfig(server.URL)
	c.Events = nil
	if _, err := m.Save(c, true); err != nil {
		t.Fatal(err)
	}
	rule := Rule{ID: "work", Name: "Work notes", Enabled: true, UserID: "user", Event: "document.*", IntegrationID: c.ID, Filters: []Filter{{"document.name", "starts_with", "Work"}}}
	bad := rule
	bad.UserID = "other"
	if m.ReplaceRules([]Rule{bad}) == nil {
		t.Fatal("accepted cross-user rule")
	}
	if err := m.ReplaceRules([]Rule{rule}); err != nil {
		t.Fatal(err)
	}
	e := NewEvent("document.updated", "user")
	e.Data.Document = &Document{Name: "Work notes"}
	bus.Publish(context.Background(), e)
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("rule not routed")
	}
	// Direct and rule subscriptions match, but should deliver once.
	c.Events = []string{"document.*"}
	if _, err := m.Save(c, false); err != nil {
		t.Fatal(err)
	}
	bus.Publish(context.Background(), e)
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("not routed")
	}
	e.Data.User.ID = "other"
	bus.Publish(context.Background(), e)
	select {
	case <-got:
		t.Fatal("duplicate or cross-user delivery")
	case <-time.After(30 * time.Millisecond):
	}
	if m.Delete(c.ID) == nil {
		t.Fatal("deleted referenced destination")
	}
	m.Close()
	reloaded, err := OpenManager(dir, bus, Policy{true, true})
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	if len(reloaded.Rules()) != 1 {
		t.Fatal("lost rules")
	}
	if err := reloaded.ReplaceRules(nil); err != nil {
		t.Fatal(err)
	}
	if err := reloaded.Delete(c.ID); err != nil {
		t.Fatal(err)
	}
}
