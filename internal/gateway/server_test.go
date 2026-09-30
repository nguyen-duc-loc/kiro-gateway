package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testToken = "test-only-gateway-credential-12345"

// covers: AC-2. Authentication applies before routing or method handling.
func TestHealth_AuthenticationAndRouting(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		auth               []string
		status             int
	}{
		{"valid credential", "GET", "/healthz", []string{"Bearer " + testToken}, 200},
		{"missing credential", "GET", "/healthz", nil, 401},
		{"wrong credential", "GET", "/healthz", []string{"Bearer " + strings.Repeat("x", 32)}, 401},
		{"tampered credential", "GET", "/healthz", []string{"Bearer " + testToken + "x"}, 401},
		{"missing scheme", "GET", "/healthz", []string{testToken}, 401},
		{"wrong scheme", "GET", "/healthz", []string{"Basic " + testToken}, 401},
		{"empty credential", "GET", "/healthz", []string{"Bearer "}, 401},
		{"duplicate valid headers", "GET", "/healthz", []string{"Bearer " + testToken, "Bearer " + testToken}, 401},
		{"valid then invalid header", "GET", "/healthz", []string{"Bearer " + testToken, "Bearer invalid"}, 401},
		{"invalid then valid header", "GET", "/healthz", []string{"Bearer invalid", "Bearer " + testToken}, 401},
		{"query credential", "GET", "/healthz?token=" + testToken, nil, 401},
		{"unknown unauthenticated route", "GET", "/private", nil, 401},
		{"unauthenticated method", "POST", "/healthz", nil, 401},
		{"unknown authenticated route", "GET", "/private", []string{"Bearer " + testToken}, 404},
		{"unsupported method", "POST", "/healthz", []string{"Bearer " + testToken}, 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := newHealthHandler(testToken, "release\"\nversion", slog.New(slog.NewTextHandler(io.Discard, nil)))
			req := httptest.NewRequest(tc.method, tc.path, nil)
			for _, value := range tc.auth {
				req.Header.Add("Authorization", value)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing response security headers")
			}
			if strings.Contains(rec.Body.String(), testToken) {
				t.Fatal("credential leaked into response")
			}
			switch tc.status {
			case 200:
				var body map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if len(body) != 2 || body["status"] != "running" || body["version"] != "release\"\nversion" {
					t.Fatalf("health = %v", body)
				}
				if rec.Header().Get("Content-Type") != "application/json" {
					t.Fatal("health must be JSON")
				}
			case 401:
				if rec.Header().Get("WWW-Authenticate") != "Bearer" {
					t.Fatal("missing authentication challenge")
				}
			case 405:
				if rec.Header().Get("Allow") != "GET" {
					t.Fatal("missing allowed method")
				}
			}
		})
	}
}

// covers: AC-2, AC-3. Invalid settings fail before serving and stay out of errors.
func TestRun_InvalidSettings(t *testing.T) {
	for _, tc := range []struct{ name, address, token, want string }{
		{"wildcard", "0.0.0.0:0", testToken, "IPv4 loopback"},
		{"private address", "192.168.1.1:0", testToken, "IPv4 loopback"},
		{"hostname", "localhost:0", testToken, "IPv4 loopback"},
		{"IPv6", "[::1]:0", testToken, "IPv4 loopback"},
		{"mapped IPv4", "[::ffff:127.0.0.1]:0", testToken, "IPv4 loopback"},
		{"missing port", "127.0.0.1", testToken, "IPv4 loopback"},
		{"port overflow", "127.0.0.1:65536", testToken, "IPv4 loopback"},
		{"negative port", "127.0.0.1:-1", testToken, "IPv4 loopback"},
		{"missing token", "127.0.0.1:0", "", "at least 32"},
		{"short token", "127.0.0.1:0", strings.Repeat("x", 31), "at least 32"},
		{"space in token", "127.0.0.1:0", testToken + " ", "visible ASCII"},
		{"newline in token", "127.0.0.1:0", testToken + "\n", "visible ASCII"},
		{"unicode token", "127.0.0.1:0", testToken + "é", "visible ASCII"},
		{"control byte", "127.0.0.1:0", testToken + "\x7f", "visible ASCII"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // A regression must not leave an accidental listener running.
			var output bytes.Buffer
			err := Run(ctx, tc.address, tc.token, "dev", slog.New(slog.NewTextHandler(&output, nil)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if tc.token != "" && strings.Contains(err.Error()+output.String(), tc.token) {
				t.Fatal("credential leaked")
			}
			if output.Len() != 0 {
				t.Fatalf("invalid configuration started serving: %s", &output)
			}
		})
	}
}

// covers: AC-3. Binding failure is actionable without exposing raw settings.
func TestRun_OccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = Run(ctx, listener.Addr().String(), testToken, "dev", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || !strings.Contains(err.Error(), "port is already in use") {
		t.Fatalf("error = %v", err)
	}
}

// covers: AC-3. Concurrent requests have distinct IDs and sanitized diagnostics.
func TestHealth_ConcurrentDiagnostics(t *testing.T) {
	var logs bytes.Buffer
	handler := newHealthHandler(testToken, "dev", slog.New(slog.NewJSONHandler(&logs, nil)))
	const count = 24
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/healthz?prompt=private-query", strings.NewReader("private-conversation"))
			req.Header.Set("X-Private", "private-header")
			want := http.StatusUnauthorized
			if i%2 == 0 {
				req.Header.Set("Authorization", "Bearer "+testToken)
				want = http.StatusOK
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != want {
				t.Errorf("status = %d, want %d", rec.Code, want)
			}
		}(i)
	}
	wg.Wait()
	for _, secret := range []string{testToken, "private-query", "private-conversation", "private-header"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("sensitive request data leaked: %q", secret)
		}
	}
	decoder := json.NewDecoder(&logs)
	seen := make(map[uint64]bool)
	for i := 0; i < count; i++ {
		var record struct {
			Event   string `json:"event"`
			ID      uint64 `json:"request_id"`
			Outcome int    `json:"outcome"`
			Elapsed int64  `json:"elapsed"`
		}
		if err := decoder.Decode(&record); err != nil {
			t.Fatal(err)
		}
		if record.ID == 0 || seen[record.ID] {
			t.Fatalf("invalid or duplicate request ID: %d", record.ID)
		}
		seen[record.ID] = true
		if record.Event != "request" || (record.Outcome != 200 && record.Outcome != 401) || record.Elapsed < 0 {
			t.Fatalf("invalid diagnostic: %+v", record)
		}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("unexpected extra diagnostic: %v", err)
	}
}

// covers: AC-3. Transport diagnostics cannot forward raw panic or request data.
func TestSafeServerLog_RedactsTransportDetails(t *testing.T) {
	var output bytes.Buffer
	writer := safeServerLog{slog.New(slog.NewTextHandler(&output, nil))}
	input := []byte("panic: private-conversation " + testToken)
	n, err := writer.Write(input)
	if err != nil || n != len(input) {
		t.Fatalf("write = %d, %v", n, err)
	}
	if strings.Contains(output.String(), testToken) || strings.Contains(output.String(), "private-conversation") {
		t.Fatal("raw transport data leaked")
	}
	if !strings.Contains(output.String(), "HTTP transport error") {
		t.Fatal("transport failure was hidden")
	}
}
