package feature

import (
	"sync"
	"time"

	"go_binance_futures/feature/strategy"
	"go_binance_futures/models"
	"go_binance_futures/types"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
)

// 行情自适应策略切换: 根据 market_condition 自动在趋势策略(line5)与震荡策略(line6)间切换
// 由 main.go 定时调用(每 10 分钟), 行情类型需连续两次判定一致才切换(防抖)
// market_condition 定义见 types/market_condition.go

var regimeLastClass = -1 // 上一次生效的行情类型: 0=趋势, 1=震荡; -1=未初始化
var regimePendingClass = -1

func regimeClassByCondition(condition int) int {
	switch condition {
	case types.MarketConditionSideways,
		types.MarketConditionBullishDivergence,
		types.MarketConditionBearishDivergence,
		types.MarketConditionHighVolatility,
		types.MarketConditionLowVolatility:
		return 1 // 震荡类
	default:
		return 0 // 趋势类(强多/偏多/偏空/强空/普涨/普跌)
	}
}

func AutoSwitchStrategyByMarket() {
	o := orm.NewOrm()
	var systemConfig models.Config
	err := o.QueryTable("config").OrderBy("id").Limit(1).One(&systemConfig)
	if err != nil {
		logs.Error("AutoSwitchStrategyByMarket read config err:", err.Error())
		return
	}
	// 同步行情状态给方向闸门(RegimeAllowsLong/Short), 供 line5/line6 过滤逆势信号。
	// 放在各早退判断之前, 保证即使策略切换被停用, 方向闸门仍然生效
	syncRegimeGate(systemConfig.MarketCondition)

	if systemConfig.FutureEnable != 1 {
		return
	}
	if systemConfig.MarketConditionIsAuto != 1 {
		// 手动指定行情状态时不动策略
		return
	}
	// 仅在这两个策略间自动切换, 用户手选了其它策略(如 line4/custom)则尊重不动
	if systemConfig.FutureStrategyTrade != "line5" && systemConfig.FutureStrategyTrade != "line6" {
		return
	}

	class := regimeClassByCondition(systemConfig.MarketCondition)
	if regimeLastClass == -1 {
		// 启动初始化: 直接对齐策略与当前行情类型
		// (若只在状态里静默记录, 防抖等待期间进程重启会丢失迁移, 策略将卡在旧档位)
		regimeLastClass = class
		regimePendingClass = -1
		want := "line5"
		if class == 1 {
			want = "line6"
		}
		if systemConfig.FutureStrategyTrade != want {
			if _, err := o.QueryTable("config").Filter("id", systemConfig.ID).Update(orm.Params{
				"future_strategy_trade": want,
			}); err != nil {
				logs.Error("AutoSwitchStrategyByMarket init align err:", err.Error())
				return
			}
			logs.Info("auto switch strategy by market condition %s: %s -> %s (startup align)",
				types.MarketConditionName(systemConfig.MarketCondition), systemConfig.FutureStrategyTrade, want)
		}
		return
	}
	target, newLast, newPending, shouldSwitch := regimeDecideNext(class, regimeLastClass, regimePendingClass)
	if !shouldSwitch {
		regimeLastClass, regimePendingClass = newLast, newPending
		if newPending != -1 {
			// 第一次看到新类型, 等下一次确认
			logs.Info("market regime changed to %s, waiting confirm before switch strategy",
				types.MarketConditionName(systemConfig.MarketCondition))
		}
		return
	}

	_, err = o.QueryTable("config").Filter("id", systemConfig.ID).Update(orm.Params{
		"future_strategy_trade": target,
	})
	if err != nil {
		logs.Error("AutoSwitchStrategyByMarket update err:", err.Error())
		return
	}
	regimeLastClass, regimePendingClass = newLast, newPending
	logs.Info("auto switch strategy by market condition %s: %s -> %s",
		types.MarketConditionName(systemConfig.MarketCondition), systemConfig.FutureStrategyTrade, target)
}

// syncRegimeGate 行情状态同步给方向闸门(独立于策略切换开关)
func syncRegimeGate(condition int) {
	strategy.SetRegimeCondition(condition)
}

// regimeDecideNext 防抖状态机(纯函数, 便于单测):
// class=本次判定(0趋势/1震荡), last=上次生效类型(-1未初始化), pending=待确认类型(-1无)
// 返回: 目标策略, 新last, 新pending, 是否切换
func regimeDecideNext(class, last, pending int) (target string, newLast, newPending int, shouldSwitch bool) {
	if last == -1 {
		return "", class, -1, false
	}
	if class == last {
		return "", last, -1, false
	}
	if pending != class {
		// 第一次看到新类型, 等下一次确认
		return "", last, class, false
	}
	if class == 1 && line6Allowed() {
		return "line6", class, -1, true
	}
	return "line5", class, -1, true
}

// ===== line6 业绩降级护栏 =====
// line6 实盘连续止损达到阈值时自动降回 line5 并冷却 12 小时, 防止均值回归在错误行情下持续失血
// (2026-09-12 实盘教训: 多头分化行情下买跌 10 笔 4 胜 6 负 -3.15U, 当晚无反馈机制只能人工止损)
const (
	line6LossStreakTrigger  = 3
	line6DemoteCooldownHour = 12
)

var (
	line6LossStreak     int
	line6DemoteUntilMs  int64
	strategyGuardMu     sync.Mutex
)

// line6Allowed 冷却期内不允许选择 line6
func line6Allowed() bool {
	strategyGuardMu.Lock()
	defer strategyGuardMu.Unlock()
	return time.Now().UnixMilli() >= line6DemoteUntilMs
}

// RecordStopLossForStrategy 由止损平仓路径调用: 累计当前策略(line6)连亏并自动降级
// 只对 line6 生效(line5 连亏已有熔断与时间止损兜底)
func RecordStopLossForStrategy(activeStrategy string) {
	if activeStrategy != "line6" {
		return
	}
	strategyGuardMu.Lock()
	line6LossStreak++
	streak := line6LossStreak
	strategyGuardMu.Unlock()
	if streak >= line6LossStreakTrigger {
		DemoteLine6()
	}
}

// DemoteLine6 立即降级: 策略切回 line5(数据库) 并冷却 12 小时
func DemoteLine6() {
	strategyGuardMu.Lock()
	line6DemoteUntilMs = time.Now().Add(line6DemoteCooldownHour * time.Hour).UnixMilli()
	line6LossStreak = 0
	strategyGuardMu.Unlock()
	o := orm.NewOrm()
	if _, err := o.QueryTable("config").Filter("future_strategy_trade", "line6").Update(orm.Params{
		"future_strategy_trade": "line5",
	}); err != nil {
		logs.Error("DemoteLine6 update config err:", err.Error())
	}
	logs.Warning("line6 demoted to line5: %d consecutive stop losses, cooldown %dh", line6LossStreakTrigger, line6DemoteCooldownHour)
}
