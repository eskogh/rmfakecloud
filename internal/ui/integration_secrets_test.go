package ui

import (
	"encoding/json"
	"github.com/ddvk/rmfakecloud/internal/model"
	"strings"
	"testing"
)

func TestLegacyWebhookSecretRoundTrip(t *testing.T) {
	old := model.IntegrationConfig{Provider: "webhook", Endpoint: "https://example.com", HMACSecret: "private-signing", Headers: map[string]string{"Authorization": "private-token"}}
	view := maskLegacyIntegration(old)
	raw, _ := json.Marshal(view)
	if strings.Contains(string(raw), "private-") || !view.HMACConfigured {
		t.Fatal("secret masking failed")
	}
	if old.Headers["Authorization"] != "private-token" {
		t.Fatal("masking mutated stored headers")
	}
	updated := view.IntegrationConfig
	preserveLegacyWebhookSecrets(&updated, old)
	if updated.HMACSecret != old.HMACSecret || updated.Headers["Authorization"] != old.Headers["Authorization"] {
		t.Fatal("lost saved secrets")
	}
}
