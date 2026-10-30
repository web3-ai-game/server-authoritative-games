// Package bj 二十一点引擎:6 副牌、庄家软 17 停牌(S17)、发牌即检查 Blackjack(peek)、
// 支持要牌/停牌/加倍(首两张),Blackjack 赔率由 settings(admin)控制,默认 3:2。
// 纯逻辑零 IO,状态由调用方持久化(JSON)。
package bj

import (
	"crypto/rand"
	"encoding/binary"
)

// Bets 允许的投注档位(与老虎机一致)
var Bets = []int64{100, 200, 500, 1000, 2000}

func ValidBet(bet int64) bool {
	for _, b := range Bets {
		if b == bet {
			return true
		}
	}
	return false
}

// 牌用 0..51 表示:rank = c%13(0=A,1=2..8=9,9=10,10=J,11=Q,12=K),suit = c/13
const NumDecks = 6

// State 一局的完整状态(JSON 持久化到 bj_state.state)
type State struct {
	Bet     int64 `json:"bet"`     // 基础注(加倍后总注 = Bet*2)
	Deck    []int `json:"deck"`    // 剩余牌堆(顶部在末尾)
	Player  []int `json:"player"`
	Dealer  []int `json:"dealer"`  // [0] 明牌,[1] 底牌
	Doubled bool  `json:"doubled"`
}

// RNG 返回 [0,n) 均匀随机整数
type RNG func(n int) int

// CryptoRNG 生产用 crypto/rand
func CryptoRNG(n int) int {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return int(binary.LittleEndian.Uint64(b[:]) % uint64(n))
}

// cardValue 单张点数(A 记 11,由 HandValue 降级)
func cardValue(c int) int {
	r := c % 13
	switch {
	case r == 0:
		return 11
	case r >= 9:
		return 10
	default:
		return r + 1
	}
}

// HandValue 手牌点数;soft 表示存在按 11 计的 A
func HandValue(cards []int) (total int, soft bool) {
	aces := 0
	for _, c := range cards {
		v := cardValue(c)
		if v == 11 {
			aces++
		}
		total += v
	}
	for total > 21 && aces > 0 {
		total -= 10
		aces--
	}
	return total, aces > 0
}

// IsBlackjack 两张组成 21
func IsBlackjack(cards []int) bool {
	if len(cards) != 2 {
		return false
	}
	t, _ := HandValue(cards)
	return t == 21
}

// NewRound 洗 6 副牌并各发两张
func NewRound(bet int64, rng RNG) *State {
	deck := make([]int, 52*NumDecks)
	for i := range deck {
		deck[i] = i % 52
	}
	for i := len(deck) - 1; i > 0; i-- { // Fisher-Yates
		j := rng(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
	s := &State{Bet: bet, Deck: deck}
	s.Player = append(s.Player, s.draw(), s.draw())
	s.Dealer = append(s.Dealer, s.draw(), s.draw())
	return s
}

func (s *State) draw() int {
	c := s.Deck[len(s.Deck)-1]
	s.Deck = s.Deck[:len(s.Deck)-1]
	return c
}

// Hit 玩家要一张,返回该牌
func (s *State) Hit() int {
	c := s.draw()
	s.Player = append(s.Player, c)
	return c
}

// DealerPlay 庄家补牌至 17+(软 17 停牌)
func (s *State) DealerPlay() {
	for {
		t, _ := HandValue(s.Dealer)
		if t >= 17 {
			return
		}
		s.Dealer = append(s.Dealer, s.draw())
	}
}

// Stake 当前总投入
func (s *State) Stake() int64 {
	if s.Doubled {
		return s.Bet * 2
	}
	return s.Bet
}

// Settle 结算:返回应入账金额(含本金)与结果。
// 调用前提:玩家爆牌,或庄家已 DealerPlay,或发牌即 BJ(peek)。
// bjPayout 为 Blackjack 净赔率(1.5 = 3:2,1.2 = 6:5)。
func (s *State) Settle(bjPayout float64) (payout int64, result string) {
	stake := s.Stake()
	pt, _ := HandValue(s.Player)
	dt, _ := HandValue(s.Dealer)
	pbj, dbj := IsBlackjack(s.Player), IsBlackjack(s.Dealer)
	switch {
	case pt > 21:
		return 0, "lose"
	case pbj && dbj:
		return stake, "push"
	case pbj:
		return stake + int64(float64(stake)*bjPayout), "bj"
	case dbj:
		return 0, "lose"
	case dt > 21:
		return stake * 2, "win"
	case pt > dt:
		return stake * 2, "win"
	case pt == dt:
		return stake, "push"
	default:
		return 0, "lose"
	}
}
