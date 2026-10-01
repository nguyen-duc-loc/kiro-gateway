package kiro

import (
	"context"
	"net"
	"net/netip"
	"net/url"
)

func wireDestinations() map[string]string {
	return map[string]string{
		"us-east-1":    "https://runtime.us-east-1.kiro.dev:443/",
		"eu-central-1": "https://runtime.eu-central-1.kiro.dev:443/",
	}
}

func wireDestination(raw string) (*url.URL, error) {
	for _, expected := range wireDestinations() {
		if raw == expected {
			return url.Parse(raw)
		}
	}
	return nil, errPlanInvalid
}

type probeResolve func(context.Context, string, string) ([]netip.Addr, error)
type probeDial func(context.Context, string, string) (net.Conn, error)

// Resolve once and connect once. DNS cannot redirect account traffic into a
// local network; the HTTP transport still verifies TLS against the fixed host.
func wireDial(resolve probeResolve, dial probeDial) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, address string) (net.Conn, error) {
		allowed := false
		for _, raw := range wireDestinations() {
			u, _ := url.Parse(raw)
			if address == u.Host {
				allowed = true
			}
		}
		if !allowed {
			return nil, errPlanInvalid
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil || port != "443" {
			return nil, errPlanInvalid
		}
		addresses, err := resolve(ctx, "ip4", host)
		if err != nil || len(addresses) == 0 {
			return nil, errNeedsEvidence
		}
		for _, a := range addresses {
			if !wirePublicIPv4(a) {
				return nil, errPlanInvalid
			}
		}
		// No fallback to another address if this connection fails.
		return dial(ctx, "tcp4", net.JoinHostPort(addresses[0].String(), port))
	}
}

func wirePublicIPv4(a netip.Addr) bool {
	if !a.Is4() || !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4"} {
		if netip.MustParsePrefix(cidr).Contains(a) {
			return false
		}
	}
	return true
}
