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
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

var responseLaunch = flag.Bool("response-discovery-launch", false, "launch one separately approved response discovery request")
var responsePlanPath = flag.String("response-discovery-plan", "", "path to the separately reviewed plan")
var responsePlanDigest = flag.String("response-discovery-plan-sha256", "", "SHA256 of the exact reviewed plan bytes")
var responseCodeCommit = flag.String("response-discovery-code-commit", "", "reviewed clean code commit")

// This manifest is an input to the later live review, not authorization.
type responsePlan struct {
	Version           int                `json:"version"`
	CodeCommit        string             `json:"code_commit"`
	Contract          map[string]any     `json:"contract"`
	Software          []responseSoftware `json:"software"`
	SyntheticChecks   []string           `json:"synthetic_checks"`
	IndependentReview *string            `json:"independent_review"`
	Command           []string           `json:"command"`
	LiveApproval      *string            `json:"live_approval"`
}

type responseSoftware struct {
	Role   string `json:"role"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func responseContract(region string) map[string]any {
	contract := map[string]any{
		"hypothesis_id": responseHypothesis, "operation": "POST /", "target": responseTarget,
		"expected_region": region, "destination": responseEndpoint(region),
		"body": responseBody("<FROM_COMBINED_SNAPSHOT>"), "headers": responseHeaders("combined_snapshot_access_token"),
		"paths": responsePaths(), "event_labels": responseLabels(), "path_kinds": responseKinds(), "status_labels": responseStatuses(), "error_labels": responseErrors(), "model_comparisons": []string{"match", "different"},
		"media_types": map[string]string{"application/json": "json", "application/x-amz-json-1.0": "json", "text/event-stream": "sse", "application/vnd.amazon.eventstream": "eventstream"},
		"channels":    []string{"json", "sse", "eventstream"}, "message_kinds": []string{"event", "error", "exception"}, "special_labels": []string{"other", "invalid"}, "done_marker": "[DONE]",
		"outcomes": []string{"response_observed", "needs_evidence"}, "cleanup_outcomes": []string{"complete", "failed"},
		"failure_categories":     strings.Fields("preflight config_missing config_invalid busy source_changed source_expired source_unavailable unsupported_region region_mismatch transport http_status response_format response_limit invalid_response timeout canceled cleanup report_limit"),
		"body_failures":          strings.Fields("informational_response unsupported_media invalid_headers limit invalid_utf8 invalid_json invalid_sse invalid_eventstream empty_body transport timeout canceled"),
		"limits":                 map[string]int{"plan_bytes": 16384, "software_file_bytes": 2 << 30, "hash_buffer_bytes": 32768, "body_detection_bytes": 1, "snapshots": 1, "dispatches": 1, "total_seconds": 30, "work_seconds": 25, "cleanup_seconds": 5, "source_seconds": 5, "failed_output_seconds": 1, "headers_bytes": 16384, "body_bytes": responseBodyLimit, "depth": 64, "sse_line_bytes": 16384, "sse_data_bytes": 65536, "frame_bytes": 65536, "frame_header_bytes": 8192, "records": 256, "sequence_records": 32, "report_bytes": responseReportLimit},
		"native_sdk_differences": []string{"fixed_user_agent", "no_ide_metadata", "no_machine_fingerprint"},
		"policies":               []string{"spec_0004_AC17_2026_10_05", "one_first_header_block_reject_all_1xx", "single_content_type_identity_encoding_utf8_charset_only", "object_json_utf8_no_duplicates_or_trailing_values", "sse_lf_crlf_join_data_one_event_no_pending_fields_at_eof", "eventstream_validate_all_headers_lengths_and_both_crcs", "public_ipv4_pinned_verified_tls_http1", "no_proxy_redirect_decompression_reuse_replay", "no_tools_continuation_renewal_fallback_mutation", "observe_through_eof_no_semantic_terminal", "finite_projection_only_no_text_or_digests", "first_observed_cause_cancellation_precedence", "failed_cleanup_retains_lock_until_process_exit", "G1_G2_G3_G4_remain_open"},
	}
	data, _ := json.Marshal(contract)
	result, err := schemaJSON(data, responseReportLimit)
	if err != nil {
		panic("invalid compiled discovery contract")
	}
	return result
}

func readResponseDiscoveryPlan(data []byte, digest, commit string) (responsePlan, error) {
	var p responsePlan
	sum := sha256.Sum256(data)
	if !schemaHex(digest, 64) || hex.EncodeToString(sum[:]) != digest || (!schemaHex(commit, 40) && !schemaHex(commit, 64)) {
		return p, errPlanInvalid
	}
	if _, err := schemaJSON(data, 16<<10); err != nil {
		return p, errPlanInvalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil || p.Version != 1 || responseEndpoint(planResponseRegion(p)) == "" || p.CodeCommit != commit || !reflect.DeepEqual(p.Contract, responseContract(planResponseRegion(p))) || p.IndependentReview == nil || strings.TrimSpace(*p.IndependentReview) == "" || p.LiveApproval == nil || *p.LiveApproval != "approved_for_one_response_discovery_run" {
		return responsePlan{}, errPlanInvalid
	}
	if !reflect.DeepEqual(p.SyntheticChecks, []string{"scripts/check:pass", "response_discovery_race:pass"}) || len(p.Software) != 2 {
		return responsePlan{}, errPlanInvalid
	}
	for i, role := range []string{"native_binary", "bundled_source"} {
		entry := p.Software[i]
		if entry.Role != role || !filepath.IsAbs(entry.Path) || !schemaHex(entry.SHA256, 64) {
			return responsePlan{}, errPlanInvalid
		}
	}
	return p, nil
}

func responseLivePreflight(ctx context.Context) (string, string, string, error) {
	invalid := func() (string, string, string, error) { return "", "", "", errPlanInvalid }
	if os.Getenv("GOTOOLCHAIN") != "local" || os.Getenv("GOWORK") != "off" || os.Getenv("CGO_ENABLED") != "1" || os.Getenv("GOFLAGS") != "" || runtime.Version() != "go1.27.1" {
		return invalid()
	}
	if flag.Lookup("test.run").Value.String() != "^TestResponseDiscoveryProbe$" || flag.Lookup("test.count").Value.String() != "1" {
		return invalid()
	}
	inputs, err := probeInputFiles("../..")
	if err != nil || checkProbeCode(ctx, *responseCodeCommit, inputs, realProbeCommand) != nil {
		return invalid()
	}
	info, err := os.Lstat(*responsePlanPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<10 {
		return invalid()
	}
	f, err := os.Open(*responsePlanPath)
	if err != nil {
		return invalid()
	}
	data, readErr := io.ReadAll(io.LimitReader(f, (16<<10)+1))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil {
		return invalid()
	}
	p, err := readResponseDiscoveryPlan(data, *responsePlanDigest, *responseCodeCommit)
	if err != nil {
		return invalid()
	}
	// Placeholders avoid a self referential plan digest. The reviewed command
	// fixes all behavior; only its digest is substituted after freezing the file.
	wantCommand := []string{"rtk", "proxy", "env", "GOTOOLCHAIN=local", "GOWORK=off", "CGO_ENABLED=1", "GOFLAGS=", "go", "test", "-mod=readonly", "-tags=responsediscovery", "./internal/kiro", "-run=^TestResponseDiscoveryProbe$", "-count=1", "-v", "-args", "-response-discovery-launch", "-response-discovery-plan=" + *responsePlanPath, "-response-discovery-plan-sha256=<PLAN_SHA256>", "-response-discovery-code-commit=" + *responseCodeCommit}
	if !reflect.DeepEqual(p.Command, wantCommand) {
		return invalid()
	}
	for _, software := range p.Software {
		digest, err := schemaHashFile(ctx, software.Path)
		if err != nil || digest != software.SHA256 {
			return invalid()
		}
	}
	if ctx.Err() != nil {
		return invalid()
	}
	return *responseCodeCommit, *responsePlanDigest, planResponseRegion(p), nil
}

func planResponseRegion(p responsePlan) string {
	s, _ := p.Contract["expected_region"].(string)
	return s
}

// TestResponseDiscoveryProbe is the sole real account entry point. Disabled
// launch skips before any preflight, home lookup, source read or network call.
func TestResponseDiscoveryProbe(t *testing.T) {
	if !*responseLaunch {
		t.Skip("response discovery launch disabled")
	}
	ctx, stop := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r := runResponseDiscovery(ctx, true, responseDependencies{
		preflight: responseLivePreflight, home: os.UserHomeDir,
		open:   func(home string) (responseStore, error) { return configstore.OpenExisting(home) },
		reader: func(home string) ProfileReader { return credentials.Reader{Home: home} },
		wire:   transport{dial: publicDial},
	})
	if !emitResponseReport(r, os.Stdout) {
		t.Error("structural report output failed")
	}
	if r.Outcome != "response_observed" {
		t.Fail()
	}
}

// Failed cleanup allows one local output attempt and then terminates. Neither
// a blocked writer nor an unresolved transport worker survives process exit.
func emitResponseReport(r responseReport, w io.Writer) bool {
	data := append(encodeResponseReport(&r), '\n')
	if r.CleanupOutcome != "failed" {
		n, err := w.Write(data)
		return err == nil && n == len(data)
	}
	done := make(chan struct{})
	go func() { defer close(done); _, _ = w.Write(data) }()
	timer := time.NewTimer(time.Second)
	select {
	case <-done:
	case <-timer.C:
	}
	timer.Stop()
	os.Exit(1)
	return false
}
