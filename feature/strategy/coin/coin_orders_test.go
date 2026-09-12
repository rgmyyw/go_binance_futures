package coin

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/beego/beego/v2/client/orm"
	_ "github.com/mattn/go-sqlite3"

	"go_binance_futures/feature/api/binance"
	"go_binance_futures/models"
)

// getLimitMinOrder(经由 listOrders 接缝): 汇总订单币种
func TestGetLimitMinOrderAggregatesSymbols(t *testing.T) {
	prev := listOrders
	listOrders = func(params binance.ListOrderParams) ([]*futures.Order, error) {
		return []*futures.Order{{Symbol: "AUSDT"}, {Symbol: "BUSDT"}, {Symbol: "AUSDT"}}, nil
	}
	defer func() { listOrders = prev }()

	got := getLimitMinOrder(5)
	if !got["AUSDT"] || !got["BUSDT"] {
		t.Fatalf("应汇总订单币种: %v", got)
	}
	if got["CUSDT"] {
		t.Fatal("未出现在订单中的币不应存在")
	}
}

// getLimitMinLocalOrder: 本地订单库, 仅统计 close 单
func TestGetLimitMinLocalOrderCloseOnly(t *testing.T) {
	setupCoinTestDB(t)
	o := orm.NewOrm()
	now := timeNowMs()
	_, _ = o.Raw("insert into `order` (order_id, symbol, side, updateTime) values (1, 'CLSUSDT', 'close', ?)", now).Exec()
	_, _ = o.Raw("insert into `order` (order_id, symbol, side, updateTime) values (2, 'OPNUSDT', 'open', ?)", now).Exec()
	_, _ = o.Raw("insert into `order` (order_id, symbol, side, updateTime) values (3, 'OLDUSDT', 'close', ?)", now-10*60*1000).Exec()

	got := getLimitMinLocalOrder(5)
	if !got["CLSUSDT"] {
		t.Fatal("5分钟内的close单应被统计")
	}
	if got["OPNUSDT"] {
		t.Fatal("open单不应被统计")
	}
	if got["OLDUSDT"] {
		t.Fatal("超时close单不应被统计")
	}
}

var coinDBOnce sync.Once
var coinDBErr error

func setupCoinTestDB(t *testing.T) {
	t.Helper()
	coinDBOnce.Do(func() {
		dir, err := os.MkdirTemp("", "coin-test-db")
		if err != nil {
			coinDBErr = err
			return
		}
		if err := orm.RegisterDataBase("default", "sqlite3", dir+"/test.db"); err != nil {
			coinDBErr = err
			return
		}
		orm.RegisterModel(new(models.Order))
		if err := orm.RunSyncdb("default", false, false); err != nil {
			coinDBErr = err
		}
	})
	if coinDBErr != nil {
		t.Fatalf("coin 测试库初始化失败: %v", coinDBErr)
	}
}

func timeNowMs() int64 { return time.Now().UnixMilli() }
