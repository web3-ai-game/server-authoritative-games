// Package wheel 幸运转盘:每日免费一次的福利机制。
// 8 个奖格,奖额 = 倍数 × wheel_base(settings,admin 可调),加权随机,服务端权威。
package wheel

import (
	"crypto/rand"
	"encoding/binary"
)

// Segment 奖格:倍数 + 权重(顺序即转盘上的排列)
type Segment struct {
	Mult   float64 `json:"mult"`
	Weight int     `json:"weight"`
}

// Segments 8 格,权重和 100。期望 ≈1.9× 基准额。
// 排列刻意打散大小奖,转盘视觉更刺激。
var Segments = []Segment{
	{0.5, 25},
	{3, 9},
	{1, 25},
	{10, 2},
	{1.5, 18},
	{5, 6},
	{2, 14},
	{20, 1},
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

// Pick 按权重抽取奖格,返回索引
func Pick(rng RNG) int {
	total := 0
	for _, s := range Segments {
		total += s.Weight
	}
	roll := rng(total)
	for i, s := range Segments {
		roll -= s.Weight
		if roll < 0 {
			return i
		}
	}
	return 0
}

// Value 奖格金额(向下取整到 10 的倍数,显示更整齐)
func Value(idx int, base int64) int64 {
	v := int64(Segments[idx].Mult * float64(base))
	return v / 10 * 10
}
