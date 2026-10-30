// Package poker 德州撲克引擎:一副牌、玩家 vs 1–4 AI、翻牌圈結構、
// 棄牌/過牌/跟注/加注、攤牌比牌。純邏輯零 IO,狀態由調用方 JSON 持久化。
// 客戶端只拿到 PublicView,牌堆與 AI 底牌在攤牌前不下發。
package poker

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sort"
)

var (
	ErrInvalidAction = errors.New("invalid action")
	ErrNotYourTurn   = errors.New("not your turn")
	ErrNoHand        = errors.New("no hand")
	ErrBusy          = errors.New("hand in progress")
	ErrBadParams     = errors.New("invalid params")
)

// Blinds 允許的盲注檔位 [SB, BB]
var Blinds = [][2]int64{{50, 100}, {100, 200}, {250, 500}, {500, 1000}}

func ValidBlinds(sb, bb int64) bool {
	for _, b := range Blinds {
		if b[0] == sb && b[1] == bb {
			return true
		}
	}
	return false
}

func ValidOpponents(n int) bool { return n >= 1 && n <= 4 }

// AINames 座位展示名(繁體)
var AINames = []string{"金龍", "夜鯊", "玄鋒", "赤焰"}

// RNG 返回 [0,n) 均勻隨機
type RNG func(n int) int

func CryptoRNG(n int) int {
	if n <= 0 {
		return 0
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return int(binary.LittleEndian.Uint64(b[:]) % uint64(n))
}

// Seat 一席
type Seat struct {
	Name     string `json:"name"`
	IsAI     bool   `json:"isAI"`
	Bet      int64  `json:"bet"`      // 本街已投入
	TotalBet int64  `json:"totalBet"` // 本手總投入
	Folded   bool   `json:"folded"`
	AllIn    bool   `json:"allIn"`
	Hole     []int  `json:"hole"`
	Acted    bool   `json:"acted"`
}

// State 一局完整狀態(含牌堆,禁止直接下發客戶端)
type State struct {
	SB          int64  `json:"sb"`
	BBVal       int64  `json:"bb"`
	Street      string `json:"street"` // preflop/flop/turn/river/showdown
	Community   []int  `json:"community"`
	Deck        []int  `json:"deck"`
	Seats       []Seat `json:"seats"`
	HeroIdx     int    `json:"heroIdx"`
	Button      int    `json:"button"`
	ToAct       int    `json:"toAct"`
	CurrentBet  int64  `json:"currentBet"`
	LastRaise   int64  `json:"lastRaise"`
	Status      string `json:"status"` // playing / showdown / idle
	Result      string `json:"result"` // win / lose / split / foldwin
	Payout      int64  `json:"payout"`
	Wagered     int64  `json:"wagered"`
	Winners     []int  `json:"winners"`
	HandsPlayed int    `json:"handsPlayed"`
	NumAI       int    `json:"numAI"`
	Settled     bool   `json:"settled"`
}

// NewHand 開新一局。heroStack 為玩家當前餘額(跟注/加注上限)。
// button 為上一手莊家,本手前進一席;首次傳 -1。
func NewHand(sb, bb int64, nAI int, heroStack int64, button int, rng RNG) (*State, int64, error) {
	if !ValidBlinds(sb, bb) || !ValidOpponents(nAI) {
		return nil, 0, ErrBadParams
	}
	n := nAI + 1
	if heroStack < bb {
		return nil, 0, ErrBadParams
	}
	deck := make([]int, 52)
	for i := range deck {
		deck[i] = i
	}
	for i := len(deck) - 1; i > 0; i-- {
		j := rng(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
	seats := make([]Seat, n)
	seats[0] = Seat{Name: "你", IsAI: false, Hole: []int{}}
	for i := 1; i < n; i++ {
		seats[i] = Seat{Name: AINames[(i-1)%len(AINames)], IsAI: true, Hole: []int{}}
	}
	btn := 0
	if button >= 0 {
		btn = (button + 1) % n
	} else {
		btn = rng(n)
	}
	s := &State{
		SB: sb, BBVal: bb, Street: "preflop", Community: []int{}, Deck: deck,
		Seats: seats, HeroIdx: 0, Button: btn, Status: "playing",
		LastRaise: bb, CurrentBet: bb, NumAI: nAI,
	}
	// 發底牌
	for r := 0; r < 2; r++ {
		for i := 0; i < n; i++ {
			idx := (btn + 1 + i) % n
			s.Seats[idx].Hole = append(s.Seats[idx].Hole, s.draw())
		}
	}
	sbIdx, bbIdx := s.blindSeats()
	s.post(sbIdx, sb, heroStack)
	s.post(bbIdx, bb, heroStack)
	s.ToAct = s.nextLive(bbIdx)
	// 若第一個行動是 AI,立刻跑完直到輪到玩家或結束
	s.autoPlay(rng, heroStack)
	return s, s.heroDeltaPosted(), nil
}

func (s *State) draw() int {
	c := s.Deck[len(s.Deck)-1]
	s.Deck = s.Deck[:len(s.Deck)-1]
	return c
}

func (s *State) blindSeats() (sbIdx, bbIdx int) {
	n := len(s.Seats)
	if n == 2 {
		return s.Button, (s.Button + 1) % n
	}
	return (s.Button + 1) % n, (s.Button + 2) % n
}

func (s *State) post(i int, amt, heroStack int64) {
	p := &s.Seats[i]
	if !p.IsAI {
		if amt > heroStack {
			amt = heroStack
			p.AllIn = true
		}
	}
	p.Bet += amt
	p.TotalBet += amt
}

func (s *State) heroDeltaPosted() int64 { return s.Seats[s.HeroIdx].TotalBet }

func (s *State) liveCount() int {
	n := 0
	for i := range s.Seats {
		if !s.Seats[i].Folded {
			n++
		}
	}
	return n
}

func (s *State) nextLive(from int) int {
	n := len(s.Seats)
	for k := 1; k <= n; k++ {
		i := (from + k) % n
		if !s.Seats[i].Folded && !s.Seats[i].AllIn {
			return i
		}
	}
	return from
}

func (s *State) bettingDone() bool {
	if s.liveCount() <= 1 {
		return true
	}
	need := 0
	for i := range s.Seats {
		p := &s.Seats[i]
		if p.Folded || p.AllIn {
			continue
		}
		need++
		if !p.Acted || p.Bet != s.CurrentBet {
			return false
		}
	}
	return need >= 1
}

func (s *State) legal(i int, heroStack int64) []string {
	p := &s.Seats[i]
	if s.Status != "playing" || p.Folded || p.AllIn {
		return nil
	}
	toCall := s.CurrentBet - p.Bet
	remain := heroStack
	if p.IsAI {
		remain = 1 << 40
	}
	out := []string{"fold"}
	if toCall <= 0 {
		out = []string{"check"}
	} else if remain > 0 {
		out = append(out, "call")
	}
	minTo := s.CurrentBet + s.LastRaise
	if minTo < s.CurrentBet+s.BBVal {
		minTo = s.CurrentBet + s.BBVal
	}
	if remain > toCall && (remain+p.Bet) >= minTo {
		out = append(out, "raise")
	} else if remain > toCall && toCall > 0 {
		// 籌碼不夠加注但可 all-in 超過跟注,仍允許 raise(短 all-in)
		out = append(out, "raise")
	}
	return out
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}

// Apply 玩家行動。raiseTo 為本街總投入目標。回傳本行動需再扣的金幣(僅 hero)。
func (s *State) Apply(act string, raiseTo int64, heroStack int64, rng RNG) (debit int64, err error) {
	if s == nil || s.Status != "playing" {
		return 0, ErrNoHand
	}
	if s.ToAct != s.HeroIdx {
		return 0, ErrNotYourTurn
	}
	before := s.Seats[s.HeroIdx].TotalBet
	if err = s.applyOne(s.HeroIdx, act, raiseTo, heroStack); err != nil {
		return 0, err
	}
	debit = s.Seats[s.HeroIdx].TotalBet - before
	s.autoPlay(rng, heroStack-debit)
	return debit, nil
}

func (s *State) applyOne(i int, act string, raiseTo, heroStack int64) error {
	p := &s.Seats[i]
	legal := s.legal(i, heroStack)
	if !contains(legal, act) {
		return ErrInvalidAction
	}
	remain := heroStack
	if p.IsAI {
		remain = 50_000_000
	}
	switch act {
	case "fold":
		p.Folded = true
		p.Acted = true
	case "check":
		p.Acted = true
	case "call":
		need := s.CurrentBet - p.Bet
		if need > remain {
			need = remain
			p.AllIn = true
		}
		p.Bet += need
		p.TotalBet += need
		p.Acted = true
		if remain-need <= 0 {
			p.AllIn = true
		}
	case "raise":
		minTo := s.CurrentBet + s.LastRaise
		if minTo < s.CurrentBet+s.BBVal {
			minTo = s.CurrentBet + s.BBVal
		}
		target := raiseTo
		maxTo := p.Bet + remain
		if target <= 0 {
			target = minTo
		}
		if target < minTo {
			target = minTo
		}
		if target > maxTo {
			target = maxTo
			p.AllIn = true
		}
		add := target - p.Bet
		if add <= 0 {
			return ErrInvalidAction
		}
		inc := target - s.CurrentBet
		if inc > 0 {
			if inc >= s.LastRaise {
				s.LastRaise = inc
				for j := range s.Seats {
					if j != i {
						s.Seats[j].Acted = false
					}
				}
			}
			s.CurrentBet = target
		}
		p.Bet += add
		p.TotalBet += add
		p.Acted = true
		if p.Bet+0 >= p.Bet+remain-add && remain-add <= 0 {
			p.AllIn = true
		}
	default:
		return ErrInvalidAction
	}
	s.ToAct = s.nextLive(i)
	return nil
}

func (s *State) autoPlay(rng RNG, heroRemain int64) {
	guard := 0
	for s.Status == "playing" && guard < 80 {
		guard++
		if s.liveCount() <= 1 {
			s.finishUncontested()
			return
		}
		if s.bettingDone() {
			s.nextStreet()
			continue
		}
		i := s.ToAct
		if s.Seats[i].Folded || s.Seats[i].AllIn {
			s.ToAct = s.nextLive(i)
			continue
		}
		if i == s.HeroIdx {
			return
		}
		act, amt := s.aiDecide(i, rng)
		_ = s.applyOne(i, act, amt, 0)
	}
}

func (s *State) nextStreet() {
	// 重置本街
	for i := range s.Seats {
		s.Seats[i].Bet = 0
		s.Seats[i].Acted = false
	}
	s.CurrentBet = 0
	s.LastRaise = s.BBVal
	switch s.Street {
	case "preflop":
		s.draw() // burn
		s.Community = append(s.Community, s.draw(), s.draw(), s.draw())
		s.Street = "flop"
	case "flop":
		s.draw()
		s.Community = append(s.Community, s.draw())
		s.Street = "turn"
	case "turn":
		s.draw()
		s.Community = append(s.Community, s.draw())
		s.Street = "river"
	default:
		s.showdown()
		return
	}
	// 若所有未棄牌者都 all-in,直接發完
	canBet := 0
	for i := range s.Seats {
		if !s.Seats[i].Folded && !s.Seats[i].AllIn {
			canBet++
		}
	}
	if canBet <= 1 {
		s.runout()
		s.showdown()
		return
	}
	s.ToAct = s.nextLive(s.Button)
}

func (s *State) runout() {
	for len(s.Community) < 5 && len(s.Deck) > 0 {
		s.draw()
		s.Community = append(s.Community, s.draw())
	}
	s.Street = "river"
}

func (s *State) finishUncontested() {
	win := -1
	for i := range s.Seats {
		if !s.Seats[i].Folded {
			win = i
			break
		}
	}
	s.Street = "showdown"
	s.Status = "showdown"
	s.Winners = []int{win}
	s.settlePayout()
	if win == s.HeroIdx {
		s.Result = "foldwin"
	} else {
		s.Result = "lose"
	}
}

func (s *State) showdown() {
	s.Street = "showdown"
	s.Status = "showdown"
	type scored struct {
		i     int
		score int64
	}
	var live []scored
	for i := range s.Seats {
		if s.Seats[i].Folded {
			continue
		}
		cards := append(append([]int{}, s.Seats[i].Hole...), s.Community...)
		live = append(live, scored{i: i, score: BestScore(cards)})
	}
	if len(live) == 0 {
		s.Result = "lose"
		s.settlePayout()
		return
	}
	best := live[0].score
	for _, x := range live {
		if x.score > best {
			best = x.score
		}
	}
	var winners []int
	for _, x := range live {
		if x.score == best {
			winners = append(winners, x.i)
		}
	}
	s.Winners = winners
	s.settlePayout()
	heroWin := false
	for _, w := range winners {
		if w == s.HeroIdx {
			heroWin = true
			break
		}
	}
	switch {
	case !heroWin || s.Seats[s.HeroIdx].Folded:
		s.Result = "lose"
	case len(winners) > 1:
		s.Result = "split"
	default:
		s.Result = "win"
	}
}

func (s *State) settlePayout() {
	hero := &s.Seats[s.HeroIdx]
	s.Wagered = hero.TotalBet
	if hero.Folded {
		s.Payout = 0
		return
	}
	isWin := false
	nWin := len(s.Winners)
	for _, w := range s.Winners {
		if w == s.HeroIdx {
			isWin = true
			break
		}
	}
	if !isWin || nWin == 0 {
		s.Payout = 0
		return
	}
	// 主池:每人投入與 hero 投入取 min 後加總,再均分給贏家
	var matched int64
	for i := range s.Seats {
		m := s.Seats[i].TotalBet
		if m > hero.TotalBet {
			m = hero.TotalBet
		}
		matched += m
	}
	s.Payout = matched / int64(nWin)
}

func (s *State) pot() int64 {
	var p int64
	for i := range s.Seats {
		p += s.Seats[i].TotalBet
	}
	return p
}

func (s *State) aiDecide(i int, rng RNG) (act string, raiseTo int64) {
	p := &s.Seats[i]
	legal := s.legal(i, 0)
	score := holeStrength(p.Hole)
	if len(s.Community) > 0 {
		cards := append(append([]int{}, p.Hole...), s.Community...)
		sc := BestScore(cards)
		cat := sc >> 20
		score = 20 + int(cat)*12
	}
	toCall := s.CurrentBet - p.Bet
	roll := rng(100)
	aggressive := score >= 36 || (score >= 24 && roll < 35)
	weak := score < 16

	if contains(legal, "check") {
		if aggressive && contains(legal, "raise") && roll < 55 {
			return "raise", s.CurrentBet + s.LastRaise*int64(1+rng(2))
		}
		return "check", 0
	}
	if weak && toCall > s.BBVal && roll < 70 {
		if contains(legal, "fold") {
			return "fold", 0
		}
	}
	if aggressive && contains(legal, "raise") && roll < 40 {
		return "raise", s.CurrentBet + s.LastRaise*int64(1+rng(3))
	}
	if contains(legal, "call") {
		return "call", 0
	}
	if contains(legal, "check") {
		return "check", 0
	}
	return "fold", 0
}

func holeStrength(h []int) int {
	if len(h) < 2 {
		return 0
	}
	r0, r1 := pokerRank(h[0]), pokerRank(h[1])
	if r0 < r1 {
		r0, r1 = r1, r0
	}
	suited := h[0]/13 == h[1]/13
	s := r0*2 + r1
	if r0 == r1 {
		s += 18
	}
	if suited {
		s += 4
	}
	if r0-r1 == 1 {
		s += 3
	}
	return s
}

// ---------- 牌力 ----------

func pokerRank(c int) int {
	r := c % 13 // 0=A,1=2..12=K
	if r == 0 {
		return 12 // Ace high
	}
	return r - 1
}

func pokerSuit(c int) int { return c / 13 }

// BestScore 7 張(或更少)中最強 5 張。高 4 bit 為牌型,其後為踢腳。
func BestScore(cards []int) int64 {
	n := len(cards)
	if n < 5 {
		return eval5pad(cards)
	}
	if n == 5 {
		return eval5(cards[0], cards[1], cards[2], cards[3], cards[4])
	}
	best := int64(0)
	var idx [7]int
	for i := 0; i < n; i++ {
		idx[i] = cards[i]
	}
	// C(n,5)
	for a := 0; a < n-4; a++ {
		for b := a + 1; b < n-3; b++ {
			for c := b + 1; c < n-2; c++ {
				for d := c + 1; d < n-1; d++ {
					for e := d + 1; e < n; e++ {
						sc := eval5(idx[a], idx[b], idx[c], idx[d], idx[e])
						if sc > best {
							best = sc
						}
					}
				}
			}
		}
	}
	return best
}

func eval5pad(cards []int) int64 {
	tmp := []int{0, 13, 26, 39, 1} // filler, never used in real hands
	copy(tmp, cards)
	return eval5(tmp[0], tmp[1], tmp[2], tmp[3], tmp[4])
}

func eval5(a, b, c, d, e int) int64 {
	cs := [5]int{a, b, c, d, e}
	ranks := [5]int{pokerRank(a), pokerRank(b), pokerRank(c), pokerRank(d), pokerRank(e)}
	suits := [5]int{pokerSuit(a), pokerSuit(b), pokerSuit(c), pokerSuit(d), pokerSuit(e)}
	sort.Sort(sort.Reverse(sort.IntSlice(ranks[:])))
	flush := suits[0] == suits[1] && suits[1] == suits[2] && suits[2] == suits[3] && suits[3] == suits[4]
	straight, sHigh := isStraight(ranks)

	cnt := map[int]int{}
	for _, r := range ranks {
		cnt[r]++
	}
	type rc struct{ r, c int }
	var groups []rc
	for r, c := range cnt {
		groups = append(groups, rc{r, c})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].c != groups[j].c {
			return groups[i].c > groups[j].c
		}
		return groups[i].r > groups[j].r
	})

	pack := func(cat int64, rs ...int) int64 {
		v := cat << 20
		sh := 16
		for _, r := range rs {
			v |= int64(r) << sh
			sh -= 4
		}
		return v
	}

	if flush && straight {
		return pack(8, sHigh)
	}
	if groups[0].c == 4 {
		k := 0
		if groups[1].r != groups[0].r {
			k = groups[1].r
		}
		return pack(7, groups[0].r, k)
	}
	if groups[0].c == 3 && len(groups) > 1 && groups[1].c == 2 {
		return pack(6, groups[0].r, groups[1].r)
	}
	if flush {
		return pack(5, ranks[0], ranks[1], ranks[2], ranks[3], ranks[4])
	}
	if straight {
		return pack(4, sHigh)
	}
	if groups[0].c == 3 {
		kick := []int{}
		for _, g := range groups[1:] {
			kick = append(kick, g.r)
		}
		for len(kick) < 2 {
			kick = append(kick, 0)
		}
		return pack(3, groups[0].r, kick[0], kick[1])
	}
	if groups[0].c == 2 && len(groups) > 1 && groups[1].c == 2 {
		p1, p2 := groups[0].r, groups[1].r
		k := 0
		if len(groups) > 2 {
			k = groups[2].r
		}
		return pack(2, p1, p2, k)
	}
	if groups[0].c == 2 {
		kick := []int{}
		for _, g := range groups[1:] {
			kick = append(kick, g.r)
		}
		for len(kick) < 3 {
			kick = append(kick, 0)
		}
		return pack(1, groups[0].r, kick[0], kick[1], kick[2])
	}
	_ = cs
	return pack(0, ranks[0], ranks[1], ranks[2], ranks[3], ranks[4])
}

func isStraight(ranksDesc [5]int) (bool, int) {
	// 去重後檢查
	u := []int{}
	for _, r := range ranksDesc {
		if len(u) == 0 || u[len(u)-1] != r {
			u = append(u, r)
		}
	}
	if len(u) != 5 {
		return false, 0
	}
	if u[0]-u[4] == 4 {
		return true, u[0]
	}
	// A-5 wheel: ranks A=12, 3,2,1,0 → 12,3,2,1,0
	if u[0] == 12 && u[1] == 3 && u[2] == 2 && u[3] == 1 && u[4] == 0 {
		return true, 3
	}
	return false, 0
}

// ---------- 公開視圖 ----------

type PublicSeat struct {
	Name     string `json:"name"`
	IsHero   bool   `json:"isHero"`
	IsAI     bool   `json:"isAI"`
	Bet      int64  `json:"bet"`
	TotalBet int64  `json:"totalBet"`
	Folded   bool   `json:"folded"`
	AllIn    bool   `json:"allIn"`
	Hole     []int  `json:"hole"`
	Won      bool   `json:"won"`
}

type PublicView struct {
	Active     bool         `json:"active"`
	Street     string       `json:"street"`
	Community  []int        `json:"community"`
	Pot        int64        `json:"pot"`
	CurrentBet int64        `json:"currentBet"`
	MinRaiseTo int64        `json:"minRaiseTo"`
	CallAmount int64        `json:"callAmount"`
	ToAct      int          `json:"toAct"`
	HeroIdx    int          `json:"heroIdx"`
	Button     int          `json:"button"`
	SB         int64        `json:"sb"`
	BB         int64        `json:"bb"`
	Seats      []PublicSeat `json:"seats"`
	Legal      []string     `json:"legal"`
	Status     string       `json:"status"`
	Result     string       `json:"result"`
	Payout     int64        `json:"payout"`
	Wagered    int64        `json:"wagered"`
	Winners    []int        `json:"winners"`
}

func (s *State) View(heroStack int64) PublicView {
	if s == nil {
		return PublicView{Active: false, Status: "idle", Seats: []PublicSeat{}, Community: []int{}, Legal: []string{}}
	}
	show := s.Status == "showdown"
	seats := make([]PublicSeat, len(s.Seats))
	winSet := map[int]bool{}
	for _, w := range s.Winners {
		winSet[w] = true
	}
	for i, p := range s.Seats {
		hole := []int{}
		if i == s.HeroIdx || (show && !p.Folded) {
			hole = append([]int{}, p.Hole...)
		} else if len(p.Hole) >= 2 {
			hole = []int{-1, -1}
		}
		seats[i] = PublicSeat{
			Name: p.Name, IsHero: i == s.HeroIdx, IsAI: p.IsAI,
			Bet: p.Bet, TotalBet: p.TotalBet, Folded: p.Folded, AllIn: p.AllIn,
			Hole: hole, Won: winSet[i],
		}
	}
	callAmt := s.CurrentBet - s.Seats[s.HeroIdx].Bet
	if callAmt < 0 {
		callAmt = 0
	}
	minTo := s.CurrentBet + s.LastRaise
	if minTo < s.CurrentBet+s.BBVal {
		minTo = s.CurrentBet + s.BBVal
	}
	v := PublicView{
		Active: s.Status == "playing", Street: s.Street, Community: append([]int{}, s.Community...),
		Pot: s.pot(), CurrentBet: s.CurrentBet, MinRaiseTo: minTo, CallAmount: callAmt,
		ToAct: s.ToAct, HeroIdx: s.HeroIdx, Button: s.Button, SB: s.SB, BB: s.BBVal,
		Seats: seats, Status: s.Status, Result: s.Result, Payout: s.Payout, Wagered: s.Wagered,
		Winners: append([]int{}, s.Winners...),
	}
	if s.Status == "playing" && s.ToAct == s.HeroIdx {
		v.Legal = s.legal(s.HeroIdx, heroStack)
	}
	if v.Legal == nil {
		v.Legal = []string{}
	}
	return v
}
