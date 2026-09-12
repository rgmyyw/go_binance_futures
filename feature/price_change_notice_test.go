package feature

import (
	"context"
	"testing"
	"time"

	agentevent "go_binance_futures/agent/event"
)

type fakeEvent struct{}

func (fakeEvent) Metadata() agentevent.Metadata { return agentevent.Metadata{} }

func setPriceChangeSnapshot(watch map[string]bool, limit float64) (restore func()) {
	priceChangeSnap.Store(priceChangeSnapshot{wsEnabled: true, limit: limit, watch: watch})
	return func() {
		priceChangeSnap.Store(priceChangeSnapshot{watch: map[string]bool{}})
		priceChangeStates.Range(func(key, value any) bool {
			priceChangeStates.Delete(key)
			return true
		})
	}
}

func TestPriceChangeNoticeEndToEnd(t *testing.T) {
	var pushed []string
	prevPush := pushPriceChangeNotice
	pushPriceChangeNotice = func(symbol string, price float64, pct float64) {
		pushed = append(pushed, symbol)
	}
	defer func() { pushPriceChangeNotice = prevPush }()

	defer setPriceChangeSnapshot(map[string]bool{"UAIUSDT": true}, 5)()

	// 非价格事件忽略
	if err := handlePriceChangeTick(context.Background(), fakeEvent{}); err != nil {
		t.Fatalf("非价格事件不应报错: %v", err)
	}
	if len(pushed) != 0 {
		t.Fatal("非价格事件不应推送")
	}

	// 首次行情只建立参考价
	mustNil(handlePriceChangeTick(context.Background(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 100, 0)))
	if len(pushed) != 0 {
		t.Fatal("首次行情只应建立参考价, 不应推送")
	}

	// +5.3% 超阈值 → 推送并重置参考价
	mustNil(handlePriceChangeTick(context.Background(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 105.3, 0)))
	if len(pushed) != 1 {
		t.Fatalf("超阈值应推送1次, 实际%d次", len(pushed))
	}

	// 冷却期内再超阈值不推送
	mustNil(handlePriceChangeTick(context.Background(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 111, 0)))
	if len(pushed) != 1 {
		t.Fatalf("冷却期内不应推送, 实际%d次", len(pushed))
	}

	// 清冷却后继续涨 → 再次推送
	if v, ok := priceChangeStates.Load("UAIUSDT"); ok {
		v.(*priceChangeEntry).lastAlertAt = 0
	} else {
		t.Fatal("应存在UAIUSDT状态条目")
	}
	mustNil(handlePriceChangeTick(context.Background(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 111, 0)))
	if len(pushed) != 2 {
		t.Fatalf("冷却解除后应再次推送, 实际%d次", len(pushed))
	}
}

func TestPriceChangeNoticeFilters(t *testing.T) {
	var pushed []string
	prevPush := pushPriceChangeNotice
	pushPriceChangeNotice = func(symbol string, price float64, pct float64) {
		pushed = append(pushed, symbol)
	}
	defer func() { pushPriceChangeNotice = prevPush }()

	// 不在监控列表的币忽略
	defer setPriceChangeSnapshot(map[string]bool{"UAIUSDT": true}, 5)()
	mustNil(handlePriceChangeTick(context.Background(), agentevent.NewPriceTick("BTCUSDT", "test", time.Now().UnixMilli(), 100, 0)))
	if len(pushed) != 0 {
		t.Fatal("未监控币种不应推送")
	}

	// 开关关闭(limit=0)时忽略
	defer setPriceChangeSnapshot(map[string]bool{"UAIUSDT": true}, 0)()
	mustNil(handlePriceChangeTick(context.Background(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 100, 0)))
	mustNil(handlePriceChangeTick(context.Background(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 200, 0)))
	if len(pushed) != 0 {
		t.Fatal("阈值0(关闭)不应推送")
	}
}

// mustNil 断言 err 为 nil(测试辅助)
func mustNil(err error) {
	if err != nil {
		panic(err)
	}
}
