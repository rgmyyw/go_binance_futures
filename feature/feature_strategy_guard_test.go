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
