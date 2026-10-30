// Package fish 「深海金鲨」捕鱼机引擎:命中计费 + 服务端判死。
// 判死概率 p = RTP / V,击杀奖励 = V × 弹价 → 单次命中 EV = RTP × 弹价,
// 与瞄准目标无关,RTP 由 settings(admin 面板)实时控制。
package fish

import (
	"crypto/rand"
	"encoding/binary"
)

// BulletBase 基础弹价(×1 档),总弹价 = BulletBase × mult
const BulletBase = 10

// Mults 允许的炮台倍率档
var Mults = []int64{1, 2, 5, 10}

// Fish 鱼种定义
type Fish struct {
	ID     string  `json:"id"`     // sprite 文件名前缀
	Name   string  `json:"name"`   // 中文名
	Value  int64   `json:"value"`  // 价值倍数 V(奖励 = V × 弹价)
	Weight int     `json:"weight"` // 生成权重(客户端刷鱼用)
	Size   float64 `json:"size"`   // 渲染基准尺寸(px @1x)
	Speed  float64 `json:"speed"`  // 游速基准
	Boss   bool    `json:"boss"`   // Boss 定时出场,不参与常规权重
}

// Table 鱼表(ID 稳定,admin 调 RTP 不改表)
var Table = []Fish{
	{ID: "fish_clown", Name: "小丑鱼", Value: 2, Weight: 30, Size: 52, Speed: 55},
	{ID: "fish_puffer", Name: "河豚", Value: 3, Weight: 22, Size: 62, Speed: 42},
	{ID: "fish_jelly", Name: "水母", Value: 4, Weight: 18, Size: 66, Speed: 30},
	{ID: "fish_lantern", Name: "灯笼鱼", Value: 5, Weight: 14, Size: 64, Speed: 46},
	{ID: "fish_turtle", Name: "海龟", Value: 8, Weight: 8, Size: 88, Speed: 26},
	{ID: "fish_ray", Name: "蝠鲼", Value: 12, Weight: 5, Size: 110, Speed: 34},
	{ID: "fish_shark", Name: "银鲨", Value: 20, Weight: 3, Size: 140, Speed: 50},
	{ID: "fish_goldshark", Name: "金鲨", Value: 50, Weight: 0, Size: 200, Speed: 38, Boss: true},
}

// ByID 查鱼种,未知返回 nil
func ByID(id string) *Fish {
	for i := range Table {
		if Table[i].ID == id {
			return &Table[i]
		}
	}
	return nil
}

// ValidMult 校验倍率档
func ValidMult(m int64) bool {
	for _, v := range Mults {
		if v == m {
			return true
		}
	}
	return false
}

// RNG 返回 [0,1) 均匀随机数
type RNG func() float64

// CryptoRNG 生产用 crypto/rand
func CryptoRNG() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return float64(binary.LittleEndian.Uint64(b[:])>>11) / (1 << 53)
}

// Resolve 结算一次命中:返回(弹价, 奖励)。奖励为 0 表示未击杀。
// rtp 来自 settings(admin 可调,建议 0.5~1.2)。
func Resolve(f *Fish, mult int64, rtp float64, rng RNG) (cost, reward int64) {
	cost = BulletBase * mult
	p := rtp / float64(f.Value)
	if p > 1 {
		p = 1
	}
	if rng() < p {
		reward = f.Value * cost
	}
	return cost, reward
}
