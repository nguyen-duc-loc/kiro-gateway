package cli

import (
	"context"
	"io"
	"strings"
	"testing"

	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
)

func TestExperimentalStartupRequiresFrozenMappingWithoutSource(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing mapping", true: "linked mapping"}[linked], func(t *testing.T) {
			home := t.TempDir()
			store, err := configstore.Open(home, true)
			if err != nil {
				t.Fatal(err)
			}
			d := config.Default()
			d.Listen = "127.0.0.1:0"
			if linked {
				d.Session = &config.Session{Source: config.Source, Fingerprint: strings.Repeat("a", 64)}
				d.Models[bridge.Model] = bridge.Model
			}
			if err := store.Save(d, true); err != nil {
				t.Fatal(err)
			}
			store.Close()
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			err = run(ctx, []string{"serve", "--experimental-bridge"}, func(string) string { return strings.Repeat("x", 32) }, io.Discard, io.Discard, "test", func() (string, error) { return home, nil })
			if linked {
				if err != nil && !strings.Contains(err.Error(), "bind") {
					t.Errorf("serve(linked, missing Kiro store)=%v, want startup or canceled bind", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "exact claude-opus-5.5 mapping") {
				t.Errorf("serve(unlinked)=%v, want setup error", err)
			}
		})
	}
}
