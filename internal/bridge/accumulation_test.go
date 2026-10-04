package bridge

import (
	"fmt"
	"strings"
	"testing"
)

var accumulationSink string

func BenchmarkFragmentedAccumulation(b *testing.B) {
	value := strings.Repeat("x", 65536)
	parts := make([]string, 1024)
	for i := range parts {
		parts[i] = value[i*64 : (i+1)*64]
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := NewResponse(Request{})
		for _, part := range parts {
			if err := s.Add(Event{Text: part}); err != nil {
				b.Fatal(err)
			}
		}
		if err := s.Complete(End{Basis: InferredCleanEOF}); err != nil {
			b.Fatal(err)
		}
		accumulationSink = s.Content[0].Text
	}
}

// covers: AC-11. Run serially, including growth and final materialization.
func TestFragmentedAccumulationAllocation(t *testing.T) {
	result := testing.Benchmark(BenchmarkFragmentedAccumulation)
	t.Logf("production accumulation: %d bytes/op", result.AllocedBytesPerOp())
	if got := result.AllocedBytesPerOp(); got > 1<<20 {
		t.Errorf("fragmented 64 KiB accumulation allocated %d bytes, want <= 1048576", got)
	}
	want := strings.Repeat("x", 65536)
	if accumulationSink != want {
		t.Error("fragmented accumulation differs from exact fixture")
	}
}

// covers: AC-11. The byte limit applies to the combined UTF-8 fragments.
// Rejected input must not change the already accepted text.
func TestResponseFragmentedTextByteLimit(t *testing.T) {
	const limit = 2 << 20
	for _, extra := range []string{"x", "界", ""} {
		t.Run(fmt.Sprintf("extra_bytes=%d", len(extra)), func(t *testing.T) {
			r := NewResponse(Request{Model: Model})
			part := strings.Repeat("界", 1024)
			want := strings.Repeat("界", limit/len("界")) + strings.Repeat("x", limit%len("界"))
			for offset := 0; offset < len(want); offset += len(part) {
				if err := r.Add(Event{Text: want[offset:min(offset+len(part), len(want))]}); err != nil {
					t.Fatalf("Add(fragment at %d) = %v, want nil", offset, err)
				}
			}
			if err := r.Add(Event{Text: extra}); err == nil {
				t.Errorf("Add(%q at text byte limit) = nil, want protocol failure", extra)
			}
			if err := r.Complete(End{Basis: InferredCleanEOF}); err != nil {
				t.Fatalf("Complete(accepted text) = %v, want nil", err)
			}
			if len(r.Content) != 1 || r.Content[0].Text != want {
				t.Error("Complete(accepted text) changed content, want exact accepted UTF-8 text")
			}
		})
	}
}
