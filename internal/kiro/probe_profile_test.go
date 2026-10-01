package kiro

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"kiro-gateway/internal/credentials"
)

// covers: spec 0003 AC-1, AC-2, AC-6, AC-8, AC-9. Profile routing remains local.
func TestOfflineProfileRegionSelection(t *testing.T) {
	for _, region := range []string{"us-east-1", "eu-central-1"} {
		t.Run(region, func(t *testing.T) {
			home, db := probeHome(t)
			if _, err := db.Exec(`UPDATE state SET value=?`, strings.Replace(probeSyntheticProfile, "us-east-1", region, 1)); err != nil {
				t.Fatal(err)
			}
			var east, eu atomic.Int32
			eastServer, roots := probeServer(t, func(http.ResponseWriter, *http.Request) { east.Add(1) })
			euServer, _ := probeServer(t, func(http.ResponseWriter, *http.Request) { eu.Add(1) })
			roots.AddCert(euServer.Certificate())
			endpoints := map[string]string{"us-east-1": eastServer.URL + "/probe", "eu-central-1": euServer.URL + "/probe"}
			p, err := openRegionalOfflineProbe(t.Context(), home, endpoints, roots, defaultProbeLimits)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.close)
			// Mutation of caller owned plan data cannot change the frozen destination set.
			endpoints[region] = "https://example.com/probe"
			got := p.dispatch(probeSyntheticBody)
			wantEast, wantEU := int32(1), int32(0)
			if region == "eu-central-1" {
				wantEast, wantEU = 0, 1
			}
			if east.Load() != wantEast || eu.Load() != wantEU || got.Attempts != 1 || got.Cause != "stream_incomplete" {
				t.Errorf("dispatch(profile %s) east=%d eu=%d result=%+v, want exactly one request to profile region", region, east.Load(), eu.Load(), got)
			}
		})
	}
}

func TestOfflineProfileChangesStopBeforeNextAttempt(t *testing.T) {
	for _, change := range []struct{ name, profile, cause string }{
		{"region", strings.Replace(probeSyntheticProfile, "us-east-1", "eu-central-1", 1), "profile_changed"},
		{"whitespace", probeSyntheticProfile + " ", "profile_changed"},
		{"ignored name", strings.Replace(probeSyntheticProfile, "sentinel-name", "different-name", 1), "profile_changed"},
		{"ignored member", strings.TrimSuffix(probeSyntheticProfile, "}") + `,"extra":"sentinel-extra"}`, "profile_changed"},
		{"invalid profile", `{"arn":"sentinel"}`, "profile_invalid"},
		{"unsupported profile", strings.Replace(probeSyntheticProfile, "us-east-1", "ap-south-1", 1), "profile_unsupported"},
	} {
		t.Run(change.name, func(t *testing.T) {
			home, db := probeHome(t)
			s, roots, requests := sequenceServer(t, func(index int, _ fixtureRequest, events []fixtureResponseEvent) []fixtureResponseEvent {
				if index == 0 {
					if _, err := db.Exec(`UPDATE state SET value=?`, change.profile); err != nil {
						t.Errorf("UPDATE(profile) error=%v, want nil", err)
					}
				}
				return events
			})
			p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
			got := runFixtureCases(p, probeMaxRetained)
			if requests.Load() != 1 || got.Attempts != 1 || got.Cases[1].Cause != change.cause || got.Verdict != "needs_evidence" || got.Cases[2].Status != "unrun" {
				t.Errorf("runFixtureCases(%s) = %+v, want one request then %s and unrun dependents", change.name, got, change.cause)
			}
			if p.endpoint.String() != s.URL+"/probe" {
				t.Error("profile change replaced frozen destination")
			}
			digest := sha256.Sum256(append([]byte("kiro-gateway/probe-profile-v1\x00"), []byte(probeSyntheticProfile)...))
			output, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"sentinel", "000000000000", hex.EncodeToString(digest[:]), p.reference.Fingerprint, "us-east-1"} {
				if strings.Contains(string(output), secret) {
					t.Error("runFixtureCases(profile change) leaked account data")
				}
			}
		})
	}
}

func TestOfflineProfileMissingAndUnmapped(t *testing.T) {
	for _, mode := range []string{"missing row", "missing table", "unmapped region"} {
		t.Run(mode, func(t *testing.T) {
			home, db := probeHome(t)
			cause := "profile_invalid"
			switch mode {
			case "missing row":
				if _, err := db.Exec(`DELETE FROM state`); err != nil {
					t.Fatal(err)
				}
			case "missing table":
				if _, err := db.Exec(`DROP TABLE state`); err != nil {
					t.Fatal(err)
				}
				cause = "source_unavailable"
			case "unmapped region":
				cause = "plan_invalid"
			}
			var requests atomic.Int32
			s, roots := probeServer(t, func(http.ResponseWriter, *http.Request) { requests.Add(1) })
			p, err := openRegionalOfflineProbe(t.Context(), home, map[string]string{"eu-central-1": s.URL + "/probe"}, roots, defaultProbeLimits)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.close)
			got := p.dispatch(probeSyntheticBody)
			if got.Cause != cause || got.Attempts != 0 || requests.Load() != 0 {
				t.Errorf("dispatch(%s) = %+v, want %s before dispatch", mode, got, cause)
			}
		})
	}
}

func TestOfflineProfilePinIsPerRun(t *testing.T) {
	home, db := probeHome(t)
	s, roots := probeServer(t, func(http.ResponseWriter, *http.Request) {})
	first := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	if got := first.dispatch(probeSyntheticBody); got.Attempts != 1 {
		t.Fatalf("dispatch(first profile) = %+v, want one attempt", got)
	}
	oldDigest := first.profileDigest
	first.close()
	if _, err := db.Exec(`UPDATE state SET value=?`, strings.Replace(probeSyntheticProfile, "us-east-1", "eu-central-1", 1)); err != nil {
		t.Fatal(err)
	}
	second := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := second.dispatch(probeSyntheticBody)
	if got.Attempts != 1 || second.profileDigest == oldDigest {
		t.Errorf("dispatch(new run profile) = %+v, want one attempt with new pin", got)
	}
	if second.reference != first.reference {
		t.Error("new profile required relinking, want unchanged token reference")
	}
}

func TestOfflineProfilePlanRejectsUnapprovedDestination(t *testing.T) {
	_, roots := probeServer(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected request") })
	p, err := openRegionalOfflineProbe(t.Context(), "absent", map[string]string{"us-east-1": "https://127.0.0.1:1234/probe", "eu-central-1": "https://example.com/probe"}, roots, defaultProbeLimits)
	if p != nil || !errors.Is(err, errPlanInvalid) {
		t.Errorf("openRegionalOfflineProbe(external destination) error=%v, want plan_invalid before store access", err)
	}
}

func TestOfflineProfileSentinelsDoNotEnterSavedConfig(t *testing.T) {
	home, _ := probeHome(t)
	s, roots, requests := sequenceServer(t, nil)
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := runFixtureCases(p, probeMaxRetained)
	if got.Verdict != "candidate_supported" || requests.Load() != 6 {
		t.Fatalf("runFixtureCases(profile source) = %+v, want six synthetic observations", got)
	}
	d, _, err := p.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := (credentials.Reader{Home: home}).ReadProfileSnapshot(t.Context(), p.reference)
	if err != nil {
		t.Fatal(err)
	}
	digest := snapshot.ProfileDigest()
	for _, secret := range []string{"sentinel", snapshot.ProfileARN(), hex.EncodeToString(digest[:]), "us-east-1"} {
		if strings.Contains(string(encoded), secret) {
			t.Error("saved config contains profile material, want original token reference only")
		}
	}
}
