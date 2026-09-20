package automation

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type handlerFunc func(context.Context, Event) error

func (f handlerFunc) HandleEvent(c context.Context, e Event) error { return f(c, e) }
func TestBusIsolation(t *testing.T) {
	bus := NewBus(2, time.Second)
	defer bus.Close()
	blocked := make(chan struct{})
	defer close(blocked)
	bus.Subscribe(handlerFunc(func(context.Context, Event) error { <-blocked; return nil }))
	bus.Subscribe(handlerFunc(func(context.Context, Event) error { panic("secret must not be logged") }))
	got := make(chan Event, 1)
	bus.Subscribe(handlerFunc(func(_ context.Context, e Event) error { got <- e; return nil }))
	done := make(chan struct{})
	go func() { bus.Publish(context.Background(), NewEvent("document.created", "user")); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publisher blocked")
	}
	select {
	case e := <-got:
		if e.Version != "1" {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("healthy subscriber blocked")
	}
}
func testConfig(endpoint string) Config {
	return Config{ID: "test", Name: "Test", Type: "webhook", UserID: "user", Enabled: true, Endpoint: endpoint, Events: []string{"document.*"}, Timeout: "100ms"}
}
func TestWebhook(t *testing.T) {
	for _, status := range []int{200, 201, 400, 404, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var attempts atomic.Int32
			var deliveryID string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				body, _ := io.ReadAll(r.Body)
				mac := hmac.New(sha256.New, []byte("secret"))
				mac.Write([]byte(r.Header.Get("X-RMFakeCloud-Timestamp") + "."))
				mac.Write(body)
				if r.Header.Get("X-RMFakeCloud-Signature") != fmt.Sprintf("sha256=%x", mac.Sum(nil)) {
					t.Error("signature mismatch")
				}
				if r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("X-Test") != "value" {
					t.Error("missing headers")
				}
				id := r.Header.Get("X-RMFakeCloud-Delivery")
				if deliveryID != "" && deliveryID != id {
					t.Error("delivery ID changed on retry")
				}
				deliveryID = id
				var event Event
				if err := json.Unmarshal(body, &event); err != nil || event.Version != "1" {
					t.Error("invalid envelope")
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			c := testConfig(server.URL)
			c.Retry = Retry{Count: 1, Delays: []string{"0s"}}
			c.Signing = Signing{Enabled: true, Secret: "secret"}
			c.Auth = Auth{Type: "bearer", Token: "token"}
			c.Headers = map[string]string{"X-Test": "value"}
			var recorded Delivery
			hook, err := NewWebhook(c, Policy{true, true}, func(d Delivery) { recorded = d })
			if err != nil {
				t.Fatal(err)
			}
			defer hook.Close()
			err = hook.HandleEvent(context.Background(), NewEvent("document.updated", "user"))
			if (err == nil) != (status < 300) {
				t.Fatalf("unexpected error %v", err)
			}
			want := int32(1)
			if status == 429 || status == 500 {
				want = 2
			}
			if attempts.Load() != want || recorded.Attempts != int(want) {
				t.Fatal("incorrect retries", recorded)
			}
		})
	}
}
func TestWebhookTimeoutAndRedirect(t *testing.T) {
	var target atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
			return
		}
		if r.URL.Path == "/target" {
			target.Add(1)
			return
		}
		http.Redirect(w, r, "/target", 302)
	}))
	defer server.Close()
	for _, path := range []string{"/slow", "/redirect"} {
		c := testConfig(server.URL + path)
		c.Timeout = "20ms"
		h, _ := NewWebhook(c, Policy{true, true}, nil)
		if err := h.HandleEvent(context.Background(), NewEvent("integration.test", "user")); err == nil {
			t.Fatal("expected error")
		}
		h.Close()
	}
	if target.Load() != 0 {
		t.Fatal("followed redirect")
	}
}
func TestPolicyAndValidation(t *testing.T) {
	for _, endpoint := range []string{"not a URL", "file:///tmp/test", "http://example.com", "https://user:password@example.com", "https://localhost", "https://127.0.0.1", "https://[::1]", "https://10.0.0.1"} {
		if err := (Policy{}).ValidateURL(endpoint); err == nil {
			t.Errorf("accepted %s", endpoint)
		}
	}
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "::1", "fc00::1", "fe80::1", "100.64.0.1"} {
		if (Policy{}).allowed(net.ParseIP(address)) {
			t.Errorf("allowed %s", address)
		}
	}
	if !(Policy{}).allowed(net.ParseIP("8.8.8.8")) {
		t.Fatal("blocked public IP")
	}
	c := testConfig("https://example.com")
	c.Timeout = "bad"
	if c.Validate(Policy{}) == nil {
		t.Fatal("invalid timeout accepted")
	}
}
func TestFiltersAndRegistry(t *testing.T) {
	e := NewEvent("document.updated", "user")
	e.Data.Document = &Document{ID: "doc", Name: "Work notes"}
	for _, f := range []Filter{{"document.name", "equals", "Work notes"}, {"document.name", "not_equals", "Receipts"}, {"document.name", "contains", "notes"}, {"document.name", "starts_with", "Work"}, {"document.id", "exists", ""}} {
		if !f.Match(e) {
			t.Fatal("filter failed", f)
		}
	}
	if (Filter{"page.id", "exists", ""}).Match(e) {
		t.Fatal("absent page exists")
	}
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count++ }))
	defer server.Close()
	c := testConfig(server.URL)
	c.Filters = []Filter{{"document.name", "starts_with", "Work"}}
	h, _ := NewWebhook(c, Policy{true, true}, nil)
	defer h.Close()
	r := routed{Integration: h, config: c}
	r.HandleEvent(context.Background(), e)
	e.Data.User.ID = "other"
	r.HandleEvent(context.Background(), e)
	e.Data.User.ID = "user"
	e.Event = "sync.completed"
	r.HandleEvent(context.Background(), e)
	if count != 1 {
		t.Fatalf("delivered %d events", count)
	}
}
