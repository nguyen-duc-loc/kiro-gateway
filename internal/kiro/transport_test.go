package kiro

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"kiro-gateway/internal/bridge"
)

type slowPipeClose struct {
	net.Conn
	delay time.Duration
	calls atomic.Int64
}

func (c *slowPipeClose) Close() error {
	c.calls.Add(1)
	time.Sleep(c.delay)
	return c.Conn.Close()
}

// pipeTransport uses real TLS and HTTP over an in memory connection so synctest
// can advance the cleanup clock without a live destination or wall clock wait.
func pipeTransport(t *testing.T, delay time.Duration, response string) (transport, *slowPipeClose) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"runtime.us-east-1.kiro.dev"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
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
	client, server := net.Pipe()
	conn := &slowPipeClose{Conn: client, delay: delay}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		secured := tls.Server(server, &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12})
		r, err := http.ReadRequest(bufio.NewReader(secured))
		if err != nil {
			t.Errorf("ReadRequest(pipe fixture) = %v, want request", err)
			return
		}
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("Read(pipe request body) = %v, want EOF", err)
			return
		}
		if err := r.Body.Close(); err != nil {
			t.Errorf("Close(pipe request body) = %v, want nil", err)
			return
		}
		if _, err := io.WriteString(secured, response); err != nil {
			t.Errorf("Write(pipe response) = %v, want complete fixture", err)
		}
	}()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close(); <-finished })
	return transport{roots: roots, dial: func(context.Context, string) (net.Conn, error) { return conn, nil }}, conn
}

// covers: AC-5, AC-6. A completed stream is not successful until cleanup finishes.
func TestTransportCleanupBoundsSuccessfulResponses(t *testing.T) {
	for _, delay := range []time.Duration{time.Second, 6 * time.Second} {
		t.Run(delay.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				wire, conn := pipeTransport(t, delay, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
				r, _ := http.NewRequest("POST", "https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse", strings.NewReader("{}"))
				start := time.Now()
				err := wire.exchange(t.Context(), r, func(response *http.Response) error {
					body, err := io.ReadAll(response.Body)
					if string(body) != "ok" {
						t.Errorf("Read(pipe response) = %q, want ok", body)
					}
					return err
				})
				want := min(delay, 5*time.Second)
				if elapsed := time.Since(start); elapsed != want {
					t.Errorf("exchange(clean EOF, Close=%v).elapsed = %v, want %v", delay, elapsed, want)
				}
				if delay > 5*time.Second {
					if err == nil || bridge.SafeFailure(err).Category != "cleanup_failed" {
						t.Errorf("exchange(clean EOF, slow Close) = %v, want cleanup_failed", err)
					}
				} else if err != nil {
					t.Errorf("exchange(clean EOF, prompt Close) = %v, want nil", err)
				}
				// Join the finite simulated Close, including on the timeout path.
				time.Sleep(delay)
				synctest.Wait()
				if conn.calls.Load() != 1 {
					t.Errorf("exchange(clean EOF).Close calls = %d, want 1", conn.calls.Load())
				}
			})
		})
	}
}

// covers: AC-6. Interrupt socket reads before waiting on an unresponsive Close.
func TestTransportCancellationUsesOneCleanupWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		wire, conn := pipeTransport(t, 6*time.Second, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n1\r\nx\r\n")
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		r, _ := http.NewRequestWithContext(ctx, "POST", "https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse", strings.NewReader("{}"))
		finished := make(chan error, 1)
		go func() {
			finished <- wire.exchange(ctx, r, func(response *http.Response) error { _, err := io.ReadAll(response.Body); return err })
		}()
		synctest.Wait()
		cancel()
		synctest.Wait()
		time.Sleep(5 * time.Second)
		synctest.Wait()
		select {
		case err := <-finished:
			if err == nil || bridge.SafeFailure(err).Category != "cleanup_failed" {
				t.Errorf("exchange(cancel during body, slow Close) = %v, want cleanup_failed", err)
			}
		default:
			t.Error("exchange(cancel during body) active after 5s, want cleanup_failed in the original cleanup window")
			time.Sleep(time.Second)
			synctest.Wait()
			<-finished
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if conn.calls.Load() != 1 {
			t.Errorf("exchange(cancel during body).Close calls = %d, want 1", conn.calls.Load())
		}
	})
}

func eventStreamResponse(body []byte) string {
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/vnd.amazon.eventstream\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
}

type closeErrorConn struct {
	net.Conn
	err error
}

func (c *closeErrorConn) Close() error {
	_ = c.Conn.Close()
	return c.err
}

// covers: AC-5, AC-6, AC-8. Close errors invalidate success without replacing
// an earlier failure or exposing the underlying connection error.
func TestTransportCloseErrorPreservesEarlierFailure(t *testing.T) {
	throttled := &bridge.Failure{Status: http.StatusTooManyRequests, Type: "rate_limit_error", Message: "Upstream rate limit reached.", Category: "upstream_throttle"}
	for _, tc := range []struct {
		name         string
		consumeErr   error
		wantCategory string
	}{
		{name: "clean response", wantCategory: "incomplete_stream"},
		{name: "throttle", consumeErr: throttled, wantCategory: "upstream_throttle"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				wire, conn := pipeTransport(t, 0, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
				dial := wire.dial
				calls := 0
				wire.dial = func(ctx context.Context, address string) (net.Conn, error) {
					calls++
					c, err := dial(ctx, address)
					return &closeErrorConn{Conn: c, err: errors.New("synthetic-close-secret")}, err
				}
				r, err := http.NewRequest("POST", "https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse", strings.NewReader("{}"))
				if err != nil {
					t.Fatal(err)
				}
				err = wire.exchange(t.Context(), r, func(response *http.Response) error {
					if _, err := io.ReadAll(response.Body); err != nil {
						return err
					}
					return tc.consumeErr
				})
				if err == nil {
					t.Fatal("exchange(close error) = nil, want failure")
				}
				if got := bridge.SafeFailure(err).Category; got != tc.wantCategory {
					t.Errorf("exchange(%s, close error).category = %q, want %q", tc.name, got, tc.wantCategory)
				}
				if strings.Contains(err.Error(), "synthetic-close-secret") {
					t.Errorf("exchange(%s, close error) = %v, want sanitized error", tc.name, err)
				}
				if calls != 1 || conn.calls.Load() != 1 {
					t.Errorf("exchange(%s, close error) dials=%d closes=%d, want one each", tc.name, calls, conn.calls.Load())
				}
			})
		})
	}
}

// covers: AC-6. Rejecting an upstream response closes its socket before closing
// its body, so an unfinished chunked body cannot hold cleanup open.
func TestTransportRejectedResponseDoesNotDrainBody(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		wire, conn := pipeTransport(t, 0, "HTTP/1.1 429 Too Many Requests\r\nTransfer-Encoding: chunked\r\n\r\n")
		r, err := http.NewRequest("POST", "https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		want := &bridge.Failure{Status: http.StatusTooManyRequests, Type: "rate_limit_error", Message: "Upstream rate limit reached.", Category: "upstream_throttle"}
		start := time.Now()
		err = wire.exchange(t.Context(), r, func(response *http.Response) error {
			if response.StatusCode != http.StatusTooManyRequests {
				t.Errorf("exchange(rejected response).status = %d, want 429", response.StatusCode)
			}
			return want
		})
		if !errors.Is(err, want) || time.Since(start) != 0 || conn.calls.Load() != 1 {
			t.Errorf("exchange(unfinished rejected body) = %v elapsed=%v closes=%d, want throttle, no wait and one close", err, time.Since(start), conn.calls.Load())
		}
	})
}

// covers: AC-6. Replace DNS I/O, retaining publicDial's production resolution path.
func TestTransportCancellationDuringResolution(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		previous := net.DefaultResolver
		t.Cleanup(func() { net.DefaultResolver = previous })
		var calls atomic.Int64
		net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			calls.Add(1)
			<-ctx.Done()
			return nil, ctx.Err()
		}}
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		r, _ := http.NewRequestWithContext(ctx, "POST", "https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse", strings.NewReader("{}"))
		finished := make(chan error, 1)
		go func() {
			finished <- (transport{}).exchange(ctx, r, func(*http.Response) error { return errors.New("unexpected response") })
		}()
		synctest.Wait()
		cancel()
		synctest.Wait()
		err := <-finished
		if !errors.Is(err, context.Canceled) || calls.Load() == 0 {
			t.Errorf("exchange(canceled DNS) = %v resolver_calls=%d, want canceled after entering resolver", err, calls.Load())
		}
	})
}

// covers: AC-6. Dial must share the earlier parent or inactivity deadline.
func TestTransportStalledDialStopsWithoutReplay(t *testing.T) {
	for _, tc := range []struct {
		name         string
		parent, idle time.Duration
		want         time.Duration
	}{
		{"parent earlier", time.Second, 30 * time.Second, time.Second},
		{"idle earlier", time.Minute, 2 * time.Second, 2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), tc.parent)
				t.Cleanup(cancel)
				calls := 0
				wire := transport{idle: tc.idle, dial: func(ctx context.Context, address string) (net.Conn, error) {
					calls++
					<-ctx.Done()
					return nil, ctx.Err()
				}}
				r, err := http.NewRequestWithContext(ctx, "POST", "https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse", strings.NewReader("{}"))
				if err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				err = wire.exchange(ctx, r, func(*http.Response) error { t.Error("exchange(stalled dial) consumed response, want none"); return nil })
				if got := time.Since(start); got != tc.want || calls != 1 || err == nil || bridge.SafeFailure(err).Category != "timed_out" {
					t.Errorf("exchange(%s) elapsed=%v calls=%d error=%v, want %v one call and timed_out", tc.name, got, calls, err, tc.want)
				}
			})
		})
	}
}

// covers: AC-6. A stalled TLS peer must be closed and joined after cancellation.
func TestTransportCanceledHandshakeClosesOwnedConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, server := net.Pipe()
		t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		calls := 0
		wire := transport{dial: func(context.Context, string) (net.Conn, error) { calls++; return client, nil }}
		r, _ := http.NewRequest("POST", "https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse", strings.NewReader("{}"))
		finished := make(chan error, 1)
		go func() {
			finished <- wire.exchange(ctx, r, func(*http.Response) error { return errors.New("unexpected response") })
		}()
		synctest.Wait()
		cancel()
		synctest.Wait()
		select {
		case err := <-finished:
			if !errors.Is(err, context.Canceled) || calls != 1 {
				t.Errorf("exchange(canceled TLS) = %v calls=%d, want canceled and one call", err, calls)
			}
		default:
			t.Error("exchange(canceled TLS) still active, want joined worker")
		}
		var b [1]byte
		if _, err := server.Read(b[:]); !errors.Is(err, io.EOF) {
			t.Errorf("Read(canceled TLS peer) = %v, want EOF", err)
		}
	})
}

// covers: AC-6. TLS cancellation must delegate Close to the same cleanup owner.
func TestTransportSlowHandshakeCloseHasOneOwner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, server := net.Pipe()
		t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
		conn := &slowPipeClose{Conn: client, delay: 6 * time.Second}
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		wire := transport{dial: func(context.Context, string) (net.Conn, error) { return conn, nil }}
		r, _ := http.NewRequest("POST", "https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse", strings.NewReader("{}"))
		finished := make(chan error, 1)
		go func() {
			finished <- wire.exchange(ctx, r, func(*http.Response) error { return errors.New("unexpected response") })
		}()
		synctest.Wait()
		cancel()
		synctest.Wait()
		time.Sleep(5 * time.Second)
		synctest.Wait()
		select {
		case err := <-finished:
			if err == nil || bridge.SafeFailure(err).Category != "cleanup_failed" {
				t.Errorf("exchange(slow TLS cancellation) = %v, want cleanup_failed", err)
			}
		default:
			t.Error("exchange(slow TLS cancellation) active after 5s, want cleanup_failed")
			time.Sleep(time.Second)
			synctest.Wait()
			<-finished
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if got := conn.calls.Load(); got != 1 {
			t.Errorf("exchange(slow TLS cancellation).Close calls = %d, want 1", got)
		}
	})
}

type delayedCloseConn struct{ delay time.Duration }

func (c *delayedCloseConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (c *delayedCloseConn) Write([]byte) (int, error)        { return 0, io.ErrClosedPipe }
func (c *delayedCloseConn) Close() error                     { time.Sleep(c.delay); return nil }
func (c *delayedCloseConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (c *delayedCloseConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (c *delayedCloseConn) SetDeadline(time.Time) error      { return io.ErrClosedPipe }
func (c *delayedCloseConn) SetReadDeadline(time.Time) error  { return nil }
func (c *delayedCloseConn) SetWriteDeadline(time.Time) error { return nil }

// covers: AC-6. The cleanup bound is a return deadline, not a later diagnosis.
func TestTransportCleanupReturnsWithinFiveSeconds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		wire := transport{dial: func(context.Context, string) (net.Conn, error) { return &delayedCloseConn{delay: 6 * time.Second}, nil }}
		r, _ := http.NewRequest("POST", "https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse", strings.NewReader("{}"))
		finished := make(chan error, 1)
		go func() { finished <- wire.exchange(t.Context(), r, func(*http.Response) error { return nil }) }()
		synctest.Wait()
		time.Sleep(5 * time.Second)
		synctest.Wait()
		select {
		case err := <-finished:
			if err == nil || bridge.SafeFailure(err).Category != "cleanup_failed" {
				t.Errorf("exchange(stalled Close) = %v, want cleanup_failed", err)
			}
		default:
			t.Error("exchange(stalled Close) still active after 5s, want cleanup_failed within the cleanup bound")
			// Release the finite fixture and join even when the implementation fails.
			time.Sleep(time.Second)
			synctest.Wait()
			<-finished
		}
		// A timed out request must not wait for this finite fixture, but the test
		// still lets its cleanup owner finish before exiting the synctest bubble.
		time.Sleep(time.Second)
		synctest.Wait()
	})
}

// covers: AC-6. The production destination policy never permits local addresses.
func TestTransportDestinationPolicyRejectsNonPublicIPv4(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1", "169.254.1.1", "100.64.0.1", "0.0.0.0", "192.0.2.1", "198.51.100.1", "203.0.113.1", "224.0.0.1", "240.0.0.1", "::1", "::ffff:127.0.0.1"} {
		if publicIPv4(netip.MustParseAddr(address)) {
			t.Errorf("publicIPv4(%s) = true, want false", address)
		}
	}
	if !publicIPv4(netip.MustParseAddr("8.8.8.8")) {
		t.Error("publicIPv4(8.8.8.8) = false, want true")
	}
}
