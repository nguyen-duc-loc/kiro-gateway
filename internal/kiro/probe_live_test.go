//go:build liveprobe

package kiro

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestProtocolProbe is excluded from ordinary checks. The current preparation
// has no live dispatcher: it can only skip or report a fixed rejection category.
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
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	inputs, err := probeInputFiles("../..")
	if err != nil || checkProbeCode(ctx, commit, inputs, realProbeCommand) != nil {
		t.Fatal(errCodeChanged)
	}
	f, err := os.Open("testdata/probe-plan.json")
	if err != nil {
		t.Fatal(errPlanInvalid)
	}
	defer f.Close()
	plan, err := readProbePlan(f, digest)
	if err != nil {
		t.Fatal(errPlanInvalid)
	}
	if checkProbeBaseline(ctx, realProbeEnvironment(), realProbeCommand) != nil {
		t.Fatal(errBaselineChanged)
	}
	// Stop before configuration, credential access, or network state.
	t.Fatal(plan.liveReadiness())
}
