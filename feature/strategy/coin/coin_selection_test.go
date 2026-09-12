package coin

import (
	"testing"

	"go_binance_futures/models"
)

func sym(symbol string, enable int, pct float64) *models.Symbols {
	return &models.Symbols{Symbol: symbol, Enable: enable, PercentChange: pct}
}

func withRecentSymbols(recent map[string]bool) (restore func()) {
	prev := getRecentOrderSymbols
	getRecentOrderSymbols = func(minute int64) map[string]bool { return recent }
	return func() { getRecentOrderSymbols = prev }
}

func withRecentLocalSymbols(recent map[string]bool) (restore func()) {
	prev := getRecentLocalOrderSymbols
	getRecentLocalOrderSymbols = func(minute int64) map[string]bool { return recent }
	return func() { getRecentLocalOrderSymbols = prev }
}

func TestCoin1PicksStrongestMoversDeterministically(t *testing.T) {
	defer withRecentSymbols(map[string]bool{})()

	coins := []*models.Symbols{
		sym("AUSDT", 1, -9),
		sym("BUSDT", 1, -8),
		sym("CUSDT", 1, 7),
		sym("DUSDT", 1, 9),   // 涨幅最强
		sym("EUSDT", 1, 8),   // 涨幅次强
		sym("FUSDT", 1, -10), // 跌幅最强
		sym("GUSDT", 0, 50),  // 未启用, 应被排除
	}
	got := TradeCoin1{}.SelectCoins(coins)
	if len(got) != 4 {
		t.Fatalf("应选出4个币, 实际%d个", len(got))
	}
	seen := map[string]bool{}
	for _, c := range got {
		seen[c.Symbol] = true
	}
	for _, want := range []string{"FUSDT", "AUSDT", "DUSDT", "EUSDT"} {
		if !seen[want] {
			t.Fatalf("应包含最强动量币 %s, 实际 %v", want, seen)
		}
	}
	if seen["GUSDT"] {
		t.Fatal("未启用币不应入选")
	}
}

func TestCoin1ExcludesRecentlyTraded(t *testing.T) {
	defer withRecentSymbols(map[string]bool{"DUSDT": true, "FUSDT": true})()

	coins := []*models.Symbols{
		sym("AUSDT", 1, -9),
		sym("BUSDT", 1, -8),
		sym("CUSDT", 1, 7),
		sym("DUSDT", 1, 9),
		sym("EUSDT", 1, 8),
		sym("FUSDT", 1, -10),
	}
	got := TradeCoin1{}.SelectCoins(coins)
	seen := map[string]bool{}
	for _, c := range got {
		seen[c.Symbol] = true
	}
	if seen["DUSDT"] || seen["FUSDT"] {
		t.Fatalf("最近5分钟交易过的币不应入选: %v", seen)
	}
}

func TestCoin1FewCoinsBoundary(t *testing.T) {
	defer withRecentSymbols(map[string]bool{})()

	// 币种不足6个时取全部中的最强
	got := TradeCoin1{}.SelectCoins([]*models.Symbols{sym("AUSDT", 1, -1), sym("BUSDT", 1, 2)})
	if len(got) != 2 {
		t.Fatalf("边界情况应选出2个, 实际%d个", len(got))
	}
	// 空列表不panic
	got = TradeCoin1{}.SelectCoins(nil)
	if len(got) != 0 {
		t.Fatalf("空输入应返回空, 实际%d个", len(got))
	}
}

func TestCoin6SelectsFromTopVolume(t *testing.T) {
	defer withRecentLocalSymbols(map[string]bool{})()

	coins := make([]*models.Symbols, 0, 10)
	// 10个启用币 + 1个未启用; 成交额决定排序
	for i := 0; i < 10; i++ {
		c := sym(string(rune('A'+i))+"USDT", 1, 0)
		c.QuoteVolume = float64(1000 - i*10)
		coins = append(coins, c)
	}
	disabled := sym("XUSDT", 0, 0)
	disabled.QuoteVolume = 99999
	coins = append(coins, disabled)

	got := TradeCoin6{}.SelectCoins(coins)
	if len(got) != 5 {
		t.Fatalf("应选出5个币, 实际%d个", len(got))
	}
	for _, c := range got {
		if c.Symbol == "XUSDT" {
			t.Fatal("未启用币不应入选")
		}
		if c.QuoteVolume < 910 {
			t.Fatalf("应从成交额前200内选取, %s 成交额 %v 异常", c.Symbol, c.QuoteVolume)
		}
	}
}

func TestGetRandArr(t *testing.T) {
	arr := []*models.Symbols{sym("A", 1, 0), sym("B", 1, 0), sym("C", 1, 0)}
	// num >= len 返回全部
	if got := GetRandArr(arr, 5); len(got) != 3 {
		t.Fatalf("num超长应返回全部, 实际%d", len(got))
	}
	// num < len 返回 num 个且不重复
	got := GetRandArr(arr, 2)
	if len(got) != 2 {
		t.Fatalf("应返回2个, 实际%d", len(got))
	}
	if got[0].Symbol == got[1].Symbol {
		t.Fatal("随机选取不应重复")
	}
}
