package kiro

import (
	"strings"
	"testing"
)

var accumulationSink string

func BenchmarkFragmentedAccumulation(b *testing.B) {
	value := `{"value":"` + strings.Repeat("x", 65524) + `"}`
	parts := make([]string, 1024)
	for i := range parts {
		parts[i] = value[i*64 : (i+1)*64]
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := &pendingTool{}
		for _, part := range parts {
			if err := s.appendInput(part); err != nil {
				b.Fatal(err)
			}
		}
		accumulationSink = s.inputString()
	}
}

// covers: AC-11. Run serially, including growth and final materialization.
func TestFragmentedAccumulationAllocation(t *testing.T) {
	result := testing.Benchmark(BenchmarkFragmentedAccumulation)
	t.Logf("production accumulation: %d bytes/op", result.AllocedBytesPerOp())
	if got := result.AllocedBytesPerOp(); got > 1<<20 {
		t.Errorf("fragmented 64 KiB accumulation allocated %d bytes, want <= 1048576", got)
	}
	want := `{"value":"` + strings.Repeat("x", 65524) + `"}`
	if accumulationSink != want {
		t.Error("fragmented accumulation differs from exact fixture")
	}
}
