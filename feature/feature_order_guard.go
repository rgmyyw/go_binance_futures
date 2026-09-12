package feature

import (
	"sync"
	"time"

	"github.com/beego/beego/v2/core/logs"
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

// 止损冷却: 某币种某方向止损平仓后, 冷却时间内不再开同方向仓。
// 解决震荡期同币反复接刀: 止损说明该方向短期判断错误, 立即重进大概率重复同样的亏损;
// 反方向不受影响, 冷却过期后趋势仍在时可正常重进
const lossReentryCooldown = 30 * time.Minute

var lossCooldown sync.Map // "SYMBOL|SIDE" -> 冷却截止的 unix 时间

func lossCooldownActive(symbol string, side string) bool {
	key := symbol + "|" + side
	if v, ok := lossCooldown.Load(key); ok {
		if time.Now().Unix() < v.(int64) {
			return true
		}
		lossCooldown.Delete(key)
	}
	return false
}

func markLossCooldown(symbol string, side string) {
	lossCooldown.Store(symbol+"|"+side, time.Now().Add(lossReentryCooldown).Unix())
}

// 震荡熔断: 30 分钟内全局发生 3 次止损平仓 → 暂停开新仓 30 分钟。
// 行情转震荡而策略还是趋势单时会连续吃小止损, 熔断让机器人等到 LLM 行情判定
// 切换完成后再进场, 避免在错误的市场状态下持续消耗(今日实测: 一段震荡期连亏 8 笔)
const (
	chopLossWindow   = 30 * time.Minute
	chopLossTrigger  = 3
	chopPauseMinutes = 30 * time.Minute
)

var (
	lossEventTimes   []int64
	lossEventMu      sync.Mutex
	chopPauseUntil   int64
	chopLoggedActive bool
)

func recordStopLossEvent() {
	now := time.Now().UnixMilli()
	lossEventMu.Lock()
	defer lossEventMu.Unlock()
	lossEventTimes = append(lossEventTimes, now)
	windowStart := now - chopLossWindow.Milliseconds()
	trimmed := lossEventTimes[:0]
	for _, t := range lossEventTimes {
		if t >= windowStart {
			trimmed = append(trimmed, t)
		}
	}
	lossEventTimes = trimmed
	if int64(len(lossEventTimes)) >= chopLossTrigger {
		lossEventTimes = nil
		chopPauseUntil = now + chopPauseMinutes.Milliseconds()
		if !chopLoggedActive {
			logs.Warning("chop circuit breaker: %d stop losses in %v, pause opening %v", chopLossTrigger, chopLossWindow, chopPauseMinutes)
			chopLoggedActive = true
		}
	}
}

func chopBreakerActive() bool {
	lossEventMu.Lock()
	defer lossEventMu.Unlock()
	if time.Now().UnixMilli() < chopPauseUntil {
		return true
	}
	chopLoggedActive = false
	return false
}
