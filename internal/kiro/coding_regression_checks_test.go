package kiro

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestUpperMutationChangesOnlyReturnIdentifier(t *testing.T) {
	source := []byte("package fixture\nfunc Clamp(value, lower, upper int) int {\n if value < lower { return lower }; if value > upper { /* keep */ return upper }; return value\n}\n")
	got, ok := upperMutation(source)
	want := strings.Replace(string(source), "/* keep */ return upper", "/* keep */ return lower", 1)
	if !ok || string(got) != want {
		t.Errorf("upperMutation(valid shape) applicable=%t exact=%t, want true and only upper return replaced", ok, string(got) == want)
	}
	for _, tc := range []struct{ name, old, new string }{
		{"renamed", "func Clamp(", "func Other("},
		{"wrong signature", "value, lower, upper int", "value, upper, lower int"},
		{"refactored", "value > upper", "upper < value"},
		{"wrong return", "/* keep */ return upper", "return value"},
		{"initializer", "if value > upper", "if x := 1; value > upper"},
		{"else", "return upper };", "return upper } else { return lower };"},
		{"duplicate", "return value", "if value > upper { return upper }; return value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := upperMutation([]byte(strings.Replace(string(source), tc.old, tc.new, 1))); ok {
				t.Error("upperMutation(inapplicable shape) succeeded, want incomplete")
			}
		})
	}
}

// covers: AC-12. Empty, wrong, nonexecuting, build failing and inapplicable
// mutations cannot award regression value. The client source remains untouched.
func TestBoundaryRegressionRequiresAssertionFailure(t *testing.T) {
	for _, mode := range []string{"valid", "empty", "wrong boundary", "wrong expected", "unreachable", "syntax", "mutation inapplicable"} {
		t.Run(mode, func(t *testing.T) {
			repo, plan := codingFixture(t)
			source := strings.Replace(plan.Files["clamp.go"], "if value < lower { return upper }", "if value < lower { return lower }", 1)
			test := boundaryTest
			switch mode {
			case "empty":
				test = "\nfunc TestClampUpperBoundary(t *testing.T) {}\n"
			case "wrong boundary":
				test = strings.Replace(test, "Clamp(11, 0, 10)", "Clamp(10, 0, 10)", 1)
			case "wrong expected":
				test = strings.Replace(test, "got != 10", "got != 0", 1)
			case "unreachable":
				test = strings.Replace(test, "\n if got", "\n return\n if got", 1)
			case "syntax":
				test = "\nfunc broken("
			case "mutation inapplicable":
				source = strings.Replace(source, "value > upper", "upper < value", 1)
			}
			writeCodingFile(t, repo, "clamp.go", source)
			writeCodingFile(t, repo, "clamp_test.go", plan.Files["clamp_test.go"]+test)
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			got := verifyBoundaryRegression(ctx, repo)
			if got != (mode == "valid") {
				t.Errorf("verifyBoundaryRegression(%s)=%t, want %t", mode, got, mode == "valid")
			}
			after, err := fixtureSource(repo, "clamp.go")
			if err != nil || string(after) != source {
				t.Error("regression check changed original source")
			}
		})
	}
}

func TestFixtureTestDeadline(t *testing.T) {
	repo, plan := codingFixture(t)
	writeCodingFile(t, repo, "clamp.go", strings.Replace(plan.Files["clamp.go"], "if value < lower { return upper }", "if value < lower { return lower }", 1))
	writeCodingFile(t, repo, "clamp_test.go", `package bridgefixture
 import ("testing"; "time"; "os")
 func TestClampUpperBoundary(t *testing.T) { _ = os.WriteFile("started",[]byte("1"),0600); time.Sleep(time.Hour) }
 `)
	if _, err := fixtureTest(t.Context(), repo, "-run", "^$", "."); err != nil {
		t.Fatal("fixture build failed")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	started := time.Now()
	_, err := fixtureTest(ctx, repo, "-count=1", "-run", "^TestClampUpperBoundary$", ".")
	if err == nil || ctx.Err() != context.DeadlineExceeded || time.Since(started) > 3*time.Second {
		t.Errorf("blocked fixture error=%v context=%v elapsed=%v, want deadline failure within 3s", err, ctx.Err(), time.Since(started))
	}
	if _, err := fixtureSource(repo, "started"); err != nil {
		t.Error("deadline fixture never entered test body")
	}
}
