package email

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestSettingsPersistenceAndFallback(t *testing.T) {
	dir := t.TempDir()
	fallback := &SMTPConfig{Server: "env.example:465", Username: "env", Password: "env-secret"}
	s, err := OpenSettings(dir, fallback)
	if err != nil {
		t.Fatal(err)
	}
	if s.View().Source != "environment" {
		t.Fatal(s.View())
	}
	v := Settings{Server: "smtp.example:587", Username: "notes", Password: "web-secret", Security: "starttls", From: "Notes <notes@example.com>"}
	if err = s.Save(v, false); err != nil {
		t.Fatal(err)
	}
	cfg := s.Current()
	if !cfg.StartTLS || !cfg.NoTLS || cfg.FromOverride.Address != "notes@example.com" {
		t.Fatal("incorrect SMTP mapping")
	}
	data, _ := json.Marshal(s.View())
	if strings.Contains(string(data), "web-secret") || strings.Contains(string(data), "env-secret") {
		t.Fatal("password leaked")
	}
	info, _ := os.Stat(s.path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("settings file permissions")
	}
	s, err = OpenSettings(dir, fallback)
	if err != nil {
		t.Fatal(err)
	}
	v.Password = ""
	v.Helo = "cloud.example.com"
	if err = s.Save(v, false); err != nil {
		t.Fatal(err)
	}
	if s.Current().Password != "web-secret" {
		t.Fatal("blank field erased password")
	}
	v.Server = "different.example:587"
	if err = s.Save(v, false); err == nil {
		t.Fatal("reused secret with a different host")
	}
	if s.Current().Server != "smtp.example:587" {
		t.Fatal("failed save changed configuration")
	}
	v.Server = "smtp.example:587"
	if err = s.Save(v, true); err != nil {
		t.Fatal(err)
	}
	if s.View().HasPassword {
		t.Fatal("explicit clear ignored")
	}
	if err = s.Reset(); err != nil {
		t.Fatal(err)
	}
	if s.Current().Password != "env-secret" || s.View().Source != "environment" {
		t.Fatal("environment fallback lost")
	}
	s, err = OpenSettings(dir, nil)
	if err != nil || s.Current() != nil {
		t.Fatal("reset did not persist")
	}
}
func TestSettingsValidationAndConcurrency(t *testing.T) {
	s, _ := OpenSettings(t.TempDir(), nil)
	for _, v := range []Settings{{Server: "host", Security: "tls"}, {Server: "host:0", Security: "tls"}, {Server: "host:587", Security: "unknown"}, {Server: "host:587", Security: "tls", From: "bad address"}, {Server: "host:587", Security: "tls", Helo: "hello\r\ninjection"}} {
		if err := s.Save(v, false); err == nil {
			t.Fatalf("accepted invalid settings: %v", v)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Save(Settings{Server: "smtp.example:465", Security: "tls"}, false); err != nil {
				t.Error(err)
			}
			s.View()
			s.Current()
		}()
	}
	wg.Wait()
}

func TestFailedSaveKeepsActiveSettings(t *testing.T) {
	s, err := OpenSettings(t.TempDir(), &SMTPConfig{Server: "env.example:465"})
	if err != nil {
		t.Fatal(err)
	}
	// Renaming a file over an existing directory must fail without publishing settings.
	if err = os.Mkdir(s.path, 0700); err != nil {
		t.Fatal(err)
	}
	if err = s.Save(Settings{Server: "new.example:587", Security: "starttls"}, false); err == nil {
		t.Fatal("expected write failure")
	}
	if s.View().Source != "environment" || s.Current().Server != "env.example:465" {
		t.Fatal("failed persistence changed live settings")
	}
}
