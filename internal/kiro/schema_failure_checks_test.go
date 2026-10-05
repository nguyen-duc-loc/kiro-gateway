//go:build schemaprobe

package kiro

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

func TestSchemaProbeDisabledAndPreflight(t *testing.T) {
	for _, key := range []string{"KIRO_GATEWAY_LIVE_PROBE", "KIRO_GATEWAY_SCHEMA_PROBE", "KIRO_GATEWAY_SCHEMA_PROBE_LAUNCH"} {
		t.Setenv(key, "1")
	}
	r := runSchemaProbe(t.Context(), false, schemaDependencies{})
	if r.FailureCategory == nil || *r.FailureCategory != "preflight" || r.RunID != nil || r.DispatchCount != 0 || r.CleanupOutcome != "complete" {
		t.Errorf("runSchemaProbe(disabled) = %+v, want preflight with no access", r)
	}
	r = runSchemaProbe(t.Context(), true, schemaDependencies{preflight: func(context.Context) (string, string, error) { return "sentinel", "sentinel", errPlanInvalid }})
	if r.RunID != nil || r.CodeCommit != nil || r.PlanDigest != nil || r.DispatchCount != 0 {
		t.Errorf("runSchemaProbe(failed preflight) = %+v, want no provenance or access", r)
	}
}

func TestSchemaProbeConfigFailures(t *testing.T) {
	for _, name := range []string{"missing directory", "missing file", "invalid", "unlinked", "mapping", "busy"} {
		t.Run(name, func(t *testing.T) {
			home, _ := probeHome(t)
			path := filepath.Join(home, ".config", "kiro-gateway", "config.json")
			want := "config_invalid"
			switch name {
			case "missing directory":
				home = t.TempDir()
				want = "config_missing"
			case "missing file":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				want = "config_missing"
			case "invalid":
				if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "busy":
				store, err := configstore.OpenExisting(home)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { store.Close() })
				want = "busy"
			default:
				store, err := configstore.OpenExisting(home)
				if err != nil {
					t.Fatal(err)
				}
				doc, _, err := store.Load()
				if err != nil {
					t.Fatal(err)
				}
				if name == "unlinked" {
					doc.Forget()
				} else {
					delete(doc.Models, probeModel)
				}
				if err := store.Save(doc, false); err != nil {
					t.Fatal(err)
				}
				store.Close()
			}
			d := schemaFixtureDependencies(t, home)
			d.reader = func(string) ProfileReader {
				t.Error("runSchemaProbe(config failure) opened reader, want no account access")
				return nil
			}
			r := runSchemaProbe(t.Context(), true, d)
			if r.FailureCategory == nil || *r.FailureCategory != want || r.DispatchCount != 0 || r.ModelFound != nil {
				t.Errorf("runSchemaProbe(%s) = %+v, want %s before source", name, r, want)
			}
			if name == "missing directory" {
				entries, err := os.ReadDir(home)
				if err != nil || len(entries) != 0 {
					t.Error("runSchemaProbe(missing directory) created paths")
				}
			}
		})
	}
}

func TestSchemaProbeSourceFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"changed", credentials.ErrChanged, "source_changed"}, {"expired", credentials.ErrExpired, "source_expired"}, {"unsafe", credentials.ErrSource, "source_unavailable"}, {"record", credentials.ErrRecord, "source_unavailable"}, {"profile", credentials.ErrProfileInvalid, "source_unavailable"}, {"region", credentials.ErrProfileUnsupported, "unsupported_region"}, {"source busy", credentials.ErrBusy, "source_unavailable"}, {"timeout", credentials.ErrTimeout, "timeout"}, {"cancel", credentials.ErrCanceled, "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			d := schemaFixtureDependencies(t, home)
			reads := 0
			d.reader = func(string) ProfileReader {
				return wireSnapshotFunc(func(ctx context.Context, _ config.Session) (credentials.ProfileSnapshot, error) {
					reads++
					if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 25*time.Second {
						t.Error("snapshot deadline absent or extended")
					}
					return credentials.ProfileSnapshot{}, tc.err
				})
			}
			r := runSchemaProbe(t.Context(), true, d)
			if r.FailureCategory == nil || *r.FailureCategory != tc.want || reads != 1 || r.DispatchCount != 0 {
				t.Errorf("runSchemaProbe(%s) = %+v reads=%d, want %s once before dispatch", tc.name, r, reads, tc.want)
			}
		})
	}
}

func TestSchemaProbeChangedRealFixture(t *testing.T) {
	home, db := probeHome(t)
	if _, err := db.Exec(`UPDATE auth_kv SET value=?`, strings.Replace(probeSyntheticRecord, "sentinel-token", "replacement-token", 1)); err != nil {
		t.Fatal(err)
	}
	r := runSchemaProbe(t.Context(), true, schemaFixtureDependencies(t, home))
	if r.FailureCategory == nil || *r.FailureCategory != "source_changed" || r.DispatchCount != 0 {
		t.Errorf("runSchemaProbe(changed SQLite) = %+v, want source_changed before dispatch", r)
	}
}

func TestSchemaResponseBounds(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, encoding, body string
		length                            int64
		failure                           string
	}{
		{"valid", "application/json", "", schemaFixture, -1, ""},
		{"media", "text/plain", "", schemaFixture, -1, "invalid_response"},
		{"charset", "application/json; charset=latin1", "", schemaFixture, -1, "invalid_response"},
		{"parameter", "application/json; other=utf-8", "", schemaFixture, -1, "invalid_response"},
		{"encoding", "application/json", "gzip", schemaFixture, -1, "invalid_response"},
		{"declared oversize", "application/json", "", "", schemaBodyLimit + 1, "invalid_response"},
		{"stream oversize", "application/json", "", strings.Repeat(" ", schemaBodyLimit+1), -1, "invalid_response"},
		{"exact byte limit", "application/json", "", schemaFixture + strings.Repeat(" ", schemaBodyLimit-len(schemaFixture)), schemaBodyLimit, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newSchemaReport()
			resp := &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: tc.length, Body: io.NopCloser(strings.NewReader(tc.body))}
			resp.Header.Set("Content-Type", tc.contentType)
			if tc.encoding != "" {
				resp.Header.Set("Content-Encoding", tc.encoding)
			}
			consumeSchemaResponse(t.Context(), resp, &r)
			got := ""
			if r.FailureCategory != nil {
				got = *r.FailureCategory
			}
			if got != tc.failure {
				t.Errorf("consumeSchemaResponse(%s) failure=%q, want %q", tc.name, got, tc.failure)
			}
		})
	}
}

// covers: AC-16. HTTP failures never trigger retry, redirect, or proxy access.
func TestSchemaProbeNoRedirectRetryProxyOrUntrustedTLS(t *testing.T) {
	var accidental atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { accidental.Add(1) }))
	t.Cleanup(proxy.Close)
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")
	for _, status := range []int{301, 307, 401, 403, 429, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			home, _ := probeHome(t)
			d := schemaFixtureDependencies(t, home)
			var calls atomic.Int32
			d.wire = schemaTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Location", proxy.URL)
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "sentinel upstream error")
			})
			r := runSchemaProbe(t.Context(), true, d)
			if r.FailureCategory == nil || *r.FailureCategory != "http_status" || r.HTTPStatus == nil || *r.HTTPStatus != status || r.DispatchCount != 1 || calls.Load() != 1 || accidental.Load() != 0 {
				t.Errorf("runSchemaProbe(HTTP %d) = %+v calls=%d proxy=%d, want one HTTP failure", status, r, calls.Load(), accidental.Load())
			}
		})
	}
	home, _ := probeHome(t)
	d := schemaFixtureDependencies(t, home)
	d.wire = schemaTLSFixture(t, func(http.ResponseWriter, *http.Request) { t.Error("untrusted TLS reached HTTP") })
	d.wire.roots = x509.NewCertPool()
	r := runSchemaProbe(t.Context(), true, d)
	if r.FailureCategory == nil || *r.FailureCategory != "transport" || r.DispatchCount != 1 {
		t.Errorf("runSchemaProbe(untrusted TLS) = %+v, want transport failure once", r)
	}
}

func TestSchemaDestinationPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, address         string
		ips                   []netip.Addr
		wantResolve, wantDial int
	}{
		{"east", "management.us-east-1.kiro.dev:443", []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("1.1.1.1")}, 1, 1},
		{"europe", "management.eu-central-1.kiro.dev:443", []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 1, 1},
		{"runtime", "runtime.us-east-1.kiro.dev:443", nil, 0, 0}, {"port", "management.us-east-1.kiro.dev:444", nil, 0, 0}, {"lookalike", "management.us-east-1.kiro.dev.evil:443", nil, 0, 0},
		{"private", "management.us-east-1.kiro.dev:443", []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}, 1, 0},
		{"empty", "management.us-east-1.kiro.dev:443", nil, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolves, dials := 0, 0
			dial := schemaDial(func(_ context.Context, network, host string) ([]netip.Addr, error) {
				resolves++
				if network != "ip4" {
					t.Errorf("schemaDial network=%s, want ip4", network)
				}
				return tc.ips, nil
			}, func(_ context.Context, network, address string) (net.Conn, error) {
				dials++
				if network != "tcp4" || address != "8.8.8.8:443" {
					t.Errorf("schemaDial target=%s/%s, want first public IPv4", network, address)
				}
				return nil, errors.New("sentinel connect failure")
			})
			_, _ = dial(t.Context(), tc.address)
			if resolves != tc.wantResolve || dials != tc.wantDial {
				t.Errorf("schemaDial(%s) resolves=%d dials=%d, want %d/%d without fallback", tc.name, resolves, dials, tc.wantResolve, tc.wantDial)
			}
		})
	}
	for _, host := range []string{"management.us-east-1.kiro.dev:443", "management.eu-central-1.kiro.dev:443"} {
		if _, err := publicDial(t.Context(), host); err == nil {
			t.Errorf("publicDial(%s) accepted research host, want rejection before DNS", host)
		}
	}
}

type schemaMemoryStore struct {
	doc      config.Document
	closeErr error
	closed   bool
}

func (s *schemaMemoryStore) Load() (config.Document, bool, error) { return s.doc, true, nil }
func (s *schemaMemoryStore) Close() error                         { s.closed = true; return s.closeErr }

func schemaMemoryDependencies(t *testing.T) (schemaDependencies, *schemaMemoryStore) {
	t.Helper()
	home, _ := probeHome(t)
	store, err := configstore.OpenExisting(home)
	if err != nil {
		t.Fatal(err)
	}
	doc, _, err := store.Load()
	store.Close()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := (credentials.Reader{Home: home}).ReadProfileSnapshot(t.Context(), *doc.Session)
	if err != nil {
		t.Fatal(err)
	}
	memory := &schemaMemoryStore{doc: doc}
	d := schemaFixtureDependencies(t, home)
	d.open = func(string) (schemaStore, error) { return memory, nil }
	d.reader = func(string) ProfileReader {
		return wireSnapshotFunc(func(context.Context, config.Session) (credentials.ProfileSnapshot, error) { return snapshot, nil })
	}
	return d, memory
}

func schemaPipe(t *testing.T, delay time.Duration, response string) (transport, *slowPipeClose) {
	t.Helper()
	cert, roots := schemaCertificate(t)
	client, server := net.Pipe()
	conn := &slowPipeClose{Conn: client, delay: delay}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer server.Close()
		secure := tls.Server(server, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
		req, err := http.ReadRequest(bufio.NewReader(secure))
		if err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()
		if response != "" {
			_, _ = io.WriteString(secure, response)
		}
		// Keep the peer alive until client cleanup. Closing a net.Pipe peer
		// immediately can make SetReadDeadline fail after a successful read.
		_, _ = io.Copy(io.Discard, secure)
	}()
	t.Cleanup(func() { client.Close(); server.Close(); <-done })
	return transport{roots: roots, dial: func(context.Context, string) (net.Conn, error) { return conn, nil }}, conn
}

func TestSchemaProbeCleanupAndTotalBudget(t *testing.T) {
	for _, tc := range []struct {
		name, response, want string
		delay                time.Duration
		preflightDelay       time.Duration
		elapsed              time.Duration
		cleanup              string
	}{
		{"success", schemaFixture, "", time.Second, 0, time.Second, "complete"},
		{"cleanup timeout", schemaFixture, "cleanup", 6 * time.Second, 0, 5 * time.Second, "failed"},
		{"first failure", `{}`, "invalid_response", 6 * time.Second, 0, 5 * time.Second, "failed"},
		{"work cutoff", "", "timeout", 0, 24 * time.Second, 25 * time.Second, "complete"},
		{"total cutoff", "", "timeout", 6 * time.Second, 24 * time.Second, 30 * time.Second, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, store := schemaMemoryDependencies(t)
			synctest.Test(t, func(t *testing.T) {
				response := ""
				if tc.response != "" {
					response = fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(tc.response), tc.response)
				}
				wire, conn := schemaPipe(t, tc.delay, response)
				d.wire = wire
				preflight := d.preflight
				d.preflight = func(ctx context.Context) (string, string, error) {
					time.Sleep(tc.preflightDelay)
					return preflight(ctx)
				}
				start := time.Now()
				r := runSchemaProbe(t.Context(), true, d)
				failure := ""
				if r.FailureCategory != nil {
					failure = *r.FailureCategory
				}
				if failure != tc.want || r.CleanupOutcome != tc.cleanup || time.Since(start) != tc.elapsed || !store.closed || r.DispatchCount != 1 {
					t.Errorf("runSchemaProbe(%s) = %+v elapsed=%v closed=%t, want %s cleanup=%s elapsed=%v", tc.name, r, time.Since(start), store.closed, tc.want, tc.cleanup, tc.elapsed)
				}
				if tc.delay > 5*time.Second {
					time.Sleep(time.Second)
					synctest.Wait()
				}
				if conn.calls.Load() != 1 {
					t.Errorf("runSchemaProbe(%s) close calls=%d, want one", tc.name, conn.calls.Load())
				}
			})
		})
	}
}

func TestSchemaProbeLockCleanupFailure(t *testing.T) {
	d, store := schemaMemoryDependencies(t)
	store.closeErr = errors.New("sentinel close")
	d.wire = schemaTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, schemaFixture)
	})
	r := runSchemaProbe(t.Context(), true, d)
	if r.FailureCategory == nil || *r.FailureCategory != "cleanup" || r.CleanupOutcome != "failed" || r.Outcome != "needs_evidence" {
		t.Errorf("runSchemaProbe(lock close error) = %+v, want cleanup failure", r)
	}
}

func TestSchemaProbeParentCancellation(t *testing.T) {
	home, _ := probeHome(t)
	d := schemaFixtureDependencies(t, home)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	d.wire = schemaTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[`)
		w.(http.Flusher).Flush()
		cancel()
		<-r.Context().Done()
	})
	r := runSchemaProbe(ctx, true, d)
	if r.FailureCategory == nil || *r.FailureCategory != "timeout" || r.DispatchCount != 1 || r.CleanupOutcome != "complete" {
		t.Errorf("runSchemaProbe(parent cancel) = %+v, want timeout and completed cleanup", r)
	}
}

// covers: AC-16. Plan approval, provenance, and the fixed contract are required.
func TestSchemaPlanGate(t *testing.T) {
	commit := strings.Repeat("a", 40)
	base := schemaPlan{Version: 1, CodeCommit: commit, Contract: schemaContract(), Software: []schemaSoftware{{Role: "native_binary", Path: "/synthetic/native", SHA256: strings.Repeat("b", 64)}, {Role: "bundled_source", Path: "/synthetic/source", SHA256: strings.Repeat("c", 64)}}, SyntheticChecks: []string{"scripts/check:pass", "schema_probe_race:pass"}, IndependentReview: schemaPtr("synthetic review"), LiveApproval: schemaPtr("approved_for_one_catalogue_run")}
	for _, tc := range []struct {
		name   string
		change func(*schemaPlan)
	}{
		{"valid", func(*schemaPlan) {}}, {"no approval", func(p *schemaPlan) { p.LiveApproval = nil }}, {"no review", func(p *schemaPlan) { p.IndependentReview = nil }}, {"wrong model", func(p *schemaPlan) { p.Contract["model"] = "other" }}, {"changed code", func(p *schemaPlan) { p.CodeCommit = strings.Repeat("b", 40) }}, {"missing check", func(p *schemaPlan) { p.SyntheticChecks = nil }}, {"missing source", func(p *schemaPlan) { p.Software = p.Software[:1] }},
		{name: "blank review", change: func(p *schemaPlan) { p.IndependentReview = schemaPtr(" \n\t") }},
		{name: "wrong approval", change: func(p *schemaPlan) { p.LiveApproval = schemaPtr("approved_for_inference") }},
		{name: "wrong version", change: func(p *schemaPlan) { p.Version = 2 }},
		{name: "relative source", change: func(p *schemaPlan) { p.Software[1].Path = "source" }},
		{name: "invalid source digest", change: func(p *schemaPlan) { p.Software[1].SHA256 = strings.Repeat("z", 64) }},
		{name: "wrong source role", change: func(p *schemaPlan) { p.Software[1].Role = "native_binary" }},
		{name: "failed check", change: func(p *schemaPlan) { p.SyntheticChecks = []string{"scripts/check:pass", "schema_probe_race:fail"} }},
		{name: "changed destination", change: func(p *schemaPlan) { p.Contract["destinations"] = []any{"https://example.invalid/"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			p.Software = append([]schemaSoftware(nil), base.Software...)
			p.Contract = schemaContract()
			tc.change(&p)
			b, _ := json.Marshal(p)
			sum := sha256.Sum256(b)
			_, err := readSchemaPlan(b, hex.EncodeToString(sum[:]), commit)
			if (err == nil) != (tc.name == "valid") {
				t.Errorf("readSchemaPlan(%s) error=%v, want valid=%t", tc.name, err, tc.name == "valid")
			}
		})
	}
	b, _ := json.Marshal(base)
	sum := sha256.Sum256(b)
	if _, err := readSchemaPlan(append(b, ' '), hex.EncodeToString(sum[:]), commit); err == nil {
		t.Error("readSchemaPlan(changed bytes) accepted stale digest")
	}
	duplicate := bytes.Replace(b, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
	sum = sha256.Sum256(duplicate)
	if _, err := readSchemaPlan(duplicate, hex.EncodeToString(sum[:]), commit); err == nil {
		t.Error("readSchemaPlan(duplicate member) accepted ambiguous plan")
	}
}

// covers: AC-16. The report has an exact byte ceiling and returns no partial
// output when that ceiling is exceeded.
func TestSchemaReportByteLimit(t *testing.T) {
	r := newSchemaReport()
	r.CodeCommit = schemaPtr("")
	base, err := encodeSchemaReport(r)
	if err != nil {
		t.Fatalf("encodeSchemaReport(empty commit) error = %v, want nil", err)
	}
	// ASCII padding makes the encoded size exact without depending on field order.
	r.CodeCommit = schemaPtr(strings.Repeat("a", (16<<10)-len(base)))
	b, err := encodeSchemaReport(r)
	if err != nil || len(b) != 16<<10 {
		t.Errorf("encodeSchemaReport(16384 bytes) size=%d error=%v, want 16384 and nil", len(b), err)
	}
	r.CodeCommit = schemaPtr(*r.CodeCommit + "a")
	b, err = encodeSchemaReport(r)
	if err == nil || b != nil {
		t.Errorf("encodeSchemaReport(16385 bytes) size=%d error=%v, want nil output and an error", len(b), err)
	}
}

// covers: AC-16. Provenance hashes cover the complete bytes without executing
// the file, including files larger than one read buffer.
func TestSchemaHashFileReadsExactBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native")
	for _, content := range []string{"", "not an executable\n" + strings.Repeat("synthetic", 8192)} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("WriteFile(synthetic native) error = %v, want nil", err)
		}
		sum := sha256.Sum256([]byte(content))
		want := hex.EncodeToString(sum[:])
		got, err := schemaHashFile(t.Context(), path)
		if err != nil || got != want {
			t.Errorf("schemaHashFile(%d bytes) = %q, %v, want %q, nil", len(content), got, err, want)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := schemaHashFile(ctx, path)
	if !errors.Is(err, errPlanInvalid) || got != "" {
		t.Errorf("schemaHashFile(canceled) = %q, %v, want empty digest and errPlanInvalid", got, err)
	}
}

// covers: AC-16. The pinned native software exceeds 1 GiB. A sparse synthetic
// file checks the complete digest at that boundary without reading real software.
func TestSchemaHashFileAboveOneGiB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(synthetic native) = %v, want file", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	const size int64 = (1 << 30) + 1
	if err := f.Truncate(size); err != nil {
		t.Fatalf("Truncate(synthetic native, %d) = %v, want nil", size, err)
	}
	// A final nonzero byte proves hashing does not stop at the old ceiling.
	if _, err := f.WriteAt([]byte{1}, size-1); err != nil {
		t.Fatalf("WriteAt(synthetic native, %d) = %v, want nil", size-1, err)
	}
	got, err := schemaHashFile(t.Context(), path)
	if err != nil {
		t.Fatalf("schemaHashFile(%d bytes) = %q, %v, want full digest and nil", size, got, err)
	}
	h := sha256.New()
	zeros := make([]byte, 1<<20)
	for remaining := size - 1; remaining > 0; remaining -= int64(len(zeros)) {
		_, _ = h.Write(zeros)
	}
	_, _ = h.Write([]byte{1})
	want := hex.EncodeToString(h.Sum(nil))
	if got != want {
		t.Errorf("schemaHashFile(%d bytes) = %q, want %q", size, got, want)
	}
}

// covers: AC-16. Uninspectable or oversized software inputs cannot satisfy
// the provenance check. All files are synthetic and remain in a temporary home.
func TestSchemaHashFileRejectsInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("synthetic"), 0600); err != nil {
		t.Fatalf("WriteFile(target) error = %v, want nil", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink(target) error = %v, want nil", err)
	}
	oversized := filepath.Join(dir, "oversized")
	f, err := os.Create(oversized)
	if err != nil {
		t.Fatalf("Create(oversized) error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	if err := f.Truncate((2 << 30) + 1); err != nil {
		t.Fatalf("Truncate(oversized) error = %v, want nil", err)
	}
	for _, tc := range []struct{ name, path string }{
		{name: "missing", path: filepath.Join(dir, "missing")},
		{name: "directory", path: dir},
		{name: "symlink", path: link},
		{name: "oversized", path: oversized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := schemaHashFile(t.Context(), tc.path)
			if !errors.Is(err, errPlanInvalid) || got != "" {
				t.Errorf("schemaHashFile(%s) = %q, %v, want empty digest and errPlanInvalid", tc.name, got, err)
			}
		})
	}
}

// covers: AC-16. DNS failure stops before a connection and has no fallback.
func TestSchemaDestinationStopsOnDNSFailure(t *testing.T) {
	resolves, dials := 0, 0
	dial := schemaDial(func(context.Context, string, string) ([]netip.Addr, error) {
		resolves++
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, errors.New("sentinel resolver failure")
	}, func(context.Context, string, string) (net.Conn, error) {
		dials++
		return nil, errors.New("unexpected connection")
	})
	conn, err := dial(t.Context(), "management.us-east-1.kiro.dev:443")
	if conn != nil || !errors.Is(err, errPlanInvalid) || resolves != 1 || dials != 0 {
		t.Errorf("schemaDial(DNS failure) = %v, %v, resolves=%d dials=%d, want nil, errPlanInvalid, 1 and 0", conn, err, resolves, dials)
	}
}

// covers: AC-16. The header bound includes the status line and terminator;
// a valid body is accepted only when the complete headers fit.
func TestSchemaProbeExactHeaderLimit(t *testing.T) {
	for _, size := range []int{16 << 10, (16 << 10) + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			d, _ := schemaMemoryDependencies(t)
			prefix := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nX-Padding: ", len(schemaFixture))
			response := prefix + strings.Repeat("x", size-len(prefix)-4) + "\r\n\r\n" + schemaFixture
			synctest.Test(t, func(t *testing.T) {
				d.wire, _ = schemaPipe(t, 0, response)
				r := runSchemaProbe(t.Context(), true, d)
				wantOutcome, wantFailure := "schema_observed", (*string)(nil)
				if size > 16<<10 {
					wantOutcome, wantFailure = "needs_evidence", schemaPtr("transport")
				}
				if r.Outcome != wantOutcome || !reflect.DeepEqual(r.FailureCategory, wantFailure) || r.DispatchCount != 1 || r.CleanupOutcome != "complete" {
					t.Errorf("runSchemaProbe(%d header bytes) = %+v, want %s with failure=%v, one dispatch and complete cleanup", size, r, wantOutcome, wantFailure)
				}
			})
		})
	}
}

func TestSchemaIntegerExactness(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want *int64
	}{{"1.0", schemaPtr(int64(1))}, {"1e2", schemaPtr(int64(100))}, {"100e-2", schemaPtr(int64(1))}, {"1.5", nil}, {"2147483647.00000001", nil}, {"2147483648", nil}, {"1e99999999999999999", nil}, {"0.0", schemaPtr(int64(0))}, {"-0", schemaPtr(int64(0))}, {"0e99999999999999999", schemaPtr(int64(0))}, {"-1", nil}} {
		if got := schemaInteger(json.Number(tc.raw)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("schemaInteger(%s) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestSchemaProbeWireBounds(t *testing.T) {
	for _, tc := range []struct{ name, response, failure string }{
		{"oversized headers", "HTTP/1.1 200 OK\r\nX-Sentinel: " + strings.Repeat("x", 16<<10) + "\r\n\r\n", "transport"},
		{"duplicate content type", "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Type: application/json\r\nContent-Length: 2\r\n\r\n{}", "invalid_response"},
		{"duplicate encoding", "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Encoding: identity\r\nContent-Encoding: gzip\r\nContent-Length: 2\r\n\r\n{}", "invalid_response"},
		{"truncated body", "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{}", "invalid_response"},
		{"valid JSON in truncated body", fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(schemaFixture)+100, schemaFixture), "invalid_response"},
		{"oversized chunk", fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nTransfer-Encoding: chunked\r\n\r\n%x\r\n%s\r\n0\r\n\r\n", schemaBodyLimit+1, strings.Repeat(" ", schemaBodyLimit+1)), "invalid_response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := schemaMemoryDependencies(t)
			d.wire = schemaTLSFixture(t, func(w http.ResponseWriter, req *http.Request) {
				_, _ = io.Copy(io.Discard, req.Body)
				conn, buffered, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error("schema raw TLS fixture could not hijack connection")
					return
				}
				defer conn.Close()
				_, _ = buffered.WriteString(tc.response)
				_ = buffered.Flush()
			})
			r := runSchemaProbe(t.Context(), true, d)
			failure := ""
			if r.FailureCategory != nil {
				failure = *r.FailureCategory
			}
			if r.FailureCategory == nil || *r.FailureCategory != tc.failure || r.DispatchCount != 1 || r.Outcome != "needs_evidence" || r.CleanupOutcome != "complete" {
				t.Errorf("runSchemaProbe(%s) category=%s dispatch=%d outcome=%s cleanup=%s, want %s once with cleanup", tc.name, failure, r.DispatchCount, r.Outcome, r.CleanupOutcome, tc.failure)
			}
		})
	}
}

func TestSchemaProbeConnectionCloseError(t *testing.T) {
	d, _ := schemaMemoryDependencies(t)
	d.wire = schemaTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, schemaFixture)
	})
	dial := d.wire.dial
	d.wire.dial = func(ctx context.Context, address string) (net.Conn, error) {
		conn, err := dial(ctx, address)
		if err != nil {
			return nil, err
		}
		return &closeErrorConn{Conn: conn, err: errors.New("sentinel close failure")}, nil
	}
	r := runSchemaProbe(t.Context(), true, d)
	if r.FailureCategory == nil || *r.FailureCategory != "cleanup" || r.CleanupOutcome != "failed" || r.Outcome != "needs_evidence" {
		t.Errorf("runSchemaProbe(connection close error) = %+v, want cleanup failure", r)
	}
}

func TestSchemaCatalogueObservationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body           string
		found, present, more *bool
		failure              string
	}{
		{"all unknown", `{"models":[{"modelId":"claude-opus-5.5","additionalModelRequestFieldsSchema":{}}]}`, schemaPtr(true), schemaPtr(true), schemaPtr(false), ""},
		{"null next token", `{"models":[{"modelId":"claude-opus-5.5","additionalModelRequestFieldsSchema":{}}],"nextToken":null}`, schemaPtr(true), schemaPtr(true), schemaPtr(false), ""},
		{"100 models", `{"models":[` + strings.Repeat(`{},`, 99) + `{"modelId":"claude-opus-5.5","additionalModelRequestFieldsSchema":{}}]}`, schemaPtr(true), schemaPtr(true), schemaPtr(false), ""},
		{"missing", `{"models":[],"nextToken":"secret"}`, schemaPtr(false), nil, schemaPtr(true), "model_missing"},
		{"schema missing", `{"models":[{"modelId":"claude-opus-5.5"}]}`, schemaPtr(true), schemaPtr(false), schemaPtr(false), "schema_missing"},
		{"ambiguous", `{"models":[{"modelId":"claude-opus-5.5"},{"modelId":"claude-opus-5.5"}]}`, nil, nil, nil, "invalid_response"},
		{"enum overflow", `{"models":[{"modelId":"claude-opus-5.5","additionalModelRequestFieldsSchema":{"properties":{"system":{"enum":[` + strings.TrimSuffix(strings.Repeat(`"high",`, 33), ",") + `]}}}}]}`, schemaPtr(true), schemaPtr(true), schemaPtr(false), "schema_unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newSchemaReport()
			extractSchema([]byte(tc.body), &r)
			failure := ""
			if r.FailureCategory != nil {
				failure = *r.FailureCategory
			}
			if failure != tc.failure || !reflect.DeepEqual(r.ModelFound, tc.found) || !reflect.DeepEqual(r.SchemaPresent, tc.present) || !reflect.DeepEqual(r.MorePages, tc.more) {
				t.Errorf("extractSchema(%s) = %+v, want category %q and nullable observations", tc.name, r, tc.failure)
			}
		})
	}
	for _, depth := range []int{64, 65} {
		data := []byte(`{"x":` + strings.Repeat(`[`, depth-1) + `0` + strings.Repeat(`]`, depth-1) + `}`)
		_, err := schemaJSON(data, schemaBodyLimit)
		if (err == nil) != (depth == 64) {
			t.Errorf("schemaJSON(depth %d) error=%v, want valid=%t", depth, err, depth == 64)
		}
	}
}
