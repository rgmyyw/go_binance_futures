package feature

import (
	"testing"
	"time"

	"github.com/beego/beego/v2/client/orm"

	"go_binance_futures/models"
)

// ===== 震荡熔断边界场景 =====

func TestChopBreakerPauseExtension(t *testing.T) {
	resetChopBreaker()
	// 触发熔断
	for i := 0; i < 3; i++ {
		recordStopLossEvent()
	}
	if !chopBreakerActive() {
		t.Fatal("应处于熔断暂停")
	}
	first := chopPauseUntil
	// 暂停期间再次发生 3 次止损: 暂停窗口顺延
	for i := 0; i < 3; i++ {
		recordStopLossEvent()
	}
	if chopPauseUntil <= first {
		t.Fatal("暂停期间的止损应顺延暂停窗口")
	}
	resetChopBreaker()
}

func TestChopBreakerWindowBoundary(t *testing.T) {
	resetChopBreaker()
	// 恰好 30 分钟前的事件仍在窗口内
	edge := time.Now().Add(-29*time.Minute).UnixMilli()
	lossEventMu.Lock()
	lossEventTimes = []int64{edge, edge}
	lossEventMu.Unlock()
	recordStopLossEvent()
	if !chopBreakerActive() {
		t.Fatal("窗口边界内第3次止损应触发熔断")
	}
	resetChopBreaker()
}

// ===== 价差过滤交叉盘口 =====

func TestEntrySpreadCrossedBook(t *testing.T) {
	// 交叉盘口(bid>ask, 数据异常): 价差为负 → 放行(不因异常数据卡死交易)
	if !entrySpreadOK(100.1, 100.0) {
		t.Fatal("交叉盘口应放行")
	}
}

// ===== ROI 阈值解析扩展 =====

func TestResolveTradeROIThresholdsNegativeAndDecimal(t *testing.T) {
	profit, loss := resolveTradeROIThresholds("0.5", "-3")
	if profit != 0.5 || loss != -3 {
		t.Fatalf("小数与负数应按字面解析: %v %v", profit, loss)
	}
}

// ===== 白名单解析扩展 =====

func TestGetExcludeSymbolsMapEmptyParts(t *testing.T) {
	m := GetExcludeSymbolsMap("BTCUSDT,,ETHUSDT,")
	if len(m) != 4 {
		t.Fatalf("空片段会产生空键: %v", m)
	}
	if !m["BTCUSDT"] || !m["ETHUSDT"] {
		t.Fatal("有效币种应被解析")
	}
}

// ===== 订单写入 =====

func TestInsertOpenOrderWritesRow(t *testing.T) {
	setupTestDB(t)
	insertOpenOrder("TSTUSDT", 0.5, "100.5", "LONG", 3, 999001)
	var rows []models.Order
	if _, err := orm.NewOrm().QueryTable("order").Filter("order_id", 999001).All(&rows); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("应写入1条订单, 实际%d条", len(rows))
	}
	if rows[0].Side != "open" || rows[0].PositionSide != "LONG" {
		t.Fatalf("订单字段错误: %+v", rows[0])
	}
}

var _ = models.Config{}
