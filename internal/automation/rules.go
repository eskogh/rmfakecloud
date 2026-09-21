package automation

import (
	"context"
	"errors"
	"strings"
)

func (m *Manager) validateRule(r Rule) error {
	if !validID.MatchString(r.ID) || r.Name == "" || r.UserID == "" || r.Event == "" || strings.HasPrefix(r.Event, "internal.") {
		return errors.New("rule requires a valid id, name, user_id and public event")
	}
	if strings.Contains(r.Event, "*") && r.Event != "*" && (!strings.HasSuffix(r.Event, ".*") || strings.Count(r.Event, "*") != 1) {
		return errors.New("invalid rule event pattern")
	}
	c, ok := m.configs[r.IntegrationID]
	if !ok || c.UserID != r.UserID {
		return errors.New("rule destination must belong to the same user")
	}
	for _, f := range r.Filters {
		if err := f.Validate(); err != nil {
			return err
		}
	}
	return nil
}
func (m *Manager) matchRules(id string, e Event) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rule := range m.rules {
		if !rule.Enabled || rule.IntegrationID != id || rule.UserID != e.Data.User.ID || !Matches(rule.Event, e.Event) {
			continue
		}
		match := true
		for _, f := range rule.Filters {
			if !f.Match(e) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
func cloneRules(rules []Rule) []Rule {
	result := append([]Rule{}, rules...)
	for i := range result {
		result[i].Filters = append([]Filter{}, result[i].Filters...)
	}
	return result
}
func (m *Manager) Rules() []Rule { m.mu.Lock(); defer m.mu.Unlock(); return cloneRules(m.rules) }

// ReplaceRules applies one validated snapshot atomically. Subscriptions and rules
// are alternative routes, so multiple matches never cause duplicate deliveries.
func (m *Manager) ReplaceRules(rules []Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(rules) > 100 {
		return errors.New("at most 100 rules are allowed")
	}
	seen := map[string]bool{}
	for _, r := range rules {
		if err := m.validateRule(r); err != nil {
			return err
		}
		if seen[r.ID] {
			return errors.New("duplicate rule ID")
		}
		seen[r.ID] = true
	}
	old := m.rules
	m.rules = cloneRules(rules)
	if err := m.persist(m.configs); err != nil {
		m.rules = old
		return errors.New("could not persist rules")
	}
	return nil
}

// Send is the server-side manual action entry point. Attachment openers must
// honor cancellation. HTTP callers should not execute this on sync handlers.
func (m *Manager) Send(ctx context.Context, id, userID string, e Event, attachment *Attachment) error {
	m.mu.Lock()
	c, ok := m.configs[id]
	m.mu.Unlock()
	if !ok || c.UserID != userID {
		return ErrNotFound
	}
	if !c.Enabled {
		return errors.New("integration is disabled")
	}
	e.Data.User.ID = userID
	e.Event = "document.send_requested"
	e.Data.Source = "manual"
	integration, err := newIntegration(c, m.policy, m.record)
	if err != nil {
		return err
	}
	defer integration.(interface{ Close() }).Close()
	ctx, cancel := context.WithTimeout(ctx, c.duration())
	defer cancel()
	if action, ok := integration.(Action); ok {
		return action.Send(ctx, e, attachment)
	}
	if attachment != nil {
		return errors.New("this integration does not support attachments")
	}
	return integration.HandleEvent(ctx, e)
}
