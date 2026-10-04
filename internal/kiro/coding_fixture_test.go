package kiro

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kiro-gateway/internal/bridge"
)

const boundaryTest = `
func TestClampUpperBoundary(t *testing.T) {
 if got := Clamp(11, 0, 10); got != 10 { t.Fatalf("Clamp(11,0,10) = %d, want 10", got) }
}
`

func codingFixture(t *testing.T) (string, bridgePlan) {
	t.Helper()
	data, err := os.ReadFile("testdata/bridge-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	var plan bridgePlan
	if json.Unmarshal(data, &plan) != nil {
		t.Fatal("fixture plan invalid")
	}
	repo := t.TempDir()
	for name, body := range plan.Files {
		writeCodingFile(t, repo, name, body)
	}
	return repo, plan
}
func writeCodingFile(t *testing.T, repo, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func codingCalls(repo string, plan bridgePlan) []bridge.Block {
	return []bridge.Block{
		{Type: "tool_use", ID: "fixture_read_code", Name: "Read", Input: map[string]any{"file_path": filepath.Join(repo, "clamp.go")}},
		{Type: "tool_use", ID: "fixture_read_tests", Name: "Read", Input: map[string]any{"file_path": filepath.Join(repo, "clamp_test.go")}},
		{Type: "tool_use", ID: "fixture_initial_edit", Name: "Edit", Input: map[string]any{"file_path": filepath.Join(repo, "clamp.go"), "old_string": "if value < lower { return upper }", "new_string": "if value < lower { return lower }"}},
		{Type: "tool_use", ID: "fixture_initial_test", Name: "Bash", Input: map[string]any{"command": codingTestCommand, "description": "Test disposable fixture"}},
		{Type: "tool_use", ID: "fixture_followup_edit", Name: "Edit", Input: map[string]any{"file_path": filepath.Join(repo, "clamp_test.go"), "old_string": plan.Files["clamp_test.go"], "new_string": plan.Files["clamp_test.go"] + boundaryTest}},
		{Type: "tool_use", ID: "fixture_followup_test", Name: "Bash", Input: map[string]any{"command": codingTestCommand, "description": "Test disposable fixture"}},
	}
}
