package fish

import (
	"math/rand"
	"testing"
)

func TestTableSane(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Table {
		if f.Value < 2 || f.ID == "" || seen[f.ID] {
			t.Fatalf("bad fish %+v", f)
		}
		seen[f.ID] = true
		if !f.Boss && f.Weight <= 0 {
			t.Fatalf("non-boss fish needs weight: %+v", f)
		}
	}
	if ByID("fish_goldshark") == nil || ByID("nope") != nil {
		t.Fatal("ByID broken")
	}
	if !ValidMult(5) || ValidMult(3) {
		t.Fatal("ValidMult broken")
	}
}

// TestResolveEV 每种鱼 100 万次命中,EV/弹价 应≈RTP(±2%)
func TestResolveEV(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	src := rand.New(rand.NewSource(20260719))
	rng := func() float64 { return src.Float64() }
	const rtp = 0.93
	for _, f := range Table {
		f := f
		var cost, reward int64
		for i := 0; i < 1_000_000; i++ {
			c, r := Resolve(&f, 2, rtp, rng)
			cost += c
			reward += r
		}
		got := float64(reward) / float64(cost)
		if got < rtp-0.02 || got > rtp+0.02 {
			t.Fatalf("%s EV = %.4f, want ≈%.2f", f.ID, got, rtp)
		}
	}
}

func TestResolveCost(t *testing.T) {
	f := ByID("fish_clown")
	rng := func() float64 { return 0.999 } // 必不中
	cost, reward := Resolve(f, 10, 0.93, rng)
	if cost != 100 || reward != 0 {
		t.Fatalf("cost=%d reward=%d", cost, reward)
	}
	rng = func() float64 { return 0.0 } // 必中
	cost, reward = Resolve(f, 10, 0.93, rng)
	if cost != 100 || reward != 200 {
		t.Fatalf("kill: cost=%d reward=%d", cost, reward)
	}
}
