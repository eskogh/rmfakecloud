package email

import (
	"encoding/json"
	"errors"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Settings is the persisted administrator override. Password is never returned by the API.
type Settings struct {
	Server      string `json:"server"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	From        string `json:"from"`
	Helo        string `json:"helo"`
	Security    string `json:"security"`
	InsecureTLS bool   `json:"insecureTLS"`
}
type SettingsView struct {
	Server      string `json:"server"`
	Username    string `json:"username"`
	From        string `json:"from"`
	Helo        string `json:"helo"`
	Security    string `json:"security"`
	InsecureTLS bool   `json:"insecureTLS"`
	HasPassword bool   `json:"hasPassword"`
	Source      string `json:"source"`
}
type SettingsStore struct {
	mu       sync.RWMutex
	path     string
	fallback *SMTPConfig
	override *Settings
}

func OpenSettings(dataDir string, fallback *SMTPConfig) (*SettingsStore, error) {
	s := &SettingsStore{path: filepath.Join(dataDir, "smtp.json"), fallback: fallback}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var v Settings
	if err = json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	if err = v.validate(); err != nil {
		return nil, err
	}
	if err = os.Chmod(s.path, 0600); err != nil {
		return nil, err
	}
	s.override = &v
	return s, nil
}
func (v Settings) validate() error {
	host, port, err := net.SplitHostPort(v.Server)
	n, e := strconv.Atoi(port)
	if err != nil || e != nil || host == "" || n < 1 || n > 65535 || strings.ContainsAny(v.Server, "\r\n /\t") {
		return errors.New("Enter an SMTP server as hostname:port (for example smtp.example.com:587)")
	}
	if v.Security != "starttls" && v.Security != "tls" && v.Security != "none" {
		return errors.New("Choose STARTTLS, TLS, or no encryption")
	}
	if strings.ContainsAny(v.Helo+v.From+v.Username, "\r\n") {
		return errors.New("SMTP fields must not contain line breaks")
	}
	if v.From != "" {
		if _, err := mail.ParseAddress(v.From); err != nil {
			return errors.New("Enter a valid sender address")
		}
	}
	return nil
}
func (v Settings) config() *SMTPConfig {
	cfg := &SMTPConfig{Server: v.Server, Username: v.Username, Password: v.Password, Helo: v.Helo, InsecureTLS: v.InsecureTLS, NoTLS: v.Security != "tls", StartTLS: v.Security == "starttls"}
	if v.From != "" {
		cfg.FromOverride, _ = mail.ParseAddress(v.From)
	}
	return cfg
}
func settingsFromConfig(c *SMTPConfig) Settings {
	v := Settings{Security: "starttls"}
	if c == nil {
		return v
	}
	v.Server = c.Server
	v.Username = c.Username
	v.Password = c.Password
	v.Helo = c.Helo
	v.InsecureTLS = c.InsecureTLS
	v.Security = "tls"
	if c.StartTLS {
		v.Security = "starttls"
	} else if c.NoTLS {
		v.Security = "none"
	}
	if c.FromOverride != nil {
		v.From = c.FromOverride.String()
	}
	return v
}
func (s *SettingsStore) current() *SMTPConfig {
	if s.override != nil {
		return s.override.config()
	}
	return s.fallback
}
func (s *SettingsStore) Current() *SMTPConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.current()
	if c == nil {
		return nil
	}
	copy := *c
	return &copy
}
func (s *SettingsStore) View() SettingsView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := settingsFromConfig(s.current())
	source := "unconfigured"
	if s.override != nil {
		source = "web"
	} else if s.fallback != nil {
		source = "environment"
	}
	return SettingsView{Server: v.Server, Username: v.Username, From: v.From, Helo: v.Helo, Security: v.Security, InsecureTLS: v.InsecureTLS, HasPassword: v.Password != "", Source: source}
}

// Save atomically publishes a complete override; failed writes keep the previous configuration.
func (s *SettingsStore) Save(v Settings, clearPassword bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := v.validate(); err != nil {
		return err
	}
	old := settingsFromConfig(s.current())
	if clearPassword {
		v.Password = ""
	} else if v.Password == "" && old.Password != "" {
		if v.Server != old.Server || v.Username != old.Username {
			return errors.New("Re-enter the password when changing the server or username, or explicitly clear it")
		}
		v.Password = old.Password
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".smtp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), s.path); err != nil {
		return err
	}
	s.override = &v
	return nil
}
func (s *SettingsStore) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s.override = nil
	return nil
}
