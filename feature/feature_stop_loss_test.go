package feature

import (
	"testing"

	"github.com/adshao/go-binance/v2/futures"
	"go_binance_futures/feature/strategy"
	"go_binance_futures/types"
)

func TestCalcStopPrice(t *testing.T) {
	// 多头: ROI 5% ÷ 3倍杠杆 = 价格下跌1.6667%
	price, side, ok := calcStopPrice(100, 5, 3, false, "0.001")
	if !ok || side != futures.SideTypeSell {
		t.Fatalf("多头止损应为SELL方向, got ok=%v side=%v", ok, side)
	}
	if price < 98.33 || price > 98.34 {
		t.Fatalf("多头止损价应约98.333, got %v", price)
	}
	// 空头: 价格上涨方向
	price, side, ok = calcStopPrice(100, 5, 3, true, "0.001")
	if !ok || side != futures.SideTypeBuy {
		t.Fatalf("空头止损应为BUY方向, got ok=%v side=%v", ok, side)
	}
	if price < 101.66 || price > 101.67 {
		t.Fatalf("空头止损价应约101.667, got %v", price)
	}
	// 非法输入
	if _, _, ok := calcStopPrice(0, 5, 3, false, "0.001"); ok {
		t.Fatal("entry为0应拒绝")
	}
	if _, _, ok := calcStopPrice(100, 0, 3, false, "0.001"); ok {
		t.Fatal("无止损配置应拒绝")
	}
	if _, _, ok := calcStopPrice(100, 5, 0, false, "0.001"); ok {
		t.Fatal("杠杆为0应拒绝")
	}
}

func TestStopOrderExists(t *testing.T) {
	orders := []futures.GetAlgoOrderResp{
		{OrderType: futures.AlgoOrderTypeStopMarket, Symbol: "AUSDT", PositionSide: futures.PositionSideTypeLong},
		{OrderType: "TAKE_PROFIT", Symbol: "AUSDT", PositionSide: futures.PositionSideTypeLong},
	}
	if !stopOrderExists(orders, "AUSDT", futures.PositionSideTypeLong) {
		t.Fatal("存在的止损单应被识别")
	}
	if stopOrderExists(orders, "AUSDT", futures.PositionSideTypeShort) {
		t.Fatal("方向不同不应误判")
	}
	if stopOrderExists(orders, "BUSDT", futures.PositionSideTypeLong) {
		t.Fatal("币种不同不应误判")
	}
	if stopOrderExists(nil, "AUSDT", futures.PositionSideTypeLong) {
		t.Fatal("空列表不应误判")
	}
}

func TestEntrySpreadOK(t *testing.T) {
	cases := []struct {
		name       string
		bid, ask   float64
		want       bool
	}{
		{"正常价差0.05%", 100.0, 100.05, true},
		{"恰好边界内0.3%", 100.0, 100.3, true},
		{"超宽价差0.5%", 100.0, 100.5, false},
		{"meme极端2%", 1.0, 1.02, false},
		{"中间价为0拒绝", 0, 0, false},
	}
	for _, c := range cases {
		if got := entrySpreadOK(c.bid, c.ask); got != c.want {
			t.Errorf("%s: entrySpreadOK(%v,%v)=%v, want %v", c.name, c.bid, c.ask, got, c.want)
		}
	}
}

func TestRegimeGateSyncedEvenWhenSwitchDisabled(t *testing.T) {
	// 方向闸门同步独立于各早退分支: 手动行情/非托管策略时闸门仍应更新
	syncRegimeGate(types.MarketConditionBroadDecline)
	if strategy.RegimeAllowsLong() {
		t.Fatal("普跌行情应禁多")
	}
	syncRegimeGate(types.MarketConditionBroadRise)
	if strategy.RegimeAllowsShort() {
		t.Fatal("普涨行情应禁空")
	}
	syncRegimeGate(0)
}
