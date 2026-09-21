package automation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestManagerSecretsHistoryAndReload(t *testing.T) {
	dir := t.TempDir()
	bus := NewBus(4, time.Second)
	defer bus.Close()
	m, err := OpenManager(dir, bus, Policy{true, true})
	if err != nil {
		t.Fatal(err)
	}
	c := testConfig("https://example.com/private-endpoint")
	c.Auth = Auth{Type: "bearer", Token: "private-token"}
	c.Signing = Signing{Enabled: true, Secret: "private-secret"}
	c.Headers = map[string]string{"X-Key": "private-header"}
	view, err := m.Save(c, true)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(view)
	for _, s := range []string{"private-endpoint", "private-token", "private-secret", "private-header"} {
		if strings.Contains(string(data), s) {
			t.Fatal("secret leaked", s)
		}
	}
	if !view.TokenConfigured || !view.SigningConfigured || !view.EndpointConfigured {
		t.Fatal("missing configured flags")
	}
	view.Name = "Changed"
	if _, err = m.Save(view.Config, false); err != nil {
		t.Fatal(err)
	}
	if m.configs[c.ID].Auth.Token != c.Auth.Token || m.configs[c.ID].Headers["X-Key"] != c.Headers["X-Key"] {
		t.Fatal("lost secrets")
	}
	for n := 0; n < 105; n++ {
		m.record(Delivery{ID: "delivery", IntegrationID: c.ID, Attempts: 1, Success: true})
	}
	h, _ := m.History(c.ID)
	if len(h) != 100 {
		t.Fatal("unbounded history", len(h))
	}
	m.Close()
	reloaded, err := OpenManager(dir, bus, Policy{true, true})
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	if reloaded.configs[c.ID].Auth.Token != c.Auth.Token {
		t.Fatal("lost saved token")
	}
	h, _ = reloaded.History(c.ID)
	if len(h) != 100 {
		t.Fatal("history not persisted")
	}
	if err := reloaded.Delete(c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reloaded.Get(c.ID); err != ErrNotFound {
		t.Fatal("delete failed")
	}
}
func TestTargetedTestEvent(t *testing.T) {
	received := make(chan Event, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var e Event
		json.NewDecoder(r.Body).Decode(&e)
		received <- e
	}))
	defer server.Close()
	bus := NewBus(8, time.Second)
	defer bus.Close()
	m, _ := OpenManager(t.TempDir(), bus, Policy{true, true})
	defer m.Close()
	for _, id := range []string{"first", "second"} {
		c := testConfig(server.URL)
		c.ID = id
		c.Enabled = false
		m.Save(c, true)
	}
	if err := m.Test("first"); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-received:
		if e.Event != "integration.test" || !e.Data.Test {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("test not delivered")
	}
	select {
	case <-received:
		t.Fatal("test broadcast to unrelated integration")
	case <-time.After(30 * time.Millisecond):
	}
}
