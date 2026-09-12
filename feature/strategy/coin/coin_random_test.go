package coin

import (
	"testing"

	"go_binance_futures/models"
)

func TestCoin2RandomThreeWithExclusion(t *testing.T) {
	defer withRecentSymbols(map[string]bool{"BUSDT": true})()
	coins := []*models.Symbols{sym("AUSDT", 1, 1), sym("BUSDT", 1, 2), sym("CUSDT", 1, 3), sym("DUSDT", 1, 4)}
	got := TradeCoin2{}.SelectCoins(coins)
	if len(got) != 3 {
		t.Fatalf("coin2 应随机选3个, 实际%d个", len(got))
	}
	for _, c := range got {
		if c.Symbol == "BUSDT" {
			t.Fatal("被排除币不应入选")
		}
	}
}

func TestCoin3RandomTwoWithExclusion(t *testing.T) {
	defer withRecentSymbols(map[string]bool{"CUSDT": true, "DUSDT": true})()
	coins := []*models.Symbols{sym("AUSDT", 1, 1), sym("BUSDT", 1, 2), sym("CUSDT", 1, 3), sym("DUSDT", 1, 4)}
	got := TradeCoin3{}.SelectCoins(coins)
	if len(got) != 2 {
		t.Fatalf("coin3 应随机选2个, 实际%d个", len(got))
	}
	for _, c := range got {
		if c.Symbol == "CUSDT" || c.Symbol == "DUSDT" {
			t.Fatal("被排除币不应入选")
		}
	}
}

func TestCoin4RandomThreeWithDisabledFilter(t *testing.T) {
	defer withRecentSymbols(map[string]bool{})()
	coins := []*models.Symbols{sym("AUSDT", 1, 1), sym("BUSDT", 1, 2), sym("CUSDT", 0, 3), sym("DUSDT", 1, 4)}
	got := TradeCoin4{}.SelectCoins(coins)
	if len(got) != 3 {
		t.Fatalf("coin4 应选3个(可用币只有3个), 实际%d个", len(got))
	}
	for _, c := range got {
		if c.Symbol == "CUSDT" {
			t.Fatal("未启用币不应入选")
		}
	}
}

func TestCoin5LocalExclusionRandomFive(t *testing.T) {
	defer withRecentLocalSymbols(map[string]bool{"EUSDT": true})()
	coins := []*models.Symbols{
		sym("AUSDT", 1, 1), sym("BUSDT", 1, 2), sym("CUSDT", 1, 3),
		sym("DUSDT", 1, 4), sym("EUSDT", 1, 5), sym("FUSDT", 1, 6),
	}
	got := TradeCoin5{}.SelectCoins(coins)
	if len(got) != 5 {
		t.Fatalf("coin5 应随机选5个, 实际%d个", len(got))
	}
	for _, c := range got {
		if c.Symbol == "EUSDT" {
			t.Fatal("本地最近平仓币不应入选")
		}
	}
}

func TestCoinStrategiesRespectEnabledFilter(t *testing.T) {
	defer withRecentSymbols(map[string]bool{})()
	defer withRecentLocalSymbols(map[string]bool{})()
	coins := []*models.Symbols{sym("AUSDT", 0, 1), sym("BUSDT", 0, 2)}
	for name, s := range map[string]interface {
		SelectCoins([]*models.Symbols) []*models.Symbols
	}{
		"coin2": TradeCoin2{}, "coin3": TradeCoin3{}, "coin4": TradeCoin4{}, "coin5": TradeCoin5{},
	} {
		if got := s.SelectCoins(coins); len(got) != 0 {
			t.Fatalf("%s: 全部未启用时应返回空, 实际%d个", name, len(got))
		}
	}
}
