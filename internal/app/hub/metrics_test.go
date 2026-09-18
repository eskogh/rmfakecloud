package hub

import (
	"sync"
	"testing"
	"time"
)

func TestMetricsIsolationAndRetention(t *testing.T) {
	h := &Hub{connected: map[string]int{"alice": 2, "bob": 1}, activity: map[string]map[string]int{}, started: time.Now()}
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	h.recordSync("alice", now.AddDate(0, 0, -7))
	h.recordSync("alice", now)
	h.recordSync("bob", now)
	clients, days, _ := h.Metrics("alice", false)
	if clients != 2 || len(days) != 1 || days["2026-09-18"] != 1 {
		t.Fatalf("account metrics leaked or retention failed: %d %v", clients, days)
	}
	days["2026-09-18"] = 99
	clients, days, _ = h.Metrics("", true)
	if clients != 3 || days["2026-09-18"] != 2 {
		t.Fatalf("snapshot mutated metrics: %d %v", clients, days)
	}
}
func TestMetricsConcurrentReads(t *testing.T) {
	h := &Hub{connected: map[string]int{}, activity: map[string]map[string]int{}}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); h.recordSync("alice", time.Now()); h.Metrics("alice", false); h.ClientCount() }()
	}
	wg.Wait()
	_, days, _ := h.Metrics("alice", false)
	if days[time.Now().UTC().Format("2006-01-02")] != 20 {
		t.Fatal(days)
	}
}
