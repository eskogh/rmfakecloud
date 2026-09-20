package email

import (
	"net/http/httptest"
	"testing"
)

func TestResolveHeloPriority(t *testing.T) {
	tests := []struct {
		name, explicit, site, cloud, forwardedHost, forwarded, host, local, want string
		trusted                                                                  bool
	}{
		{name: "explicit", explicit: "chosen.example", site: "https://site.example", want: "chosen.example"},
		{name: "site URL", site: "https://site.example:8443/path", cloud: "cloud.example", want: "site.example"},
		{name: "cloud host", cloud: "cloud.example:443", host: "request.example", want: "cloud.example"},
		{name: "trusted forwarded host", forwardedHost: "proxy.example:443, other.example", forwarded: "host=other.example", host: "request.example", trusted: true, want: "proxy.example"},
		{name: "trusted Forwarded", forwarded: "for=192.0.2.1;host=\"proxy.example:443\";proto=https", host: "request.example", trusted: true, want: "proxy.example"},
		{name: "untrusted headers", forwardedHost: "evil.example", forwarded: "host=evil.example", host: "request.example:8080", want: "request.example"},
		{name: "invalid forwarded host", forwardedHost: "bad/path", forwarded: "host=proxy.example", trusted: true, want: "proxy.example"},
		{name: "local hostname", host: "bad_host", local: "machine.example", want: "machine.example"},
		{name: "invalid local hostname", local: "bad_host", want: ""},
		{name: "blank explicit", explicit: "  ", site: "https://site.example", want: "site.example"},
		{name: "invalid URL", site: "https://bad_host", host: "request.example", want: "request.example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://unused.example", nil)
			r.Host = tt.host
			r.Header.Set("X-Forwarded-Host", tt.forwardedHost)
			r.Header.Set("Forwarded", tt.forwarded)
			if got := resolveHelo(tt.explicit, tt.site, tt.cloud, r, tt.trusted, tt.local); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHeloHostnameValidation(t *testing.T) {
	for _, value := range []string{"bad\r\nhost", "bad/path", "user@host", "-host", "host-", "a..b", "host:bad", "host:70000", "127.0.0.1", "[::1]:443", "bad_host"} {
		if got := heloHostname(value); got != "" {
			t.Errorf("accepted %q as %q", value, got)
		}
	}
}
