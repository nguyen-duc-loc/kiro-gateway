package kiro

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/config"
	"kiro-gateway/internal/credentials"
)

func adapterFixture(t *testing.T, home string, h http.HandlerFunc) *Adapter {
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
	server := httptest.NewUnstartedServer(h)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	server.StartTLS()
	t.Cleanup(server.Close)
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	reader := credentials.Reader{Home: home}
	ref, err := reader.Capture(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	doc := config.Default()
	doc.Link(ref)
	doc.Models[bridge.Model] = bridge.Model
	a, err := New(reader, doc)
	if err != nil {
		t.Fatal(err)
	}
	a.metadata = func() (string, error) { return strings.Repeat("0", 64), nil }
	a.transport = transport{roots: roots, dial: func(ctx context.Context, address string) (net.Conn, error) {
		if address != "runtime.us-east-1.kiro.dev:443" && address != "runtime.eu-central-1.kiro.dev:443" {
			return nil, errors.New("bad fixture destination")
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp4", server.Listener.Addr().String())
	}}
	return a
}
func adapterRequest(t *testing.T) bridge.Request {
	t.Helper()
	r, err := bridge.Parse([]byte(`{"model":"claude-opus-5.5","max_tokens":1,"system":"T","messages":[{"role":"user","content":"U"},{"role":"system","content":"S"}],"tools":[{"name":"Read","input_schema":{"type":"object","additionalProperties":false}}]}`), false)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAdapterSyntheticTextToolsAndProfilePin(t *testing.T) {
	home, db := probeHome(t)
	var dispatches atomic.Int64
	a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
		dispatches.Add(1)
		if r.Header.Get("Authorization") != "Bearer sentinel-token" || r.URL.Path != "/generateAssistantResponse" || r.ProtoMajor != 1 || !r.Close {
			t.Error("wire auth, path, or lifecycle differs from contract")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("wire body invalid")
		}
		state := body["conversationState"].(map[string]any)
		u := state["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
		if u["content"] != "T\n\nU\n\nS" {
			t.Error("wire instruction placement changed")
		}
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"one"}`))
		w.(http.Flusher).Flush()
		_, _ = w.Write(probeFrame("reasoningContentEvent", `{"invented":"discard"}`))
		_, _ = w.Write(probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{\"n\":9007199254740993}","stop":true}`))
	})
	r := adapterRequest(t)
	var events []bridge.Event
	end, err := a.Generate(t.Context(), r, func(e bridge.Event) error { events = append(events, e); return nil })
	if err != nil || end.Basis != bridge.InferredCleanEOF || end.ReasoningEvents != 1 || len(events) != 2 {
		t.Fatalf("Generate(synthetic) end=%v err=%v events=%d, want clean text/tool", end, err, len(events))
	}
	if events[1].Tool.Input["n"] != json.Number("9007199254740993") {
		t.Error("tool integer lost precision")
	}
	if _, err := db.Exec(`UPDATE state SET value=?`, probeSyntheticProfile+" "); err != nil {
		t.Fatal(err)
	}
	_, err = a.Generate(t.Context(), r, func(bridge.Event) error { t.Error("changed source emitted output"); return nil })
	if err == nil || bridge.SafeFailure(err).Category != "profile_changed" || dispatches.Load() != 1 {
		t.Errorf("Generate(changed profile)=%v dispatches=%d, want profile_changed and 1", err, dispatches.Load())
	}
}

func TestAdapterWithholdsToolsOnInvalidEnd(t *testing.T) {
	for _, suffix := range []struct {
		name string
		data []byte
	}{
		{"unfinished", probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{"}`)},
		{"truncated after stopped tool", append(probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{}","stop":true}`), 1)},
		{"text after tool", append(probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{}","stop":true}`), probeFrame("assistantResponseEvent", `{"content":"late"}`)...)},
		{"duplicate argument", probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{\"a\":1,\"a\":2}","stop":true}`)},
		{"unknown semantic field", probeFrame("assistantResponseEvent", `{"content":"late","unknown":1}`)},
	} {
		t.Run(suffix.name, func(t *testing.T) {
			home, _ := probeHome(t)
			a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"partial"}`))
				_, _ = w.Write(suffix.data)
			})
			tools, texts := 0, 0
			end, err := a.Generate(t.Context(), adapterRequest(t), func(e bridge.Event) error {
				if e.Tool != nil {
					tools++
				} else {
					texts++
				}
				return nil
			})
			if err == nil || end.Basis != "" || tools != 0 || texts != 1 {
				t.Errorf("Generate(%s) end=%v err=%v tools=%d texts=%d, want partial text only", suffix.name, end, err, tools, texts)
			}
		})
	}
}

func TestAdapterCancellationAndNoReplay(t *testing.T) {
	home, _ := probeHome(t)
	var dispatches atomic.Int64
	a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
		dispatches.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"partial"}`))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := time.Now()
	_, err := a.Generate(ctx, adapterRequest(t), func(bridge.Event) error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) || time.Since(started) > 5*time.Second || dispatches.Load() != 1 {
		t.Errorf("Generate(cancel)=%v dispatches=%d, want canceled once", err, dispatches.Load())
	}
}

func TestEncodeHistoryPairingPreservesInputsAndResults(t *testing.T) {
	body := `{"model":"claude-opus-5.5","max_tokens":1,"system":"T","messages":[{"role":"user","content":"U1"},{"role":"system","content":"S1"},{"role":"assistant","content":[{"type":"tool_use","id":"c","name":"Old","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"c","content":[],"is_error":true}]},{"role":"system","content":"S2"}]}`
	r, err := bridge.Parse([]byte(body), false)
	if err != nil {
		t.Fatal(err)
	}
	before := bridge.Canonical(r)
	encoded, err := encodeRequest(r, "invented-profile", "invented-conversation")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if json.Unmarshal(encoded, &got) != nil {
		t.Fatal("decode failed")
	}
	state := got["conversationState"].(map[string]any)
	history := state["history"].([]any)
	first := history[0].(map[string]any)["userInputMessage"].(map[string]any)
	current := state["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	results := current["userInputMessageContext"].(map[string]any)["toolResults"]
	want := []any{map[string]any{"toolUseId": "c", "status": "error", "content": []any{map[string]any{"text": ""}}}}
	if len(history) != 2 || first["content"] != "T\n\nU1\n\nS1" || current["content"] != "\n\nS2" || !reflect.DeepEqual(results, want) || !bytes.Equal(before, bridge.Canonical(r)) {
		t.Error("encodeRequest(history) changed pairing, results, or normalized input")
	}
	again, err := encodeRequest(r, "invented-profile", "invented-conversation")
	if err != nil || !bytes.Equal(encoded, again) {
		t.Error("repeated encoding accumulated instructions")
	}
}

func TestAdapterSourceFailuresNeverDispatch(t *testing.T) {
	for _, tc := range []struct {
		source   error
		category string
		status   int
	}{
		{credentials.ErrSource, "source_unavailable", 503}, {credentials.ErrRecord, "credential_invalid", 503}, {credentials.ErrExpired, "credential_expired", 503}, {credentials.ErrBusy, "source_busy", 503}, {credentials.ErrTimeout, "source_timeout", 504}, {credentials.ErrChanged, "session_changed", 409}, {credentials.ErrProfileInvalid, "profile_invalid", 503}, {credentials.ErrProfileUnsupported, "profile_unsupported", 503}, {credentials.ErrCanceled, "source_canceled", 503},
	} {
		t.Run(tc.category, func(t *testing.T) {
			home, _ := probeHome(t)
			a := adapterFixture(t, home, func(http.ResponseWriter, *http.Request) { t.Error("source failure dispatched") })
			a.reader = wireSnapshotFunc(func(context.Context, config.Session) (credentials.ProfileSnapshot, error) {
				return credentials.ProfileSnapshot{}, fmt.Errorf("synthetic private diagnostic: %w", tc.source)
			})
			_, err := a.Generate(t.Context(), adapterRequest(t), func(bridge.Event) error { t.Error("source failure emitted"); return nil })
			if err == nil {
				t.Fatal("Generate(source error) succeeded")
			}
			f := bridge.SafeFailure(err)
			if f.Category != tc.category || f.Status != tc.status || strings.Contains(f.Message, "synthetic") {
				t.Errorf("Generate(source) category=%s status=%d, want %s %d", f.Category, f.Status, tc.category, tc.status)
			}
		})
	}
}
func TestAdapterRedirectThrottleAndIdle(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		category string
	}{{"redirect", 307, "upstream_status"}, {"throttle", 429, "upstream_throttle"}, {"forbidden", 403, "upstream_status"}, {"idle", 0, "timed_out"}} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			var attempts atomic.Int64
			a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				if tc.status == 0 {
					<-r.Context().Done()
					return
				}
				w.Header().Set("Location", "http://127.0.0.1:1/forbidden")
				w.WriteHeader(tc.status)
			})
			a.transport.idle = 100 * time.Millisecond
			_, err := a.Generate(t.Context(), adapterRequest(t), func(bridge.Event) error { return nil })
			if err == nil || bridge.SafeFailure(err).Category != tc.category || attempts.Load() != 1 {
				t.Errorf("Generate(%s)=%v attempts=%d, want category=%s once", tc.name, err, attempts.Load(), tc.category)
			}
		})
	}
}
func TestFrameChecksumsAndLimits(t *testing.T) {
	valid := probeFrame("assistantResponseEvent", `{"content":"ok"}`)
	for _, offset := range []int{8, len(valid) - 1} {
		bad := append([]byte{}, valid...)
		bad[offset] ^= 1
		called := false
		err := readFrames(bytes.NewReader(bad), func(string, []byte) error { called = true; return nil })
		if err == nil || called {
			t.Errorf("readFrames(corrupt byte %d) err=%v called=%t, want reject before consume", offset, err, called)
		}
	}
}
