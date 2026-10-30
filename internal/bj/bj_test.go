package bj

import (
	"math/rand"
	"testing"
)

// 手工构造:rank→card(rank r, suit 0) r: 0=A..12=K
func card(r int) int { return r }

func TestHandValue(t *testing.T) {
	cases := []struct {
		cards []int
		total int
		soft  bool
	}{
		{[]int{card(0), card(9)}, 21, true},          // A + 10 = BJ
		{[]int{card(0), card(0)}, 12, true},          // A + A = 12 软
		{[]int{card(0), card(4), card(9)}, 16, false}, // A+5+10 = 16 硬
		{[]int{card(9), card(11), card(12)}, 30, false},
		{[]int{card(0), card(6)}, 18, true},          // A+7 软 18
		{[]int{card(4), card(5)}, 11, false},
	}
	for _, c := range cases {
		total, soft := HandValue(c.cards)
		if total != c.total || soft != c.soft {
			t.Fatalf("HandValue(%v) = %d/%v, want %d/%v", c.cards, total, soft, c.total, c.soft)
		}
	}
	if !IsBlackjack([]int{card(0), card(10)}) || IsBlackjack([]int{card(0), card(4), card(4)}) {
		t.Fatal("IsBlackjack broken")
	}
}

func TestDealerPlaySoft17(t *testing.T) {
	// 庄家 A+6(软 17)应停牌
	s := &State{Deck: []int{card(9), card(9)}, Dealer: []int{card(0), card(5)}}
	s.DealerPlay()
	if len(s.Dealer) != 2 {
		t.Fatalf("dealer should stand on soft 17, drew to %v", s.Dealer)
	}
	// 庄家 16 必须补
	s = &State{Deck: []int{card(1)}, Dealer: []int{card(9), card(5)}}
	s.DealerPlay()
	if len(s.Dealer) != 3 {
		t.Fatal("dealer must hit 16")
	}
}

func TestSettle(t *testing.T) {
	mk := func(p, d []int, doubled bool) *State { return &State{Bet: 100, Player: p, Dealer: d, Doubled: doubled} }
	cases := []struct {
		s      *State
		payout int64
		result string
	}{
		{mk([]int{card(9), card(9)}, []int{card(9), card(8)}, false), 200, "win"},   // 20 vs 19
		{mk([]int{card(9), card(8)}, []int{card(9), card(9)}, false), 0, "lose"},    // 19 vs 20
		{mk([]int{card(9), card(9)}, []int{card(9), card(9)}, false), 100, "push"},  // 20 vs 20
		{mk([]int{card(0), card(9)}, []int{card(9), card(8)}, false), 250, "bj"},    // BJ 3:2
		{mk([]int{card(0), card(9)}, []int{card(0), card(9)}, false), 100, "push"},  // 双 BJ
		{mk([]int{card(9), card(9)}, []int{card(0), card(9)}, false), 0, "lose"},    // 庄 BJ
		{mk([]int{card(9), card(6), card(9)}, []int{card(9), card(8)}, false), 0, "lose"}, // 爆牌
		{mk([]int{card(9), card(9)}, []int{card(9), card(5), card(9)}, false), 200, "win"}, // 庄爆
		{mk([]int{card(9), card(9)}, []int{card(9), card(8)}, true), 400, "win"},    // 加倍赢
	}
	for i, c := range cases {
		p, r := c.s.Settle(1.5)
		if p != c.payout || r != c.result {
			t.Fatalf("case %d: got %d/%s, want %d/%s", i, p, r, c.payout, c.result)
		}
	}
	// 6:5 赔率
	p, _ := mk([]int{card(0), card(9)}, []int{card(9), card(8)}, false).Settle(1.2)
	if p != 220 {
		t.Fatalf("6:5 BJ payout = %d, want 220", p)
	}
}

// TestRTPSim 10 万局「补到 17」笨策略仿真,RTP 应落在 0.88~1.00(基本策略约 0.995,笨策略更低)
func TestRTPSim(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	src := rand.New(rand.NewSource(20260719))
	rng := func(n int) int { return src.Intn(n) }
	const rounds = 100_000
	var wagered, won int64
	for i := 0; i < rounds; i++ {
		s := NewRound(100, rng)
		wagered += 100
		if IsBlackjack(s.Player) || IsBlackjack(s.Dealer) {
			p, _ := s.Settle(1.5)
			won += p
			continue
		}
		for {
			pt, _ := HandValue(s.Player)
			if pt >= 17 {
				break
			}
			s.Hit()
		}
		if pt, _ := HandValue(s.Player); pt <= 21 {
			s.DealerPlay()
		}
		p, _ := s.Settle(1.5)
		won += p
	}
	rtp := float64(won) / float64(wagered)
	t.Logf("BJ RTP (hit-to-17) = %.4f", rtp)
	if rtp < 0.88 || rtp > 1.0 {
		t.Fatalf("RTP %.4f out of range", rtp)
	}
}

func TestNewRoundDeck(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	s := NewRound(100, func(n int) int { return src.Intn(n) })
	if len(s.Deck) != 52*NumDecks-4 || len(s.Player) != 2 || len(s.Dealer) != 2 {
		t.Fatalf("bad round: deck=%d p=%d d=%d", len(s.Deck), len(s.Player), len(s.Dealer))
	}
	c := s.Hit()
	if c < 0 || c > 51 || len(s.Player) != 3 {
		t.Fatal("hit broken")
	}
}
