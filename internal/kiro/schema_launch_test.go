//go:build schemaprobe

package kiro

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

var schemaLaunch = flag.Bool("schema-probe-launch", false, "launch one separately approved catalogue probe")
var schemaPlanPath = flag.String("schema-probe-plan", "", "path to the separately reviewed plan")
var schemaPlanDigest = flag.String("schema-probe-plan-sha256", "", "SHA256 of the exact reviewed plan bytes")
var schemaCodeCommit = flag.String("schema-probe-code-commit", "", "reviewed clean code commit")

// This manifest is an input to the later live review, not authorization.
type schemaPlan struct {
	Version           int              `json:"version"`
	CodeCommit        string           `json:"code_commit"`
	Contract          map[string]any   `json:"contract"`
	Software          []schemaSoftware `json:"software"`
	SyntheticChecks   []string         `json:"synthetic_checks"`
	IndependentReview *string          `json:"independent_review"`
	Command           []string         `json:"command"`
	LiveApproval      *string          `json:"live_approval"`
}

type schemaSoftware struct {
	Role   string `json:"role"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func schemaContract() map[string]any {
	// JSON numbers and arrays keep the comparison independent of Go types.
	m, err := schemaJSON([]byte(`{
		"operation":"POST /",
		"target":"KiroControlPlaneBearerService.ListAvailableModels",
		"destinations":["https://management.us-east-1.kiro.dev:443/","https://management.eu-central-1.kiro.dev:443/"],
		"region_source":"combined_snapshot_profile_arn",
		"body":{"origin":"KIRO_CLI","maxResults":100,"profileArn":"combined_snapshot"},
		"headers":{"Authorization":"Bearer combined_snapshot_access_token","Content-Type":"application/x-amz-json-1.0","Accept":"application/json","Accept-Encoding":"identity","User-Agent":"kiro-gateway-schema-probe/1","X-Amzn-Codewhisperer-Optout":"true"},
		"native_sdk_differences":["fixed_user_agent","no_machine_fingerprint","no_sdk_retry_metadata","no_account_type_guess","explicit_json_accept","identity_encoding"],
		"model":"claude-opus-5.5",
		"paths":["system","system_prompt","messages","max_tokens","output_config.effort","reasoning.effort","thinking.type","thinking.display"],
		"allowed_values":["low","medium","high","max","minimal","none","xhigh","disabled","enabled","adaptive","summarized","omitted"],
		"limits":{"snapshots":1,"dispatches":1,"inference":0,"total_seconds":30,"work_seconds":25,"cleanup_seconds":5,"source_seconds":5,"headers_bytes":16384,"body_bytes":1048576,"depth":64,"models":100,"enum_entries":32,"report_bytes":16384,"safe_integer_max":2147483647},
		"policy":"spec_0004_AC16_unknown_path_rule_2026_10_05",
		"pagination":"never_follow",
		"recovery":"no_retry_renewal_fallback_or_mutation",
		"retention":"structural_report_only_no_raw_catalogue_schema_or_account_values",
		"ga_gates":"G1_G2_G3_G4_remain_open"
	}`), 16<<10)
	if err != nil {
		panic("invalid local schema contract")
	}
	return m
}

func schemaHex(s string, length int) bool {
	return len(s) == length && strings.Trim(s, "0123456789abcdef") == ""
}

func readSchemaPlan(data []byte, digest, commit string) (schemaPlan, error) {
	var p schemaPlan
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
	if d.Decode(&p) != nil || p.Version != 1 || p.CodeCommit != commit || !reflect.DeepEqual(p.Contract, schemaContract()) || p.IndependentReview == nil || strings.TrimSpace(*p.IndependentReview) == "" || p.LiveApproval == nil || *p.LiveApproval != "approved_for_one_catalogue_run" {
		return schemaPlan{}, errPlanInvalid
	}
	if !reflect.DeepEqual(p.SyntheticChecks, []string{"scripts/check:pass", "schema_probe_race:pass"}) || len(p.Software) != 2 {
		return schemaPlan{}, errPlanInvalid
	}
	for i, role := range []string{"native_binary", "bundled_source"} {
		entry := p.Software[i]
		if entry.Role != role || !filepath.IsAbs(entry.Path) || !schemaHex(entry.SHA256, 64) {
			return schemaPlan{}, errPlanInvalid
		}
	}
	return p, nil
}

// schemaHashFile never executes the native binary or bundled source. Regular
// file checks also prevent a named pipe from holding preflight indefinitely.
func schemaHashFile(ctx context.Context, path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<30 {
		return "", errPlanInvalid
	}
	f, err := os.Open(path)
	if err != nil {
		return "", errPlanInvalid
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return "", errPlanInvalid
	}
	h := sha256.New()
	buf := make([]byte, 32<<10)
	var total int64
	for {
		if ctx.Err() != nil {
			return "", errPlanInvalid
		}
		n, err := f.Read(buf)
		total += int64(n)
		if total > 1<<30 {
			return "", errPlanInvalid
		}
		_, _ = h.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", errPlanInvalid
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func schemaLivePreflight(ctx context.Context) (string, string, error) {
	invalid := func() (string, string, error) { return "", "", errPlanInvalid }
	if os.Getenv("GOTOOLCHAIN") != "local" || os.Getenv("GOWORK") != "off" || os.Getenv("CGO_ENABLED") != "1" || os.Getenv("GOFLAGS") != "" || runtime.Version() != "go1.27.1" {
		return invalid()
	}
	inputs, err := probeInputFiles("../..")
	if err != nil || checkProbeCode(ctx, *schemaCodeCommit, inputs, realProbeCommand) != nil {
		return invalid()
	}
	info, err := os.Lstat(*schemaPlanPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<10 {
		return invalid()
	}
	f, err := os.Open(*schemaPlanPath)
	if err != nil {
		return invalid()
	}
	data, readErr := io.ReadAll(io.LimitReader(f, (16<<10)+1))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil {
		return invalid()
	}
	p, err := readSchemaPlan(data, *schemaPlanDigest, *schemaCodeCommit)
	if err != nil {
		return invalid()
	}
	// Placeholders avoid a self referential plan digest. The reviewed command
	// fixes all behavior; only its digest is substituted after freezing the file.
	wantCommand := []string{"rtk", "proxy", "env", "GOTOOLCHAIN=local", "GOWORK=off", "CGO_ENABLED=1", "GOFLAGS=", "go", "test", "-mod=readonly", "-tags=schemaprobe", "./internal/kiro", "-run=^TestSchemaProbe$", "-count=1", "-v", "-args", "-schema-probe-launch", "-schema-probe-plan=" + *schemaPlanPath, "-schema-probe-plan-sha256=<PLAN_SHA256>", "-schema-probe-code-commit=" + *schemaCodeCommit}
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
	return *schemaCodeCommit, *schemaPlanDigest, nil
}

// TestSchemaProbe is the only real account entry point. Neither environment
// variables nor compiling the tag can enable its default false launch flag.
func TestSchemaProbe(t *testing.T) {
	if !*schemaLaunch {
		t.Skip("schema probe launch disabled")
	}
	ctx, stop := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r := runSchemaProbe(ctx, true, schemaDependencies{
		preflight: schemaLivePreflight,
		home:      os.UserHomeDir,
		open:      func(home string) (schemaStore, error) { return configstore.OpenExisting(home) },
		reader:    func(home string) ProfileReader { return credentials.Reader{Home: home} },
		wire:      transport{dial: schemaDial(net.DefaultResolver.LookupNetIP, (&net.Dialer{}).DialContext)},
	})
	b, err := encodeSchemaReport(r)
	if err != nil {
		t.Fatal("structural report encoding failed")
	}
	t.Log(string(b))
	if r.Outcome != "schema_observed" {
		t.Fail()
	}
}
