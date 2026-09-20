package automation

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Policy struct {
	AllowHTTP            bool
	AllowPrivateNetworks bool
}

func (p Policy) ValidateURL(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("endpoint must be an absolute URL without userinfo or fragment")
	}
	if u.Scheme != "https" && !(p.AllowHTTP && u.Scheme == "http") {
		return errors.New("endpoint must use HTTPS (HTTP requires explicit server policy)")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return errors.New("invalid endpoint port")
		}
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !p.allowed(ip) {
		return errors.New("endpoint address is blocked by network policy")
	}
	if !p.AllowPrivateNetworks && (u.Hostname() == "localhost" || u.Hostname() == "localhost.") {
		return errors.New("localhost is blocked by network policy")
	}
	return nil
}
func (p Policy) allowed(ip net.IP) bool {
	return p.AllowPrivateNetworks || (ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !sharedIP(ip))
}
func sharedIP(ip net.IP) bool {
	v := ip.To4()
	return v != nil && v[0] == 100 && v[1] >= 64 && v[1] <= 127
}

// Resolve and validate at dial time, then dial the checked address directly.
// This prevents a second DNS lookup from bypassing private-network restrictions.
func (p Policy) client(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // proxy DNS resolution would bypass the address checks
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("invalid destination")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, errors.New("destination DNS lookup failed")
		}
		if len(ips) == 0 {
			return nil, errors.New("destination has no addresses")
		}
		for _, ip := range ips {
			if !p.allowed(ip.IP) {
				return nil, errors.New("destination blocked by network policy")
			}
		}
		dialer := net.Dialer{Timeout: timeout}
		for _, ip := range ips {
			conn, e := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if e == nil {
				return conn, nil
			}
		}
		return nil, errors.New("destination connection failed")
	}
	return &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
