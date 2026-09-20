package email

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// ResolveHelo chooses an identity before SMTP connects. Proxy headers are only
// considered when the caller's proxy trust policy permits them.
func ResolveHelo(explicit, siteURL, cloudHost string, request *http.Request, trustProxy bool) string {
	hostname, _ := os.Hostname()
	return resolveHelo(explicit, siteURL, cloudHost, request, trustProxy, hostname)
}

func resolveHelo(explicit, siteURL, cloudHost string, request *http.Request, trustProxy bool, localHost string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	candidates := []string{}
	if u, err := url.Parse(siteURL); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		candidates = append(candidates, u.Host)
	}
	candidates = append(candidates, cloudHost)
	if request != nil {
		if trustProxy {
			candidates = append(candidates, strings.SplitN(request.Header.Get("X-Forwarded-Host"), ",", 2)[0])
			// Use the first proxy element, matching X-Forwarded-Host ordering.
			for _, field := range strings.Split(strings.SplitN(request.Header.Get("Forwarded"), ",", 2)[0], ";") {
				key, value, ok := strings.Cut(strings.TrimSpace(field), "=")
				if ok && strings.EqualFold(key, "host") {
					value = strings.TrimSpace(value)
					if strings.HasPrefix(value, "\"") {
						unquoted, err := strconv.Unquote(value)
						if err != nil {
							continue
						}
						value = unquoted
					}
					candidates = append(candidates, value)
					break
				}
			}
		}
		candidates = append(candidates, request.Host)
	}
	candidates = append(candidates, localHost)
	for _, candidate := range candidates {
		if host := heloHostname(candidate); host != "" {
			return host
		}
	}
	return ""
}

func heloHostname(value string) string {
	value = strings.TrimSpace(value)
	if host, port, err := net.SplitHostPort(value); err == nil {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return ""
		}
		value = host
	}
	value = strings.TrimSuffix(value, ".")
	if len(value) == 0 || len(value) > 253 || net.ParseIP(value) != nil {
		return ""
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return ""
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return ""
			}
		}
	}
	return value
}
