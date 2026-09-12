package line

import (
	"math"
	"testing"
	"time"

	"go_binance_futures/feature/strategy"
	"go_binance_futures/models"
	"go_binance_futures/types"

	"github.com/adshao/go-binance/v2/futures"
)

// ===== 阈值边界(运行时浮点语义) =====

func TestLine5MomentumBoundaryValues(t *testing.T) {
	// +0.91%: 触发
	bars := flatBars("100", 30)
	bars[1] = k("100", "100", "100", "100", 999)
	bars[0] = k("100", "100.91", "100", "100.91", 1000)
	defer withKlines(bars, nil)()
	res := TradeLine5{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if !res.CanLong {
		t.Fatalf("+0.91%%应触发做多, got %+v", res)
	}

	// +0.88%: 不触发
	bars2 := flatBars("100", 30)
	bars2[1] = k("100", "100", "100", "100", 999)
	bars2[0] = k("100", "100.88", "100", "100.88", 1000)
	defer withKlines(bars2, nil)()
	res = TradeLine5{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanLong || res.CanShort {
		t.Fatalf("+0.88%%不应触发, got %+v", res)
	}
}

func TestLine6ThresholdRuntimeFloatBoundary(t *testing.T) {
	defer withRegime(0)()
	// 恰好 0.3%: 运行时浮点 (100-99.7)/100 = 0.0029999... < 0.003 → 不触发(固化现状)
	defer withKlines([]*futures.Kline{k("100", "100", "99.7", "99.7", 1000)}, nil)()
	res := TradeLine6{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanLong || res.CanShort {
		t.Fatalf("恰好0.3%%在运行时浮点下不应触发, got %+v", res)
	}
	// 0.31%: 触发
	defer withKlines([]*futures.Kline{k("100", "100", "99.69", "99.69", 1000)}, nil)()
	res = TradeLine6{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if !res.CanLong {
		t.Fatalf("0.31%%应触发做多, got %+v", res)
	}
}

// ===== 震荡行情(高波动/低波动)双向放行 =====

func TestLine5RegimeSidewaysAllowsBoth(t *testing.T) {
	defer withRegime(types.MarketConditionHighVolatility)()
	upBars := flatBars("100", 30)
	upBars[1] = k("100", "100", "100", "100", 999)
	upBars[0] = k("100", "101", "100", "100.95", 1000)
	defer withKlines(upBars, nil)()
	tl5 := TradeLine5{}
	res := tl5.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if !res.CanLong {
		t.Fatalf("高波动行情应允许多, got %+v", res)
	}
}

// ===== AutoStopOrder 盈亏边界 =====

func TestLine5AutoStopProfitBoundaries(t *testing.T) {
	defer withKlines(flatBars("100", 50), nil)()
	trade := TradeLine5{}
	pos := types.FuturesPosition{Symbol: "TESTUSDT", Side: "LONG", CreateTime: time.Now().UnixMilli()}

	// 恰好 ±3: 落入日级反转判断区间(平坦K线 → 无反转 → false)
	if res := trade.AutoStopOrder(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: pos, NowProfit: 3}); res.Complete {
		t.Fatal("NowProfit=3 不应触发日级反转退出")
	}
	if res := trade.AutoStopOrder(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: pos, NowProfit: -3}); res.Complete {
		t.Fatal("NowProfit=-3 不应触发日级反转退出")
	}
	if res := trade.AutoStopOrder(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: pos, NowProfit: 3.0001}); res.Complete {
		t.Fatal("NowProfit>3 不应触发日级反转退出")
	}
}

func TestLine5TimeStopBoundary(t *testing.T) {
	defer withKlines(nil, nil)()
	trade := TradeLine5{}

	// 恰好 60 分钟: 不触发
	exact := types.FuturesPosition{Symbol: "TESTUSDT", Side: "LONG", CreateTime: time.Now().Add(-time.Hour).UnixMilli()}
	if res := trade.AutoStopOrder(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: exact, NowProfit: 0}); res.Complete {
		t.Fatal("持仓恰好60分钟不应时间止损(严格大于)")
	}
	// CreateTime=0(未知开仓时间): 跳过时间止损
	unknown := types.FuturesPosition{Symbol: "TESTUSDT", Side: "LONG", CreateTime: 0}
	if res := trade.AutoStopOrder(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: unknown, NowProfit: 0}); res.Complete {
		t.Fatal("CreateTime=0 应跳过时间止损")
	}
}

// ===== 平仓函数方向语义矩阵 =====

func TestCanOrderCompleteSideSemantics(t *testing.T) {
	fallingBars := []*futures.Kline{k("100", "100", "99", "99", 1000), k("100", "100", "100", "100", 998)}
	defer withKlines(fallingBars, nil)()

	// line1: 错误路径返回 false(与 line2-6 的 true 不同, 固化各自现状)
	line1 := TradeLine1{}
	defer withKlines(nil, errTest)()
	tl1res := line1.CanOrderComplete(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "T"}, Position: types.FuturesPosition{Side: "LONG"}})
	if tl1res.Complete {
		t.Fatal("line1 K线失败语义: 不放行平仓")
	}
	defer withKlines(fallingBars, nil)()

	// 各策略 LONG 持仓遇下跌: 均允许平仓
	for name, s := range map[string]interface {
		CanOrderComplete(strategy.CloseParams) strategy.CloseResult
	}{
		"line1": TradeLine1{}, "line2": TradeLine2{}, "line3": TradeLine3{},
		"line4": TradeLine4{}, "line5": TradeLine5{}, "line6": TradeLine6{}, "line7": TradeLine7{},
	} {
		res := s.CanOrderComplete(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "T"}, Position: types.FuturesPosition{Side: "LONG"}})
		if !res.Complete {
			t.Fatalf("%s: LONG遇下跌应允许平仓", name)
		}
	}
}

func TestLineCanOrderCompleteUnknownSide(t *testing.T) {
	fallingBars := []*futures.Kline{k("100", "100", "99", "99", 1000), k("100", "100", "100", "100", 998)}
	defer withKlines(fallingBars, nil)()

	// 未知方向(BOTH): 全部策略均有 else 分支 → 放行平仓(固化现状)
	tl1 := TradeLine1{}
	tl1res := tl1.CanOrderComplete(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "T"}, Position: types.FuturesPosition{Side: "BOTH"}})
	if !tl1res.Complete {
		t.Fatal("line1 未知方向应放行平仓")
	}
	tl2 := TradeLine2{}
	tl2res := tl2.CanOrderComplete(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "T"}, Position: types.FuturesPosition{Side: "BOTH"}})
	if !tl2res.Complete {
		t.Fatal("line2 未知方向按既有实现(else)应放行平仓")
	}
	tl5 := TradeLine5{}
	tl5res := tl5.CanOrderComplete(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "T"}, Position: types.FuturesPosition{Side: "BOTH"}})
	if !tl5res.Complete {
		t.Fatal("line5 未知方向按既有实现(else)应放行平仓")
	}
}

// ===== line3 金叉/死叉路径(复用 line2 夹具) =====

// shapeBarsFromCloses 用收盘价构造 OHLC, 并在最低/最高收盘K线上合成 line3 要求的大实体形态
func shapeBarsFromCloses(closes []float64, longShape bool) []*futures.Kline {
	n := len(closes)
	bars := make([][4]float64, n)
	for i := 0; i < n; i++ {
		c := closes[i]
		prev := c
		if i < n-1 {
			prev = closes[i+1] // 时间上的前一根收盘
		}
		o := prev
		h := math.Max(o, c) + 0.01
		l := math.Min(o, c) - 0.01
		bars[i] = [4]float64{o, h, l, c}
	}
	mi, xi := 0, 0
	for i := range bars {
		if bars[i][3] < bars[mi][3] {
			mi = i
		}
		if bars[i][3] > bars[xi][3] {
			xi = i
		}
	}
	if longShape {
		c := bars[mi][3]
		bars[mi] = [4]float64{c * 1.04, c + 0.05, c - 0.05, c} // 阴线大实体小影线, 仍为最低
	} else {
		c := bars[xi][3]
		bars[xi] = [4]float64{c * 0.96, c + 0.05, c - 0.05, c} // 阳线大实体小上影, 仍为最高
	}
	return ohlcBars(bars)
}

func TestLine3GoldenCrossPath(t *testing.T) {
	// line2 多头夹具同样满足 line3 的 EMA3/7 金叉 + rsi>40 条件
	defer withKlines(shapeBarsFromCloses(line2LongCloses, true), nil)()
	res := TradeLine3{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if !res.CanLong {
		t.Fatalf("line3 金叉路径应触发做多, got %+v", res)
	}
}

func TestLine3DeathCrossPath(t *testing.T) {
	defer withKlines(shapeBarsFromCloses(line2ShortCloses, false), nil)()
	res := TradeLine3{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if !res.CanShort {
		t.Fatalf("line3 死叉路径应触发做空, got %+v", res)
	}
}
