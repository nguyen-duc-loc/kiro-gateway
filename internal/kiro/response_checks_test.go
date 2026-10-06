//go:build responsediscovery

package kiro

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

// covers: AC-17. Real temporary settings and SQLite, verified synthetic TLS,
// exact request, one snapshot, one dispatch, lock ownership and redaction.
func TestResponseDiscoverySyntheticPath(t *testing.T) {
	for _, region := range []string{"us-east-1", "eu-central-1"} {
		t.Run(region, func(t *testing.T) {
			plan := newResponseDiscoveryPreflightFixture(t)
			plan.plan.Contract = responseContract(region)
			plan.save(t)
			home, db := probeHome(t)
			if _, err := db.Exec(`UPDATE state SET value=?`, strings.Replace(probeSyntheticProfile, "us-east-1", region, 1)); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(home, ".config", "kiro-gateway", "config.json")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			d := responseFixtureDependencies(t, home)
			d.preflight = responseLivePreflight
			reads := 0
			var requests atomic.Int32
			d.reader = func(home string) ProfileReader {
				return wireSnapshotFunc(func(ctx context.Context, ref config.Session) (credentials.ProfileSnapshot, error) {
					reads++
					return (credentials.Reader{Home: home}).ReadProfileSnapshot(ctx, ref)
				})
			}
			d.wire = responseTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != "POST" || r.URL.RequestURI() != "/" || r.Host != "runtime."+region+".kiro.dev:443" || !r.Close || r.Proto != "HTTP/1.1" {
					t.Error("request route or transport differs from fixed discovery hypothesis")
				}
				want := http.Header{}
				// Pin the reviewed hypothesis independently of the request builder.
				for k, v := range map[string]string{
					"Authorization": "Bearer sentinel-token",
					"Content-Type":  "application/x-amz-json-1.0",
					"Accept":        "*/*", "Accept-Encoding": "identity",
					"User-Agent":                  "kiro-gateway-response-discovery/1",
					"X-Amzn-Codewhisperer-Optout": "true",
					"X-Amz-Target":                "KiroRuntimeService.CreateResponse",
				} {
					want.Set(k, v)
				}
				want.Set("Connection", "close")
				want.Set("Content-Length", r.Header.Get("Content-Length"))
				if !reflect.DeepEqual(r.Header, want) {
					t.Error("request headers differ from finite discovery headers")
				}
				var body map[string]any
				decoder := json.NewDecoder(r.Body)
				decoder.UseNumber()
				expected := map[string]any{
					"model": "claude-opus-5.5", "input": "Return exactly OK.",
					"origin": "KIRO_CLI", "stream": true,
					"instructions":    "This is a protocol discovery test. Reply with exactly OK.",
					"maxOutputTokens": json.Number("1024"),
					"profileArn":      "arn:aws:codewhisperer:" + region + ":000000000000:profile/sentinel-profile",
				}
				if decoder.Decode(&body) != nil || !reflect.DeepEqual(body, expected) {
					t.Error("request body differs from fixed hypothesis and selected profile")
				}
				lock, err := configstore.OpenExisting(home)
				if !errors.Is(err, configstore.ErrLocked) {
					if lock != nil {
						lock.Close()
					}
					t.Error("settings lock missing during response")
				}
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				_, _ = io.WriteString(w, `{"type":"response.completed","model":"claude-opus-5.5","status":"failed","usage":{"input_tokens":87,"output_tokens":34},"output":[{"text":"sentinel-secret"}],"unknown-secret":"sentinel-token"}`)
			})
			r := runResponseDiscovery(t.Context(), true, d)
			if r.Outcome != "response_observed" || r.FailureCategory != nil || r.CleanupOutcome != "complete" || reads != 1 || requests.Load() != 1 || r.DispatchCount != 1 || r.Region == nil || *r.Region != region || r.BodySummary == nil {
				t.Fatalf("discovery(%s) = %+v reads=%d requests=%d, want one complete structural observation", region, r, reads, requests.Load())
			}
			if !reflect.DeepEqual(r.BodySummary.StatusLabels, []string{"failed"}) {
				t.Errorf("status labels=%v, want [failed] without inference verdict", r.BodySummary.StatusLabels)
			}
			data := encodeResponseReport(&r)
			if bytes.Contains(data, []byte("sentinel")) || bytes.Contains(data, []byte("arn:aws")) || len(data) > responseReportLimit {
				t.Error("report retained forbidden content or exceeded bound")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Error("settings bytes changed")
			}
			lock, err := configstore.OpenExisting(home)
			if err != nil {
				t.Fatalf("lock after discovery=%v, want released", err)
			}
			lock.Close()
		})
	}
}

func responseFrame(headers []byte, payload string) []byte {
	b := make([]byte, 12)
	binary.BigEndian.PutUint32(b[:4], uint32(16+len(headers)+len(payload)))
	binary.BigEndian.PutUint32(b[4:8], uint32(len(headers)))
	binary.BigEndian.PutUint32(b[8:], crc32.ChecksumIEEE(b[:8]))
	b = append(b, headers...)
	b = append(b, payload...)
	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(b))
}
func responseFrameHeader(name, value string) []byte {
	b := append([]byte{byte(len(name))}, name...)
	b = append(b, 7, byte(len(value)>>8), byte(len(value)))
	return append(b, value...)
}
func responseExceptionFrame(payload string) []byte {
	return responseFrame(responseFrameHeader(":message-type", "exception"), payload)
}

func TestResponseDiscoveryProjection(t *testing.T) {
	body := `{"type":null,"status":"private-status","model":"private-model","response":{"status":"completed","model":"claude-opus-5.5","usage":{"input_tokens":null,"output_tokens":false}},"usage":5,"output":[],"error":{"type":"secret#ValidationException","code":42},"__type":"AccessDeniedException","code":"secret","arbitrary-secret":{"password":"never retained"}}`
	s, f := observeResponse(t.Context(), []byte(body), "json")
	if f != "" {
		t.Fatal(f)
	}
	expected := map[string][]string{
		"/type": {"null"}, "/status": {"string"}, "/model": {"string"}, "/output": {"array"}, "/usage": {"number"}, "/usage/input_tokens": {"unreachable"}, "/usage/output_tokens": {"unreachable"}, "/response": {"object"}, "/response/status": {"string"}, "/response/model": {"string"}, "/response/output": {"absent"}, "/response/usage": {"object"}, "/response/usage/input_tokens": {"null"}, "/response/usage/output_tokens": {"boolean"}, "/error": {"object"}, "/error/type": {"string"}, "/error/code": {"number"}, "/code": {"string"}, "/__type": {"string"},
	}
	if !reflect.DeepEqual(s.PathKinds, expected) || !reflect.DeepEqual(s.StatusLabels, []string{"completed", "other"}) || !reflect.DeepEqual(s.ErrorLabels, []string{"AccessDeniedException", "ValidationException", "other"}) || !reflect.DeepEqual(s.ModelComparisons, []string{"match", "different"}) || *s.EventSequence[0].PayloadType != "invalid" {
		t.Errorf("projection=%+v, want fixed ordered sets and precise path kinds", s)
	}
	data, _ := json.Marshal(s)
	for _, secret := range []string{"private-", "secret", "password", "never retained"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Errorf("projection leaked %q", secret)
		}
	}
}

func TestResponseDiscoveryFraming(t *testing.T) {
	for _, tc := range []struct {
		name, media string
		data        []byte
		failure     string
		records     int
	}{
		{"json", "json", []byte(`{}`), "", 1},
		{"JSON duplicate", "json", []byte(`{"x":{"a":1,"a":2}}`), "invalid_json", 0},
		{"escaped duplicate", "json", []byte(`{"x":1,"\u0078":2}`), "invalid_json", 0},
		{"UTF8", "json", []byte("{\"x\":\"\xff\"}"), "invalid_utf8", 0},
		{"array root", "json", []byte(`[]`), "invalid_json", 0},
		{"trailing", "json", []byte(`{} {}`), "invalid_json", 0},
		{"depth 64", "json", []byte(`{"x":` + strings.Repeat(`[`, 63) + `0` + strings.Repeat(`]`, 63) + `}`), "", 1},
		{"depth 65", "json", []byte(`{"x":` + strings.Repeat(`[`, 64) + `0` + strings.Repeat(`]`, 64) + `}`), "invalid_json", 0},
		{"body max", "json", []byte(`{}` + strings.Repeat(" ", responseBodyLimit-2)), "", 1},
		{"body over", "json", []byte(strings.Repeat(" ", responseBodyLimit+1)), "limit", 0},
		{"empty", "json", nil, "empty_body", 0},
		{"SSE multiline", "sse", []byte("event: response.completed\r\ndata: {\r\ndata: \"type\":\"response.completed\"}\r\n\r\ndata: [DONE]\n\ndata: {}\n\n"), "", 3},
		{"SSE ignored", "sse", []byte(": comment\nid: secret\nretry: 42\nx: private\ndata: {}\n\nid: trailing\n: ignored\n"), "", 1},
		{"SSE missing final line", "sse", []byte("data: {}"), "invalid_sse", 0},
		{"SSE pending event", "sse", []byte("data: {}\n\nevent: message\n"), "invalid_sse", 0},
		{"SSE pending data", "sse", []byte("data: {}\n"), "invalid_sse", 0},
		{"SSE duplicate event", "sse", []byte("event: message\nevent: error\ndata: {}\n\n"), "invalid_sse", 0},
		{"SSE bare CR", "sse", []byte("data: {\r}\n\n"), "invalid_sse", 0},
		{"SSE done only", "sse", []byte("data: [DONE]\n\n"), "empty_body", 0},
		{"SSE done continues", "sse", []byte("data: {}\n\ndata: [DONE]\n\ndata: INVALID\n\n"), "invalid_json", 0},
		{"SSE one space", "sse", []byte("data:  [DONE]\n\n"), "invalid_json", 0},
		{"SSE comment max", "sse", []byte(":" + strings.Repeat("x", (16<<10)-1) + "\ndata: {}\n\n"), "", 1},
		{"SSE line over", "sse", []byte(":" + strings.Repeat("x", 16<<10) + "\n"), "limit", 0},
		{"SSE events 256", "sse", []byte(strings.Repeat("data: {}\n\n", 256)), "", 256},
		{"SSE events 257", "sse", []byte(strings.Repeat("data: {}\n\n", 257)), "limit", 0},
		{"binary event", "eventstream", probeFrame("response.completed", `{}`), "", 1},
		{"binary exception", "eventstream", responseExceptionFrame(`{"__type":"ValidationException"}`), "", 1},
		{"binary unknown label", "eventstream", probeFrame("private-event", `{"type":"private-type"}`), "", 1},
		{"binary unfinished", "eventstream", append(probeFrame("message", `{}`), 0), "invalid_eventstream", 0},
		{"binary invalid JSON", "eventstream", probeFrame("message", `[]`), "invalid_json", 0},
		{"binary records 257", "eventstream", bytes.Repeat(probeFrame("message", `{}`), 257), "limit", 0},
		{"binary kind", "eventstream", responseFrame(responseFrameHeader(":message-type", "secret"), `{}`), "invalid_eventstream", 0},
		{"binary event missing", "eventstream", responseFrame(responseFrameHeader(":message-type", "event"), `{}`), "invalid_eventstream", 0},
		{"binary duplicate header", "eventstream", responseFrame(append(responseFrameHeader(":message-type", "error"), responseFrameHeader(":message-type", "error")...), `{}`), "invalid_eventstream", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := observeResponse(t.Context(), tc.data, tc.media)
			if f != tc.failure {
				t.Fatalf("observe(%s) failure=%q, want %q", tc.name, f, tc.failure)
			}
			if f != "" {
				if s != nil {
					t.Error("failed observation kept summary")
				}
				return
			}
			if s.RecordCount != tc.records || s.SequenceTruncated != (tc.records > 32) {
				t.Errorf("observe(%s)=%+v, want count=%d bounded sequence", tc.name, s, tc.records)
			}
			b, _ := json.Marshal(s)
			if bytes.Contains(b, []byte("private-")) {
				t.Error("unknown labels escaped finite vocabulary")
			}
		})
	}
}

func TestResponseDiscoveryFrameIntegrityAndLimits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		build   func() []byte
		failure string
	}{
		{"prelude CRC", func() []byte { b := probeFrame("message", `{}`); b[8] ^= 1; return b }, "invalid_eventstream"},
		{"message CRC", func() []byte { b := probeFrame("message", `{}`); b[len(b)-1] ^= 1; return b }, "invalid_eventstream"},
		{"frame max", func() []byte {
			b := probeFrame("message", `{}`)
			return probeFrame("message", `{}`+strings.Repeat(" ", (64<<10)-len(b)))
		}, ""},
		{"frame over", func() []byte { return probeFrame("message", `{}`+strings.Repeat(" ", 64<<10)) }, "limit"},
		{"headers max", func() []byte {
			h := responseFrameHeader(":message-type", "error")
			h = append(h, responseFrameHeader("unknown", strings.Repeat("a", (8<<10)-len(h)-len("unknown")-4))...)
			return responseFrame(h, `{}`)
		}, ""},
		{"headers over", func() []byte {
			h := responseFrameHeader(":message-type", "error")
			h = append(h, responseFrameHeader("unknown", strings.Repeat("a", 8<<10))...)
			return responseFrame(h, `{}`)
		}, "limit"},
		{"bad header type", func() []byte { return responseFrame([]byte{1, 'x', 10}, `{}`) }, "invalid_eventstream"},
		{"nonstring event", func() []byte {
			h := responseFrameHeader(":message-type", "error")
			h = append(h, 11)
			h = append(h, ":event-type"...)
			h = append(h, 0)
			return responseFrame(h, `{}`)
		}, "invalid_eventstream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, f := observeResponse(t.Context(), tc.build(), "eventstream")
			if f != tc.failure {
				t.Errorf("frame(%s)=%s, want %s", tc.name, f, tc.failure)
			}
		})
	}
}

func TestResponseDiscoverySSEJoinedDataBound(t *testing.T) {
	for _, size := range []int{64 << 10, (64 << 10) + 1} {
		t.Run(string(rune(size)), func(t *testing.T) {
			// Five data fields stay below the line limit while reaching the joined cap.
			chunks := []string{"{", strings.Repeat(" ", 14000), strings.Repeat(" ", 14000), strings.Repeat(" ", 14000), strings.Repeat(" ", 14000), ""}
			chunks[5] = strings.Repeat(" ", size-56007) + "}"
			body := "data: " + strings.Join(chunks, "\ndata: ") + "\n\n"
			_, f := observeResponse(t.Context(), []byte(body), "sse")
			want := ""
			if size > 64<<10 {
				want = "limit"
			}
			if f != want {
				t.Errorf("joined size %d failure=%s, want %s", size, f, want)
			}
		})
	}
}

func TestResponseDiscoveryTailAggregatesAfterSequenceLimit(t *testing.T) {
	body := strings.Repeat("data: {}\n\n", 32) + "event: private-event\ndata: {\"status\":\"completed\",\"usage\":0,\"type\":false}\n\n"
	s, f := observeResponse(t.Context(), []byte(body), "sse")
	if f != "" || s == nil {
		t.Fatalf("tail observation failure=%s, want complete summary", f)
	}
	if len(s.EventSequence) != 32 || !s.SequenceTruncated || s.RecordCount != 33 || !reflect.DeepEqual(s.StatusLabels, []string{"completed"}) || !reflect.DeepEqual(s.PathKinds["/usage"], []string{"absent", "number"}) || !reflect.DeepEqual(s.PathKinds["/usage/input_tokens"], []string{"absent", "unreachable"}) {
		t.Errorf("tail aggregate=%+v, want later records counted and inspected without retaining sequence", s)
	}
	_, f = observeResponse(t.Context(), append([]byte(body), []byte("data: []\n\n")...), "sse")
	if f != "invalid_json" {
		t.Errorf("invalid tail=%s, want validation after truncation", f)
	}
	for _, kind := range []string{"event", "error", "exception"} {
		headers := append(responseFrameHeader(":message-type", kind), responseFrameHeader(":event-type", "")...)
		s, f := observeResponse(t.Context(), responseFrame(headers, `{}`), "eventstream")
		if f != "" || s == nil || s.EventSequence[0].TransportLabel == nil || *s.EventSequence[0].TransportLabel != "other" {
			t.Errorf("present empty event type(%s)=%+v,%s, want other", kind, s, f)
		}
	}
}

// covers: AC-17. Fragmented transport must preserve structural observations,
// read past candidate completion, and release the settings lock before return.
func TestResponseDiscoveryFragmentedStreams(t *testing.T) {
	for _, tc := range []struct {
		name, media, body string
		sequence          []responseRecord
		jsonRecords       int
		errorLabels       []string
	}{
		{
			name: "SSE continues after done", media: "text/event-stream",
			body: "event: response.completed\r\ndata: {\"type\":\"response.completed\",\"text\":\"private-你好\"}\r\n\r\ndata: [DONE]\n\nevent: private-event\ndata: {\"type\":false,\"__type\":\"private-prefix#ValidationException\"}\n\n",
			sequence: []responseRecord{
				{Channel: "sse", TransportLabel: responsePtr("response.completed"), PayloadType: responsePtr("response.completed")},
				{Channel: "sse", DoneMarker: true},
				{Channel: "sse", TransportLabel: responsePtr("other"), PayloadType: responsePtr("invalid")},
			},
			jsonRecords: 2, errorLabels: []string{"ValidationException"},
		},
		{
			name: "binary exception after completed", media: "application/vnd.amazon.eventstream",
			body: string(append(probeFrame("response.completed", `{"type":"response.completed","text":"private-你好"}`), responseExceptionFrame(`{"__type":"private-prefix#ValidationException"}`)...)),
			sequence: []responseRecord{
				{Channel: "eventstream", MessageKind: responsePtr("event"), TransportLabel: responsePtr("response.completed"), PayloadType: responsePtr("response.completed")},
				{Channel: "eventstream", MessageKind: responsePtr("exception")},
			},
			jsonRecords: 2, errorLabels: []string{"ValidationException"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			path := filepath.Join(home, ".config", "kiro-gateway", "config.json")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			d := responseFixtureDependencies(t, home)
			var requests atomic.Int32
			d.wire = responseTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", tc.media)
				// Each byte is a separate HTTP chunk, including UTF 8 and frame headers.
				for i := range len(tc.body) {
					if _, err := io.WriteString(w, tc.body[i:i+1]); err != nil {
						t.Errorf("fragmented response write(%s) = %v, want nil", tc.name, err)
						return
					}
					w.(http.Flusher).Flush()
				}
			})
			r := runResponseDiscovery(t.Context(), true, d)
			if r.Outcome != "response_observed" || r.FailureCategory != nil || r.BodyFailure != nil || r.CleanupOutcome != "complete" || r.BodySummary == nil {
				t.Fatalf("runResponseDiscovery(%s) = %+v, want a complete structural observation", tc.name, r)
			}
			if r.DispatchCount != 1 || requests.Load() != 1 || r.TransportComplete == nil || !*r.TransportComplete || r.DecodeComplete == nil || !*r.DecodeComplete {
				t.Errorf("runResponseDiscovery(%s) = %+v, requests = %d, want one fully decoded dispatch", tc.name, r, requests.Load())
			}
			s := r.BodySummary
			if !reflect.DeepEqual(s.EventSequence, tc.sequence) || s.RecordCount != len(tc.sequence) || s.JSONRecordCount != tc.jsonRecords || s.SequenceTruncated || !reflect.DeepEqual(s.ErrorLabels, tc.errorLabels) {
				t.Errorf("runResponseDiscovery(%s) summary = %+v, want sequence %+v, JSON count %d, errors %v", tc.name, s, tc.sequence, tc.jsonRecords, tc.errorLabels)
			}
			if data := encodeResponseReport(&r); bytes.Contains(data, []byte("private-")) || bytes.Contains(data, []byte("sentinel")) {
				t.Errorf("runResponseDiscovery(%s) retained private content, want finite labels only", tc.name)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Errorf("runResponseDiscovery(%s) settings error = %v, unchanged = %t, want nil and true", tc.name, err, bytes.Equal(before, after))
			}
			lock, err := configstore.OpenExisting(home)
			if err != nil {
				t.Fatalf("OpenExisting(after %s) = %v, want released lock", tc.name, err)
			}
			t.Cleanup(func() { _ = lock.Close() })
		})
	}
}

// covers: AC-17. Aggregation is ordered by the contract, independent of arrival
// order, and distinguishes absent paths from parents that cannot be traversed.
func TestResponseDiscoveryProjectionOrdersAndDeduplicatesKinds(t *testing.T) {
	bodies := []string{
		`{"usage":{"input_tokens":{}},"status":"private-status","model":"private-model"}`,
		`{"usage":{"input_tokens":[]},"status":"cancelled"}`,
		`{"usage":{"input_tokens":"private-tokens"},"status":"failed"}`,
		`{"usage":{"input_tokens":900123456},"status":"incomplete"}`,
		`{"usage":{"input_tokens":true},"status":"completed","model":"claude-opus-5.5"}`,
		`{"usage":{"input_tokens":null},"status":"in_progress"}`,
		`{"usage":[],"status":"queued"}`, `{}`,
	}
	var body strings.Builder
	for range 2 {
		for _, record := range bodies {
			body.WriteString("data: " + record + "\n\n")
		}
	}
	s, failure := observeResponse(t.Context(), []byte(body.String()), "sse")
	if failure != "" || s == nil {
		t.Fatalf("observeResponse(reversed kinds) = %v, %q, want complete summary", s, failure)
	}
	wantKinds := []string{"absent", "unreachable", "null", "boolean", "number", "string", "array", "object"}
	if got := s.PathKinds["/usage/input_tokens"]; !reflect.DeepEqual(got, wantKinds) {
		t.Errorf("observeResponse(reversed kinds) input token kinds = %v, want %v", got, wantKinds)
	}
	wantStatuses := []string{"queued", "in_progress", "completed", "incomplete", "failed", "cancelled", "other"}
	if !reflect.DeepEqual(s.StatusLabels, wantStatuses) || !reflect.DeepEqual(s.ModelComparisons, []string{"match", "different"}) || s.RecordCount != 16 {
		t.Errorf("observeResponse(repeated records) statuses = %v, models = %v, count = %d, want %v, [match different], 16", s.StatusLabels, s.ModelComparisons, s.RecordCount, wantStatuses)
	}
	data, err := json.Marshal(s)
	if err != nil || bytes.Contains(data, []byte("private-")) || bytes.Contains(data, []byte("900123456")) {
		t.Errorf("marshal(reversed kinds) error = %v, want no error or retained scalar values", err)
	}
}
