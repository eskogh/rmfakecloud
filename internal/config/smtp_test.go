package config

import (
	"net/http/httptest"
	"testing"

	"github.com/ddvk/rmfakecloud/internal/email"
)

func TestSMTPForRequestDoesNotMutateSettings(t *testing.T) {
	original := &email.SMTPConfig{Server: "smtp.example:587"}
	cfg := &Config{SMTPConfig: original, StorageURL: "https://cloud.example:8443"}
	got := cfg.SMTPForRequest(httptest.NewRequest("POST", "http://request.example", nil))
	if got.Helo != "cloud.example" || got.Server != original.Server {
		t.Fatalf("unexpected resolved config: %+v", got)
	}
	if original.Helo != "" || got == original {
		t.Fatal("mutated original settings")
	}
}
