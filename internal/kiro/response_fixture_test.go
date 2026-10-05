//go:build responsediscovery

package kiro

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func responseCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"runtime.us-east-1.kiro.dev", "runtime.eu-central-1.kiro.dev"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, roots
}

func responseTLSFixture(t *testing.T, handler http.HandlerFunc) transport {
	t.Helper()
	cert, roots := responseCertificate(t)
	s := httptest.NewUnstartedServer(handler)
	s.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	s.StartTLS()
	t.Cleanup(s.Close)
	return transport{roots: roots, dial: func(ctx context.Context, address string) (net.Conn, error) {
		if address != "runtime.us-east-1.kiro.dev:443" && address != "runtime.eu-central-1.kiro.dev:443" {
			return nil, errPlanInvalid
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp4", s.Listener.Addr().String())
	}}
}

func responseFixtureDependencies(t *testing.T, home string) responseDependencies {
	t.Helper()
	return responseDependencies{
		preflight: func(context.Context) (string, string, string, error) {
			return strings.Repeat("a", 40), strings.Repeat("b", 64), "us-east-1", nil
		},
		home:   func() (string, error) { return home, nil },
		open:   func(h string) (responseStore, error) { return configstore.OpenExisting(h) },
		reader: func(h string) ProfileReader { return credentials.Reader{Home: h} },
		wire: transport{dial: func(context.Context, string) (net.Conn, error) {
			t.Error("unexpected discovery dial, want predispatch rejection")
			return nil, errPlanInvalid
		}},
	}
}
