package kiro

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
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

func TestServiceErrorDiscriminatorIsBoundedAndAllowlisted(t *testing.T) {
	for _, tc := range []struct {
		name, header, body, want string
	}{
		{name: "header preferred", header: "AccessDeniedError", body: "unread sentinel", want: "access_denied"},
		{name: "decorated header", header: "sentinel.namespace#AccessDeniedError:sentinel-suffix", want: "access_denied"},
		{name: "unknown header", header: "sentinel-private-code", want: "unknown"},
		{name: "control suffix", header: "AccessDeniedError:\nsentinel", want: "unknown"},
		{name: "large header", header: strings.Repeat("x", 257), want: "unknown"},
		{name: "body type", body: `{"__type":"AccessDeniedError","message":"sentinel-private-message"}`, want: "access_denied"},
		{name: "body route error", body: `{"code":"MissingAuthenticationTokenException"}`, want: "missing_authentication_token"},
		{name: "body unknown", body: `{"code":"sentinel-code"}`, want: "unknown"},
		{name: "body both", body: `{"code":"AccessDeniedError","__type":"ns#AccessDeniedError"}`, want: "access_denied"},
		{name: "body conflict", body: `{"code":"AccessDeniedError","__type":"ThrottlingError"}`, want: "ambiguous"},
		{name: "duplicate", body: `{"code":"AccessDeniedError","code":"ThrottlingError"}`, want: "unparseable"},
		{name: "nested duplicate", body: `{"code":"AccessDeniedError","unused":{"a":1,"a":2}}`, want: "unparseable"},
		{name: "wrong type", body: `{"code":123}`, want: "unparseable"},
		{name: "null type", body: `{"__type":null}`, want: "unparseable"},
		{name: "message only", body: `{"message":"sentinel-private-message"}`, want: "absent"},
		{name: "array", body: `[]`, want: "unparseable"},
		{name: "oversized", body: strings.Repeat("x", probeErrorBodyLimit+100), want: "oversized"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{"Content-Type": []string{"application/json"}}
			if tc.header != "" {
				h.Set("X-Amzn-Errortype", tc.header)
			}
			r := strings.NewReader(tc.body)
			got, format, n := probeServiceError(h, r)
			if got != tc.want || format != "json" || n > probeErrorBodyLimit+1 {
				t.Errorf("probeServiceError(%s) label=%s format=%s bytes=%d, want %s/json within bound", tc.name, got, format, n, tc.want)
			}
			if tc.header != "" && (n != 0 || r.Len() != len(tc.body)) {
				t.Error("header classification unnecessarily read body")
			}
			if strings.Contains(got, "sentinel") {
				t.Error("service error classifier leaked input")
			}
		})
	}
}

func TestServiceErrorRejectsAmbiguousHeadersWithoutReadingBody(t *testing.T) {
	for _, h := range []http.Header{
		{"Content-Type": []string{"application/json"}, "X-Amzn-Errortype": []string{"AccessDeniedError", "AccessDeniedError"}},
		{"Content-Type": []string{"application/json", "application/json"}},
		{"Content-Type": []string{"text/html"}},
	} {
		r := strings.NewReader("sentinel-private-body")
		_, _, n := probeServiceError(h, r)
		if n != 0 || r.Len() != len("sentinel-private-body") {
			t.Error("ambiguous or non JSON response body was read")
		}
	}
}

type diagnosticFailedReader struct{}

func (diagnosticFailedReader) Read([]byte) (int, error) {
	return 0, errors.New("sentinel-private-read-error")
}

func TestServiceErrorReadFailureIsSanitized(t *testing.T) {
	label, _, n := probeServiceError(http.Header{"Content-Type": []string{"application/json"}}, diagnosticFailedReader{})
	if label != "unavailable" || n != 0 {
		t.Errorf("probeServiceError(read failure)=%s/%d, want unavailable/0", label, n)
	}
}

func TestWireJSONDenialRemainsInconclusiveWithoutLeaking(t *testing.T) {
	home, _ := probeHome(t)
	p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"__type":"AccessDeniedError","message":"sentinel-account-message","account":"sentinel-account"}`))
	})
	got := runWireCases(p, wireTestID, probeMaxRetained)
	c := got.Cases[0]
	if got.Attempts != 1 || got.Verdict != "needs_evidence" || c.ServiceError != "access_denied" || c.ErrorResponseFormat != "json" || c.FailureStage != "http_status" || c.ReceivedBytes == 0 || got.Cases[1].Status != "unrun" {
		t.Errorf("wire JSON denial=%+v, want one inconclusive denial with error class and dependent case unrun", got)
	}
	b, _ := json.Marshal(got)
	if strings.Contains(string(b), "sentinel") {
		t.Error("wire error diagnostics leaked response fields")
	}
}
