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

// This explicit limitation applies only to the approved experiment.
const probeInstructionPolicy = wireInstructionPolicy

var (
	errPlanInvalid        = errors.New("plan_invalid")
	errNeedsEvidence      = errors.New("needs_evidence")
	errCodeChanged        = errors.New("code_changed")
	errConfiguration      = errors.New("configuration_invalid")
	errSessionChanged     = errors.New("session_changed")
	errProfileChanged     = errors.New("profile_changed")
	errProfileInvalid     = errors.New("profile_invalid")
	errProfileUnsupported = errors.New("profile_unsupported")
	errExpired            = errors.New("credential_expired")
	errSource             = errors.New("source_unavailable")
	errCanceled           = errors.New("canceled")
	errTimedOut           = errors.New("timed_out")
	errBudget             = errors.New("budget_exhausted")
	errIncomplete         = errors.New("stream_incomplete")
	errContract           = errors.New("contract_mismatch")
)

// probePlan binds a reviewable experiment to fixed implemented semantics.
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
	Limits             json.RawMessage   `json:"limits"`
	ObservationPolicy  json.RawMessage   `json:"observation_policy"`
	Cases              []json.RawMessage `json:"cases"`
	RequestExamples    []json.RawMessage `json:"request_examples"`
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
	if d.Decode(&p) != nil || p.SchemaVersion != 2 || p.Status != "prepared_limited" || p.ClientMapping != probeModel || p.TargetModel != probeModel || p.InstructionPolicy != probeInstructionPolicy || p.Baseline.ClaudeCode != "2.1.286" || p.Baseline.KiroCLI != "2.8.0" {
		return probePlan{}, errPlanInvalid
	}
	if err := p.liveReadiness(); err != nil {
		return probePlan{}, errPlanInvalid
	}
	return p, nil
}

func preparationPlan(t *testing.T) ([]byte, string) {
	t.Helper()
	b, err := os.ReadFile("testdata/probe-plan.json")
	if err != nil {
		t.Fatalf("ReadFile(preparation plan) error = %v, want nil", err)
	}
	h := sha256.Sum256(b)
	return b, hex.EncodeToString(h[:])
}

func TestPreparationPlanMatchesLimitedContract(t *testing.T) {
	b, digest := preparationPlan(t)
	p, err := readProbePlan(bytes.NewReader(b), digest)
	if err != nil {
		t.Fatalf("readProbePlan(prepared limited plan) error=%v, want nil", err)
	}
	if err := p.liveReadiness(); err != nil {
		t.Errorf("liveReadiness(limited plan) error=%v, want nil", err)
	}
	if len(p.Cases) != 6 || len(p.RequestExamples) != 6 || len(p.MissingContract) != 0 {
		t.Error("prepared plan does not contain six concrete cases and examples")
	}
}

func TestProbePlanRejectsDriftAndAmbiguity(t *testing.T) {
	b, _ := preparationPlan(t)
	for name, value := range map[string][]byte{
		"duplicate":                 bytes.Replace(b, []byte(`"schema_version": 2`), []byte(`"schema_version": 2, "schema_version": 2`), 1),
		"unknown":                   bytes.Replace(b, []byte(`"schema_version": 2`), []byte(`"unknown": "sentinel", "schema_version": 2`), 1),
		"model":                     bytes.ReplaceAll(b, []byte(probeModel), []byte("other-model")),
		"trailing":                  append(append([]byte{}, b...), []byte(`{}`)...),
		"too large":                 bytes.Repeat([]byte(" "), (64<<10)+1),
		"self promotion":            bytes.Replace(b, []byte(`"status": "prepared_limited"`), []byte(`"status": "ready"`), 1),
		"instruction policy drift":  bytes.Replace(b, []byte(probeInstructionPolicy), []byte("preserve_distinct_system_role"), 1),
		"instruction policy absent": bytes.Replace(b, []byte(`"instruction_policy": "translate_into_user_context",`), nil, 1),
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
