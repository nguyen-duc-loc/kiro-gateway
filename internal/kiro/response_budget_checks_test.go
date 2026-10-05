//go:build responsediscovery

package kiro

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/credentials"
)

func TestResponseDiscoveryExactWorkBudget(t *testing.T) {
	for _, parentSeconds := range []int{0, 20} {
		t.Run(time.Duration(parentSeconds).String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				parent := t.Context()
				if parentSeconds != 0 {
					ctx, cancel := context.WithTimeout(parent, time.Duration(parentSeconds)*time.Second)
					defer cancel()
					parent = ctx
				}
				d := responseDependencies{preflight: func(ctx context.Context) (string, string, string, error) {
					<-ctx.Done()
					return "", "", "", errPlanInvalid
				}}
				started := time.Now()
				r := runResponseDiscovery(parent, true, d)
				want := 25 * time.Second
				if parentSeconds != 0 {
					want = 15 * time.Second
				}
				if r.FailureCategory == nil || *r.FailureCategory != "timeout" || r.DispatchCount != 0 || time.Since(started) != want || r.ElapsedMillis != want.Milliseconds() {
					t.Errorf("work deadline=%+v elapsed=%v, want timeout at %v before setup", r, time.Since(started), want)
				}
			})
		})
	}
}
func TestResponseDiscoverySourceBudget(t *testing.T) {
	home, _ := probeHome(t)
	d := responseFixtureDependencies(t, home)
	d.reader = func(string) ProfileReader {
		return wireSnapshotFunc(func(ctx context.Context, _ config.Session) (credentials.ProfileSnapshot, error) {
			<-ctx.Done()
			return credentials.ProfileSnapshot{}, credentials.ErrChanged
		})
	}
	synctest.Test(t, func(t *testing.T) {
		started := time.Now()
		r := runResponseDiscovery(t.Context(), true, d)
		if r.FailureCategory == nil || *r.FailureCategory != "timeout" || time.Since(started) != 5*time.Second || r.DispatchCount != 0 || r.CleanupOutcome != "complete" {
			t.Errorf("source budget=%+v elapsed=%v, want timeout after five seconds", r, time.Since(started))
		}
	})
}
func TestResponseDiscoveryHeaderLimit(t *testing.T) {
	home, _ := probeHome(t)
	d := responseFixtureDependencies(t, home)
	d.wire = responseTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Synthetic", strings.Repeat("x", 17<<10))
		_, _ = io.WriteString(w, `{}`)
	})
	r := runResponseDiscovery(t.Context(), true, d)
	if r.FailureCategory == nil || *r.FailureCategory != "response_limit" || r.DispatchCount != 1 || r.BodySummary != nil {
		t.Errorf("header limit=%+v, want bounded rejection once", r)
	}
}
