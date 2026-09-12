package feature

import (
	"testing"
	"time"
)

func TestOpenFailCooldown(t *testing.T) {
	openFailCooldown.Delete("XUSDT|LONG")
	if openOnCooldown("XUSDT", "LONG") {
		t.Fatal("未标记时不应冷却")
	}
	markOpenFail("XUSDT", "LONG")
	if !openOnCooldown("XUSDT", "LONG") {
		t.Fatal("标记后应处于冷却")
	}
	if openOnCooldown("XUSDT", "SHORT") {
		t.Fatal("同币不同方向不应被冷却波及")
	}
	openFailCooldown.Store("XUSDT|LONG", time.Now().Add(-time.Minute).Unix())
	if openOnCooldown("XUSDT", "LONG") {
		t.Fatal("过期后应解除冷却")
	}
	if _, ok := openFailCooldown.Load("XUSDT|LONG"); ok {
		t.Fatal("过期条目应被清理")
	}
}

func TestLossCooldown(t *testing.T) {
	lossCooldown.Delete("YUSDT|SHORT")
	markLossCooldown("YUSDT", "SHORT")
	if !lossCooldownActive("YUSDT", "SHORT") {
		t.Fatal("止损后同方向应冷却")
	}
	if lossCooldownActive("YUSDT", "LONG") {
		t.Fatal("反方向不应被冷却波及")
	}
	lossCooldown.Store("YUSDT|SHORT", time.Now().Add(-time.Minute).Unix())
	if lossCooldownActive("YUSDT", "SHORT") {
		t.Fatal("冷却过期后应允许重进")
	}
}

func resetChopBreaker() {
	lossEventMu.Lock()
	lossEventTimes = nil
	chopPauseUntil = 0
	chopLoggedActive = false
	lossEventMu.Unlock()
}

func TestChopBreakerTriggersOnThirdLoss(t *testing.T) {
	resetChopBreaker()
	recordStopLossEvent()
	recordStopLossEvent()
	if chopBreakerActive() {
		t.Fatal("两次止损不应触发熔断")
	}
	recordStopLossEvent()
	if !chopBreakerActive() {
		t.Fatal("窗口内三次止损应触发熔断")
	}
	// 过期后恢复
	lossEventMu.Lock()
	chopPauseUntil = time.Now().Add(-time.Minute).UnixMilli()
	lossEventMu.Unlock()
	if chopBreakerActive() {
		t.Fatal("暂停窗口过期后应恢复开仓")
	}
	resetChopBreaker()
}

func TestChopBreakerIgnoresOutOfWindowEvents(t *testing.T) {
	resetChopBreaker()
	old := time.Now().Add(-31 * time.Minute).UnixMilli()
	lossEventMu.Lock()
	lossEventTimes = []int64{old, old}
	lossEventMu.Unlock()
	recordStopLossEvent() // 窗口内只有这 1 次, 旧事件应被裁剪
	if chopBreakerActive() {
		t.Fatal("窗口外事件不应计入熔断")
	}
	lossEventMu.Lock()
	if n := len(lossEventTimes); n != 1 {
		t.Fatalf("窗口裁剪后应剩1条, 实际%d条", n)
	}
	lossEventMu.Unlock()
	resetChopBreaker()
}
