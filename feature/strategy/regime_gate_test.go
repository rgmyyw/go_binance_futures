package strategy

import (
	"testing"

	"go_binance_futures/types"
)

func TestRegimeGateMatrix(t *testing.T) {
	cases := []struct {
		condition int
		long      bool
		short     bool
	}{
		{0, true, true},  // 未初始化: 全放行(安全默认)
		{99, true, true}, // 未知码: 全放行
		{types.MarketConditionStrongBull, true, false},
		{types.MarketConditionBull, true, false},
		{types.MarketConditionBroadRise, true, false},
		{types.MarketConditionStrongBear, false, true},
		{types.MarketConditionBear, false, true},
		{types.MarketConditionBroadDecline, false, true},
		{types.MarketConditionSideways, true, true},
		{types.MarketConditionBullishDivergence, true, true},
		{types.MarketConditionBearishDivergence, true, true},
		{types.MarketConditionHighVolatility, true, true},
		{types.MarketConditionLowVolatility, true, true},
	}
	for _, c := range cases {
		SetRegimeCondition(c.condition)
		if got := RegimeAllowsLong(); got != c.long {
			t.Errorf("condition=%d RegimeAllowsLong=%v, want %v", c.condition, got, c.long)
		}
		if got := RegimeAllowsShort(); got != c.short {
			t.Errorf("condition=%d RegimeAllowsShort=%v, want %v", c.condition, got, c.short)
		}
	}
	SetRegimeCondition(0)
}
