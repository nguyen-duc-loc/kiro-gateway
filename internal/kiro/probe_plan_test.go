package kiro

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"kiro-gateway/internal/jsonobject"
)

const probeModel = "claude-sonnet-5"

// The operator explicitly retained this requirement after inspecting the
// current binary. A synthetic instructions field is not an upstream role.
const probeInstructionPolicy = "preserve_distinct_system_role"

var (
	errPlanInvalid    = errors.New("plan_invalid")
	errNeedsEvidence  = errors.New("needs_evidence")
	errCodeChanged    = errors.New("code_changed")
	errConfiguration  = errors.New("configuration_invalid")
	errSessionChanged = errors.New("session_changed")
	errExpired        = errors.New("credential_expired")
	errSource         = errors.New("source_unavailable")
	errCanceled       = errors.New("canceled")
	errTimedOut       = errors.New("timed_out")
	errBudget         = errors.New("budget_exhausted")
	errIncomplete     = errors.New("stream_incomplete")
	errContract       = errors.New("contract_mismatch")
)

// This is an incomplete preparation artifact, not an executable request schema.
// A future concrete candidate must add its decoder and assertions before it can
// replace this closed gate. Editing status or supplying environment values alone
// cannot make this implementation dispatch a live request.
type probePlan struct {
	SchemaVersion     int    `json:"schema_version"`
	Status            string `json:"status"`
	ClientMapping     string `json:"client_mapping"`
	TargetModel       string `json:"target_model"`
	InstructionPolicy string `json:"instruction_policy"`
	Baseline          struct {
		ClaudeCode string `json:"claude_code"`
		KiroCLI    string `json:"kiro_cli"`
	} `json:"baseline"`
	Destination        json.RawMessage   `json:"destination"`
	RegionRule         json.RawMessage   `json:"region_rule"`
	Authentication     json.RawMessage   `json:"authentication"`
	RequestSchema      json.RawMessage   `json:"request_schema"`
	InstructionMapping json.RawMessage   `json:"instruction_mapping"`
	Controls           json.RawMessage   `json:"controls"`
	Completion         json.RawMessage   `json:"completion"`
	Cases              []json.RawMessage `json:"cases"`
	MissingContract    []string          `json:"missing_contract"`
	OfflineFraming     string            `json:"offline_framing"`
	OfflineEventLabels []string          `json:"offline_event_labels"`
	Sources            []struct {
		ID        string `json:"id"`
		Reference string `json:"reference"`
		SHA256    string `json:"sha256,omitempty"`
		Finding   string `json:"finding"`
	} `json:"sources"`
}

func readProbePlan(r io.Reader, expectedDigest string) (probePlan, error) {
	var p probePlan
	b, err := io.ReadAll(io.LimitReader(r, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		return p, errPlanInvalid
	}
	digest := sha256.Sum256(b)
	if hex.EncodeToString(digest[:]) != expectedDigest {
		return p, errPlanInvalid
	}
	if _, err := jsonobject.Parse(b, 64<<10); err != nil {
		return p, errPlanInvalid
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil || p.SchemaVersion != 1 || p.Status != "needs_evidence" || p.ClientMapping != probeModel || p.TargetModel != probeModel || p.InstructionPolicy != probeInstructionPolicy || p.Baseline.ClaudeCode != "2.1.285" || p.Baseline.KiroCLI != "2.8.0" {
		return probePlan{}, errPlanInvalid
	}
	return p, nil
}

// No live destination, request schema, or success decoder is implemented yet.
// The gate stays closed even if someone fills the plan fields speculatively.
func (p probePlan) liveReadiness() error { return errNeedsEvidence }

func preparationPlan(t *testing.T) ([]byte, string) {
	t.Helper()
	b, err := os.ReadFile("testdata/probe-plan.json")
	if err != nil {
		t.Fatalf("ReadFile(preparation plan) error = %v, want nil", err)
	}
	h := sha256.Sum256(b)
	return b, hex.EncodeToString(h[:])
}

func TestPreparationPlanRemainsClosed(t *testing.T) {
	b, digest := preparationPlan(t)
	p, err := readProbePlan(bytes.NewReader(b), digest)
	if err != nil {
		t.Fatalf("readProbePlan(checked artifact) error = %v, want nil", err)
	}
	if got := p.liveReadiness(); !errors.Is(got, errNeedsEvidence) {
		t.Errorf("liveReadiness(incomplete contract) = %v, want needs_evidence", got)
	}
	if len(p.MissingContract) != 8 || len(p.Cases) != 0 {
		t.Error("preparation plan claims runnable cases, want eight explicit gaps and no cases")
	}
}

func TestProbePlanRejectsDriftAndAmbiguity(t *testing.T) {
	b, _ := preparationPlan(t)
	for name, value := range map[string][]byte{
		"duplicate":                 bytes.Replace(b, []byte(`"schema_version": 1`), []byte(`"schema_version": 1, "schema_version": 1`), 1),
		"unknown":                   bytes.Replace(b, []byte(`"schema_version": 1`), []byte(`"unknown": "sentinel", "schema_version": 1`), 1),
		"model":                     bytes.ReplaceAll(b, []byte(probeModel), []byte("other-model")),
		"trailing":                  append(append([]byte{}, b...), []byte(`{}`)...),
		"too large":                 bytes.Repeat([]byte(" "), (64<<10)+1),
		"self promotion":            bytes.Replace(b, []byte(`"status": "needs_evidence"`), []byte(`"status": "ready"`), 1),
		"instruction downgrade":     bytes.Replace(b, []byte(probeInstructionPolicy), []byte("translate_into_user_context"), 1),
		"instruction policy absent": bytes.Replace(b, []byte(`"instruction_policy": "preserve_distinct_system_role",`), nil, 1),
	} {
		t.Run(name, func(t *testing.T) {
			h := sha256.Sum256(value)
			_, err := readProbePlan(bytes.NewReader(value), hex.EncodeToString(h[:]))
			if !errors.Is(err, errPlanInvalid) {
				t.Errorf("readProbePlan(%s) error = %v, want plan_invalid", name, err)
			}
		})
	}
	_, err := readProbePlan(bytes.NewReader(b), "wrong-digest")
	if !errors.Is(err, errPlanInvalid) {
		t.Errorf("readProbePlan(wrong digest) error = %v, want plan_invalid", err)
	}
}
