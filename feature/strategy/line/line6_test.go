package line

import (
	"testing"
	"time"

	"go_binance_futures/feature/strategy"
	"go_binance_futures/models"
	"go_binance_futures/types"

	"github.com/adshao/go-binance/v2/futures"
)

func TestLine6LongSignalBuysDip(t *testing.T) {
	defer withKlines([]*futures.Kline{k("100", "100", "99.7", "99.7", 1000)}, nil)()
	defer withRegime(0)()

	res := TradeLine6{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if !res.CanLong {
		t.Fatalf("3分钟下跌0.3%%应触发做多(逆势), got %+v", res)
	}
	if res.CanShort {
		t.Fatal("下跌不应触发做空")
	}
}

func TestLine6ShortSignalFadesRip(t *testing.T) {
	defer withKlines([]*futures.Kline{k("100", "100.3", "100", "100.3", 1000)}, nil)()

	res := TradeLine6{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if !res.CanShort {
		t.Fatalf("3分钟上涨0.3%%应触发做空(逆势), got %+v", res)
	}
}

func TestLine6ThresholdBoundary(t *testing.T) {
	// 恰好0.3%: 触发 (99.7/100)
	defer withKlines([]*futures.Kline{k("100", "100", "99.7", "99.7", 1000)}, nil)()
	res := TradeLine6{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if !res.CanLong {
		t.Fatalf("恰好0.3%%应触发, got %+v", res)
	}
	// 0.25%: 不触发
	defer withKlines([]*futures.Kline{k("100", "100", "99.75", "99.75", 1000)}, nil)()
	res = TradeLine6{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanLong || res.CanShort {
		t.Fatalf("0.25%%不应触发, got %+v", res)
	}
}

func TestLine6RegimeGateBlocksCounterTrend(t *testing.T) {
	// 普涨(8) 禁空: 上涨0.3%的反转信号被压制
	defer withRegime(types.MarketConditionBroadRise)()
	defer withKlines([]*futures.Kline{k("100", "100.3", "100", "100.3", 1000)}, nil)()
	res := TradeLine6{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanShort {
		t.Fatal("普涨行情反转信号不应开空")
	}

	// 普跌(9) 禁多
	defer withRegime(types.MarketConditionBroadDecline)()
	defer withKlines([]*futures.Kline{k("100", "100", "99.7", "99.7", 1000)}, nil)()
	res = TradeLine6{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanLong {
		t.Fatal("普跌行情反转信号不应开多")
	}
}

func TestLine6Guards(t *testing.T) {
	defer withRegime(0)()
	// nil 币种
	res := TradeLine6{}.GetCanLongOrShort(strategy.OpenParams{Symbols: nil})
	if res.CanLong || res.CanShort {
		t.Fatal("nil 币种信息不应产生信号")
	}
	// 空K线
	defer withKlines([]*futures.Kline{}, nil)()
	res = TradeLine6{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanLong || res.CanShort {
		t.Fatal("空K线不应产生信号且不应panic")
	}
}

func TestLine6CanOrderComplete(t *testing.T) {
	trade := TradeLine6{}
	shortPos := types.FuturesPosition{Symbol: "TESTUSDT", Side: "SHORT"}
	longPos := types.FuturesPosition{Symbol: "TESTUSDT", Side: "LONG"}

	if res := trade.CanOrderComplete(strategy.CloseParams{Symbols: nil, Position: shortPos}); !res.Complete {
		t.Fatal("nil 币种信息应允许平仓")
	}

	defer withKlines([]*futures.Kline{k("100", "101", "100", "101", 1000), k("100", "100", "100", "100", 998)}, nil)()
	// 上涨中 → 空仓允许平
	if res := trade.CanOrderComplete(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: shortPos}); !res.Complete {
		t.Fatal("空仓遇上涨应允许平仓")
	}
	if res := trade.CanOrderComplete(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: longPos}); res.Complete {
		t.Fatal("多仓遇上涨不应平仓")
	}
}

func TestLine6TimeStop(t *testing.T) {
	defer withKlines(nil, nil)()
	trade := TradeLine6{}

	fresh := types.FuturesPosition{Symbol: "TESTUSDT", Side: "SHORT", CreateTime: time.Now().UnixMilli()}
	if res := trade.AutoStopOrder(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: fresh, NowProfit: 0}); res.Complete {
		t.Fatal("持仓未满90分钟不应时间止损")
	}

	stale := types.FuturesPosition{Symbol: "TESTUSDT", Side: "SHORT", CreateTime: time.Now().Add(-91 * time.Minute).UnixMilli()}
	if res := trade.AutoStopOrder(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: stale, NowProfit: 0}); !res.Complete {
		t.Fatal("持仓超90分钟应时间止损")
	}
}
