package automation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

var ErrNotFound = errors.New("integration not found")
var validID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

type View struct {
	Config
	TokenConfigured    bool      `json:"tokenConfigured"`
	SigningConfigured  bool      `json:"signingConfigured"`
	EndpointConfigured bool      `json:"endpointConfigured"`
	LastDelivery       *Delivery `json:"last_delivery,omitempty"`
}
type state struct {
	Integrations []Config `json:"integrations"`
	Rules        []Rule   `json:"rules,omitempty"`
}

// Rule routes matching events to one integration owned by the same user.
type Rule struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	UserID        string   `json:"user_id"`
	Event         string   `json:"event"`
	Filters       []Filter `json:"filters"`
	IntegrationID string   `json:"integration_id"`
}
type Manager struct {
	mu       sync.Mutex
	dir      string
	policy   Policy
	bus      *Bus
	registry *Registry
	configs  map[string]Config
	history  map[string][]Delivery
	rules    []Rule
}

func OpenManager(dir string, bus *Bus, policy Policy) (*Manager, error) {
	dir = filepath.Join(dir, "automation")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	m := &Manager{dir: dir, policy: policy, bus: bus, registry: NewRegistry(bus), configs: map[string]Config{}, history: map[string][]Delivery{}}
	raw, err := os.ReadFile(filepath.Join(dir, "integrations.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		var saved state
		if json.Unmarshal(raw, &saved) != nil {
			return nil, errors.New("invalid automation settings file")
		}
		if len(saved.Integrations) > 100 {
			return nil, errors.New("too many integrations")
		}
		for _, c := range saved.Integrations {
			if !validID.MatchString(c.ID) || c.ID == "rules" {
				return nil, errors.New("invalid integration ID")
			}
			if _, exists := m.configs[c.ID]; exists {
				return nil, errors.New("duplicate integration ID")
			}
			if err := c.Validate(policy); err != nil {
				return nil, errors.New("invalid saved automation configuration")
			}
			m.configs[c.ID] = c
			if data, err := os.ReadFile(m.historyPath(c.ID)); err == nil {
				var h []Delivery
				if json.Unmarshal(data, &h) == nil {
					if len(h) > 100 {
						h = h[len(h)-100:]
					}
					m.history[c.ID] = h
				}
			}
		}
		if len(saved.Rules) > 100 {
			return nil, errors.New("too many automation rules")
		}
		ruleIDs := map[string]bool{}
		for _, rule := range saved.Rules {
			if ruleIDs[rule.ID] {
				return nil, errors.New("duplicate automation rule ID")
			}
			ruleIDs[rule.ID] = true
			if err := m.validateRule(rule); err != nil {
				return nil, errors.New("invalid saved automation rule")
			}
		}
		m.rules = saved.Rules
	}
	for _, c := range m.configs {
		m.register(c)
	}
	return m, nil
}
func (m *Manager) Close() { m.registry.Close() }
func (m *Manager) register(c Config) {
	h, err := newIntegration(c, m.policy, m.record)
	if err == nil {
		m.registry.register(h, c, func(e Event) bool { return m.matchRules(c.ID, e) })
	}
}
func (m *Manager) historyPath(id string) string { return filepath.Join(m.dir, "history-"+id+".json") }
func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".automation-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (m *Manager) persist(configs map[string]Config) error {
	saved := state{Rules: m.rules}
	for _, c := range configs {
		saved.Integrations = append(saved.Integrations, c)
	}
	sort.Slice(saved.Integrations, func(i, j int) bool { return saved.Integrations[i].ID < saved.Integrations[j].ID })
	return writeJSON(filepath.Join(m.dir, "integrations.json"), saved)
}
func cloneConfig(c Config) Config {
	b, _ := json.Marshal(c)
	var copy Config
	json.Unmarshal(b, &copy)
	return copy
}
func (m *Manager) Save(c Config, create bool) (View, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if create && c.ID == "" {
		c.ID = uuid.NewString()
	}
	if !validID.MatchString(c.ID) || c.ID == "rules" {
		return View{}, errors.New("invalid integration ID")
	}
	old, exists := m.configs[c.ID]
	if create && exists {
		return View{}, errors.New("integration ID already exists")
	}
	if !create && !exists {
		return View{}, ErrNotFound
	}
	if create && len(m.configs) >= 100 {
		return View{}, errors.New("integration limit reached")
	}
	c = cloneConfig(c)
	if exists {
		if c.Type != old.Type || c.UserID != old.UserID {
			return View{}, errors.New("integration type and owner cannot be changed")
		}
		if c.Endpoint == "" {
			c.Endpoint = old.Endpoint
		}
		if c.Auth.Token == "" {
			c.Auth.Token = old.Auth.Token
		}
		if c.Signing.Secret == "" {
			c.Signing.Secret = old.Signing.Secret
		}
		for key, value := range c.Headers {
			if value == "" {
				c.Headers[key] = old.Headers[key]
			}
		}
	}
	if err := c.Validate(m.policy); err != nil {
		return View{}, err
	}
	next := make(map[string]Config, len(m.configs)+1)
	for id, value := range m.configs {
		next[id] = value
	}
	next[c.ID] = c
	if err := m.persist(next); err != nil {
		return View{}, errors.New("could not persist integration settings")
	}
	m.configs = next
	m.register(c)
	return m.view(c), nil
}
func (m *Manager) view(c Config) View {
	c = cloneConfig(c)
	v := View{Config: c, TokenConfigured: c.Auth.Token != "", SigningConfigured: c.Signing.Secret != "", EndpointConfigured: c.Endpoint != ""}
	v.Endpoint = ""
	v.Auth.Token = ""
	v.Signing.Secret = ""
	for key := range v.Headers {
		v.Headers[key] = ""
	}
	if h := m.history[c.ID]; len(h) > 0 {
		last := h[len(h)-1]
		v.LastDelivery = &last
	}
	return v
}
func (m *Manager) List() []View {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := []View{}
	for _, c := range m.configs {
		result = append(result, m.view(c))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
func (m *Manager) Get(id string) (View, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.configs[id]
	if !ok {
		return View{}, ErrNotFound
	}
	return m.view(c), nil
}
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.configs[id]; !ok {
		return ErrNotFound
	}
	for _, rule := range m.rules {
		if rule.IntegrationID == id {
			return errors.New("delete rules targeting this integration first")
		}
	}
	next := make(map[string]Config, len(m.configs))
	for key, c := range m.configs {
		if key != id {
			next[key] = c
		}
	}
	if err := m.persist(next); err != nil {
		return errors.New("could not persist integration settings")
	}
	m.configs = next
	m.registry.Remove(id)
	delete(m.history, id)
	os.Remove(m.historyPath(id))
	return nil
}
func (m *Manager) record(d Delivery) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.configs[d.IntegrationID]; !ok {
		return
	}
	h := append(m.history[d.IntegrationID], d)
	if len(h) > 100 {
		h = h[len(h)-100:]
	}
	m.history[d.IntegrationID] = h
	if err := writeJSON(m.historyPath(d.IntegrationID), h); err != nil {
		log.Warn("Could not persist automation delivery history")
	}
}
func (m *Manager) History(id string) ([]Delivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.configs[id]; !ok {
		return nil, ErrNotFound
	}
	return append([]Delivery{}, m.history[id]...), nil
}
func (m *Manager) Test(id string) error {
	m.mu.Lock()
	c, ok := m.configs[id]
	m.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	event := NewEvent("integration.test", c.UserID)
	event.Data.Test = true
	event.TargetID = id
	m.bus.Publish(context.Background(), event)
	return nil
}
