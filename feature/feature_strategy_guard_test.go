package feature

import (
	"testing"
	"time"
)

func TestLine6DemotionAfterThreeStops(t *testing.T) {
	setupTestDB(t)
	resetSwitchState()
	upsertConfigRow(t, "line6", 6, 1, 1)

	RecordStopLossForStrategy("line6")
	RecordStopLossForStrategy("line6")
	if line6Allowed() != true {
		t.Fatal("未达阈值不应进入冷却")
	}
	RecordStopLossForStrategy("line6")
	if line6Allowed() {
		t.Fatal("3连亏应触发line6降级冷却")
	}
	// 数据库应已切回 line5
	if got := readConfigStrategy(t); got != "line5" {
		t.Fatalf("降级应写入数据库, got %s", got)
	}
	resetSwitchState()
}

func TestLine5StopsDoNotDemote(t *testing.T) {
	setupTestDB(t)
	resetSwitchState()
	upsertConfigRow(t, "line5", 8, 1, 1)
	for i := 0; i < 5; i++ {
		RecordStopLossForStrategy("line5")
	}
	if !line6Allowed() {
		t.Fatal("line5 的止损不应触发 line6 降级")
	}
	resetSwitchState()
}

func TestLine6CooldownExpiry(t *testing.T) {
	strategyGuardMu.Lock()
	line6DemoteUntilMs = time.Now().Add(-time.Minute).UnixMilli()
	strategyGuardMu.Unlock()
	if !line6Allowed() {
		t.Fatal("冷却过期后应重新允许 line6")
	}
	strategyGuardMu.Lock()
	line6DemoteUntilMs = 0
	strategyGuardMu.Unlock()
}

func TestRealizedCloseEventDrivesDemotion(t *testing.T) {
	setupTestDB(t)
	resetSwitchState()
	initRealizedCloseSeen()
	upsertConfigRow(t, "line6", 6, 1, 1)

	// 三笔交易所侧止损事件(去重: 同一订单只计一次)
	for i := 0; i < 2; i++ {
		mustNil(handleRealizedCloseForTest(int64(100+i), -1.8))
	}
	if line6Allowed() != true {
		t.Fatal("未达3笔不应降级")
	}
	mustNil(handleRealizedCloseForTest(100, -1.8)) // 重复订单去重
	mustNil(handleRealizedCloseForTest(102, -1.8))
	if line6Allowed() {
		t.Fatal("交易所侧3笔止损应触发line6降级")
	}
	if got := readConfigStrategy(t); got != "line5" {
		t.Fatalf("降级应写库, got %s", got)
	}
	// 小额动量亏损不计数
	resetSwitchState()
	mustNil(handleRealizedCloseForTest(200, -0.05))
	mustNil(handleRealizedCloseForTest(201, -0.29))
	if line6Allowed() != true || line6LossStreak != 0 {
		t.Fatal("小额亏损不应计入止损连亏")
	}
	resetSwitchState()
}

func handleRealizedCloseForTest(orderId int64, pnl float64) error {
	return handleRealizedClose(evtFakeRealizedClose(orderId, pnl))
}

type evtFakeRealizedClose struct{ id int64; pnl float64 }
func (e evtFakeRealizedClose) Metadata() agentevent.Metadata { return agentevent.Metadata{} }
