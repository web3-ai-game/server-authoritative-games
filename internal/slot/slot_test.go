package slot

import (
	"math/rand"
	"testing"
)

// mkResult 用指定盘面构造 Result 并结算(grid[reel][row])
func mkResult(grid [Reels][Rows]int, totalBet int64) Result {
	r := Result{Grid: grid}
	evaluate(&r, totalBet, 1)
	return r
}

func flat(sym int) [Reels][Rows]int {
	var g [Reels][Rows]int
	for i := range g {
		for j := range g[i] {
			g[i][j] = sym
		}
	}
	return g
}

func TestFiveOfAKindMiddleLine(t *testing.T) {
	g := flat(SymJ)
	for reel := 0; reel < Reels; reel++ {
		g[reel][1] = Caishen // 中线 5 连財神
	}
	r := mkResult(g, 2000)
	lineBet := int64(2000 / NumLines)
	want := PayTable[Caishen][5] * lineBet // 1000 * 100
	found := false
	for _, w := range r.Wins {
		if w.Line == 0 {
			if w.Symbol != Caishen || w.Count != 5 || w.Win != want {
				t.Fatalf("line0 = %+v, want caishen x5 win %d", w, want)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("middle line win not found")
	}
}

func TestWildSubstitution(t *testing.T) {
	g := flat(SymJ)
	g[0][1], g[1][1], g[2][1] = Koi, Wild, Koi // 中线 koi wild koi j j → koi x3
	r := mkResult(g, 2000)
	for _, w := range r.Wins {
		if w.Line == 0 {
			if w.Symbol != Koi || w.Count != 3 {
				t.Fatalf("line0 = %+v, want koi x3", w)
			}
			return
		}
	}
	t.Fatal("wild substitution win not found")
}

func TestWildLineBeatsBase(t *testing.T) {
	g := flat(SymQ)
	g[0][1], g[1][1], g[2][1] = Wild, Wild, Wild // wild wild wild q q
	r := mkResult(g, 2000)
	lineBet := int64(100)
	// q x5 = 60, wild x3 = 100 → 应按 wild x3 计
	for _, w := range r.Wins {
		if w.Line == 0 {
			if w.Symbol != Wild || w.Count != 3 || w.Win != PayTable[Wild][3]*lineBet {
				t.Fatalf("line0 = %+v, want wild x3 win %d", w, PayTable[Wild][3]*lineBet)
			}
			return
		}
	}
	t.Fatal("wild line win not found")
}

func TestScatterPayAndFreeSpins(t *testing.T) {
	g := flat(SymJ)
	g[0][0], g[2][2], g[4][1] = Scatter, Scatter, Scatter
	r := mkResult(g, 1000)
	if r.ScatterCount != 3 || r.ScatterWin != ScatterPay[3]*1000 || r.FreeSpins != FreeSpinsWon {
		t.Fatalf("scatter: count=%d win=%d free=%d", r.ScatterCount, r.ScatterWin, r.FreeSpins)
	}
}

func TestScatterNotSubstitutedByWild(t *testing.T) {
	g := flat(SymJ)
	g[0][1], g[1][1] = Scatter, Scatter // 中线 scatter scatter j j j:j 只从 reel3 起不连左 → 无线奖
	r := mkResult(g, 1000)
	for _, w := range r.Wins {
		if w.Line == 0 {
			t.Fatalf("unexpected line0 win %+v", w)
		}
	}
	if r.ScatterCount != 2 || r.ScatterWin != 0 || r.FreeSpins != 0 {
		t.Fatalf("2 scatter should not pay: %+v", r)
	}
}

func TestValidBet(t *testing.T) {
	for _, b := range Bets {
		if !ValidBet(b) {
			t.Fatalf("bet %d should be valid", b)
		}
	}
	for _, b := range []int64{0, 50, 150, 99999, -100} {
		if ValidBet(b) {
			t.Fatalf("bet %d should be invalid", b)
		}
	}
}

func TestSpinGridMatchesStops(t *testing.T) {
	rng := func(n int) int { return rand.Intn(n) }
	for i := 0; i < 1000; i++ {
		r := Spin(2000, rng, 1)
		for reel := 0; reel < Reels; reel++ {
			strip := reelStrips[reel]
			for row := 0; row < Rows; row++ {
				if r.Grid[reel][row] != strip[(r.Stops[reel]+row)%len(strip)] {
					t.Fatal("grid does not match stops")
				}
			}
		}
	}
}

// TestRTP 100 万次仿真(含免费旋转 EV),要求 RTP 落在 90%~97%。
func TestRTP(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	src := rand.New(rand.NewSource(20260719))
	rng := func(n int) int { return src.Intn(n) }
	const spins = 1_000_000
	const bet = int64(2000)
	var wagered, won int64
	pendingFree := 0
	for i := 0; i < spins; i++ {
		wagered += bet
		r := Spin(bet, rng, 1)
		won += r.TotalWin
		pendingFree += r.FreeSpins
		// 免费旋转不扣注,奖励 ×FreeSpinMult,可重触发
		for pendingFree > 0 {
			pendingFree--
			fr := Spin(bet, rng, FreeSpinMult)
			won += fr.TotalWin
			pendingFree += fr.FreeSpins
		}
	}
	rtp := float64(won) / float64(wagered)
	t.Logf("RTP = %.4f (wagered %d, won %d)", rtp, wagered, won)
	if rtp < 0.90 || rtp > 0.97 {
		t.Fatalf("RTP %.4f out of range [0.90, 0.97]", rtp)
	}
}
