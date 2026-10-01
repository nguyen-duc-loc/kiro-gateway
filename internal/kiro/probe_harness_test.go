package kiro

import (
	"bytes"
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

const probeSyntheticRecord = `{"access_token":"sentinel-token","expires_at":"2099-01-01T00:00:00Z","region":"synthetic-region","start_url":"https://sentinel-account.example/start"}`
const probeSyntheticBody = `{"fixture":"text","model":"claude-sonnet-5"}`

func probeHome(t *testing.T) (string, *sql.DB) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Application Support", "kiro-cli")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll(fixture) error = %v, want nil", err)
	}
	path := filepath.Join(dir, "data.sqlite3")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("Open(fixture database) error = %v, want nil", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err = db.Exec(`CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO auth_kv VALUES(?,?)`, "kirocli:odic:token", probeSyntheticRecord); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	ref, err := (credentials.Reader{Home: home}).Capture(t.Context())
	if err != nil {
		t.Fatalf("Capture(synthetic fixture) error = %v, want nil", err)
	}
	d := config.Default()
	d.Link(ref)
	d.Models[probeModel] = probeModel
	store, err := configstore.Open(home, true)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Save(d, true); err != nil {
		t.Fatal(err)
	}
	return home, db
}

func probeServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	s := httptest.NewTLSServer(h)
	t.Cleanup(s.Close)
	roots := x509.NewCertPool()
	roots.AddCert(s.Certificate())
	return s, roots
}

func localProbe(t *testing.T, ctx context.Context, home string, s *httptest.Server, roots *x509.CertPool, limits probeLimits) *offlineProbe {
	t.Helper()
	p, err := openOfflineProbe(ctx, home, s.URL+"/probe", roots, limits)
	if err != nil {
		t.Fatalf("openOfflineProbe(synthetic home) error = %v, want nil", err)
	}
	t.Cleanup(p.close)
	return p
}

// Fixtures are invented, never captured live responses. The envelope implements
// generic Amazon EventStream framing, not a proven Kiro response contract.
func probeFrame(event, payload string) []byte {
	var headers []byte
	for _, pair := range [][2]string{{":message-type", "event"}, {":event-type", event}} {
		headers = append(headers, byte(len(pair[0])))
		headers = append(headers, pair[0]...)
		headers = append(headers, 7, byte(len(pair[1])>>8), byte(len(pair[1])))
		headers = append(headers, pair[1]...)
	}
	b := make([]byte, 12, 16+len(headers)+len(payload))
	binary.BigEndian.PutUint32(b[:4], uint32(16+len(headers)+len(payload)))
	binary.BigEndian.PutUint32(b[4:8], uint32(len(headers)))
	binary.BigEndian.PutUint32(b[8:12], crc32.ChecksumIEEE(b[:8]))
	b = append(b, headers...)
	b = append(b, payload...)
	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(b))
}

// covers: spec 0003 AC-2, AC-5, AC-6, AC-8. One path through real local I/O.
func TestOfflineProbeTextThread(t *testing.T) {
	home, _ := probeHome(t)
	configPath := filepath.Join(home, ".config", "kiro-gateway", "config.json")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	s, roots := probeServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "Bearer sentinel-token" {
			t.Error("request auth differs, want selected synthetic token")
		}
		if r.URL.Path != "/probe" || r.Method != http.MethodPost {
			t.Error("request route differs, want POST /probe")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != probeSyntheticBody {
			t.Error("request body differs, want fixed synthetic envelope")
		}
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		w.Write(probeFrame("assistantResponseEvent", `{"content":"sentinel-conversation-one"}`))
		w.(http.Flusher).Flush()
		w.Write(probeFrame("assistantResponseEvent", `{"content":"sentinel-conversation-two"}`))
	})
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := p.dispatch(probeSyntheticBody)
	if got.Attempts != 1 || got.TextEvents != 2 || got.Completion != nil || got.Outcome != "needs_evidence" || got.Cause != "stream_incomplete" {
		t.Errorf("dispatch(text frames then EOF) = %+v, want two frames, one attempt, unknown completion, stream_incomplete", got)
	}
	if requests.Load() != 1 {
		t.Errorf("requests(text thread) = %d, want 1", requests.Load())
	}
	after, err := os.ReadFile(configPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Error("dispatch changed configuration, want exact original bytes")
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{"sentinel", p.reference.Fingerprint, "2099", "synthetic-region"} {
		if strings.Contains(string(b), sentinel) {
			t.Error("observation contains sensitive sentinel, want allowed fields only")
		}
	}
	second := p.dispatch(probeSyntheticBody)
	if second.Attempts != 1 || requests.Load() != 1 {
		t.Error("dispatch(after incomplete) replayed, want no new request")
	}
}

func TestOfflineProbeRedirectAndProxy(t *testing.T) {
	home, _ := probeHome(t)
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	t.Cleanup(target.Close)
	t.Setenv("HTTPS_PROXY", target.URL)
	t.Setenv("HTTP_PROXY", target.URL)
	t.Setenv("NO_PROXY", "")
	var requests atomic.Int32
	s, roots := probeServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	})
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := p.dispatch(probeSyntheticBody)
	if got.Cause != "needs_evidence" || requests.Load() != 1 || redirected.Load() != 0 {
		t.Errorf("dispatch(redirect/proxy) = %+v, destination=%d proxy=%d, want stopped and only one destination request", got, requests.Load(), redirected.Load())
	}
}

func TestOfflineProbeImmutableSelection(t *testing.T) {
	home, db := probeHome(t)
	var requests atomic.Int32
	s, roots := probeServer(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1) })
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	// Manual edits bypass the lock but must not replace the frozen reference.
	changed := strings.Replace(probeSyntheticRecord, "sentinel-token", "replacement-token", 1)
	if _, err := db.Exec(`UPDATE auth_kv SET value=?`, changed); err != nil {
		t.Fatal(err)
	}
	ref, err := (credentials.Reader{Home: home}).Capture(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	d := config.Default()
	d.Link(ref)
	d.Models[probeModel] = "different-model"
	b, err := config.Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(home, ".config", "kiro-gateway", "config.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	got := p.dispatch(probeSyntheticBody)
	if got.Cause != "session_changed" || got.Attempts != 0 || requests.Load() != 0 {
		t.Errorf("dispatch(changed settings and source) = %+v, requests=%d, want session_changed before dispatch", got, requests.Load())
	}
}

func TestOfflineProbeLockAndMissingConfig(t *testing.T) {
	home, _ := probeHome(t)
	s, roots := probeServer(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected request") })
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	other, err := openOfflineProbe(t.Context(), home, s.URL+"/probe", roots, defaultProbeLimits)
	if other != nil || !errors.Is(err, errConfiguration) {
		t.Errorf("openOfflineProbe(held lock) error = %v, want configuration_invalid", err)
	}
	p.close()
	absent := t.TempDir()
	other, err = openOfflineProbe(t.Context(), absent, s.URL+"/probe", roots, defaultProbeLimits)
	if other != nil || !errors.Is(err, errConfiguration) {
		t.Errorf("openOfflineProbe(absent settings) error = %v, want configuration_invalid", err)
	}
	entries, err := os.ReadDir(absent)
	if err != nil || len(entries) != 0 {
		t.Error("openOfflineProbe(absent settings) created paths, want empty home")
	}
}

func TestOfflineProbeRejectsExternalDestinations(t *testing.T) {
	for _, endpoint := range []string{"http://127.0.0.1:443/probe", "https://example.com:443/probe", "https://localhost:443/probe", "https://[::1]:443/probe", "https://127.0.0.1:443/other", "https://user@127.0.0.1:443/probe", "https://127.0.0.1:443/probe?q=1"} {
		t.Run(endpoint, func(t *testing.T) {
			p, err := openOfflineProbe(t.Context(), "missing", endpoint, x509.NewCertPool(), defaultProbeLimits)
			if p != nil || !errors.Is(err, errPlanInvalid) {
				t.Errorf("openOfflineProbe(%q) error = %v, want plan_invalid before store access", endpoint, err)
			}
		})
	}
}

func TestOfflineProbeParentCancellation(t *testing.T) {
	home, _ := probeHome(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	s, roots := probeServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Write(probeFrame("assistantResponseEvent", `{"content":"synthetic"}`))
		w.(http.Flusher).Flush()
		cancel()
		<-r.Context().Done()
	})
	p := localProbe(t, ctx, home, s, roots, defaultProbeLimits)
	start := time.Now()
	got := p.dispatch(probeSyntheticBody)
	if got.Cause != "canceled" || got.Attempts != 1 || time.Since(start) > 5*time.Second {
		t.Errorf("dispatch(parent cancellation) = %+v, want canceled within five seconds", got)
	}
	if next := p.dispatch(probeSyntheticBody); next.Attempts != 1 {
		t.Errorf("dispatch(after cancellation) attempts = %d, want 1", next.Attempts)
	}
}

func TestOfflineProbeIdleDeadline(t *testing.T) {
	home, _ := probeHome(t)
	s, roots := probeServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	})
	limits := defaultProbeLimits
	limits.idle = 150 * time.Millisecond
	p := localProbe(t, t.Context(), home, s, roots, limits)
	start := time.Now()
	got := p.dispatch(probeSyntheticBody)
	if got.Cause != "timed_out" || time.Since(start) > 5*time.Second {
		t.Errorf("dispatch(idle before headers) = %+v, want timed_out within five seconds", got)
	}
}

func TestOfflineProbeAttemptDeadlineDespiteBytes(t *testing.T) {
	home, _ := probeHome(t)
	s, roots := probeServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				w.Write([]byte{0})
				w.(http.Flusher).Flush()
			}
		}
	})
	limits := defaultProbeLimits
	limits.request = 75 * time.Millisecond
	limits.idle = time.Second
	p := localProbe(t, t.Context(), home, s, roots, limits)
	got := p.dispatch(probeSyntheticBody)
	if got.Cause != "timed_out" {
		t.Errorf("dispatch(byte trickle) = %+v, want request timeout", got)
	}
}

func TestOfflineProbeBudgetBeforeDispatch(t *testing.T) {
	home, _ := probeHome(t)
	s, roots := probeServer(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected request beyond budget") })
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	p.attempts = 6
	got := p.dispatch(probeSyntheticBody)
	if got.Attempts != 6 || got.Cause != "budget_exhausted" {
		t.Errorf("dispatch(seventh attempt) = %+v, want budget_exhausted and six attempts", got)
	}
}

func TestProbeFramingIncompleteAndFiltered(t *testing.T) {
	text := probeFrame("assistantResponseEvent", `{"content":"sentinel-text"}`)
	tool := probeFrame("toolUseEvent", `{"toolUseId":"sentinel-id","name":"sentinel-name","input":"{}","stop":true}`)
	for name, stream := range map[string][]byte{"empty": nil, "text EOF": text, "tool EOF": tool, "inside frame": text[:len(text)-2], "after visible tool": append(append([]byte{}, tool...), text[:13]...)} {
		t.Run(name, func(t *testing.T) {
			var out probeObservation
			err := observeProbeFrames(bytes.NewReader(stream), &out)
			if !errors.Is(err, errIncomplete) || out.Completion != nil {
				t.Errorf("observeProbeFrames(%s) error = %v, completion=%v, want incomplete and unknown", name, err, out.Completion)
			}
		})
	}
	var out probeObservation
	err := observeProbeFrames(bytes.NewReader(probeFrame("sentinel-unknown", `{"sentinel":"secret"}`)), &out)
	if !errors.Is(err, errNeedsEvidence) || out.UnknownEvents != 1 {
		t.Errorf("observeProbeFrames(unknown) = %+v, %v, want one unknown and needs_evidence", out, err)
	}
	b, _ := json.Marshal(out)
	if strings.Contains(string(b), "sentinel") {
		t.Error("observeProbeFrames(unknown) leaked input in observation")
	}
}

func TestProbeFramingIntegrityAndAllocationBounds(t *testing.T) {
	valid := probeFrame("assistantResponseEvent", `{"content":"synthetic"}`)
	badPrelude := append([]byte{}, valid...)
	badPrelude[8] ^= 1
	badMessage := append([]byte{}, valid...)
	badMessage[len(badMessage)-1] ^= 1
	oversize := append([]byte{}, valid[:12]...)
	binary.BigEndian.PutUint32(oversize[:4], probeMaxResponse+1)
	binary.BigEndian.PutUint32(oversize[8:12], crc32.ChecksumIEEE(oversize[:8]))
	for _, tc := range []struct {
		name string
		data []byte
		want error
	}{{"prelude CRC", badPrelude, errContract}, {"message CRC", badMessage, errContract}, {"announced oversize without body", oversize, errBudget}} {
		t.Run(tc.name, func(t *testing.T) {
			var out probeObservation
			err := observeProbeFrames(bytes.NewReader(tc.data), &out)
			if !errors.Is(err, tc.want) {
				t.Errorf("observeProbeFrames(%s) error = %v, want %v", tc.name, err, tc.want)
			}
		})
	}
}

func TestOfflineProbeMissingMappingAndExpiredSource(t *testing.T) {
	home, db := probeHome(t)
	var requests atomic.Int32
	s, roots := probeServer(t, func(http.ResponseWriter, *http.Request) { requests.Add(1) })
	store, err := configstore.OpenExisting(home)
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	delete(d.Models, probeModel)
	if err := store.Save(d, false); err != nil {
		t.Fatal(err)
	}
	store.Close()
	p, err := openOfflineProbe(t.Context(), home, s.URL+"/probe", roots, defaultProbeLimits)
	if p != nil || !errors.Is(err, errConfiguration) {
		t.Errorf("openOfflineProbe(missing mapping) error = %v, want configuration_invalid", err)
	}
	store, err = configstore.OpenExisting(home)
	if err != nil {
		t.Fatal(err)
	}
	d.Models[probeModel] = probeModel
	if err := store.Save(d, false); err != nil {
		t.Fatal(err)
	}
	store.Close()
	p = localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	if _, err := db.Exec(`UPDATE auth_kv SET value=?`, strings.Replace(probeSyntheticRecord, "2099-", "2000-", 1)); err != nil {
		t.Fatal(err)
	}
	got := p.dispatch(probeSyntheticBody)
	if got.Cause != "credential_expired" || got.Attempts != 0 || requests.Load() != 0 {
		t.Errorf("dispatch(expired source) = %+v, requests=%d, want credential_expired before dispatch", got, requests.Load())
	}
}

func TestOfflineProbeVerifiesTLS(t *testing.T) {
	home, _ := probeHome(t)
	var requests atomic.Int32
	s, _ := probeServer(t, func(http.ResponseWriter, *http.Request) { requests.Add(1) })
	p := localProbe(t, t.Context(), home, s, x509.NewCertPool(), defaultProbeLimits)
	got := p.dispatch(probeSyntheticBody)
	if got.Cause != "needs_evidence" || got.Attempts != 1 || requests.Load() != 0 {
		t.Errorf("dispatch(untrusted certificate) = %+v, requests=%d, want one failed attempt without an HTTP request", got, requests.Load())
	}
	if next := p.dispatch(probeSyntheticBody); next.Attempts != 1 {
		t.Errorf("dispatch(after TLS failure) attempts = %d, want 1 without retry", next.Attempts)
	}
}

func TestOfflineProbeRejectsOversizeRequest(t *testing.T) {
	home, _ := probeHome(t)
	s, roots := probeServer(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected request") })
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := p.dispatch(strings.Repeat("x", probeMaxRequest+1))
	if got.Cause != "budget_exhausted" || got.Attempts != 0 {
		t.Errorf("dispatch(oversize request) = %+v, want budget_exhausted before dispatch", got)
	}
}

func TestProbeFramingHeaderAmbiguity(t *testing.T) {
	// Duplicate event labels cannot choose different framing interpretations.
	frame := probeFrame("assistantResponseEvent", "")
	n := int(binary.BigEndian.Uint32(frame[4:8]))
	headers := append(append([]byte{}, frame[12:12+n]...), frame[12:12+n]...)
	_, _, err := probeEventHeaders(headers)
	if !errors.Is(err, errContract) {
		t.Errorf("probeEventHeaders(duplicate names) error = %v, want contract_mismatch", err)
	}
	for name, raw := range map[string][]byte{
		"empty name":      {0},
		"truncated name":  {10, 'x'},
		"invalid type":    {1, 'x', 255},
		"invalid UTF8":    {1, 255, 0},
		"truncated value": {1, 'x', 7, 0, 10, 'a'},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := probeEventHeaders(raw)
			if !errors.Is(err, errContract) {
				t.Errorf("probeEventHeaders(%s) error = %v, want contract_mismatch", name, err)
			}
		})
	}
}
