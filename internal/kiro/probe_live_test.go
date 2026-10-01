//go:build liveprobe

package kiro

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// TestProtocolProbe is excluded from ordinary checks. Human approval for this
// exact plan and code must precede an explicit launch; env values are not consent.
func TestProtocolProbe(t *testing.T) {
	gate := os.Getenv("KIRO_GATEWAY_LIVE_PROBE")
	digest := os.Getenv("KIRO_GATEWAY_PROBE_PLAN_SHA256")
	model := os.Getenv("KIRO_GATEWAY_PROBE_MODEL")
	commit := os.Getenv("KIRO_GATEWAY_PROBE_CODE_COMMIT")
	if gate != "1" {
		t.Skip("live probe disabled")
	}
	if model != probeModel || len(digest) != 64 {
		t.Fatal(errPlanInvalid)
	}
	parent, stop := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	inputs, err := probeInputFiles("../..")
	if err != nil || checkProbeCode(ctx, commit, inputs, realProbeCommand) != nil {
		t.Fatal(errCodeChanged)
	}
	f, err := os.Open("testdata/probe-plan.json")
	if err != nil {
		t.Fatal(errPlanInvalid)
	}
	plan, err := readProbePlan(f, digest)
	f.Close()
	if err != nil || plan.liveReadiness() != nil {
		t.Fatal(errPlanInvalid)
	}
	if checkProbeBaseline(ctx, realProbeEnvironment(), realProbeCommand) != nil {
		t.Fatal(errBaselineChanged)
	}
	// Account access begins only after all launch checks pass.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(errConfiguration)
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatal(errNeedsEvidence)
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	runID := fmt.Sprintf("%x-%x-%x-%x-%x", id[0:4], id[4:6], id[6:8], id[8:10], id[10:16])
	destinations := make(map[string]*url.URL)
	for region, raw := range wireDestinations() {
		u, err := wireDestination(raw)
		if err != nil {
			t.Fatal(errPlanInvalid)
		}
		destinations[region] = u
	}
	started := time.Now()
	runContext, cancelRun := context.WithTimeout(parent, defaultProbeLimits.run)
	defer cancelRun()
	p, err := openProbeState(runContext, home, destinations, nil, defaultProbeLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()
	p.wire = true
	p.dial = wireDial(net.DefaultResolver.LookupNetIP, (&net.Dialer{}).DialContext)
	result := runWireCases(p, runID, probeMaxRetained)
	destination := ""
	if p.endpoint != nil {
		destination = p.endpoint.String()
	}
	summary := struct {
		RunID          string        `json:"run_id"`
		PlanDigest     string        `json:"plan_digest"`
		CodeCommit     string        `json:"code_commit"`
		Platform       string        `json:"platform"`
		ClaudeCode     string        `json:"claude_code"`
		KiroCLI        string        `json:"kiro_cli"`
		RequestedModel string        `json:"requested_model"`
		Destination    string        `json:"destination"`
		Started        time.Time     `json:"started"`
		ElapsedMillis  int64         `json:"elapsed_millis"`
		Result         wireRunResult `json:"result"`
	}{runID, digest, commit, runtime.GOOS + "/" + runtime.GOARCH, "2.1.286", "2.8.0", probeModel, destination, started.UTC(), time.Since(started).Milliseconds(), result}
	b, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(errNeedsEvidence)
	}
	t.Log(string(b))
	if result.Verdict != "limited_candidate_observed" {
		t.Fail()
	}
}
