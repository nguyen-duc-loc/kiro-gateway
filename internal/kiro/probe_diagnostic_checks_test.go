package kiro

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

func TestWireHTTPRejectionDiagnosticsExcludeResponseData(t *testing.T) {
	for _, tc := range []struct {
		status int
		label  string
	}{
		{400, "bad_request"}, {401, "unauthorized"}, {403, "forbidden"}, {404, "not_found"}, {429, "throttled"}, {503, "server_error"}, {302, "redirect"}, {418, "other"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			home, _ := probeHome(t)
			p := localWireProbe(t, home, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Private", "sentinel-header")
				w.WriteHeader(tc.status)
				w.Write([]byte(`{"message":"sentinel-account-message"}`))
			})
			got := runWireCases(p, wireTestID, probeMaxRetained)
			c := got.Cases[0]
			if c.FailureStage != "http_status" || c.HTTPStatus != tc.label || c.TransportFailure != "" || c.ReceivedBytes != 0 || got.Attempts != 1 || got.Verdict != "needs_evidence" {
				t.Errorf("wire rejection(%d) = %+v, want http_status/%s, zero body bytes, one attempt, and inconclusive verdict", tc.status, c, tc.label)
			}
			b, _ := json.Marshal(got)
			if strings.Contains(string(b), "sentinel") {
				t.Error("HTTP diagnostic exposed response contents")
			}
		})
	}
}

func TestWireTransportDiagnosticsRemainFixed(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failDNS   bool
		addresses []netip.Addr
		want      string
	}{
		{name: "DNS", failDNS: true, want: "dns"},
		{name: "no addresses", want: "dns"},
		{name: "local address", addresses: []netip.Addr{netip.MustParseAddr("127.0.0.1")}, want: "destination_policy"},
		{name: "connect", addresses: []netip.Addr{netip.MustParseAddr("8.8.8.8")}, want: "connect"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolve := func(context.Context, string, string) ([]netip.Addr, error) {
				if tc.failDNS {
					return nil, errors.New("sentinel-DNS-error")
				}
				return tc.addresses, nil
			}
			dial := func(context.Context, string, string) (net.Conn, error) {
				return nil, errors.New("sentinel-connect-error")
			}
			_, err := wireDial(resolve, dial)(t.Context(), "runtime.us-east-1.kiro.dev:443")
			if got := probeTransportLabel(err, false); got != tc.want {
				t.Errorf("probeTransportLabel(%s)=%s, want %s", tc.name, got, tc.want)
			}
			if err == nil || err.Error() != "transport_failed" {
				t.Error("dial diagnostic did not mask underlying error")
			}
		})
	}
}

func TestWireTLSFailureIsDistinctFromHTTPRejection(t *testing.T) {
	home, _ := probeHome(t)
	p := localWireProbe(t, home, func(http.ResponseWriter, *http.Request) { t.Error("untrusted TLS reached HTTP handler") })
	p.roots = x509.NewCertPool()
	got := runWireCases(p, wireTestID, probeMaxRetained)
	c := got.Cases[0]
	if c.FailureStage != "transport" || c.TransportFailure != "tls" || c.HTTPStatus != "" || got.Attempts != 1 || c.Assertions.Completion != nil {
		t.Errorf("wire untrusted TLS = %+v, want transport/tls, no HTTP status and no completion", c)
	}
}
