package feature

import (
	"sync"
	"time"

	"github.com/beego/beego/v2/core/logs"
)

var strategyLogLast sync.Map

// 策略等待信号日志: 同一币种 10 分钟最多记录一次, 避免每轮扫描刷屏
func logStrategyOnce(symbol string, msg string) {
	if msg == "" {
		msg = "no trading strategy conditions passed"
	}
	now := time.Now()
	if v, ok := strategyLogLast.Load(symbol); ok {
		if t, ok2 := v.(time.Time); ok2 && now.Sub(t) < 10*time.Minute {
			return
		}
	}
	strategyLogLast.Store(symbol, now)
	logs.Info("%s:%s", symbol, msg)
}
