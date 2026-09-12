package line

import (
	"testing"

	"github.com/beego/beego/v2/client/orm"

	"go_binance_futures/models"
)

type baseCheckRow struct {
	symbol string
	pct    float64
}

// setupSymbolsForBaseCheck 写入指定分布的币种涨跌数据, 测试结束自动清理
func setupSymbolsForBaseCheck(t *testing.T, rows []baseCheckRow) {
	t.Helper()
	setupLineTestDB(t)
	o := orm.NewOrm()
	if _, err := o.Raw("delete from symbols where symbol like 'BC%'").Exec(); err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	for _, r := range rows {
		if _, err := o.Raw("insert into symbols (symbol, enable, percentChange, type, technology, strategy) values (?, 0, ?, 'USDT', '', '')", r.symbol, r.pct).Exec(); err != nil {
			t.Fatalf("插入失败: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = o.Raw("delete from symbols where symbol like 'BC%' or symbol = 'BTCUSDT'").Exec()
	})
}

func TestBaseCheckRiseOver75BlocksShort(t *testing.T) {
	rows := make([]baseCheckRow, 0, 10)
	for i := 0; i < 10; i++ {
		pct := 2.0
		if i < 9 { // 9/10 = 90% 在涨
			pct = 6.0
		}
		rows = append(rows, baseCheckRow{"BCA" + string(rune('A'+i)) + "USDT", pct})
	}
	setupSymbolsForBaseCheck(t, rows)
	canLong, canShort := BaseCheckCanLongOrShort()
	if !canLong {
		t.Fatal("普涨(90%上涨, BTC未涨5%)不应禁多")
	}
	if canShort {
		t.Fatal("90%币种上涨应禁做空")
	}
}

func TestBaseCheckFallWithBtcDropBlocksLong(t *testing.T) {
	rows := make([]baseCheckRow, 0, 10)
	for i := 0; i < 10; i++ {
		pct := 2.0
		if i < 7 { // 70% 在跌: 超过60%
			pct = -6.0
		}
		rows = append(rows, baseCheckRow{"BCB" + string(rune('A'+i)) + "USDT", pct})
	}
	// 最后一行替换为 BTCUSDT 跌 6%: 触发 BTC 联动禁多条件
	rows[9] = baseCheckRow{"BTCUSDT", -6.0}
	setupSymbolsForBaseCheck(t, rows)
	canLong, _ := BaseCheckCanLongOrShort()
	if canLong {
		t.Fatal("60%以上币种下跌且BTC跌超5%应禁做多")
	}
}

func TestBaseCheckBoundaryAt75PercentNoBlock(t *testing.T) {
	// 恰好75%下跌: 未超过75%阈值 → 不禁多
	rows := make([]baseCheckRow, 0, 100)
	for i := 0; i < 100; i++ {
		pct := 2.0
		if i < 75 {
			pct = -2.0
		}
		rows = append(rows, baseCheckRow{"BCC" + string(rune('A'+i%26)) + string(rune('A'+i/26)) + "USDT", pct})
	}
	setupSymbolsForBaseCheck(t, rows)
	canLong, _ := BaseCheckCanLongOrShort()
	if !canLong {
		t.Fatal("恰好75%下跌未超阈值不应禁多")
	}
}

var _ = models.Symbols{}
