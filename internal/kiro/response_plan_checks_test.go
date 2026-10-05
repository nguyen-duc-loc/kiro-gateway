//go:build responsediscovery

package kiro

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestResponseDiscoveryPlanGate(t *testing.T) {
	commit := strings.Repeat("a", 40)
	base := responsePlan{Version: 1, CodeCommit: commit, Contract: responseContract("us-east-1"), Software: []responseSoftware{{Role: "native_binary", Path: "/synthetic/native", SHA256: strings.Repeat("b", 64)}, {Role: "bundled_source", Path: "/synthetic/source", SHA256: strings.Repeat("c", 64)}}, SyntheticChecks: []string{"scripts/check:pass", "response_discovery_race:pass"}, IndependentReview: responsePtr("synthetic review"), LiveApproval: responsePtr("approved_for_one_response_discovery_run")}
	for _, tc := range []struct {
		name   string
		change func(*responsePlan)
	}{
		{"valid", func(*responsePlan) {}}, {"no approval", func(p *responsePlan) { p.LiveApproval = nil }}, {"no review", func(p *responsePlan) { p.IndependentReview = nil }}, {"wrong model", func(p *responsePlan) { p.Contract["model"] = "other" }}, {"changed code", func(p *responsePlan) { p.CodeCommit = strings.Repeat("b", 40) }}, {"missing check", func(p *responsePlan) { p.SyntheticChecks = nil }}, {"missing source", func(p *responsePlan) { p.Software = p.Software[:1] }},
		{name: "blank review", change: func(p *responsePlan) { p.IndependentReview = responsePtr(" \n\t") }},
		{name: "wrong approval", change: func(p *responsePlan) { p.LiveApproval = responsePtr("approved_for_inference") }},
		{name: "wrong version", change: func(p *responsePlan) { p.Version = 2 }},
		{name: "relative source", change: func(p *responsePlan) { p.Software[1].Path = "source" }},
		{name: "invalid source digest", change: func(p *responsePlan) { p.Software[1].SHA256 = strings.Repeat("z", 64) }},
		{name: "wrong source role", change: func(p *responsePlan) { p.Software[1].Role = "native_binary" }},
		{name: "failed check", change: func(p *responsePlan) {
			p.SyntheticChecks = []string{"scripts/check:pass", "response_discovery_race:fail"}
		}},
		{name: "changed destination", change: func(p *responsePlan) { p.Contract["destinations"] = []any{"https://example.invalid/"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			p.Software = append([]responseSoftware(nil), base.Software...)
			p.Contract = responseContract("us-east-1")
			tc.change(&p)
			b, _ := json.Marshal(p)
			sum := sha256.Sum256(b)
			_, err := readResponseDiscoveryPlan(b, hex.EncodeToString(sum[:]), commit)
			if (err == nil) != (tc.name == "valid") {
				t.Errorf("readResponseDiscoveryPlan(%s) error=%v, want valid=%t", tc.name, err, tc.name == "valid")
			}
		})
	}
	b, _ := json.Marshal(base)
	sum := sha256.Sum256(b)
	if _, err := readResponseDiscoveryPlan(append(b, ' '), hex.EncodeToString(sum[:]), commit); err == nil {
		t.Error("readResponseDiscoveryPlan(changed bytes) accepted stale digest")
	}
	duplicate := bytes.Replace(b, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
	sum = sha256.Sum256(duplicate)
	if _, err := readResponseDiscoveryPlan(duplicate, hex.EncodeToString(sum[:]), commit); err == nil {
		t.Error("readResponseDiscoveryPlan(duplicate member) accepted ambiguous plan")
	}
}

func TestResponseDiscoveryPlanRejectsMalformedEnvelope(t *testing.T) {
	for _, data := range []string{`{"version":1,"unknown":1}`, `{"contract":{"x":1,"x":2}}`, `[]`, `{} {}`, "{\"x\":\"\xff\"}", strings.Repeat(" ", (16<<10)+1), `{"x":` + strings.Repeat(`[`, 64) + `0` + strings.Repeat(`]`, 64) + `}`} {
		sum := sha256.Sum256([]byte(data))
		if _, err := readResponseDiscoveryPlan([]byte(data), hex.EncodeToString(sum[:]), strings.Repeat("a", 40)); err == nil {
			t.Error("malformed discovery plan accepted")
		}
	}
}
func TestResponseDiscoveryContractEveryFieldBound(t *testing.T) {
	for key := range responseContract("us-east-1") {
		t.Run(key, func(t *testing.T) {
			f := newResponseDiscoveryPreflightFixture(t)
			f.plan.Contract[key] = "changed"
			f.save(t)
			responsePreflightRejectsBeforeAccess(t, t.Context())
		})
	}
}
func TestResponseDiscoveryPlanRequiresDedicatedSelection(t *testing.T) {
	for _, field := range []string{"test.run", "test.count"} {
		t.Run(field, func(t *testing.T) {
			newResponseDiscoveryPreflightFixture(t)
			value := ".*"
			if field == "test.count" {
				value = "2"
			}
			_ = flag.Set(field, value)
			responsePreflightRejectsBeforeAccess(t, t.Context())
		})
	}
}
func TestResponseDiscoveryReportLimit(t *testing.T) {
	for _, first := range []string{"", "http_status"} {
		t.Run(first, func(t *testing.T) {
			r := newResponseReport()
			if first != "" {
				r.fail(first)
			}
			r.BodySummary = &responseSummary{StatusLabels: []string{strings.Repeat("x", responseReportLimit)}}
			data := encodeResponseReport(&r)
			want := first
			if want == "" {
				want = "report_limit"
			}
			if r.FailureCategory == nil || *r.FailureCategory != want || r.BodySummary != nil || len(data) > responseReportLimit || r.Outcome != "needs_evidence" {
				t.Errorf("report overflow=%+v bytes=%d, want small envelope and %s", r, len(data), want)
			}
		})
	}
}
func TestResponseDiscoveryDisabledIsInert(t *testing.T) {
	r := runResponseDiscovery(t.Context(), false, responseDependencies{preflight: func(context.Context) (string, string, string, error) { panic("disabled preflight") }})
	if r.DispatchCount != 0 || r.CodeCommit != nil || r.RunID != nil || r.CleanupOutcome != "complete" {
		t.Errorf("disabled discovery=%+v, want no observations", r)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestResponseDiscoveryProbe$", "-test.v", "-test.count=1", "-response-discovery-launch=false", "-response-discovery-plan=/does/not/exist")
	cmd.Env = append(os.Environ(), "RESPONSE_DISCOVERY_LAUNCH=true", "KIRO_RESPONSE_DISCOVERY_LAUNCH=true", "GOFLAGS=-args -response-discovery-launch=true")
	out, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("response discovery launch disabled")) || !bytes.Contains(out, []byte("SKIP")) {
		t.Errorf("disabled entry=%v output=%s, want skip under hostile environment", err, out)
	}
}
func TestResponseDiscoveryDeadlineBeforeSetup(t *testing.T) {
	// Reserving five seconds for cleanup consumes this shorter parent budget.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	r := runResponseDiscovery(ctx, true, responseDependencies{preflight: func(context.Context) (string, string, string, error) { panic("setup after cutoff") }})
	if r.FailureCategory == nil || *r.FailureCategory != "timeout" || r.DispatchCount != 0 || r.ElapsedMillis > 100 {
		t.Errorf("short parent budget=%+v, want immediate timeout before setup", r)
	}
}
func TestResponseDiscoveryCanceledDecoder(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, media := range []string{"json", "sse", "eventstream"} {
		body := []byte(`{}`)
		if media == "sse" {
			body = []byte("data: {}\n\n")
		}
		if media == "eventstream" {
			body = probeFrame("message", `{}`)
		}
		s, f := observeResponse(ctx, body, media)
		if s != nil || f != "canceled" {
			t.Errorf("canceled %s=%+v,%s, want nil,canceled", media, s, f)
		}
	}
}
func TestResponseDiscoveryOutputFailure(t *testing.T) {
	r := newResponseReport()
	if emitResponseReport(r, responseBrokenWriter{}) {
		t.Error("failed output reported success")
	}
}

type responseBrokenWriter struct{}

func (responseBrokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
