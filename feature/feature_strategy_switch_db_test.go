package feature

import (
	"os"
	"sync"
	"testing"

	"github.com/beego/beego/v2/client/orm"
	_ "github.com/mattn/go-sqlite3"

	"go_binance_futures/feature/strategy"
	"go_binance_futures/models"
	"go_binance_futures/types"
)

var testDBOnce sync.Once
var testDBErr error

func setupTestDB(t *testing.T) {
	t.Helper()
	testDBOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gbf-test-db")
		if err != nil {
			testDBErr = err
			return
		}
		if err := orm.RegisterDataBase("default", "sqlite3", dir+"/test.db"); err != nil {
			testDBErr = err
			return
		}
		_ = orm.RegisterDriver("sqlite3", orm.DRSqlite)
		orm.RegisterModel(new(models.Config), new(models.TestStrategyResults), new(models.Order))
		if err := orm.RunSyncdb("default", false, false); err != nil {
			testDBErr = err
		}
	})
	if testDBErr != nil {
		t.Fatalf("测试库初始化失败: %v", testDBErr)
	}
}

func upsertConfigRow(t *testing.T, strategyTrade string, condition int, isAuto int, futureEnable int) {
	t.Helper()
	o := orm.NewOrm()
	cfg := models.Config{ID: 1}
	if _, _, err := o.ReadOrCreate(&cfg, "ID"); err != nil {
		t.Fatal(err)
	}
	cfg.FutureStrategyTrade = strategyTrade
	cfg.MarketCondition = condition
	cfg.MarketConditionIsAuto = isAuto
	cfg.FutureEnable = futureEnable
	if _, err := o.Update(&cfg); err != nil {
		t.Fatal(err)
	}
}

func readConfigStrategy(t *testing.T) string {
	t.Helper()
	var s string
	if err := orm.NewOrm().Raw("select future_strategy_trade from config where id=1").QueryRow(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func resetSwitchState() {
	regimeLastClass = -1
	regimePendingClass = -1
	strategyGuardMu.Lock()
	line6LossStreak = 0
	line6DemoteUntilMs = 0
	strategyGuardMu.Unlock()
}

func TestAutoSwitchStrategyByMarketEndToEnd(t *testing.T) {
	setupTestDB(t)
	upsertConfigRow(t, "line5", types.MarketConditionBroadRise, 1, 1)
	resetSwitchState()

	// 首次调用: 初始化状态, 不切换
	AutoSwitchStrategyByMarket()
	if got := readConfigStrategy(t); got != "line5" {
		t.Fatalf("首启不应切换, got %s", got)
	}

	// 行情变震荡: 第一次只登记
	upsertConfigRow(t, "line5", types.MarketConditionSideways, 1, 1)
	AutoSwitchStrategyByMarket()
	if got := readConfigStrategy(t); got != "line5" {
		t.Fatalf("防抖期间不应切换, got %s", got)
	}
	// 第二次确认 → 切到 line6
	AutoSwitchStrategyByMarket()
	if got := readConfigStrategy(t); got != "line6" {
		t.Fatalf("确认后应切换到line6, got %s", got)
	}

	// 行情回趋势: 防抖两拍 → 切回 line5
	upsertConfigRow(t, "line6", types.MarketConditionBroadRise, 1, 1)
	AutoSwitchStrategyByMarket()
	AutoSwitchStrategyByMarket()
	if got := readConfigStrategy(t); got != "line5" {
		t.Fatalf("趋势确认后应切回line5, got %s", got)
	}
}

func TestAutoSwitchRespectsManualConfig(t *testing.T) {
	setupTestDB(t)
	resetSwitchState()

	// 行情判定为手动: 不切换
	upsertConfigRow(t, "line5", types.MarketConditionSideways, 0, 1)
	AutoSwitchStrategyByMarket()
	AutoSwitchStrategyByMarket()
	if got := readConfigStrategy(t); got != "line5" {
		t.Fatalf("手动行情模式不应切换, got %s", got)
	}

	// 用户手选 line4: 切换器让位
	upsertConfigRow(t, "line4", types.MarketConditionSideways, 1, 1)
	AutoSwitchStrategyByMarket()
	AutoSwitchStrategyByMarket()
	if got := readConfigStrategy(t); got != "line4" {
		t.Fatalf("用户手选策略不应被覆盖, got %s", got)
	}
}

func TestAutoSwitchStartupAlignsStrategyAfterRestart(t *testing.T) {
	setupTestDB(t)
	// 模拟重启: 状态清零; 库中行情为震荡但策略仍是 line5(旧进程防抖期被杀的场景)
	resetSwitchState()
	upsertConfigRow(t, "line5", types.MarketConditionSideways, 1, 1)
	AutoSwitchStrategyByMarket() // 首次调用即应对齐到 line6, 而非静默记录
	if got := readConfigStrategy(t); got != "line6" {
		t.Fatalf("重启后首次调用应对齐策略到line6, got %s", got)
	}
}

func TestAutoSwitchGateSyncAlwaysRuns(t *testing.T) {
	setupTestDB(t)
	resetSwitchState()

	// 即使行情为手动, 方向闸门也应同步(独立生效)
	upsertConfigRow(t, "line5", types.MarketConditionBroadDecline, 0, 1)
	AutoSwitchStrategyByMarket()
	if strategy.RegimeAllowsLong() {
		t.Fatal("普跌行情闸门应禁多(即使行情判定为手动)")
	}

	// 合约开关关闭: 闸门仍同步, 策略不动
	upsertConfigRow(t, "line5", types.MarketConditionBroadRise, 1, 0)
	AutoSwitchStrategyByMarket()
	if strategy.RegimeAllowsShort() {
		t.Fatal("普涨行情闸门应禁空(即使合约开关关闭)")
	}
	syncRegimeGate(0)
}
