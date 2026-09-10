package feature

import (
	"sync"
	"time"
)

// 开仓失败冷却: 同一币种同一方向下单失败后, 冷却时间内不再尝试开仓,
// 避免持续性失败(保证金不足/最小数量不足等)每轮循环都重试打爆 API
const orderFailRetryCooldown = 5 * time.Minute

var openFailCooldown sync.Map // "SYMBOL|SIDE" -> 冷却截止的 unix 时间

func openOnCooldown(symbol string, side string) bool {
	key := symbol + "|" + side
	if v, ok := openFailCooldown.Load(key); ok {
		if time.Now().Unix() < v.(int64) {
			return true
		}
		openFailCooldown.Delete(key)
	}
	return false
}

func markOpenFail(symbol string, side string) {
	openFailCooldown.Store(symbol+"|"+side, time.Now().Add(orderFailRetryCooldown).Unix())
}
