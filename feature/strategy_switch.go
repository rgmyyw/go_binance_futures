package feature

import (
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
	strategy.SetRegimeCondition(systemConfig.MarketCondition)

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
	if class == 1 {
		return "line6", class, -1, true
	}
	return "line5", class, -1, true
}
