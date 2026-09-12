package feature

import (
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
		regimeLastClass = class
		regimePendingClass = -1
		return
	}
	if class == regimeLastClass {
		regimePendingClass = -1
		return
	}
	if regimePendingClass != class {
		// 第一次看到新类型, 等下一次确认
		regimePendingClass = class
		logs.Info("market regime changed to %s, waiting confirm before switch strategy",
			types.MarketConditionName(systemConfig.MarketCondition))
		return
	}

	target := "line5"
	if class == 1 {
		target = "line6"
	}
	_, err = o.QueryTable("config").Filter("id", systemConfig.Id).Update(orm.Params{
		"future_strategy_trade": target,
	})
	if err != nil {
		logs.Error("AutoSwitchStrategyByMarket update err:", err.Error())
		return
	}
	regimeLastClass = class
	regimePendingClass = -1
	logs.Info("auto switch strategy by market condition %s: %s -> %s",
		types.MarketConditionName(systemConfig.MarketCondition), systemConfig.FutureStrategyTrade, target)
}
