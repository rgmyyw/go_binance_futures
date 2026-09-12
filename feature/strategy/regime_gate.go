package strategy

import "sync/atomic"

// 行情方向闸门: 由 feature.AutoSwitchStrategyByMarket 在读取 market_condition 后同步写入。
// 趋势类行情里逆势开仓是实盘胜率的主要损耗来源(普涨期连续做空亏损),
// line5/line6 的开仓信号在方向上先过这道闸。
// 值为 0(未初始化)时不限制, 保持安全默认。

var regimeConditionValue atomic.Int64

func SetRegimeCondition(condition int) {
	regimeConditionValue.Store(int64(condition))
}

func RegimeAllowsLong() bool {
	switch int(regimeConditionValue.Load()) {
	case 4, 5, 9: // 偏空/强空/普跌 禁开多
		return false
	}
	return true
}

func RegimeAllowsShort() bool {
	switch int(regimeConditionValue.Load()) {
	case 1, 2, 8: // 强多/偏多/普涨 禁开空
		return false
	}
	return true
}
