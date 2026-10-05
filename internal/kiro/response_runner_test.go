//go:build responsediscovery

package kiro

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

type responseStore interface {
	Load() (config.Document, bool, error)
	Close() error
}

// Dependencies and shortened budgets exist only in synthetic tagged tests.
// The live constructor has no destination or header override.
type responseDependencies struct {
	preflight      func(context.Context) (string, string, string, error)
	home           func() (string, error)
	open           func(string) (responseStore, error)
	reader         func(string) ProfileReader
	uuid           func() (string, error)
	wire           transport
	total, cleanup time.Duration
	encode         func(*responseReport) []byte
}

func responseCancellation(ctx context.Context) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout"
	}
	if ctx.Err() != nil {
		return "canceled"
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return "timeout"
	}
	return ""
}
func responseBoundary(parent, phase context.Context, r *responseReport, category string) bool {
	c := responseCancellation(parent)
	if c == "" {
		c = responseCancellation(phase)
	}
	if c != "" {
		r.fail(c)
		return false
	}
	if category != "" {
		r.fail(category)
		return false
	}
	return true
}
func responseEndpoint(region string) string {
	if region != "us-east-1" && region != "eu-central-1" {
		return ""
	}
	return "https://runtime." + region + ".kiro.dev:443/"
}
func responseBody(profile string) map[string]any {
	return map[string]any{"model": probeModel, "input": "Return exactly OK.", "origin": "KIRO_CLI", "instructions": "This is a protocol discovery test. Reply with exactly OK.", "stream": true, "maxOutputTokens": 1024, "profileArn": profile}
}
func responseHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/x-amz-json-1.0", "Accept": "*/*", "Accept-Encoding": "identity", "User-Agent": "kiro-gateway-response-discovery/1", "X-Amzn-Codewhisperer-Optout": "true", "X-Amz-Target": responseTarget}
}
func responseSourceCategory(err error) string {
	switch {
	case errors.Is(err, credentials.ErrChanged):
		return "source_changed"
	case errors.Is(err, credentials.ErrExpired):
		return "source_expired"
	case errors.Is(err, credentials.ErrProfileUnsupported):
		return "unsupported_region"
	case errors.Is(err, credentials.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, credentials.ErrCanceled), errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return "source_unavailable"
	}
}

// runResponseDiscovery owns the lock until transport cleanup succeeds. On a
// failed cleanup its caller MUST terminate the dedicated process. It cannot be
// reused; the unresolved worker retains ownership of the connection and lock.
func runResponseDiscovery(parent context.Context, launch bool, d responseDependencies) (r responseReport) {
	r = newResponseReport()
	if !launch {
		r.fail("preflight")
		return
	}
	started := time.Now()
	total, allowance := d.total, d.cleanup
	if total == 0 {
		total = 30 * time.Second
	}
	if allowance == 0 {
		allowance = 5 * time.Second
	}
	end := started.Add(total)
	if deadline, ok := parent.Deadline(); ok && deadline.Before(end) {
		end = deadline
	}
	absolute, stop := context.WithDeadline(parent, end)
	defer stop()
	work, cancel := context.WithDeadline(parent, end.Add(-allowance))
	defer cancel()
	encode := d.encode
	if encode == nil {
		encode = encodeResponseReport
	}
	var store responseStore
	var wire *responseConnection
	defer func() {
		// All work, including projection and size validation, shares the work cutoff.
		if responseBoundary(parent, work, &r, "") {
			encode(&r)
			responseBoundary(parent, work, &r, "")
		}
		cleanupEnd := time.Now().Add(allowance)
		if end.Before(cleanupEnd) {
			cleanupEnd = end
		}
		responseBoundary(parent, absolute, &r, "")
		transportOK := true
		if wire != nil {
			transportOK, cleanupEnd = wire.finish(cleanupEnd, allowance)
		}
		responseBoundary(parent, absolute, &r, "")
		if !transportOK {
			r.CleanupOutcome = "failed"
			r.BodySummary = nil
			r.fail("cleanup")
		} else if store != nil {
			// No deferred release may run while a worker remains unresolved.
			released := closeResponseStore(store, cleanupEnd)
			responseBoundary(parent, absolute, &r, "")
			if !released {
				r.CleanupOutcome = "failed"
				r.BodySummary = nil
				r.fail("cleanup")
			}
		}
		r.ElapsedMillis = time.Since(started).Milliseconds()
		if r.FailureCategory == nil && r.CleanupOutcome == "complete" && r.BodySummary != nil {
			r.Outcome = "response_observed"
		}
		encodeResponseReport(&r)
	}()
	if !responseBoundary(parent, work, &r, "") {
		return
	}
	commit, digest, expected, err := d.preflight(work)
	category := ""
	if err != nil {
		category = "preflight"
	}
	if !responseBoundary(parent, work, &r, category) {
		return
	}
	r.CodeCommit = &commit
	r.PlanDigest = &digest
	if responseEndpoint(expected) == "" {
		r.fail("preflight")
		return
	}
	uuid := d.uuid
	if uuid == nil {
		uuid = newUUID
	}
	id, err := uuid()
	category = ""
	if err != nil {
		category = "preflight"
	}
	if !responseBoundary(parent, work, &r, category) {
		return
	}
	r.RunID = &id
	home, err := d.home()
	category = ""
	if err != nil {
		category = "config_invalid"
	}
	if !responseBoundary(parent, work, &r, category) {
		return
	}
	store, err = d.open(home)
	category = ""
	if err != nil {
		store = nil
		category = "config_invalid"
		if errors.Is(err, configstore.ErrMissing) {
			category = "config_missing"
		}
		if errors.Is(err, configstore.ErrLocked) {
			category = "busy"
		}
	}
	if !responseBoundary(parent, work, &r, category) {
		return
	}
	document, exists, err := store.Load()
	category = ""
	if err != nil {
		category = "config_invalid"
	} else if !exists {
		category = "config_missing"
	} else if document.Validate() != nil || document.Session == nil || document.Models[probeModel] != probeModel {
		category = "config_invalid"
	}
	if !responseBoundary(parent, work, &r, category) {
		return
	}
	source, sourceCancel := context.WithTimeout(work, 5*time.Second)
	selected, err := d.reader(home).ReadProfileSnapshot(source, *document.Session)
	category = ""
	if err != nil {
		category = responseSourceCategory(err)
	}
	ok := responseBoundary(parent, source, &r, category)
	sourceCancel()
	if !ok {
		return
	}
	region := selected.ProfileRegion()
	if responseEndpoint(region) == "" {
		r.fail("unsupported_region")
		return
	}
	r.Region = &region
	if region != expected {
		r.fail("region_mismatch")
		return
	}
	token := selected.Credential()
	if !config.VisibleASCII(token.AccessToken(), config.MaxBytes) {
		r.fail("source_unavailable")
		return
	}
	body, err := json.Marshal(responseBody(selected.ProfileARN()))
	if err != nil {
		r.fail("preflight")
		return
	}
	req, err := http.NewRequestWithContext(work, http.MethodPost, responseEndpoint(region), bytes.NewReader(body))
	if err != nil {
		r.fail("preflight")
		return
	}
	for k, v := range responseHeaders(token.AccessToken()) {
		req.Header.Set(k, v)
	}
	if !responseBoundary(parent, work, &r, "") {
		return
	}
	if !token.ExpiresAt().After(time.Now()) {
		r.fail("source_expired")
		return
	}
	if d.wire.dial == nil {
		r.fail("preflight")
		return
	}
	r.DispatchCount = 1
	// Dial observes cancellation before any TLS work. There is no retry path.
	conn, err := d.wire.dial(work, req.URL.Host)
	if conn != nil {
		wire = newResponseConnection(work, conn)
	}
	category = ""
	if err != nil {
		category = "transport"
	}
	if !responseBoundary(parent, work, &r, category) {
		return
	}
	if wire == nil {
		r.fail("transport")
		return
	}
	wire.exchange(parent, work, d.wire, req, &r)
	return
}

// One worker closes the socket, then the body, and signals its exit. The body
// is handed to it only after synchronous response handling ends. A stuck OS
// close remains owned by this worker until the dedicated process exits.
type responseConnection struct {
	conn         net.Conn
	trigger      chan struct{}
	started      chan time.Time
	body         chan io.ReadCloser
	done         chan struct{}
	closeErr     error
	finished     time.Time
	responseBody io.ReadCloser
}

func newResponseConnection(ctx context.Context, conn net.Conn) *responseConnection {
	c := &responseConnection{conn: conn, trigger: make(chan struct{}, 1), started: make(chan time.Time, 1), body: make(chan io.ReadCloser, 1), done: make(chan struct{})}
	go func() {
		defer close(c.done)
		select {
		case <-ctx.Done():
		case <-c.trigger:
		}
		c.started <- time.Now()
		deadlineErr := conn.SetDeadline(time.Now())
		c.closeErr = conn.Close()
		if c.closeErr == nil {
			c.closeErr = deadlineErr
		}
		body := <-c.body
		if body != nil {
			err := body.Close()
			if c.closeErr == nil {
				c.closeErr = err
			}
		}
		c.finished = time.Now()
	}()
	return c
}
func (c *responseConnection) requestClose() {
	select {
	case c.trigger <- struct{}{}:
	default:
	}
}
func (c *responseConnection) finish(end time.Time, allowance time.Duration) (bool, time.Time) {
	c.body <- c.responseBody
	c.requestClose()
	// The trigger time is shared by cancellation and ordinary cleanup.
	started := <-c.started
	if cap := started.Add(allowance); cap.Before(end) {
		end = cap
	}
	timer := time.NewTimer(time.Until(end))
	defer timer.Stop()
	select {
	case <-c.done:
		return !c.finished.After(end) && c.closeErr == nil, end
	case <-timer.C:
		return false, end
	}
}
func (c *responseConnection) exchange(parent, ctx context.Context, t transport, req *http.Request, r *responseReport) {
	fail := func(err error) bool {
		category := ""
		if err != nil {
			category = "transport"
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				category = "timeout"
			}
		}
		return responseBoundary(parent, ctx, r, category)
	}
	deadline, _ := ctx.Deadline()
	if !fail(c.conn.SetDeadline(deadline)) {
		return
	}
	tlsConn := tls.Client(&idleConn{Conn: c.conn, idle: 25 * time.Second, requestClose: c.requestClose}, &tls.Config{RootCAs: t.roots, ServerName: req.URL.Hostname(), MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}})
	if !fail(tlsConn.HandshakeContext(ctx)) {
		return
	}
	req.Close = true
	if !fail(req.Write(tlsConn)) {
		return
	}
	reader := &headerReader{r: tlsConn, left: 16 << 10}
	resp, err := http.ReadResponse(bufio.NewReader(reader), req)
	if resp != nil {
		c.responseBody = resp.Body
	}
	if err != nil && reader.left <= 0 {
		responseBoundary(parent, ctx, r, "response_limit")
		return
	}
	if !fail(err) {
		return
	}
	reader.headers = false
	consumeResponse(ctx, resp, r)
	responseBoundary(parent, ctx, r, "")
}

// A lock release shares the original cleanup deadline. Its worker is joined
// before success; an unresolved OS close follows the same process exit rule.
func closeResponseStore(store responseStore, end time.Time) bool {
	done := make(chan struct{})
	var result connectionCloseResult
	go func() { defer close(done); result.err = store.Close(); result.finishedAt = time.Now() }()
	timer := time.NewTimer(time.Until(end))
	defer timer.Stop()
	select {
	case <-done:
		return result.err == nil && !result.finishedAt.After(end)
	case <-timer.C:
		return false
	}
}
