package ui

import "github.com/ddvk/rmfakecloud/internal/model"

// Preserve the legacy provider schema while protecting the newly supported
// signing secret and arbitrary headers. Existing provider credentials retain
// their established API contract.
type legacyIntegrationView struct {
	model.IntegrationConfig
	HMACConfigured bool
}

func maskLegacyIntegration(c model.IntegrationConfig) legacyIntegrationView {
	view := legacyIntegrationView{IntegrationConfig: c, HMACConfigured: c.HMACSecret != ""}
	view.HMACSecret = ""
	if c.Headers != nil {
		view.Headers = make(map[string]string, len(c.Headers))
		for key := range c.Headers {
			view.Headers[key] = ""
		}
	}
	return view
}
func preserveLegacyWebhookSecrets(next *model.IntegrationConfig, old model.IntegrationConfig) {
	if next.Provider != old.Provider {
		return
	}
	if next.HMACSecret == "" {
		next.HMACSecret = old.HMACSecret
	}
	for key, value := range next.Headers {
		if value == "" {
			next.Headers[key] = old.Headers[key]
		}
	}
}
