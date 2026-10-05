//go:build schemaprobe

package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

const schemaTarget = "KiroControlPlaneBearerService.ListAvailableModels"

type schemaStore interface {
	Load() (config.Document, bool, error)
	Close() error
}

// All injection points are confined to this tagged test binary. The live
// constructor supplies the fixed store, reader, destination policy and budgets.
type schemaDependencies struct {
	preflight func(context.Context) (string, string, error)
	home      func() (string, error)
	open      func(string) (schemaStore, error)
	reader    func(string) ProfileReader
	wire      transport
}

func schemaEndpoint(region string) string {
	switch region {
	case "us-east-1", "eu-central-1":
		return "https://management." + region + ".kiro.dev:443/"
	default:
		return ""
	}
}

// schemaDial shares the production IP policy but has its own finite host list.
// This function cannot dispatch inference and is absent from product builds.
func schemaDial(resolve probeResolve, dial probeDial) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, address string) (net.Conn, error) {
		if address != "management.us-east-1.kiro.dev:443" && address != "management.eu-central-1.kiro.dev:443" {
			return nil, errPlanInvalid
		}
		host, port, _ := net.SplitHostPort(address)
		addresses, err := resolve(ctx, "ip4", host)
		if err != nil || len(addresses) == 0 {
			return nil, errPlanInvalid
		}
		for _, addr := range addresses {
			if !publicIPv4(addr) {
				return nil, errPlanInvalid
			}
		}
		return dial(ctx, "tcp4", net.JoinHostPort(addresses[0].String(), port))
	}
}

type schemaCloseConn struct {
	net.Conn
	complete atomic.Bool
	failed   atomic.Bool
}

func (c *schemaCloseConn) Close() error {
	err := c.Conn.Close()
	c.failed.Store(err != nil)
	c.complete.Store(true)
	return err
}

// runSchemaProbe accepts launch only through a boolean supplied by the explicit
// test flag. A false gate performs no preflight, home lookup, or I/O.
func runSchemaProbe(parent context.Context, launch bool, d schemaDependencies) (r schemaReport) {
	r = newSchemaReport()
	if !launch {
		r.fail("preflight")
		return
	}
	started := time.Now()
	ctx, cancel := context.WithDeadline(parent, started.Add(25*time.Second))
	defer cancel()
	defer func() {
		r.ElapsedMillis = time.Since(started).Milliseconds()
		if r.FailureCategory == nil && r.CleanupOutcome == "complete" {
			r.Outcome = "schema_observed"
		}
	}()
	commit, digest, err := d.preflight(ctx)
	if err != nil {
		r.fail("preflight")
		return
	}
	r.CodeCommit, r.PlanDigest = schemaPtr(commit), schemaPtr(digest)
	id, err := newUUID()
	if err != nil {
		r.fail("preflight")
		return
	}
	r.RunID = &id
	if ctx.Err() != nil {
		r.fail("timeout")
		return
	}
	home, err := d.home()
	if err != nil {
		r.fail("config_invalid")
		return
	}
	store, err := d.open(home)
	if err != nil {
		switch {
		case errors.Is(err, configstore.ErrMissing):
			r.fail("config_missing")
		case errors.Is(err, configstore.ErrLocked):
			r.fail("busy")
		default:
			r.fail("config_invalid")
		}
		return
	}
	defer func() {
		if store.Close() != nil {
			r.CleanupOutcome = "failed"
			r.fail("cleanup")
		}
	}()
	document, exists, err := store.Load()
	if err != nil {
		r.fail("config_invalid")
		return
	}
	if !exists {
		r.fail("config_missing")
		return
	}
	if document.Validate() != nil || document.Session == nil || document.Models[probeModel] != probeModel {
		r.fail("config_invalid")
		return
	}
	if ctx.Err() != nil {
		r.fail("timeout")
		return
	}
	selected, err := d.reader(home).ReadProfileSnapshot(ctx, *document.Session)
	if err != nil {
		r.fail(schemaSourceCategory(err))
		return
	}
	if ctx.Err() != nil {
		r.fail("timeout")
		return
	}
	region := selected.ProfileRegion()
	endpoint := schemaEndpoint(region)
	if endpoint == "" {
		r.fail("unsupported_region")
		return
	}
	r.Region = &region
	token := selected.Credential()
	if !token.ExpiresAt().After(time.Now()) {
		r.fail("source_expired")
		return
	}
	if !config.VisibleASCII(token.AccessToken(), config.MaxBytes) {
		r.fail("source_unavailable")
		return
	}
	body, err := json.Marshal(struct {
		Origin     string `json:"origin"`
		MaxResults int    `json:"maxResults"`
		ProfileARN string `json:"profileArn"`
	}{"KIRO_CLI", 100, selected.ProfileARN()})
	if err != nil {
		r.fail("preflight")
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		r.fail("preflight")
		return
	}
	req.Header = http.Header{}
	for k, v := range map[string]string{
		"Authorization": "Bearer " + token.AccessToken(),
		"X-Amz-Target":  schemaTarget,
		"Content-Type":  "application/x-amz-json-1.0",
		"Accept":        "application/json", "Accept-Encoding": "identity",
		"User-Agent": "kiro-gateway-schema-probe/1", "X-Amzn-Codewhisperer-Optout": "true",
	} {
		req.Header.Set(k, v)
	}
	wire := d.wire
	// No default dial fallback to the production inference policy.
	if wire.dial == nil {
		r.fail("preflight")
		return
	}
	dial := wire.dial
	var owned *schemaCloseConn
	wire.dial = func(ctx context.Context, address string) (net.Conn, error) {
		conn, err := dial(ctx, address)
		if err != nil {
			return nil, err
		}
		owned = &schemaCloseConn{Conn: conn}
		return owned, nil
	}
	if ctx.Err() != nil {
		r.fail("timeout")
		return
	}
	r.DispatchCount = 1
	consumed := false
	err = wire.exchange(ctx, req, func(resp *http.Response) error {
		consumed = true
		consumeSchemaResponse(ctx, resp, &r)
		if r.FailureCategory != nil {
			return errors.New(*r.FailureCategory)
		}
		return nil
	})
	// Preserve a response failure even when transport cleanup also fails.
	if err != nil && r.FailureCategory == nil {
		if ctx.Err() != nil || bridge.SafeFailure(err).Category == "timed_out" {
			r.fail("timeout")
		} else if !consumed || bridge.SafeFailure(err).Category != "cleanup_failed" && (owned == nil || !owned.failed.Load()) {
			r.fail("transport")
		}
	}
	if owned != nil && (!owned.complete.Load() || owned.failed.Load()) || err != nil && bridge.SafeFailure(err).Category == "cleanup_failed" {
		r.CleanupOutcome = "failed"
		r.fail("cleanup")
	}
	return
}

func schemaSourceCategory(err error) string {
	switch {
	case errors.Is(err, credentials.ErrChanged):
		return "source_changed"
	case errors.Is(err, credentials.ErrExpired):
		return "source_expired"
	case errors.Is(err, credentials.ErrProfileUnsupported):
		return "unsupported_region"
	case errors.Is(err, credentials.ErrTimeout), errors.Is(err, credentials.ErrCanceled), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "source_unavailable"
	}
}

func consumeSchemaResponse(ctx context.Context, resp *http.Response, r *schemaReport) {
	if ctx.Err() != nil {
		r.fail("timeout")
		return
	}
	if resp.StatusCode != http.StatusOK {
		r.HTTPStatus = schemaPtr(resp.StatusCode)
		r.fail("http_status")
		return
	}
	media, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	encoding := resp.Header.Get("Content-Encoding")
	if err != nil || (media != "application/json" && media != "application/x-amz-json-1.0") || len(params) > 1 || (len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8")) || (encoding != "" && !strings.EqualFold(encoding, "identity")) || len(resp.Header.Values("Content-Type")) != 1 || len(resp.Header.Values("Content-Encoding")) > 1 || resp.ContentLength > schemaBodyLimit {
		r.fail("invalid_response")
		return
	}
	// Fixed capacity bounds the read before growth; the extra byte detects an
	// oversized body even with chunked framing or no declared content length.
	buf := make([]byte, schemaBodyLimit+1)
	used := 0
	for {
		n, readErr := resp.Body.Read(buf[used:])
		used += n
		if ctx.Err() != nil {
			r.fail("timeout")
			return
		}
		if used > schemaBodyLimit {
			r.fail("invalid_response")
			return
		}
		if readErr == io.EOF {
			break
		}
		// Keep HTTP truncation distinct from a normal body EOF. ReadFull would
		// turn both into ErrUnexpectedEOF for a body shorter than our buffer.
		if readErr != nil {
			r.fail("invalid_response")
			return
		}
	}
	extractSchema(buf[:used], r)
}

func encodeSchemaReport(r schemaReport) ([]byte, error) {
	b, err := json.Marshal(r)
	if err != nil || len(b) > 16<<10 {
		return nil, errors.New("invalid structural report")
	}
	return b, nil
}
