package feature

import "testing"

func TestGetExcludeSymbolsMap(t *testing.T) {
	m := GetExcludeSymbolsMap("BTCUSDT,ETHUSDT , BNBUSDT")
	if len(m) != 3 {
		t.Fatalf("应解析出3个白名单币, 实际%d个: %v", len(m), m)
	}
	for _, s := range []string{"BTCUSDT", "ETHUSDT ", " BNBUSDT"} {
		if !m[s] {
			t.Fatalf("币种 %q 应在白名单中", s)
		}
	}
	if m["XRPUSDT"] {
		t.Fatal("未列出的币不应在白名单")
	}
	// 空串: 只有一个空键, 不会误伤真实币种
	empty := GetExcludeSymbolsMap("")
	if empty["BTCUSDT"] {
		t.Fatal("空白名单不应包含任何币")
	}
}
