package bridge

import (
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
