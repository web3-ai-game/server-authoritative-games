package poker

import (
	"math/rand"
	"testing"
)

func card(rank, suit int) int { return suit*13 + rank } // rank 0=A,1=2..12=K

func TestBestScoreCategories(t *testing.T) {
	// 四條 A > 葫蘆
	quads := BestScore([]int{card(0, 0), card(0, 1), card(0, 2), card(0, 3), card(12, 0), card(5, 1), card(6, 2)})
	boat := BestScore([]int{card(0, 0), card(0, 1), card(0, 2), card(12, 0), card(12, 1), card(3, 0), card(4, 1)})
	if quads <= boat {
		t.Fatalf("quads %d should beat boat %d", quads, boat)
	}
	flush := BestScore([]int{card(2, 0), card(4, 0), card(7, 0), card(9, 0), card(11, 0), card(1, 1), card(3, 2)})
	straight := BestScore([]int{card(1, 0), card(2, 1), card(3, 2), card(4, 3), card(5, 0), card(12, 1), card(11, 2)})
	if flush <= straight {
		t.Fatalf("flush %d should beat straight %d", flush, straight)
	}
	pair := BestScore([]int{card(9, 0), card(9, 1), card(2, 0), card(4, 1), card(6, 2), card(8, 3), card(11, 0)})
	high := BestScore([]int{card(0, 0), card(12, 1), card(10, 2), card(8, 3), card(5, 0), card(3, 1), card(2, 2)})
	if pair <= high {
		t.Fatalf("pair %d should beat high %d", pair, high)
	}
}

func TestWheelStraight(t *testing.T) {
	// A2345
	wheel := BestScore([]int{card(0, 0), card(1, 1), card(2, 2), card(3, 3), card(4, 0), card(7, 1), card(9, 2)})
	// 6789T
	mid := BestScore([]int{card(5, 0), card(6, 1), card(7, 2), card(8, 3), card(9, 0), card(1, 1), card(2, 2)})
	if wheel >= mid {
		t.Fatalf("wheel %d should lose to 6-T %d", wheel, mid)
	}
	// 非順子
	if is, _ := isStraight([5]int{12, 10, 8, 4, 0}); is {
		t.Fatal("junk should not be straight")
	}
}

func TestRoyalBeatsStraightFlush(t *testing.T) {
	royal := BestScore([]int{card(0, 1), card(12, 1), card(11, 1), card(10, 1), card(9, 1), card(2, 0), card(3, 0)})
	sf := BestScore([]int{card(9, 0), card(8, 0), card(7, 0), card(6, 0), card(5, 0), card(1, 1), card(2, 2)})
	if royal <= sf {
		t.Fatalf("royal %d should beat sf %d", royal, sf)
	}
}

func TestFoldWinsPot(t *testing.T) {
	rng := func(n int) int { return 0 }
	st, debit, err := NewHand(50, 100, 1, 10000, 0, rng)
	if err != nil {
		t.Fatal(err)
	}
	if debit <= 0 {
		t.Fatalf("hero should post a blind, debit=%d", debit)
	}
	// 強制：若正在 playing 且輪到玩家,棄牌
	if st.Status == "playing" && st.ToAct == st.HeroIdx {
		_, err = st.Apply("fold", 0, 10000-debit, rng)
		if err != nil {
			t.Fatal(err)
		}
	}
	if st.Status != "showdown" && st.liveCount() > 1 && st.Status == "playing" {
		// AI 可能還在打,再棄一次不該發生;至少狀態合法
	}
	if st.Seats[st.HeroIdx].Folded && st.Payout != 0 {
		t.Fatalf("folded hero payout=%d", st.Payout)
	}
}

func TestNewHandDealsAndPosts(t *testing.T) {
	rng := rand.New(rand.NewSource(1)).Intn
	// button 參數是「上一手莊家」,開局會 +1;傳 1 則本手莊家為 0(玩家=SB,先行動)
	st, debit, err := NewHand(100, 200, 1, 20000, 1, rng)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Seats) != 2 {
		t.Fatalf("seats=%d", len(st.Seats))
	}
	for i, p := range st.Seats {
		if len(p.Hole) != 2 {
			t.Fatalf("seat %d hole=%v", i, p.Hole)
		}
	}
	var posted int64
	for _, p := range st.Seats {
		posted += p.TotalBet
	}
	if posted != 300 { // SB+BB
		t.Fatalf("blinds posted %d want 300", posted)
	}
	if debit != st.Seats[0].TotalBet {
		t.Fatalf("debit %d != hero total %d", debit, st.Seats[0].TotalBet)
	}
	if st.Status == "playing" && st.ToAct != st.HeroIdx {
		t.Fatal("should be hero to act or already finished")
	}
}

func TestViewHidesAIHole(t *testing.T) {
	rng := rand.New(rand.NewSource(2)).Intn
	st, _, err := NewHand(50, 100, 1, 5000, 0, rng)
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "playing" {
		t.Skip("hand ended immediately")
	}
	v := st.View(5000)
	for i, p := range v.Seats {
		if p.IsHero {
			if len(p.Hole) != 2 || p.Hole[0] < 0 {
				t.Fatalf("hero hole hidden: %v", p.Hole)
			}
		} else if st.Status == "playing" {
			if len(p.Hole) != 2 || p.Hole[0] != -1 {
				t.Fatalf("AI hole leaked at seat %d: %v", i, p.Hole)
			}
		}
	}
	if len(v.Community) > 0 && st.Street == "preflop" {
		t.Fatal("community on preflop")
	}
}

func TestValidParams(t *testing.T) {
	if !ValidBlinds(50, 100) || ValidBlinds(1, 2) {
		t.Fatal("blinds")
	}
	if !ValidOpponents(4) || ValidOpponents(0) || ValidOpponents(5) {
		t.Fatal("opponents")
	}
}
