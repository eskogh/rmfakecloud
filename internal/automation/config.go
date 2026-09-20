package automation

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Auth struct {
	Type  string `json:"type,omitempty"`
	Token string `json:"token,omitempty"`
}
type Signing struct {
	Enabled bool   `json:"enabled"`
	Secret  string `json:"secret,omitempty"`
}
type Retry struct {
	Count  int      `json:"count"`
	Delays []string `json:"delays,omitempty"`
}
type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value,omitempty"`
}
type Config struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
	// Every integration belongs to one user; events never cross this boundary.
	UserID   string            `json:"user_id"`
	Endpoint string            `json:"endpoint"`
	Method   string            `json:"method,omitempty"`
	Events   []string          `json:"events"`
	Headers  map[string]string `json:"headers,omitempty"`
	Auth     Auth              `json:"auth"`
	Signing  Signing           `json:"signing"`
	Timeout  string            `json:"timeout"`
	Retry    Retry             `json:"retry"`
	Filters  []Filter          `json:"filters,omitempty"`
	ChatID   string            `json:"chat_id,omitempty"`
}

func (c Config) duration() time.Duration {
	d, err := time.ParseDuration(c.Timeout)
	if err != nil || d <= 0 {
		return 10 * time.Second
	}
	return d
}
func (c Config) Validate(policy Policy) error {
	if c.ID == "" || c.Name == "" || c.UserID == "" {
		return errors.New("id, name and user_id are required")
	}
	if c.Type != "webhook" {
		return errors.New("unsupported integration type")
	}
	if err := policy.ValidateURL(c.Endpoint); err != nil {
		return err
	}
	if c.Method != "" && c.Method != "POST" {
		return errors.New("only POST is supported")
	}
	if c.Timeout != "" {
		d, err := time.ParseDuration(c.Timeout)
		if err != nil || d <= 0 || d > time.Minute {
			return errors.New("timeout must be a duration between 0 and 1m")
		}
	}
	if c.Retry.Count < 0 || c.Retry.Count > 3 {
		return errors.New("retry count must be between 0 and 3")
	}
	if len(c.Retry.Delays) > 3 {
		return errors.New("at most three retry delays are allowed")
	}
	for _, delay := range c.Retry.Delays {
		d, e := time.ParseDuration(delay)
		if e != nil || d < 0 || d > 2*time.Minute {
			return errors.New("retry delays must be between 0 and 2m")
		}
	}
	if c.Auth.Type != "" && c.Auth.Type != "bearer" {
		return errors.New("only bearer authentication is supported")
	}
	if c.Auth.Type == "bearer" && c.Auth.Token == "" {
		return errors.New("bearer token is required")
	}
	if c.Signing.Enabled && c.Signing.Secret == "" {
		return errors.New("signing secret is required")
	}
	for k, v := range c.Headers {
		if k == "" || strings.ContainsAny(k, " ()<>@,;:\\\"/[]?={}\t\r\n") || strings.ContainsAny(v, "\r\n") {
			return errors.New("invalid header")
		}
		for _, ch := range k {
			if ch < 33 || ch > 126 {
				return errors.New("invalid header name")
			}
		}
		switch strings.ToLower(k) {
		case "host", "content-length", "transfer-encoding", "connection":
			return errors.New("reserved header")
		}
	}
	for _, p := range c.Events {
		if p == "" || (strings.Contains(p, "*") && p != "*" && (!strings.HasSuffix(p, ".*") || strings.Count(p, "*") != 1)) {
			return errors.New("invalid event subscription")
		}
	}
	for _, f := range c.Filters {
		if err := f.Validate(); err != nil {
			return err
		}
	}
	return nil
}
func (f Filter) Validate() error {
	switch f.Field {
	case "event", "user.id", "document.id", "document.name", "document.type", "page.id", "content.text":
	default:
		return fmt.Errorf("unsupported filter field %q", f.Field)
	}
	switch f.Operator {
	case "equals", "not_equals", "contains", "starts_with", "exists":
	default:
		return errors.New("unsupported filter operator")
	}
	return nil
}
func (f Filter) Match(e Event) bool {
	value, present := "", false
	switch f.Field {
	case "event":
		value, present = e.Event, true
	case "user.id":
		value, present = e.Data.User.ID, true
	case "document.id", "document.name", "document.type":
		if d := e.Data.Document; d != nil {
			present = true
			switch f.Field {
			case "document.id":
				value = d.ID
			case "document.name":
				value = d.Name
			case "document.type":
				value = d.Type
			}
		}
	case "page.id":
		if e.Data.Page != nil {
			value, present = e.Data.Page.ID, true
		}
	case "content.text":
		if e.Data.Content != nil {
			value, present = e.Data.Content.Text, true
		}
	}
	if f.Operator == "exists" {
		return present
	}
	if !present {
		return false
	}
	switch f.Operator {
	case "equals":
		return value == f.Value
	case "not_equals":
		return value != f.Value
	case "contains":
		return strings.Contains(value, f.Value)
	case "starts_with":
		return strings.HasPrefix(value, f.Value)
	}
	return false
}
