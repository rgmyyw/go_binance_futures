package feature

import (
	"testing"

	"go_binance_futures/types"
)

func TestRegimeClassByCondition(t *testing.T) {
	choppy := []int{
		types.MarketConditionSideways,
		types.MarketConditionBullishDivergence,
		types.MarketConditionBearishDivergence,
		types.MarketConditionHighVolatility,
		types.MarketConditionLowVolatility,
	}
	for _, c := range choppy {
		if got := regimeClassByCondition(c); got != 1 {
			t.Errorf("condition=%d class=%d, want 1(震荡类)", c, got)
		}
	}
	trending := []int{
		types.MarketConditionStrongBull,
		types.MarketConditionBull,
		types.MarketConditionStrongBear,
		types.MarketConditionBear,
		types.MarketConditionBroadRise,
		types.MarketConditionBroadDecline,
		99, // 未知码按趋势处理
	}
	for _, c := range trending {
		if got := regimeClassByCondition(c); got != 0 {
			t.Errorf("condition=%d class=%d, want 0(趋势类)", c, got)
		}
	}
}

func TestRegimeDecideNextDebounce(t *testing.T) {
	cases := []struct {
		name            string
		class           int
		last            int
		pending         int
		wantTarget      string
		wantLast        int
		wantPending     int
		wantShouldSwitch bool
	}{
		{"首次运行只记录", 0, -1, -1, "", 0, -1, false},
		{"行情未变不切换", 0, 0, -1, "", 0, -1, false},
		{"同类型时清理pending", 1, 1, 0, "", 1, -1, false},
		{"新类型第一次只登记等待确认", 1, 0, -1, "", 0, 1, false},
		{"新类型第二次确认切换(line6禁用→维持line5)", 1, 0, 1, "line5", 1, -1, true},
		{"新类型第二次确认切换为趋势策略", 0, 1, 0, "line5", 0, -1, true},
		{"pending期间又见旧类型则重置pending", 0, 0, 1, "", 0, -1, false},
		{"反复横跳不切换", 1, 0, 0, "", 0, 1, false},
	}
	for _, c := range cases {
		target, newLast, newPending, shouldSwitch := regimeDecideNext(c.class, c.last, c.pending)
		if target != c.wantTarget || newLast != c.wantLast || newPending != c.wantPending || shouldSwitch != c.wantShouldSwitch {
			t.Errorf("%s: got(target=%q last=%d pending=%d switch=%v), want(target=%q last=%d pending=%d switch=%v)",
				c.name, target, newLast, newPending, shouldSwitch, c.wantTarget, c.wantLast, c.wantPending, c.wantShouldSwitch)
		}
	}
}
