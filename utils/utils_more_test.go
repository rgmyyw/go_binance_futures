package utils

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/beego/beego/v2/client/orm"
	_ "github.com/mattn/go-sqlite3"

	"go_binance_futures/models"
)

func TestIntervals(t *testing.T) {
	intervals := Intervals()
	if len(intervals) != 15 {
		t.Fatalf("应支持15个周期, 实际%d个", len(intervals))
	}
	found := map[string]bool{}
	for _, v := range intervals {
		found[v] = true
	}
	for _, want := range []string{"1m", "15m", "1h", "4h", "1d", "1M"} {
		if !found[want] {
			t.Fatalf("缺少周期 %s", want)
		}
	}
}

func TestEscapeJSONPassthroughOnSQLite(t *testing.T) {
	// 默认驱动非 mysql → 原样返回
	in := "line1\nline2"
	if got := EscapeJSON(in); got != in {
		t.Fatalf("非mysql驱动应原样返回, got %q", got)
	}
}

func TestToJson(t *testing.T) {
	if got := ToJson(map[string]int{"a": 1}); got != `{"a":1}` {
		t.Fatalf("ToJson 结果错误: %s", got)
	}
	if got := ToJson(make(chan int)); got != "" {
		t.Fatalf("不可序列化应返回空串, got %q", got)
	}
}

func TestResJson(t *testing.T) {
	res := ResJson(200, map[string]interface{}{"k": "v"})
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatal("应返回map")
	}
	if m["code"] != 200 || m["msg"] != "success" {
		t.Fatalf("默认成功消息错误: %v", m)
	}
	res = ResJson(500, nil, "boom")
	m = res.(map[string]interface{})
	if m["code"] != 500 || m["msg"] != "boom" {
		t.Fatalf("自定义消息错误: %v", m)
	}
}

func TestFuturesSymbolType(t *testing.T) {
	cases := []struct {
		symbol, quote, want string
	}{
		{"BTCUSDT", "", "USDT"},
		{"BTCUSDT", "usdc", "USDC"},
		{"ETHFDUSD", "", "FDUSD"},
		{"anything", "fdusd", "FDUSD"},
		{"SP500", "", ""},
	}
	for _, c := range cases {
		if got := FuturesSymbolType(c.symbol, c.quote); got != c.want {
			t.Errorf("FuturesSymbolType(%q,%q)=%q, want %q", c.symbol, c.quote, got, c.want)
		}
	}
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken("admin", 1)
	if err != nil || token == "" {
		t.Fatalf("GenerateToken 应成功: %v %q", err, token)
	}
}

// GetSystemConfig 需要数据库别名
var utilDBOnce sync.Once
var utilDBErr error

func setupUtilTestDB(t *testing.T) {
	t.Helper()
	utilDBOnce.Do(func() {
		dir, err := os.MkdirTemp("", "utils-test-db")
		if err != nil {
			utilDBErr = err
			return
		}
		if err := orm.RegisterDataBase("default", "sqlite3", filepath.Join(dir, "test.db")); err != nil {
			utilDBErr = err
			return
		}
		orm.RegisterModel(new(models.Config))
		if err := orm.RunSyncdb("default", false, false); err != nil {
			utilDBErr = err
		}
	})
	if utilDBErr != nil {
		t.Fatalf("utils 测试库初始化失败: %v", utilDBErr)
	}
}

func TestGetSystemConfig(t *testing.T) {
	setupUtilTestDB(t)
	o := orm.NewOrm()
	cfg := models.Config{ID: 1, FutureEnable: 1, LossMaxCount: 7}
	if _, err := o.Insert(&cfg); err != nil {
		t.Fatal(err)
	}
	got, err := GetSystemConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.FutureEnable != 1 || got.LossMaxCount != 7 {
		t.Fatalf("GetSystemConfig 字段不匹配: %+v", got)
	}
}
