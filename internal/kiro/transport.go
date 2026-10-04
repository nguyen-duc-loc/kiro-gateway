package kiro

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"time"

	"kiro-gateway/internal/bridge"
)

// transport opens exactly one HTTP/1.1 connection and has no replay path.
// Private injection points are available only to synthetic package tests.
type transport struct {
	roots *x509.CertPool
	dial  func(context.Context, string) (net.Conn, error)
	idle  time.Duration
}

type idleConn struct {
	net.Conn
	idle         time.Duration
	requestClose func()
}

// TLS may close its connection when HandshakeContext is canceled. Delegate that
// close to the transport owner rather than starting a second blocking Close.
func (c *idleConn) Close() error {
	c.requestClose()
	return nil
}

type connectionCloseResult struct {
	err        error
	finishedAt time.Time
}

func (c *idleConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		if e := c.SetReadDeadline(time.Now().Add(c.idle)); err == nil {
			err = e
		}
	}
	return n, err
}

func (t transport) exchange(ctx context.Context, req *http.Request, consume func(*http.Response) error) (err error) {
	idle := t.idle
	if idle == 0 {
		idle = 30 * time.Second
	}
	start := time.Now()
	dialCtx, cancel := context.WithDeadline(ctx, start.Add(idle))
	defer cancel()
	dial := t.dial
	if dial == nil {
		dial = publicDial
	}
	conn, err := dial(dialCtx, req.URL.Host)
	if err != nil {
		return transportError(ctx, err)
	}
	// One worker owns connection cleanup. Interrupt I/O before Close so a slow
	// close cannot hold the exchange in a socket read or write after cancellation.
	// A timeout leaves this worker owning the connection; the adapter then
	// disables inference, so unresolved cleanup cannot accumulate across requests.
	closing := make(chan struct{}, 1)
	requestClose := func() {
		select {
		case closing <- struct{}{}:
		default:
		}
	}
	closeStarted := make(chan time.Time, 1)
	closed := make(chan connectionCloseResult, 1)
	go func() {
		select {
		case <-ctx.Done():
		case <-closing:
		}
		closeStarted <- time.Now()
		_ = conn.SetDeadline(time.Now())
		closeErr := conn.Close()
		closed <- connectionCloseResult{err: closeErr, finishedAt: time.Now()}
	}()
	var responseBody io.ReadCloser
	defer func() {
		requestClose()
		// Cancellation and normal completion share the same cleanup start time.
		// Do not grant another five seconds if cancellation already started it.
		deadline := (<-closeStarted).Add(5 * time.Second)
		if requestDeadline, ok := ctx.Deadline(); ok && requestDeadline.Add(5*time.Second).Before(deadline) {
			deadline = requestDeadline.Add(5 * time.Second)
		}
		cleanupFailed := func() {
			err = &bridge.Failure{Status: http.StatusServiceUnavailable, Type: "api_error", Message: "Inference cleanup failed. Restart the gateway.", Category: "cleanup_failed"}
		}
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()
		select {
		case result := <-closed:
			if result.finishedAt.After(deadline) {
				cleanupFailed()
				return
			}
			if err == nil && result.err != nil {
				err = bridge.ProtocolFailure()
			}
			// The socket is closed, so Body.Close cannot drain upstream traffic.
			if responseBody != nil {
				if bodyErr := responseBody.Close(); err == nil && bodyErr != nil {
					err = bridge.ProtocolFailure()
				}
			}
		case <-timer.C:
			cleanupFailed()
		}
	}()
	if err = conn.SetDeadline(start.Add(idle)); err != nil {
		return bridge.ProtocolFailure()
	}
	tlsConn := tls.Client(&idleConn{Conn: conn, idle: idle, requestClose: requestClose}, &tls.Config{RootCAs: t.roots, ServerName: req.URL.Hostname(), MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}})
	if err = tlsConn.HandshakeContext(dialCtx); err != nil {
		return transportError(ctx, err)
	}
	// req.Write never rewinds the body, follows redirects, or uses environment proxies.
	req.Close = true
	if err = req.Write(tlsConn); err != nil {
		return transportError(ctx, err)
	}
	reader := &headerReader{r: tlsConn, left: 16 << 10}
	buffered := bufio.NewReader(reader)
	resp, err := http.ReadResponse(buffered, req)
	if err != nil {
		return transportError(ctx, err)
	}
	reader.headers = false
	responseBody = resp.Body
	err = consume(resp)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return transportError(ctx, err)
	}
	return nil
}

// headerReader reads the header boundary one byte at a time so a small header
// followed by a large body cannot be mistaken for an oversized header.
type headerReader struct {
	r       net.Conn
	left    int
	headers bool
	tail    uint32
	started bool
}

func (r *headerReader) Read(b []byte) (int, error) {
	if !r.started {
		r.started = true
		r.headers = true
	}
	if !r.headers {
		return r.r.Read(b)
	}
	if r.left <= 0 {
		return 0, bridge.ProtocolFailure()
	}
	if len(b) > 1 {
		b = b[:1]
	}
	n, err := r.r.Read(b)
	r.left -= n
	if n > 0 {
		r.tail = r.tail<<8 | uint32(b[0])
		if r.tail == 0x0d0a0d0a {
			r.headers = false
		}
	}
	return n, err
}
func transportError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	var failure *bridge.Failure
	if errors.As(err, &failure) {
		return failure
	}
	if e, ok := err.(net.Error); ok && e.Timeout() {
		return &bridge.Failure{Status: http.StatusGatewayTimeout, Type: "api_error", Message: "Inference timed out.", Category: "timed_out"}
	}
	return bridge.ProtocolFailure()
}
func publicDial(ctx context.Context, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" || (host != "runtime.us-east-1.kiro.dev" && host != "runtime.eu-central-1.kiro.dev") {
		return nil, bridge.ProtocolFailure()
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip4", host)
	if err != nil || len(addresses) == 0 {
		return nil, bridge.ProtocolFailure()
	}
	for _, a := range addresses {
		if !publicIPv4(a) {
			return nil, bridge.ProtocolFailure()
		}
	}
	return (&net.Dialer{}).DialContext(ctx, "tcp4", net.JoinHostPort(addresses[0].String(), port))
}
func publicIPv4(a netip.Addr) bool {
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
