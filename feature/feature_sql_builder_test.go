package feature

import (
	"strings"
	"testing"
)

func TestBuildBatchUpdateSymbolsTradePrecisionSQL(t *testing.T) {
	// 空输入
	q, args := buildBatchUpdateSymbolsTradePrecisionSQL(nil)
	if q != "" || args != nil {
		t.Fatal("空输入应返回空SQL")
	}
	items := []futuresSymbolPrecisionUpdate{
		{Symbol: "AUSDT", TickSize: "0.01", StepSize: "0.001", Type: "USDT"},
		{Symbol: "BUSDT", TickSize: "0.1", StepSize: "1", Type: "USDT"},
	}
	q, args = buildBatchUpdateSymbolsTradePrecisionSQL(items)
	if q == "" {
		t.Fatal("SQL不应为空")
	}
	if len(args) != 8 { // 每项4个参数
		t.Fatalf("参数应为8个, 实际%d个", len(args))
	}
	if n := strings.Count(q, "SELECT ? AS symbol"); n != 2 {
		t.Fatalf("应包含2个SELECT段, 实际%d个: %s", n, q)
	}
	if !strings.Contains(q, "UPDATE") || !strings.Contains(q, "symbols") {
		t.Fatalf("应为UPDATE语句: %s", q)
	}
}

func TestBuildBatchInsertFuturesSymbolsSQL(t *testing.T) {
	q, args := buildBatchInsertFuturesSymbolsSQL(nil)
	if q != "" || args != nil {
		t.Fatal("空输入应返回空SQL")
	}
	items := []futuresSymbolInsert{
		{Symbol: "AUSDT", Enable: 1, Leverage: 3, TickSize: "0.01", StepSize: "0.001", Usdt: "10", Profit: "3", Loss: "5", UpdateTime: 123},
	}
	q, args = buildBatchInsertFuturesSymbolsSQL(items)
	if q == "" {
		t.Fatal("SQL不应为空")
	}
	if len(args) != 28 { // 每项28列
		t.Fatalf("参数应为28个, 实际%d个", len(args))
	}
	if n := strings.Count(q, "?"); n != 28 {
		t.Fatalf("占位符应为28个, 实际%d个", n)
	}
	if !strings.Contains(q, "INSERT") {
		t.Fatalf("应为INSERT语句: %s", q)
	}
}
