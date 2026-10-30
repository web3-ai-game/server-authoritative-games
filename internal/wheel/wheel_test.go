package wheel

import (
	"math/rand"
	"testing"
)

func TestSegmentsSane(t *testing.T) {
	total := 0
	for _, s := range Segments {
		if s.Weight <= 0 || s.Mult <= 0 {
			t.Fatalf("bad segment %+v", s)
		}
		total += s.Weight
	}
	if total != 100 {
		t.Fatalf("weights sum = %d, want 100", total)
	}
	if len(Segments) != 8 {
		t.Fatalf("want 8 segments, got %d", len(Segments))
	}
}

// TestEV 100 万次抽取,期望值应在 1.5×~2.5× 基准额之间(福利力度可预估)
func TestEV(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	src := rand.New(rand.NewSource(20260719))
	rng := func(n int) int { return src.Intn(n) }
	const base = int64(1000)
	var sum int64
	const n = 1_000_000
	counts := make([]int, len(Segments))
	for i := 0; i < n; i++ {
		idx := Pick(rng)
		counts[idx]++
		sum += Value(idx, base)
	}
	ev := float64(sum) / float64(n)
	t.Logf("wheel EV = %.1f (base %d)", ev, base)
	if ev < 1500 || ev > 2500 {
		t.Fatalf("EV %.1f out of range", ev)
	}
	// 每格命中率应接近权重
	for i, s := range Segments {
		got := float64(counts[i]) / n * 100
		if got < float64(s.Weight)*0.8-0.5 || got > float64(s.Weight)*1.2+0.5 {
			t.Fatalf("segment %d rate %.2f%% far from weight %d%%", i, got, s.Weight)
		}
	}
}

func TestValue(t *testing.T) {
	if Value(0, 1000) != 500 || Value(7, 1000) != 20000 {
		t.Fatalf("value calc broken: %d %d", Value(0, 1000), Value(7, 1000))
	}
	if Value(1, 333) != 990 { // 3×333=999 → 990(取整到 10)
		t.Fatalf("rounding broken: %d", Value(1, 333))
	}
}
