package line

import (
	"encoding/json"
	"testing"

	"github.com/adshao/go-binance/v2/futures"

	"go_binance_futures/feature/strategy"
	"go_binance_futures/models"
	"go_binance_futures/technology"
	"go_binance_futures/types"
)

type futuresKlineHelper struct{ Open, High, Low, Close string }

func (h []futuresKlineHelper) toKlines() []*futures.Kline {
	out := make([]*futures.Kline, len(h))
	for i, b := range h {
		out[i] = &futures.Kline{Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, OpenTime: int64(1000 - i)}
	}
	return out
}

func typesFuturesPosition(side string) types.FuturesPosition {
	return types.FuturesPosition{Symbol: "T", Side: side, Amount: "1", EntryPrice: "100", MarkPrice: "99"}
}

// ValidateTechnologyConfigJSON JSON 便捷封装
func ValidateTechnologyConfigJSON(raw string) error {
	var cfg technology.TechnologyConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return err
	}
	return ValidateTechnologyConfig(cfg)
}

// customStrategyJSON 构造自定义策略配置
func customStrategyJSON(code string, typ string) string {
	return `[{"enable":true,"type":"` + typ + `","code":"` + code + `"}]`
}

func TestLineCustomNullConfigNoSignal(t *testing.T) {
	for _, cfg := range []string{"", "null", "  "} {
		res := TradeLineCustom{}.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "T", Strategy: cfg}})
		if res.CanLong || res.CanShort {
			t.Fatalf("空配置(%q)不应产生信号", cfg)
		}
	}
}

func TestLineCustomLongExprTriggers(t *testing.T) {
	setupLineTestDB(t)
	// NowPrice 来自 symbols 表(测试库中该币不存在 → NowPrice 缺失), 用恒真表达式
	defer withKlines(nil, nil)()
	sym := &models.Symbols{Symbol: "T", Strategy: customStrategyJSON("SystemStartTime >= 0", "long")}
	res := TradeLineCustom{}.GetCanLongOrShort(strategy.OpenParams{Symbols: sym})
	if !res.CanLong {
		t.Fatalf("恒真 long 表达式应触发做多, got %+v", res)
	}
}

func TestLineCustomShortExprTriggers(t *testing.T) {
	setupLineTestDB(t)
	defer withKlines(nil, nil)()
	sym := &models.Symbols{Symbol: "T", Strategy: customStrategyJSON("SystemStartTime >= 0", "short")}
	res := TradeLineCustom{}.GetCanLongOrShort(strategy.OpenParams{Symbols: sym})
	if !res.CanShort {
		t.Fatalf("恒真 short 表达式应触发做空, got %+v", res)
	}
}

func TestLineCustomInvalidJSONNoPanic(t *testing.T) {
	sym := &models.Symbols{Symbol: "T", Strategy: "{invalid json"}
	res := TradeLineCustom{}.GetCanLongOrShort(strategy.OpenParams{Symbols: sym})
	if res.CanLong || res.CanShort {
		t.Fatal("非法JSON不应产生信号")
	}
}

func TestLineCustomCloseStrategy(t *testing.T) {
	setupLineTestDB(t)
	defer withKlines([]*futuresKlineHelper{{Open: "100", High: "100", Low: "99", Close: "99"}, {Open: "100", High: "100", Low: "100", Close: "100"}}.toKlines(), nil)()
	pos := typesFuturesPosition("LONG")

	// 平多表达式恒真 → 平仓
	sym := &models.Symbols{Symbol: "T", Strategy: customStrategyJSON("ROI > -100", "close_long")}
	res := TradeLineCustom{}.CanOrderComplete(strategy.CloseParams{Symbols: sym, Position: pos, NowProfit: 1})
	if !res.Complete {
		t.Fatal("恒真平多表达式应放行平仓")
	}

	// 平空表达式对多仓: 跳过(close_short 仅作用于 SHORT 仓) → findStrategy=false → 走 simpleCloseStrategy
	symShortClose := &models.Symbols{Symbol: "T", Strategy: customStrategyJSON("ROI > -100", "close_short")}
	res = TradeLineCustom{}.CanOrderComplete(strategy.CloseParams{Symbols: symShortClose, Position: pos, NowProfit: 1})
	if res.Complete {
		t.Fatal("多仓遇close_short策略应跳过且 ±3 内不平仓")
	}
}

func TestLineCustomSimpleCloseStrategyFixedGate(t *testing.T) {
	defer withKlines([]*futuresKlineHelper{{Open: "100", High: "100", Low: "99", Close: "99"}, {Open: "100", High: "100", Low: "100", Close: "100"}}.toKlines(), nil)()
	pos := typesFuturesPosition("LONG")
	sym := &models.Symbols{Symbol: "T"}

	// 修复后的语义: ±3 内不平仓
	res := TradeLineCustom{}.simpleCloseStrategy(strategy.CloseParams{Symbols: sym, Position: pos, NowProfit: 1})
	if res.Complete {
		t.Fatal("±3 内 simpleCloseStrategy 不应平仓")
	}
	// 盈亏超 ±3 且多头遇下跌 → 平仓
	res = TradeLineCustom{}.simpleCloseStrategy(strategy.CloseParams{Symbols: sym, Position: pos, NowProfit: 5})
	if !res.Complete {
		t.Fatal("超±3且多头遇下跌应平仓")
	}
}

func TestLineCustomAutoStopAlwaysFalse(t *testing.T) {
	res := TradeLineCustom{}.AutoStopOrder(strategy.CloseParams{})
	if res.Complete {
		t.Fatal("line_custom AutoStopOrder 应恒为 false(无自动止损逻辑)")
	}
}

func TestIsNullAndNormalizeCustomConfig(t *testing.T) {
	for _, v := range []string{"", "null", "  "} {
		if !isNullCustomConfig(v) {
			t.Fatalf("%q 应判定为空配置", v)
		}
	}
	if isNullCustomConfig("{}") {
		t.Fatal("{} 不应判定为空配置")
	}
	if got := normalizeCustomTechnology(""); got != "{}" {
		t.Fatalf("normalizeCustomTechnology(\"\") 应为 {{}}, got %s", got)
	}
	if got := normalizeCustomTechnology("{\"ma\":[]}"); got != "{\"ma\":[]}" {
		t.Fatalf("非空 technology 应原样返回, got %s", got)
	}
}

func TestValidateAndParseTechnologyConfig(t *testing.T) {
	// 合法配置
	valid := `{"ma":[{"name":"ma5","klineInterval":"15m","period":5,"enable":true}]}`
	if err := ValidateTechnologyConfigJSON(valid); err != nil {
		t.Fatalf("合法配置不应报错: %v", err)
	}
	// 保留名
	reserved := `{"ma":[{"name":"ROI","klineInterval":"15m","period":5,"enable":true}]}`
	if err := ValidateTechnologyConfigJSON(reserved); err == nil {
		t.Fatal("保留名应报错")
	}
	// 不支持的周期
	badInterval := `{"ma":[{"name":"ma5","klineInterval":"7m","period":5,"enable":true}]}`
	if err := ValidateTechnologyConfigJSON(badInterval); err == nil {
		t.Fatal("非法K线周期应报错")
	}
	// 重名
	dup := `{"ma":[{"name":"m","klineInterval":"15m","period":5,"enable":true},{"name":"m","klineInterval":"1h","period":10,"enable":true}]}`
	if err := ValidateTechnologyConfigJSON(dup); err == nil {
		t.Fatal("重名指标应报错")
	}
	// MACD fast>=slow
	badMacd := `{"macd":[{"name":"macd1","klineInterval":"15m","fastPeriod":26,"slowPeriod":12,"signalPeriod":9,"enable":true}]}`
	if err := ValidateTechnologyConfigJSON(badMacd); err == nil {
		t.Fatal("MACD fast>=slow 应报错")
	}
}

func TestParseTechnologyConfigWithSeam(t *testing.T) {
	defer withKlines(flatBars("100", 150), nil)()
	config, klineMap := ParseTechnologyConfig("TESTUSDT", `{"ma":[{"name":"ma5","klineInterval":"15m","period":5,"enable":true}]}`)
	cd, ok := config["ma5"]
	if !ok {
		t.Fatal("启用的 MA 指标应出现在配置中")
	}
	_ = cd
	if _, ok := klineMap["15m"]; !ok {
		t.Fatal("应缓存 15m K线数据")
	}
	// 非法JSON: 返回空map
	config, klineMap = ParseTechnologyConfig("TESTUSDT", "{bad")
	if len(config) != 0 || len(klineMap) != 0 {
		t.Fatal("非法JSON应返回空结果")
	}
}

func TestGetLineFloatValues(t *testing.T) {
	bars := []*futuresKlineHelper{
		{Open: "1", High: "2", Low: "0.5", Close: "1.5"},
		{Open: "2", High: "3", Low: "1", Close: "2.5"},
	}.toKlines()
	high, low, close, open := GetLineFloatValues(bars)
	if len(high) != 2 || high[1] != 3 || low[0] != 0.5 || close[1] != 2.5 || open[0] != 1 {
		t.Fatalf("GetLineFloatValues 结果错误: %v %v %v %v", high, low, close, open)
	}
}
