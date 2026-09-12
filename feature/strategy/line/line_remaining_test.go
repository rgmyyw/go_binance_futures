package line

import (
	"testing"

	"go_binance_futures/feature/strategy"
	"go_binance_futures/models"
	"go_binance_futures/types"

	"github.com/adshao/go-binance/v2/futures"
)

// line1/line2 AutoStopOrder: 无自动止损逻辑, 恒为 false
func TestLine1And2AutoStopOrderAlwaysFalse(t *testing.T) {
	if res := (TradeLine1{}).AutoStopOrder(strategy.CloseParams{}); res.Complete {
		t.Fatal("line1 AutoStopOrder 应恒为 false")
	}
	if res := (TradeLine2{}).AutoStopOrder(strategy.CloseParams{}); res.Complete {
		t.Fatal("line2 AutoStopOrder 应恒为 false")
	}
}

// line3/line4/line7 AutoStopOrder: ±3 门外不动作, 门内走 MarketReversal
func TestLine347AutoStopOrderGateAndReversal(t *testing.T) {
	defer withKlines(closeBars(marketReversalLongExitCloses), nil)()
	sym := &models.Symbols{Symbol: "TESTUSDT"}
	longPos := types.FuturesPosition{Symbol: "TESTUSDT", Side: "LONG", CreateTime: 1}

	for name, s := range map[string]interface {
		AutoStopOrder(strategy.CloseParams) strategy.CloseResult
	}{
		"line3": TradeLine3{}, "line4": TradeLine4{}, "line7": TradeLine7{},
	} {
		// 盈亏门外: 不动作
		if res := s.AutoStopOrder(strategy.CloseParams{Symbols: sym, Position: longPos, NowProfit: 5}); res.Complete {
			t.Fatalf("%s: NowProfit>3 不应触发反转退出", name)
		}
		// 盈亏门内 + 金叉反转K线: 触发
		if res := s.AutoStopOrder(strategy.CloseParams{Symbols: sym, Position: longPos, NowProfit: 0}); !res.Complete {
			t.Fatalf("%s: 多仓反转K线应触发退出", name)
		}
	}
}

// line7.MarketReversal: 函数体被注释, 恒为 false(固化现状)
func TestLine7MarketReversalAlwaysFalse(t *testing.T) {
	defer withKlines(closeBars(marketReversalLongExitCloses), nil)()
	if (TradeLine7{}).MarketReversal("TESTUSDT", "LONG") {
		t.Fatal("line7 MarketReversal 当前为空实现, 应恒为 false")
	}
}

// line2 CanOrderComplete: K线失败 → 放行平仓
func TestLine2CanOrderCompleteErrorPath(t *testing.T) {
	defer withKlines(nil, errTest)()
	res := (TradeLine2{}).CanOrderComplete(strategy.CloseParams{
		Symbols:  &models.Symbols{Symbol: "T"},
		Position: types.FuturesPosition{Side: "LONG"},
	})
	if !res.Complete {
		t.Fatal("line2 K线失败应放行平仓")
	}
}

// line4 crossState: 交叉状态描述
func TestCrossState(t *testing.T) {
	if got := crossState(nil, nil); got != "无数据" {
		t.Fatalf("空输入应返回无数据, got %s", got)
	}
	// 最新在上, 1根前在下 → 金叉1根K线前
	if got := crossState([]float64{5, 1, 1}, []float64{4, 2, 2}); got != "金叉1根K线前" {
		t.Fatalf("金叉描述错误: %s", got)
	}
	// 最新在下, 1根前在上 → 死叉1根K线前
	if got := crossState([]float64{1, 5, 5}, []float64{2, 4, 4}); got != "死叉1根K线前" {
		t.Fatalf("死叉描述错误: %s", got)
	}
	// 持续在上
	if got := crossState([]float64{5, 5, 5}, []float64{4, 4, 4}); got != "持续在上" {
		t.Fatalf("持续在上描述错误: %s", got)
	}
}

// OBV 负成交额错误路径
func TestCalculateOBVNegativeAmount(t *testing.T) {
	if _, err := CalculateOBV([]float64{1, 2}, []float64{1, -1}); err == nil {
		t.Fatal("负成交额应报错")
	}
}
