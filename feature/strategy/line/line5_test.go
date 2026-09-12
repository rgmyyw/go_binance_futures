package line

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"go_binance_futures/feature/strategy"
	"go_binance_futures/models"
	"go_binance_futures/types"

	"github.com/adshao/go-binance/v2/futures"
)

// k 构造一根K线(字段为字符串, 与交易所返回一致)
func k(o, h, l, c string, openTime int64) *futures.Kline {
	return &futures.Kline{Open: o, High: h, Low: l, Close: c, OpenTime: openTime}
}

// flatBars 构造 n 根无波动K线(新→旧), 单价 price
func flatBars(price string, n int) []*futures.Kline {
	bars := make([]*futures.Kline, n)
	for i := 0; i < n; i++ {
		bars[i] = k(price, price, price, price, int64(1000-i))
	}
	return bars
}

func mustFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func withKlines(bars []*futures.Kline, err error) (restore func()) {
	prev := getKlineData
	getKlineData = func(symbol string, interval string, limit int) ([]*futures.Kline, error) {
		return bars, err
	}
	return func() { getKlineData = prev }
}

func withRegime(condition int) (restore func()) {
	strategy.SetRegimeCondition(condition)
	return func() { strategy.SetRegimeCondition(0) }
}

func TestLine5LongSignalOnMomentum(t *testing.T) {
	defer withKlines(nil, nil)()
	defer withRegime(0)()

	// 前一根开盘100, 当前收100.95 (+0.95% ≥0.9%), 窗口内无≥1.5%振幅K线
	bars := flatBars("100", 30)
	bars[1] = k("100", "100", "100", "100", 999)
	bars[0] = k("100", "100.95", "100", "100.95", 1000)

	res := TradeLine5{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if !res.CanLong {
		t.Fatalf("+0.95%%动量应触发做多, got %+v", res)
	}
	if res.CanShort {
		t.Fatal("上涨动量不应触发做空")
	}
}

func TestLine5ShortSignalOnDownMomentum(t *testing.T) {
	defer withKlines(nil, nil)()
	defer withRegime(0)()

	bars := flatBars("100", 30)
	bars[1] = k("100", "100", "100", "100", 999)
	bars[0] = k("100", "100", "99.0", "99.0", 1000) // -1%

	res := TradeLine5{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if !res.CanShort {
		t.Fatalf("下跌动量应触发做空, got %+v", res)
	}
}

func TestLine5BelowThresholdNoSignal(t *testing.T) {
	defer withKlines(nil, nil)()

	bars := flatBars("100", 30)
	bars[1] = k("100", "100", "100", "100", 999)
	bars[0] = k("100", "100.5", "100", "100.5", 1000) // +0.5% < 0.9%

	res := TradeLine5{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanLong || res.CanShort {
		t.Fatalf("0.5%%动量不应触发任何信号, got %+v", res)
	}
}

func TestLine5SpikeWindowBlocksEntry(t *testing.T) {
	defer withKlines(nil, nil)()

	// 窗口内第10根出现过 2% 振幅(突变), 即使满足动量也放弃
	bars := flatBars("100", 30)
	bars[10] = k("100", "102", "100", "101", 990)
	bars[1] = k("100", "100", "100", "100", 999)
	bars[0] = k("100", "101", "100", "100.95", 1000)

	res := TradeLine5{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanLong || res.CanShort {
		t.Fatalf("窗口内有突变K线应放弃入场, got %+v", res)
	}
}

func TestLine5RegimeGateBlocksCounterTrend(t *testing.T) {
	defer withKlines(nil, nil)()

	downBars := flatBars("100", 30)
	downBars[1] = k("100", "100", "100", "100", 999)
	downBars[0] = k("100", "100", "99.0", "99.0", 1000)

	// 普涨(8): 禁空 → 下跌动量不开空
	defer withRegime(types.MarketConditionBroadRise)()
	defer withKlines(downBars, nil)()
	res := TradeLine5{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanShort {
		t.Fatal("普涨行情下跌动量不应开空")
	}

	// 普跌(9): 禁多 → 上涨动量不开多
	upBars := flatBars("100", 30)
	upBars[1] = k("100", "100", "100", "100", 999)
	upBars[0] = k("100", "101", "100", "100.95", 1000)
	defer withRegime(types.MarketConditionBroadDecline)()
	defer withKlines(upBars, nil)()
	res = TradeLine5{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanLong {
		t.Fatal("普跌行情上涨动量不应开多")
	}
}

func TestLine5NilSymbolsAndShortKlineGuard(t *testing.T) {
	defer withKlines([]*futures.Kline{k("100", "100", "100", "100", 1)}, nil)()
	res := TradeLine5{}.GetCanLongOrShort(strategy.OpenParams{Symbols: nil})
	if res.CanLong || res.CanShort {
		t.Fatal("nil 币种信息不应产生信号")
	}
	// K线不足2根不应越界panic
	res = TradeLine5{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
	if res.CanLong || res.CanShort {
		t.Fatal("K线不足时不应产生信号")
	}
}

func TestLine5CanOrderComplete(t *testing.T) {
	trade := TradeLine5{}
	longPos := types.FuturesPosition{Symbol: "TESTUSDT", Side: "LONG"}
	shortPos := types.FuturesPosition{Symbol: "TESTUSDT", Side: "SHORT"}

	if res := trade.CanOrderComplete(strategy.CloseParams{Symbols: nil, Position: longPos}); !res.Complete {
		t.Fatal("nil 币种信息应允许平仓")
	}

	defer withKlines([]*futures.Kline{k("100", "100", "99", "99", 1000), k("100", "100", "100", "100", 998)}, nil)()
	// 新K线收99 < 旧K线收100: 下跌中 → 多仓允许平
	if res := trade.CanOrderComplete(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: longPos}); !res.Complete {
		t.Fatal("多仓遇下跌应允许平仓")
	}
	if res := trade.CanOrderComplete(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: shortPos}); res.Complete {
		t.Fatal("空仓遇下跌不应平仓")
	}

	defer withKlines(nil, errTest)()
	if res := trade.CanOrderComplete(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: longPos}); !res.Complete {
		t.Fatal("K线获取失败应保守放行平仓")
	}
}


func TestLine5TimeStop(t *testing.T) {
	defer withKlines(nil, nil)()
	trade := TradeLine5{}

	fresh := types.FuturesPosition{Symbol: "TESTUSDT", Side: "LONG", CreateTime: time.Now().UnixMilli()}
	if res := trade.AutoStopOrder(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: fresh, NowProfit: 0}); res.Complete {
		t.Fatal("持仓未满60分钟不应时间止损(且未到反转条件)")
	}

	stale := types.FuturesPosition{Symbol: "TESTUSDT", Side: "LONG", CreateTime: time.Now().Add(-61 * time.Minute).UnixMilli()}
	if res := trade.AutoStopOrder(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: stale, NowProfit: 0}); !res.Complete {
		t.Fatal("持仓超60分钟应时间止损")
	}

	// 盈亏超出±3时日级反转退出被跳过
	if res := trade.AutoStopOrder(strategy.CloseParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}, Position: fresh, NowProfit: 5}); res.Complete {
		t.Fatal("浮盈超±3不应触发日级反转退出")
	}
}

var errTest = errors.New("test kline error")

// 接缝 getKlineData 生产实现见 kline_fetch.go
