package line

import (
	"testing"

	"go_binance_futures/feature/strategy"
	"go_binance_futures/models"
)

// 全策略家族的公共防护: nil 币种与 K 线失败路径不得 panic 且不产生信号
func TestLineFamilyNilSymbolsNoSignal(t *testing.T) {
	strategies := map[string]strategy.LineStrategy{
		"line1": TradeLine1{},
		"line2": TradeLine2{},
		"line3": TradeLine3{},
		"line4": TradeLine4{},
		"line7": TradeLine7{},
	}
	for name, s := range strategies {
		res := s.GetCanLongOrShort(strategy.OpenParams{Symbols: nil})
		if res.CanLong || res.CanShort {
			t.Fatalf("%s: nil 币种不应产生信号", name)
		}
	}
}

func TestLineFamilyKlineErrorNoSignal(t *testing.T) {
	defer withKlines(nil, errTest)()
	strategies := map[string]strategy.LineStrategy{
		"line1": TradeLine1{},
		"line2": TradeLine2{},
		"line3": TradeLine3{},
		"line4": TradeLine4{},
		"line7": TradeLine7{},
	}
	for name, s := range strategies {
		res := s.GetCanLongOrShort(strategy.OpenParams{Symbols: &models.Symbols{Symbol: "TESTUSDT"}})
		if res.CanLong || res.CanShort {
			t.Fatalf("%s: K线失败不应产生信号", name)
		}
	}
}
