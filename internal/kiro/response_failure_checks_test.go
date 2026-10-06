//go:build responsediscovery

package kiro

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

type responseBrokenReader struct{ read func([]byte) (int, error) }

func (r responseBrokenReader) Read(b []byte) (int, error) { return r.read(b) }
func (responseBrokenReader) Close() error                 { return nil }

func TestResponseDiscoveryHTTPObservation(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		status               int
		media                string
		extra                http.Header
		body                 string
		length               int64
		failure, bodyFailure string
		read                 bool
	}{
		{name: "success", status: 200, media: "application/json", body: `{}`, length: 2, read: true},
		{name: "non200 projected", status: 403, media: "application/json", body: `{"__type":"secret#AccessDeniedException"}`, failure: "http_status", read: true},
		{name: "non200 invalid", status: 503, media: "application/json", body: `[]`, failure: "http_status", bodyFailure: "invalid_json", read: true},
		{name: "non200 media", status: 404, media: "text/html", body: "secret", failure: "http_status", bodyFailure: "unsupported_media"},
		{name: "other media", status: 200, media: "text/html", body: "{}", failure: "response_format", bodyFailure: "unsupported_media"},
		{name: "encoding", status: 200, media: "application/json", extra: http.Header{"Content-Encoding": {"gzip"}}, failure: "response_format", bodyFailure: "invalid_headers"},
		{name: "empty encoding", status: 200, media: "application/json", extra: http.Header{"Content-Encoding": {""}}, failure: "response_format", bodyFailure: "invalid_headers"},
		{name: "duplicate encoding", status: 200, media: "application/json", extra: http.Header{"Content-Encoding": {"identity", "identity"}}, failure: "response_format", bodyFailure: "invalid_headers"},
		{name: "duplicate type", status: 200, media: "application/json", extra: http.Header{"Content-Type": {"application/json", "application/json"}}, failure: "response_format", bodyFailure: "invalid_headers"},
		{name: "bad charset", status: 200, media: "application/json; charset=latin1", failure: "response_format", bodyFailure: "invalid_headers"},
		{name: "unknown parameter", status: 200, media: "text/event-stream; secret=value", failure: "response_format", bodyFailure: "invalid_headers"},
		{name: "binary parameter", status: 200, media: "application/vnd.amazon.eventstream; charset=utf-8", failure: "response_format", bodyFailure: "invalid_headers"},
		{name: "missing type", status: 200, failure: "response_format", bodyFailure: "invalid_headers"},
		{name: "length bound", status: 200, media: "application/json", length: responseBodyLimit + 1, failure: "response_limit", bodyFailure: "limit"},
		{name: "body bound", status: 200, media: "application/json", body: strings.Repeat(" ", responseBodyLimit+1), failure: "response_limit", bodyFailure: "limit", read: true},
		{name: "empty body", status: 200, media: "application/json", failure: "invalid_response", bodyFailure: "empty_body", read: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{}
			if tc.media != "" {
				header.Set("Content-Type", tc.media)
			}
			for k, v := range tc.extra {
				header[k] = v
			}
			reads := 0
			reader := strings.NewReader(tc.body)
			resp := &http.Response{StatusCode: tc.status, Header: header, ContentLength: tc.length, Body: responseBrokenReader{read: func(b []byte) (int, error) { reads++; return reader.Read(b) }}}
			r := newResponseReport()
			consumeResponse(t.Context(), resp, &r)
			got, body := "", ""
			if r.FailureCategory != nil {
				got = *r.FailureCategory
			}
			if r.BodyFailure != nil {
				body = *r.BodyFailure
			}
			if got != tc.failure || body != tc.bodyFailure || (reads > 0) != tc.read {
				t.Errorf("consume(%s) failure=%s body=%s reads=%d, want %s %s read=%t", tc.name, got, body, reads, tc.failure, tc.bodyFailure, tc.read)
			}
			if !tc.read && (r.TransportComplete != nil || r.DecodeComplete != nil || r.BodySummary != nil) {
				t.Errorf("consume(%s) retained observations for unread body", tc.name)
			}
			if tc.bodyFailure != "" && r.BodySummary != nil {
				t.Error("failed body kept summary")
			}
		})
	}
}

func TestResponseDiscoveryInterimStopsAtFirstHeader(t *testing.T) {
	for _, status := range []int{100, 101, 103} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			home, _ := probeHome(t)
			d := responseFixtureDependencies(t, home)
			requests := 0
			d.wire = responseTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				h := w.(http.Hijacker)
				conn, buffer, err := h.Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.Close()
				_, _ = buffer.WriteString("HTTP/1.1 " + strconv.Itoa(status) + " Interim\r\nContent-Type: text/private\r\n\r\nHTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 2\r\n\r\n{}")
				_ = buffer.Flush()
			})
			r := runResponseDiscovery(t.Context(), true, d)
			if r.FailureCategory == nil || *r.FailureCategory != "response_format" || r.BodyFailure == nil || *r.BodyFailure != "informational_response" || r.HTTPStatus != nil || r.MediaKind != nil || r.TransportComplete != nil || r.DecodeComplete != nil || r.BodySummary != nil || requests != 1 || r.CleanupOutcome != "complete" {
				t.Errorf("interim(%d)=%+v requests=%d, want null observations and one cleaned dispatch", status, r, requests)
			}
		})
	}
}

func TestResponseDiscoveryTruncatedBodyDiscardsPrefix(t *testing.T) {
	home, _ := probeHome(t)
	d := responseFixtureDependencies(t, home)
	d.wire = responseTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "200")
		_, _ = io.WriteString(w, `{}`)
	})
	r := runResponseDiscovery(t.Context(), true, d)
	if r.FailureCategory == nil || *r.FailureCategory != "transport" || r.BodySummary != nil || r.TransportComplete == nil || *r.TransportComplete || r.DecodeComplete != nil {
		t.Errorf("truncated body=%+v, want transport failure without summary", r)
	}
}

// covers: AC-17. A read may return bytes and EOF together. Only EOF grants a
// complete transport, and the detection byte must still enforce the body cap.
func TestResponseDiscoveryBodyReadBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, body, failure string
		readError           error
		transportComplete   bool
	}{
		{name: "bytes with EOF", body: `{}`, readError: io.EOF, transportComplete: true},
		{name: "exact body cap with EOF", body: `{}` + strings.Repeat(" ", (256<<10)-2), readError: io.EOF, transportComplete: true},
		{name: "detection byte with EOF", body: `{}` + strings.Repeat(" ", (256<<10)-1), readError: io.EOF, failure: "limit"},
		{name: "valid prefix with read error", body: `{}`, readError: errors.New("private-read-error"), failure: "transport"},
		{name: "empty EOF", readError: io.EOF, failure: "empty_body", transportComplete: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := 0
			resp := &http.Response{
				StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, ContentLength: -1,
				Body: responseBrokenReader{read: func(b []byte) (int, error) {
					reads++
					if reads > 1 {
						t.Errorf("consumeResponse(%s) reads = %d, want 1 after terminal read", tc.name, reads)
						return 0, io.EOF
					}
					return copy(b, tc.body), tc.readError
				}},
			}
			r := newResponseReport()
			consumeResponse(t.Context(), resp, &r)
			failure := ""
			if r.BodyFailure != nil {
				failure = *r.BodyFailure
			}
			if failure != tc.failure || reads != 1 || r.TransportComplete == nil || *r.TransportComplete != tc.transportComplete {
				t.Errorf("consumeResponse(%s) = %+v, reads = %d, want body failure %q, transport complete %t, reads 1", tc.name, r, reads, tc.failure, tc.transportComplete)
			}
			if tc.failure == "" {
				if r.BodySummary == nil || r.DecodeComplete == nil || !*r.DecodeComplete || r.FailureCategory != nil {
					t.Errorf("consumeResponse(%s) = %+v, want complete JSON summary", tc.name, r)
				}
			} else if r.BodySummary != nil || !tc.transportComplete && r.DecodeComplete != nil {
				t.Errorf("consumeResponse(%s) = %+v, want no summary or decoding after read failure", tc.name, r)
			}
			if bytes.Contains(encodeResponseReport(&r), []byte("private-read-error")) {
				t.Errorf("consumeResponse(%s) retained reader error text, want a fixed failure label", tc.name)
			}
		})
	}
}

type responseStoreHook struct {
	responseStore
	load  func() (config.Document, bool, error)
	close func() error
}

func (s responseStoreHook) Load() (config.Document, bool, error) {
	if s.load != nil {
		return s.load()
	}
	return s.responseStore.Load()
}
func (s responseStoreHook) Close() error {
	if s.close != nil {
		return s.close()
	}
	return s.responseStore.Close()
}

func TestResponseDiscoveryStopsBeforeAccount(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		setup      func(*responseDependencies, string)
	}{
		{"preflight", "preflight", func(d *responseDependencies, _ string) {
			d.preflight = func(context.Context) (string, string, string, error) { return "", "", "", errPlanInvalid }
		}},
		{"UUID", "preflight", func(d *responseDependencies, _ string) { d.uuid = func() (string, error) { return "", errPlanInvalid } }},
		{"home", "config_invalid", func(d *responseDependencies, _ string) { d.home = func() (string, error) { return "", errPlanInvalid } }},
		{"missing", "config_missing", func(d *responseDependencies, _ string) {
			d.open = func(string) (responseStore, error) { return nil, configstore.ErrMissing }
		}},
		{"busy", "busy", func(d *responseDependencies, h string) {
			s, e := configstore.OpenExisting(h)
			if e != nil {
				panic(e)
			}
			t.Cleanup(func() { s.Close() })
		}},
		{"invalid", "config_invalid", func(d *responseDependencies, _ string) {
			open := d.open
			d.open = func(h string) (responseStore, error) {
				s, e := open(h)
				return responseStoreHook{responseStore: s, load: func() (config.Document, bool, error) { return config.Document{}, true, errPlanInvalid }}, e
			}
		}},
		{"mapping", "config_invalid", func(d *responseDependencies, _ string) {
			open := d.open
			d.open = func(h string) (responseStore, error) {
				s, e := open(h)
				return responseStoreHook{responseStore: s, load: func() (config.Document, bool, error) {
					doc, ok, e := s.Load()
					delete(doc.Models, probeModel)
					return doc, ok, e
				}}, e
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			d := responseFixtureDependencies(t, home)
			tc.setup(&d, home)
			reads := 0
			d.reader = func(string) ProfileReader { reads++; return nil }
			r := runResponseDiscovery(t.Context(), true, d)
			if r.FailureCategory == nil || *r.FailureCategory != tc.want || r.DispatchCount != 0 || reads != 0 || r.CleanupOutcome != "complete" {
				t.Errorf("predispatch(%s)=%+v reads=%d, want %s before account access", tc.name, r, reads, tc.want)
			}
		})
	}
}

func TestResponseDiscoverySourceFailureAndRegion(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		err        error
	}{
		{"changed", "source_changed", credentials.ErrChanged}, {"expired", "source_expired", credentials.ErrExpired}, {"unsupported", "unsupported_region", credentials.ErrProfileUnsupported}, {"unavailable", "source_unavailable", errPlanInvalid}, {"source timeout", "timeout", credentials.ErrTimeout}, {"source canceled", "canceled", credentials.ErrCanceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			d := responseFixtureDependencies(t, home)
			reads := 0
			d.reader = func(string) ProfileReader {
				return wireSnapshotFunc(func(context.Context, config.Session) (credentials.ProfileSnapshot, error) {
					reads++
					return credentials.ProfileSnapshot{}, tc.err
				})
			}
			r := runResponseDiscovery(t.Context(), true, d)
			if r.FailureCategory == nil || *r.FailureCategory != tc.want || r.Region != nil || r.DispatchCount != 0 || reads != 1 {
				t.Errorf("source(%s)=%+v reads=%d, want %s once", tc.name, r, reads, tc.want)
			}
		})
	}
	home, db := probeHome(t)
	_, err := db.Exec(`UPDATE state SET value=?`, strings.Replace(probeSyntheticProfile, "us-east-1", "eu-central-1", 1))
	if err != nil {
		t.Fatal(err)
	}
	r := runResponseDiscovery(t.Context(), true, responseFixtureDependencies(t, home))
	if r.FailureCategory == nil || *r.FailureCategory != "region_mismatch" || r.DispatchCount != 0 || r.Region == nil || *r.Region != "eu-central-1" {
		t.Errorf("region mismatch=%+v, want snapshot region before zero dispatch", r)
	}
}

// Each cancellation is observed at the same boundary as an injected phase
// failure. The first established cause must follow that observation order.
func TestResponseDiscoveryCancellationBoundaries(t *testing.T) {
	for _, phase := range []string{"before", "preflight", "uuid", "home", "open", "load", "source", "dial", "body", "encoding"} {
		t.Run(phase, func(t *testing.T) {
			home, _ := probeHome(t)
			d := responseFixtureDependencies(t, home)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch phase {
			case "before":
				cancel()
			case "preflight":
				d.preflight = func(context.Context) (string, string, string, error) { cancel(); return "", "", "", errPlanInvalid }
			case "uuid":
				d.uuid = func() (string, error) { cancel(); return "", errPlanInvalid }
			case "home":
				d.home = func() (string, error) { cancel(); return "", errPlanInvalid }
			case "open":
				d.open = func(string) (responseStore, error) { cancel(); return nil, configstore.ErrMissing }
			case "load":
				open := d.open
				d.open = func(h string) (responseStore, error) {
					s, e := open(h)
					return responseStoreHook{responseStore: s, load: func() (config.Document, bool, error) { cancel(); return config.Document{}, false, errPlanInvalid }}, e
				}
			case "source":
				d.reader = func(string) ProfileReader {
					return wireSnapshotFunc(func(context.Context, config.Session) (credentials.ProfileSnapshot, error) {
						cancel()
						return credentials.ProfileSnapshot{}, credentials.ErrChanged
					})
				}
			case "dial":
				d.wire.dial = func(context.Context, string) (net.Conn, error) { cancel(); return nil, errPlanInvalid }
			case "body":
				d.wire = responseTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Content-Length", "2")
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
					cancel()
				})
			case "encoding":
				d.wire = responseTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{}`)
				})
				d.encode = func(r *responseReport) []byte { cancel(); return encodeResponseReport(r) }
			}
			r := runResponseDiscovery(ctx, true, d)
			if r.FailureCategory == nil || *r.FailureCategory != "canceled" || r.Outcome != "needs_evidence" || r.CleanupOutcome != "complete" {
				t.Errorf("cancel at %s=%+v, want canceled with complete cleanup", phase, r)
			}
		})
	}
}

func TestResponseDiscoveryFirstCauseAndBodyCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r := newResponseReport()
	resp := &http.Response{StatusCode: 403, Header: http.Header{"Content-Type": {"application/json"}}, Body: responseBrokenReader{read: func([]byte) (int, error) { cancel(); return 0, errors.New("private") }}}
	consumeResponse(ctx, resp, &r)
	if r.FailureCategory == nil || *r.FailureCategory != "http_status" || r.BodyFailure == nil || *r.BodyFailure != "canceled" || r.BodySummary != nil {
		t.Errorf("non200 then cancel=%+v, want http_status preserved and canceled body", r)
	}
}

type responseCloseHook struct {
	net.Conn
	close func() error
}

func (c responseCloseHook) Close() error { return c.close() }
func TestResponseDiscoveryCleanupBoundaries(t *testing.T) {
	for _, phase := range []string{"connection error", "body error", "lock error", "parent canceled", "work cutoff", "absolute deadline"} {
		t.Run(phase, func(t *testing.T) {
			home, _ := probeHome(t)
			d := responseFixtureDependencies(t, home)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			d.wire = responseTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if phase == "absolute deadline" {
					_, _ = io.Copy(io.Discard, r.Body)
					<-r.Context().Done()
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{}`)
			})
			var ownedStore responseStore
			baseOpen := d.open
			d.open = func(h string) (responseStore, error) {
				s, e := baseOpen(h)
				if e == nil {
					ownedStore = s
				}
				return s, e
			}
			t.Cleanup(func() {
				if ownedStore != nil {
					ownedStore.Close()
				}
			})
			if phase == "lock error" {
				open := d.open
				d.open = func(h string) (responseStore, error) {
					s, e := open(h)
					return responseStoreHook{responseStore: s, close: func() error { _ = s.Close(); return errPlanInvalid }}, e
				}
			} else if phase == "body error" {
				// Direct worker exercise checks body close errors without changing HTTP parsing.
				client, server := net.Pipe()
				t.Cleanup(func() { server.Close() })
				owner := newResponseConnection(ctx, client)
				owner.responseBody = responseCloseBody{}
				if complete, _ := owner.finish(time.Now().Add(time.Second), time.Second); complete {
					t.Error("body close error reported complete")
				}
				return
			} else {
				dial := d.wire.dial
				d.wire.dial = func(ctx context.Context, address string) (net.Conn, error) {
					conn, e := dial(ctx, address)
					if e != nil {
						return nil, e
					}
					return responseCloseHook{Conn: conn, close: func() error {
						err := conn.Close()
						switch phase {
						case "connection error":
							return errPlanInvalid
						case "parent canceled":
							cancel()
						case "work cutoff":
							time.Sleep(600 * time.Millisecond)
						case "absolute deadline":
							time.Sleep(100 * time.Millisecond)
						}
						return err
					}}, nil
				}
			}
			if phase == "work cutoff" {
				d.total = 2 * time.Second
				d.cleanup = 1500 * time.Millisecond
			}
			if phase == "absolute deadline" {
				d.total = 150 * time.Millisecond
				d.cleanup = 100 * time.Millisecond
			}
			r := runResponseDiscovery(ctx, true, d)
			want := "cleanup"
			if phase == "parent canceled" {
				want = "canceled"
			}
			if phase == "work cutoff" {
				want = ""
			}
			if phase == "absolute deadline" {
				want = "timeout"
				time.Sleep(110 * time.Millisecond)
			}
			got := ""
			if r.FailureCategory != nil {
				got = *r.FailureCategory
			}
			if got != want {
				t.Errorf("cleanup(%s)=%+v, want %s", phase, r, want)
			}
			if strings.HasSuffix(phase, "error") && (r.BodySummary != nil || r.CleanupOutcome != "failed") {
				t.Error("cleanup error retained body or reported complete")
			}
		})
	}
}

type responseCloseBody struct{}

func (responseCloseBody) Read([]byte) (int, error) { return 0, io.EOF }
func (responseCloseBody) Close() error             { return errPlanInvalid }

type responseBlockedWriter struct{}

func (responseBlockedWriter) Write([]byte) (int, error) { select {} }

// The permanent fake close exists only in a disposable child process. Its
// settings lock must remain held until the bounded report attempt exits it.
func TestResponseDiscoveryFailedCleanupProcess(t *testing.T) {
	if mode := os.Getenv("KIRO_SYNTHETIC_DISCOVERY_CHILD"); mode != "" {
		home, _ := probeHome(t)
		d := responseFixtureDependencies(t, home)
		d.cleanup = 30 * time.Millisecond
		d.total = 2 * time.Second
		d.wire = responseTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if mode == "http_error" {
				w.WriteHeader(403)
			}
			_, _ = io.WriteString(w, `{}`)
		})
		if mode == "blocked_lock" {
			open := d.open
			d.open = func(h string) (responseStore, error) {
				store, e := open(h)
				return responseStoreHook{responseStore: store, close: func() error { select {} }}, e
			}
		} else {
			dial := d.wire.dial
			d.wire.dial = func(ctx context.Context, address string) (net.Conn, error) {
				conn, e := dial(ctx, address)
				if e != nil {
					return nil, e
				}
				return responseCloseHook{Conn: conn, close: func() error { _ = conn.Close(); select {} }}, nil
			}
		}
		r := runResponseDiscovery(t.Context(), true, d)
		wantFailure := "cleanup"
		if mode == "http_error" {
			wantFailure = "http_status"
		}
		if r.CleanupOutcome != "failed" || r.FailureCategory == nil || *r.FailureCategory != wantFailure || r.BodySummary != nil {
			os.Exit(41)
		}
		lock, err := configstore.OpenExisting(home)
		if lock != nil {
			lock.Close()
		}
		if !errors.Is(err, configstore.ErrLocked) {
			os.Exit(42)
		}
		if mode == "blocked_output" {
			_, _ = io.WriteString(os.Stdout, "synthetic_lock_retained\n")
			emitResponseReport(r, responseBlockedWriter{})
		} else {
			emitResponseReport(r, os.Stdout)
		}
		os.Exit(43)
	}
	for _, mode := range []string{"report", "blocked_output", "blocked_lock", "http_error"} {
		t.Run(mode, func(t *testing.T) {
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "-test.run=^TestResponseDiscoveryFailedCleanupProcess$", "-test.count=1", "-response-discovery-launch=false")
			cmd.Env = append(os.Environ(), "KIRO_SYNTHETIC_DISCOVERY_CHILD="+mode, "TMPDIR="+t.TempDir())
			started := time.Now()
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || time.Since(started) > 3*time.Second {
				t.Fatalf("failed cleanup child(%s)=%v after %v, output=%s, want exit 1 within 3 seconds", mode, err, time.Since(started), out)
			}
			if mode != "blocked_output" && (!bytes.Contains(out, []byte(`"cleanup_outcome":"failed"`)) || !bytes.Contains(out, []byte(`"body_summary":null`)) || bytes.Contains(out, []byte("sentinel"))) {
				t.Errorf("child report=%s, want bounded redacted failed cleanup", out)
			}
			if mode == "blocked_output" && !bytes.Contains(out, []byte("synthetic_lock_retained")) {
				t.Error("blocked writer child did not prove retained lock")
			}
		})
	}
}
