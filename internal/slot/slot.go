// Package slot 实现「金運財神」老虎机引擎:5 轮 × 3 行,20 条赔付线。
// 纯逻辑零 IO,结果由服务端 RNG 决定(客户端只做演出)。
package slot

import (
	"crypto/rand"
	"encoding/binary"
)

// 符号索引(与前端 /api/slot/config 返回顺序一致)
const (
	Wild = iota // 神龙,替代除 Scatter 外全部符号
	Scatter
	Caishen
	Koi
	Hongbao
	Fu
	SymA
	SymK
	SymQ
	SymJ
	numSymbols
)

// SymbolIDs 是符号在前端/资产文件中的稳定 ID。
var SymbolIDs = [numSymbols]string{
	"wild", "scatter", "caishen", "koi", "hongbao", "fu", "sym_a", "sym_k", "sym_q", "sym_j",
}

// SymbolNames 中文名(赔付表展示用)
var SymbolNames = [numSymbols]string{
	"神龙 WILD", "金元宝 SCATTER", "財神", "锦鲤", "红包", "福", "A", "K", "Q", "J",
}

const (
	Reels        = 5
	Rows         = 3
	NumLines     = 20
	FreeSpinsWon = 8 // 3+ Scatter 触发的免费旋转次数
	FreeSpinMult = 2 // 免费旋转期间全部奖励倍数
)

// Bets 允许的投注档位(总注,均可被 20 整除)
var Bets = []int64{100, 200, 500, 1000, 2000}

// Lines 20 条固定赔付线:每条给出 5 轮各自的行号(0 顶 1 中 2 底)
var Lines = [NumLines][Reels]int{
	{1, 1, 1, 1, 1}, {0, 0, 0, 0, 0}, {2, 2, 2, 2, 2}, {0, 1, 2, 1, 0}, {2, 1, 0, 1, 2},
	{0, 0, 1, 0, 0}, {2, 2, 1, 2, 2}, {1, 0, 0, 0, 1}, {1, 2, 2, 2, 1}, {0, 1, 1, 1, 0},
	{2, 1, 1, 1, 2}, {1, 0, 1, 0, 1}, {1, 2, 1, 2, 1}, {0, 1, 0, 1, 0}, {2, 1, 2, 1, 2},
	{1, 1, 0, 1, 1}, {1, 1, 2, 1, 1}, {0, 0, 2, 0, 0}, {2, 2, 0, 2, 2}, {0, 2, 0, 2, 0},
}

// PayTable[sym][count] = 单线赔付倍数(乘以线注 = 总注/20);count 3..5 有效。
// Scatter 不走线,见 ScatterPay。
var PayTable = [numSymbols][Reels + 1]int64{
	Wild:    {0, 0, 0, 100, 500, 2000},
	Caishen: {0, 0, 0, 50, 200, 1000},
	Koi:     {0, 0, 0, 30, 100, 400},
	Hongbao: {0, 0, 0, 25, 75, 250},
	Fu:      {0, 0, 0, 20, 60, 200},
	SymA:    {0, 0, 0, 10, 25, 100},
	SymK:    {0, 0, 0, 8, 20, 80},
	SymQ:    {0, 0, 0, 5, 15, 60},
	SymJ:    {0, 0, 0, 5, 15, 60},
}

// ScatterPay[count] = 总注倍数(全盘任意位置计数)
var ScatterPay = [Reels + 1]int64{0, 0, 0, 2, 10, 50}

// reelStrips 加权卷轴条:权重经 RTP 仿真调校(见 slot_test.go,目标 ~95%)。
var reelStrips = [Reels][]int{
	buildStrip([numSymbols]int{Wild: 2, Scatter: 2, Caishen: 4, Koi: 4, Hongbao: 4, Fu: 5, SymA: 6, SymK: 7, SymQ: 8, SymJ: 8}),
	buildStrip([numSymbols]int{Wild: 2, Scatter: 2, Caishen: 4, Koi: 4, Hongbao: 4, Fu: 5, SymA: 6, SymK: 7, SymQ: 8, SymJ: 8}),
	buildStrip([numSymbols]int{Wild: 3, Scatter: 2, Caishen: 4, Koi: 4, Hongbao: 4, Fu: 5, SymA: 6, SymK: 7, SymQ: 8, SymJ: 8}),
	buildStrip([numSymbols]int{Wild: 3, Scatter: 2, Caishen: 4, Koi: 4, Hongbao: 4, Fu: 5, SymA: 6, SymK: 7, SymQ: 8, SymJ: 8}),
	buildStrip([numSymbols]int{Wild: 2, Scatter: 2, Caishen: 4, Koi: 4, Hongbao: 4, Fu: 5, SymA: 6, SymK: 7, SymQ: 8, SymJ: 8}),
}

// buildStrip 按权重展开成打散的卷轴条(交错铺开避免同符号连片)。
func buildStrip(weights [numSymbols]int) []int {
	total := 0
	for _, w := range weights {
		total += w
	}
	strip := make([]int, 0, total)
	remaining := weights
	for len(strip) < total {
		for s := 0; s < numSymbols; s++ {
			if remaining[s] > 0 {
				strip = append(strip, s)
				remaining[s]--
			}
		}
	}
	return strip
}

// LineWin 一条中奖线
type LineWin struct {
	Line   int   `json:"line"`   // 线号 0..19
	Symbol int   `json:"symbol"` // 判定符号
	Count  int   `json:"count"`  // 连续个数(自左)
	Win    int64 `json:"win"`    // 金币
}

// Result 一次旋转的完整结果
type Result struct {
	Grid         [Reels][Rows]int `json:"grid"`  // grid[reel][row] = 符号索引
	Stops        [Reels]int       `json:"stops"` // 各轮停点(前端演出用)
	Wins         []LineWin        `json:"wins"`
	ScatterCount int              `json:"scatterCount"`
	ScatterWin   int64            `json:"scatterWin"`
	FreeSpins    int              `json:"freeSpins"` // 本次新赢得的免费旋转
	TotalWin     int64            `json:"totalWin"`
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

// Spin 执行一次旋转。totalBet 必须是合法档位(调用方校验);
// mult 为奖励倍数(普通=1,免费旋转=FreeSpinMult)。
func Spin(totalBet int64, rng RNG, mult int64) Result {
	var r Result
	for reel := 0; reel < Reels; reel++ {
		strip := reelStrips[reel]
		stop := rng(len(strip))
		r.Stops[reel] = stop
		for row := 0; row < Rows; row++ {
			r.Grid[reel][row] = strip[(stop+row)%len(strip)]
		}
	}
	evaluate(&r, totalBet, mult)
	return r
}

// evaluate 结算赔付线 + Scatter
func evaluate(r *Result, totalBet, mult int64) {
	lineBet := totalBet / NumLines * mult
	for li, line := range Lines {
		var syms [Reels]int
		for reel := 0; reel < Reels; reel++ {
			syms[reel] = r.Grid[reel][line[reel]]
		}
		// 自左连续 wild 数
		wildRun := 0
		for _, s := range syms {
			if s != Wild {
				break
			}
			wildRun++
		}
		// 基准符号 = 首个非 wild;全 wild 则为 wild
		base, count := Wild, wildRun
		if wildRun < Reels {
			cand := syms[wildRun]
			if cand != Scatter { // scatter 不走线且不被 wild 替代
				base, count = cand, wildRun
				for i := wildRun; i < Reels; i++ {
					if syms[i] == cand || syms[i] == Wild {
						count++
					} else {
						break
					}
				}
			}
		}
		basePay := PayTable[base][count] * lineBet
		wildPay := PayTable[Wild][wildRun] * lineBet
		// 取较优判法(如 wild wild wild koi x → wild×3 优于 koi×4 时按 wild 计)
		pay, sym, cnt := basePay, base, count
		if wildPay > pay {
			pay, sym, cnt = wildPay, Wild, wildRun
		}
		if pay > 0 {
			r.Wins = append(r.Wins, LineWin{Line: li, Symbol: sym, Count: cnt, Win: pay})
			r.TotalWin += pay
		}
	}
	for reel := 0; reel < Reels; reel++ {
		for row := 0; row < Rows; row++ {
			if r.Grid[reel][row] == Scatter {
				r.ScatterCount++
			}
		}
	}
	if r.ScatterCount >= 3 {
		n := r.ScatterCount
		if n > Reels {
			n = Reels
		}
		r.ScatterWin = ScatterPay[n] * totalBet * mult
		r.TotalWin += r.ScatterWin
		r.FreeSpins = FreeSpinsWon
	}
}

// ValidBet 校验投注档位
func ValidBet(bet int64) bool {
	for _, b := range Bets {
		if b == bet {
			return true
		}
	}
	return false
}
