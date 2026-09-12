package feature

import (
	"context"
	"testing"
	"time"

	agentevent "go_binance_futures/agent/event"
	"github.com/adshao/go-binance/v2/futures"
	"go_binance_futures/models"
)

func ctx() context.Context { return context.Background() }

// ===== 价格变动提醒扩展场景 =====

func TestPriceChangeNoticeDownMoveAndCooldownExpiry(t *testing.T) {
	var pushed []float64
	prevPush := pushPriceChangeNotice
	pushPriceChangeNotice = func(symbol string, price float64, pct float64) {
		pushed = append(pushed, symbol)
	}
	defer func() { pushPriceChangeNotice = prevPush }()

	defer setPriceChangeSnapshot(map[string]bool{"UAIUSDT": true}, 5)()

	// 建立参考价
	mustNil(handlePriceChangeTick(ctx(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 100, 0)))

	// 暴跌 -6%: 同样触发推送(双向监控)
	mustNil(handlePriceChangeTick(ctx(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 94, 0)))
	if len(pushed) != 1 {
		t.Fatalf("暴跌超阈值应推送, 实际%d次", len(pushed))
	}

	// 冷却过期后: 新参考价 94 再跌 6% → 再次推送
	value, ok := priceChangeStates.Load("UAIUSDT")
	if !ok {
		t.Fatal("应存在状态条目")
	}
	value.(*priceChangeEntry).lastAlertAt = time.Now().Add(-11 * time.Minute).UnixMilli()
	mustNil(handlePriceChangeTick(ctx(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 88, 0)))
	if len(pushed) != 2 {
		t.Fatalf("冷却过期后应再次推送, 实际%d次", len(pushed))
	}
}

func TestPriceChangeNoticeDisabledSnapshot(t *testing.T) {
	var pushed []string
	prevPush := pushPriceChangeNotice
	pushPriceChangeNotice = func(symbol string, price float64, pct float64) {
		pushed = append(pushed, symbol)
	}
	defer func() { pushPriceChangeNotice = prevPush }()

	defer setPriceChangeSnapshot(map[string]bool{"UAIUSDT": true}, 5)()

	// 建立参考价并触发一次
	mustNil(handlePriceChangeTick(ctx(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 100, 0)))
	mustNil(handlePriceChangeTick(ctx(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 106, 0)))
	if len(pushed) != 1 {
		t.Fatalf("应推送1次, 实际%d次", len(pushed))
	}

	// 快照关闭(wsEnabled=false): 即使继续暴涨也不推送
	priceChangeSnap.Store(priceChangeSnapshot{wsEnabled: false, limit: 5, watch: map[string]bool{"UAIUSDT": true}})
	mustNil(handlePriceChangeTick(ctx(), agentevent.NewPriceTick("UAIUSDT", "test", time.Now().UnixMilli(), 113, 0)))
	if len(pushed) != 1 {
		t.Fatalf("监控关闭时不应推送, 实际%d次", len(pushed))
	}
}

// ===== 交易所止损单下单接缝 =====

func TestPlaceSymbolStopLossCallsAlgoOrder(t *testing.T) {
	var capturedSymbol, capturedSide, capturedPositionSide string
	var capturedPrice float64
	prev := placeStopLossAlgo
	placeStopLossAlgo = func(symbol string, stopPrice float64, side futures.SideType, positionSide futures.PositionSideType) (*futures.CreateAlgoOrderResp, error) {
		capturedSymbol, capturedPrice = symbol, stopPrice
		capturedSide, capturedPositionSide = string(side), string(positionSide)
		return nil, nil
	}
	defer func() { placeStopLossAlgo = prev }()

	coin := &models.Symbols{Symbol: "TSTUSDT", Loss: "5", TickSize: "0.001"}
	PlaceSymbolStopLoss(coin, "100", 3, futures.PositionSideTypeLong)
	if capturedSymbol != "TSTUSDT" {
		t.Fatalf("应调用下单: %s", capturedSymbol)
	}
	if capturedPrice < 98.33 || capturedPrice > 98.34 {
		t.Fatalf("多头止损价应约98.333, got %v", capturedPrice)
	}
	if capturedSide != "SELL" || capturedPositionSide != "LONG" {
		t.Fatalf("方向错误: %s %s", capturedSide, capturedPositionSide)
	}
}

func TestPlaceSymbolStopLossSkipsWhenNoLossConfig(t *testing.T) {
	called := false
	prev := placeStopLossAlgo
	placeStopLossAlgo = func(symbol string, stopPrice float64, side futures.SideType, positionSide futures.PositionSideType) (*futures.CreateAlgoOrderResp, error) {
		called = true
		return nil, nil
	}
	defer func() { placeStopLossAlgo = prev }()

	coin := &models.Symbols{Symbol: "TSTUSDT", Loss: "0"}
	PlaceSymbolStopLoss(coin, "100", 3, futures.PositionSideTypeLong)
	if called {
		t.Fatal("未配置止损阈值不应调用下单")
	}
}
