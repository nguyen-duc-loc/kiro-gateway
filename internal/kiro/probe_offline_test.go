package kiro

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

const (
	probeMaxRequest      = 64 << 10
	probeMaxHeaders      = 16 << 10
	probeMaxResponse     = 8 << 20
	probeMaxAttempts     = 6
	probeMaxRetained     = 16 << 20
	probeMaxEvent        = 64 << 10
	probeSourceAllowance = 32 * config.MaxBytes
)

type probeLimits struct{ run, request, idle time.Duration }

var defaultProbeLimits = probeLimits{10 * time.Minute, 2 * time.Minute, 30 * time.Second}

// Output is built only from fixed labels and local counts. Payloads, tokens,
// references, raw errors and upstream header values have no output field.
type probeObservation struct {
	ServiceError        string `json:"service_error,omitempty"`
	ErrorResponseFormat string `json:"error_response_format,omitempty"`
	FailureStage        string `json:"failure_stage,omitempty"`
	TransportFailure    string `json:"transport_failure,omitempty"`
	HTTPStatus          string `json:"http_status_category,omitempty"`
	Outcome             string `json:"outcome"`
	Cause               string `json:"cause"`
	Attempts            int    `json:"attempts"`
	ReceivedBytes       int64  `json:"received_bytes"`
	TextEvents          int    `json:"text_events"`
	ToolEvents          int    `json:"tool_events"`
	UnknownEvents       int    `json:"unknown_events"`
	Completion          *bool  `json:"observed_completion"`
	CleanupCompleted    *bool  `json:"completed_cleanup"`
	CleanupMillis       int64  `json:"cleanup_millis"`
}

// snapshotReader belongs to the consuming boundary. The adapter must validate
// and return one snapshot; Capture followed by another token read is not valid.
type snapshotReader interface {
	ReadProfileSnapshot(context.Context, config.Session) (credentials.ProfileSnapshot, error)
}

// protocolProbe owns one locked configuration and one sequential run. Offline
// factories restrict it to loopback; the tagged live factory supplies a fixed dialer.
type protocolProbe struct {
	ctx           context.Context
	cancel        context.CancelFunc
	store         *configstore.Store
	reference     config.Session
	reader        snapshotReader
	endpoint      *url.URL
	destinations  map[string]*url.URL
	profileDigest [sha256.Size]byte
	profilePinned bool
	roots         *x509.CertPool
	limits        probeLimits
	attempts      int
	stopped       bool
	wire          bool
	dial          func(context.Context, string) (net.Conn, error)
}

func openOfflineProbe(parent context.Context, home, endpoint string, roots *x509.CertPool, limits probeLimits) (*protocolProbe, error) {
	return openRegionalOfflineProbe(parent, home, map[string]string{"us-east-1": endpoint, "eu-central-1": endpoint}, roots, limits)
}

func offlineDestination(endpoint string) (*url.URL, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/probe" || u.RawPath != "" {
		return nil, errPlanInvalid
	}
	addr, err := netip.ParseAddrPort(u.Host)
	if err != nil || !addr.Addr().Is4() || !addr.Addr().IsLoopback() || addr.Port() == 0 {
		return nil, errPlanInvalid
	}
	return u, nil
}

func openRegionalOfflineProbe(parent context.Context, home string, endpoints map[string]string, roots *x509.CertPool, limits probeLimits) (*protocolProbe, error) {
	if len(endpoints) == 0 || len(endpoints) > 2 || roots == nil {
		return nil, errPlanInvalid
	}
	// Copy and validate the entire finite fixture map before any source access.
	destinations := make(map[string]*url.URL, len(endpoints))
	for region, endpoint := range endpoints {
		if region != "us-east-1" && region != "eu-central-1" {
			return nil, errPlanInvalid
		}
		u, err := offlineDestination(endpoint)
		if err != nil {
			return nil, err
		}
		destinations[region] = u
	}
	return openProbeState(parent, home, destinations, roots.Clone(), limits)
}

func openProbeState(parent context.Context, home string, destinations map[string]*url.URL, roots *x509.CertPool, limits probeLimits) (*protocolProbe, error) {
	if limits.run <= 0 || limits.run > defaultProbeLimits.run || limits.request <= 0 || limits.request > defaultProbeLimits.request || limits.idle <= 0 || limits.idle > defaultProbeLimits.idle {
		return nil, errPlanInvalid
	}
	if err := parent.Err(); err != nil {
		return nil, probeContextError(parent, err)
	}
	store, err := configstore.OpenExisting(home)
	if err != nil {
		return nil, errConfiguration
	}
	d, exists, err := store.Load()
	if err != nil || !exists || d.Session == nil || d.Models[probeModel] != probeModel {
		store.Close()
		return nil, errConfiguration
	}
	ctx, cancel := context.WithTimeout(parent, limits.run)
	return &protocolProbe{ctx: ctx, cancel: cancel, store: store, reference: *d.Session,
		reader: credentials.Reader{Home: home}, destinations: destinations, roots: roots, limits: limits}, nil
}

func (p *protocolProbe) close() {
	p.cancel()
	p.store.Close()
	p.stopped = true
}

type probeStreamConsumer func(context.Context, context.CancelFunc, io.Reader, *probeObservation) error

func (p *protocolProbe) dispatch(body string) probeObservation {
	result, _ := p.exchange(body, func(_ context.Context, _ context.CancelFunc, r io.Reader, out *probeObservation) error {
		return observeProbeFrames(r, out)
	})
	p.stopped = true
	return result
}

// exchange owns one attempt and returns only after local request cleanup. The
// case runner decides whether an expected injected failure permits another case.
func (p *protocolProbe) exchange(body string, consume probeStreamConsumer) (probeObservation, error) {
	if len(body) > probeMaxRequest {
		return probeObservation{Outcome: "needs_evidence", Cause: errBudget.Error(), Attempts: p.attempts}, errBudget
	}
	return p.exchangeBuilt(func(credentials.ProfileSnapshot) (string, error) { return body, nil }, consume)
}

func (p *protocolProbe) exchangeBuilt(build func(credentials.ProfileSnapshot) (string, error), consume probeStreamConsumer) (result probeObservation, outcome error) {
	result.Outcome = "needs_evidence"
	result.FailureStage = "pre_dispatch"
	fail := func(err error) (probeObservation, error) {
		result.Cause = err.Error() // Only fixed category errors reach this closure.
		result.Attempts = p.attempts
		return result, err
	}
	if p.ctx.Err() != nil {
		return fail(probeContextError(p.ctx, p.ctx.Err()))
	}
	if p.attempts >= probeMaxAttempts {
		return fail(errBudget)
	}
	if p.stopped {
		return fail(errNeedsEvidence)
	}
	ctx, cancel := context.WithTimeout(p.ctx, p.limits.request)
	var responseBody io.ReadCloser
	var transport *http.Transport
	defer func() {
		start := time.Now()
		cancel()
		if responseBody != nil {
			responseBody.Close()
		}
		if transport != nil {
			transport.CloseIdleConnections()
		}
		result.CleanupMillis = time.Since(start).Milliseconds()
		cleaned := time.Since(start) <= 5*time.Second
		result.CleanupCompleted = &cleaned
		if !cleaned {
			result.Cause = errTimedOut.Error()
			result.FailureStage = "cleanup"
			outcome = errTimedOut
		}
	}()
	result.FailureStage = "source"
	selected, err := p.reader.ReadProfileSnapshot(ctx, p.reference)
	if err != nil {
		switch {
		case errors.Is(err, credentials.ErrChanged):
			return fail(errSessionChanged)
		case errors.Is(err, credentials.ErrProfileInvalid):
			return fail(errProfileInvalid)
		case errors.Is(err, credentials.ErrProfileUnsupported):
			return fail(errProfileUnsupported)
		case errors.Is(err, credentials.ErrExpired):
			return fail(errExpired)
		case errors.Is(err, credentials.ErrCanceled):
			return fail(errCanceled)
		case errors.Is(err, credentials.ErrTimeout):
			return fail(errTimedOut)
		default:
			return fail(errSource)
		}
	}
	digest := selected.ProfileDigest()
	if p.profilePinned && subtle.ConstantTimeCompare(digest[:], p.profileDigest[:]) != 1 {
		return fail(errProfileChanged)
	}
	if !p.profilePinned {
		endpoint, ok := p.destinations[selected.ProfileRegion()]
		if !ok {
			return fail(errPlanInvalid)
		}
		p.profileDigest, p.profilePinned, p.endpoint = digest, true, endpoint
	}
	snapshot := selected.Credential()
	if !snapshot.ExpiresAt().After(time.Now()) {
		return fail(errExpired)
	}
	if !config.VisibleASCII(snapshot.AccessToken(), config.MaxBytes) {
		return fail(errSource)
	}
	result.FailureStage = "request_build"
	body, err := build(selected)
	if err != nil {
		return fail(err)
	}
	if len(body) > probeMaxRequest {
		return fail(errBudget)
	}
	var tlsFailed atomic.Bool
	trace := &httptrace.ClientTrace{TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
		if err != nil {
			tlsFailed.Store(true)
		}
	}}
	requestContext := httptrace.WithClientTrace(ctx, trace)
	// NopCloser prevents GetBody from enabling request replay.
	req, err := http.NewRequestWithContext(requestContext, http.MethodPost, p.endpoint.String(), io.NopCloser(strings.NewReader(body)))
	if err != nil {
		return fail(errPlanInvalid)
	}
	req.Header.Set("Authorization", "Bearer "+snapshot.AccessToken())
	req.Header.Set("Content-Type", "application/json")
	if p.wire {
		req.Header.Set("Content-Type", wireContentType)
		req.Header.Set("X-Amz-Target", wireTarget)
		req.Header.Set("Accept", "application/vnd.amazon.eventstream")
	}
	req.ContentLength = int64(len(body))
	start := time.Now()
	transport = &http.Transport{
		Proxy: nil, DisableKeepAlives: true, DisableCompression: true, MaxResponseHeaderBytes: probeMaxHeaders,
		TLSClientConfig: &tls.Config{RootCAs: p.roots, MinVersion: tls.VersionTLS12},
		// Explicitly disable HTTP/2 as well as connection reuse and GetBody.
		TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != p.endpoint.Host {
				return nil, &probeDialFailure{stage: "destination_policy", cause: errPlanInvalid}
			}
			var c net.Conn
			var err error
			if p.dial != nil {
				c, err = p.dial(ctx, address)
			} else {
				c, err = (&net.Dialer{}).DialContext(ctx, "tcp4", address)
				if err != nil {
					err = &probeDialFailure{stage: "connect", cause: err}
				}
			}
			if err != nil {
				return nil, err
			}
			return &probeIdleConn{Conn: c, lastByte: start, idle: p.limits.idle}, nil
		},
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if ctx.Err() != nil {
		return fail(probeContextError(ctx, ctx.Err()))
	}
	result.FailureStage = "transport"
	p.attempts++ // A failed connection consumes one attempt. Nothing retries.
	resp, err := client.Do(req)
	if err != nil {
		result.TransportFailure = probeTransportLabel(err, tlsFailed.Load())
		return fail(probeContextError(ctx, err))
	}
	responseBody = resp.Body
	result.HTTPStatus = probeHTTPLabel(resp.StatusCode)
	result.FailureStage = "http_status"
	if resp.StatusCode != http.StatusOK {
		if p.wire {
			result.ServiceError, result.ErrorResponseFormat, result.ReceivedBytes = probeServiceError(resp.Header, resp.Body)
			if ctx.Err() != nil {
				return fail(probeContextError(ctx, ctx.Err()))
			}
		}
		return fail(errNeedsEvidence)
	}
	result.FailureStage = "response_headers"
	if p.wire {
		media, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil || media != "application/vnd.amazon.eventstream" || len(params) != 0 || resp.Header.Get("Content-Encoding") != "" {
			return fail(errContract)
		}
	}
	if resp.ContentLength > probeMaxResponse {
		return fail(errBudget)
	}
	result.FailureStage = "stream"
	err = consume(ctx, cancel, resp.Body, &result)
	if p.ctx.Err() != nil {
		return fail(probeContextError(p.ctx, p.ctx.Err()))
	}
	if errors.Is(err, errInjectedCancel) && errors.Is(ctx.Err(), context.Canceled) {
		return fail(errInjectedCancel)
	}
	if ctx.Err() != nil {
		return fail(probeContextError(ctx, ctx.Err()))
	}
	if err == nil {
		result.Attempts = p.attempts
		result.FailureStage = ""
		return result, nil
	}
	for _, category := range []error{errIncomplete, errContract, errBudget, errContradicted, errInjectedCutoff, errNeedsEvidence} {
		if errors.Is(err, category) {
			return fail(category)
		}
	}
	return fail(probeContextError(ctx, err))
}

// net/http owns one reader for this connection. Read resets the idle deadline
// only after actual bytes arrive; parsing work and byte trickles cannot extend
// the parent or request context deadlines.
type probeIdleConn struct {
	net.Conn
	lastByte time.Time
	idle     time.Duration
}

func (c *probeIdleConn) Read(b []byte) (int, error) {
	if err := c.SetReadDeadline(c.lastByte.Add(c.idle)); err != nil {
		return 0, err
	}
	n, err := c.Conn.Read(b)
	if n > 0 {
		c.lastByte = time.Now()
	}
	return n, err
}

func probeContextError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return errCanceled
	}
	var timeout net.Error
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
		return errTimedOut
	}
	return errNeedsEvidence
}

// The generic framing path never infers completion. The synthetic case decoder
// supplies its own explicitly invented completion event through visit.
func observeProbeFrames(input io.Reader, out *probeObservation) error {
	err := walkProbeFrames(input, out, &probeMemory{limit: probeMaxRetained, used: probeSourceAllowance}, nil)
	return probeReadError(err)
}

type probeFrameVisitor func(string, []byte) error

func walkProbeFrames(input io.Reader, out *probeObservation, memory *probeMemory, visit probeFrameVisitor) error {
	return walkFramedEvents(input, out, memory, visit, func(event string) bool {
		return event == "assistantResponseEvent" || event == "toolUseEvent" || (visit != nil && event == fixtureCompletionEvent)
	})
}

func walkFramedEvents(input io.Reader, out *probeObservation, memory *probeMemory, visit probeFrameVisitor, allowed func(string) bool) error {
	r := &probeCountingReader{r: input, count: &out.ReceivedBytes}
	for {
		var prelude [12]byte
		n, err := io.ReadFull(r, prelude[:])
		if n == 0 && errors.Is(err, io.EOF) {
			return io.EOF
		}
		if err != nil {
			return probeReadError(err)
		}
		err = func() error {
			total := int64(binary.BigEndian.Uint32(prelude[0:4]))
			headers := int64(binary.BigEndian.Uint32(prelude[4:8]))
			if crc32.ChecksumIEEE(prelude[:8]) != binary.BigEndian.Uint32(prelude[8:12]) {
				return errContract
			}
			if total < 16 || headers > total-16 {
				return errContract
			}
			if total-12 > probeMaxResponse-out.ReceivedBytes || headers > probeMaxHeaders {
				return errBudget
			}
			payloadSize := total - headers - 16
			// Only the offline decoder retains event payloads. Its declared 64 KiB
			// event bound is additional to the total response and retained byte limits.
			if visit != nil && payloadSize > probeMaxEvent {
				return errBudget
			}
			scratch := headers*64 + 65536
			if visit != nil {
				scratch += payloadSize * 64
			}
			// This conservative reservation includes framing buffers, JSON member
			// tracking, decoded copies, and parser work until visit returns.
			if err := memory.reserve(scratch); err != nil {
				return err
			}
			defer memory.release(scratch)
			h := make([]byte, int(headers))
			if _, err := io.ReadFull(r, h); err != nil {
				return probeReadError(err)
			}
			kind, event, err := probeEventHeaders(h)
			if err != nil {
				return err
			}
			checksum := crc32.NewIEEE()
			checksum.Write(prelude[:])
			checksum.Write(h)
			var payload []byte
			if visit != nil {
				payload = make([]byte, int(payloadSize))
				if _, err := io.ReadFull(r, payload); err != nil {
					return probeReadError(err)
				}
				checksum.Write(payload)
			} else if _, err := io.CopyN(checksum, r, payloadSize); err != nil {
				return probeReadError(err)
			}
			var trailer [4]byte
			if _, err := io.ReadFull(r, trailer[:]); err != nil {
				return probeReadError(err)
			}
			if checksum.Sum32() != binary.BigEndian.Uint32(trailer[:]) {
				return errContract
			}
			if kind != "event" {
				return errNeedsEvidence
			}
			if !allowed(event) {
				out.UnknownEvents++
				return errNeedsEvidence
			}
			switch event {
			case "assistantResponseEvent":
				out.TextEvents++
			case "toolUseEvent":
				out.ToolEvents++
			}
			if visit != nil {
				return visit(event, payload)
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}
}

// All retained conversation strings and temporary copies reserve space before
// allocation. State objects have a fixed allowance; peak includes scratch space.
type probeMemory struct{ used, peak, limit int64 }

func (m *probeMemory) reserve(n int64) error {
	if n < 0 || m.used > m.limit-n {
		return errBudget
	}
	m.used += n
	if m.used > m.peak {
		m.peak = m.used
	}
	return nil
}
func (m *probeMemory) release(n int64) { m.used -= n }
func (m *probeMemory) appendString(dst *string, part string) error {
	size := int64(len(*dst) + len(part))
	if err := m.reserve(size); err != nil {
		return err
	}
	old := len(*dst)
	*dst = *dst + part
	m.release(int64(old))
	return nil
}

type probeCountingReader struct {
	r     io.Reader
	count *int64
}

func (r *probeCountingReader) Read(b []byte) (int, error) {
	remaining := int64(probeMaxResponse) - *r.count
	if remaining <= 0 {
		return 0, errBudget
	}
	if int64(len(b)) > remaining {
		b = b[:remaining]
	}
	n, err := r.r.Read(b)
	*r.count += int64(n)
	return n, err
}

func probeReadError(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return errIncomplete
	}
	if errors.Is(err, errBudget) {
		return errBudget
	}
	return err // The runner sanitizes transport errors; this value is never output.
}

// Header values are kept local and discarded. Even unknown header spellings
// cannot enter an observation. Duplicate names and malformed types are rejected.
func probeEventHeaders(b []byte) (kind, event string, result error) {
	seen := map[string]bool{}
	for len(b) > 0 {
		n := int(b[0])
		b = b[1:]
		if n == 0 || len(b) < n+1 || !utf8.Valid(b[:n]) {
			return "", "", errContract
		}
		name := string(b[:n])
		typ := b[n]
		b = b[n+1:]
		if seen[name] {
			return "", "", errContract
		}
		seen[name] = true
		var size int
		switch typ {
		case 0, 1:
			size = 0
		case 2:
			size = 1
		case 3:
			size = 2
		case 4:
			size = 4
		case 5, 8:
			size = 8
		case 9:
			size = 16
		case 6, 7:
			if len(b) < 2 {
				return "", "", errContract
			}
			size = int(binary.BigEndian.Uint16(b[:2]))
			b = b[2:]
		default:
			return "", "", errContract
		}
		if len(b) < size || typ == 7 && !utf8.Valid(b[:size]) {
			return "", "", errContract
		}
		if name == ":message-type" || name == ":event-type" {
			if typ != 7 {
				return "", "", errContract
			}
			if name == ":message-type" {
				kind = string(b[:size])
			} else {
				event = string(b[:size])
			}
		}
		b = b[size:]
	}
	if kind == "" || kind == "event" && event == "" {
		return "", "", errContract
	}
	return kind, event, nil
}
