package strategy

import "sync/atomic"

// 方向闸门: 由确定性规则(币种涨跌广度 + BTC 联动)驱动, 不依赖 AI 行情判定。
// 语义: 普跌禁多、普涨禁空, 中性市场双向放行。
// 默认放行(安全默认, 未初始化时不拦截交易)。

var (
	gateAllowLong  atomic.Bool
	gateAllowShort atomic.Bool
)

func init() {
	gateAllowLong.Store(true)
	gateAllowShort.Store(true)
}

// SetDirectionGate 规则化方向闸门写入
func SetDirectionGate(allowLong, allowShort bool) {
	gateAllowLong.Store(allowLong)
	gateAllowShort.Store(allowShort)
}

// SetRegimeCondition 兼容入口: 按行情状态码推导闸门(旧 AI 判定链路, 当前未接线)
func SetRegimeCondition(condition int) {
	switch condition {
	case 1, 2, 8: // 强多/偏多/普涨: 禁空
		SetDirectionGate(true, false)
	case 4, 5, 9: // 强空/偏空/普跌: 禁多
		SetDirectionGate(false, true)
	default:
		SetDirectionGate(true, true)
	}
}

func RegimeAllowsLong() bool {
	return gateAllowLong.Load()
}

func RegimeAllowsShort() bool {
	return gateAllowShort.Load()
}
