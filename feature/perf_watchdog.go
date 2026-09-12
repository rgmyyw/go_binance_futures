package feature

import (
	"fmt"
	"sync"
	"time"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"

	"go_binance_futures/notify"
)

// 绩效看门狗: 每逢累计平仓笔数跨过 50 的倍数, 推送最近50笔的胜率/期望报告,
// 作为仓位升降级的决策依据(仅报告, 不自动改仓, 升降级需人工确认)
const (
	perfWatchdogWindow = 50
	perfGoodEv         = 0.10 // 每笔期望≥0.10%: 可考虑升仓
	perfBadEv          = 0.0  // 每笔期望≤0: 建议降仓观察
)

var (
	perfMu                sync.Mutex
	perfLastReportedBatch = -1
)

func StartPerfWatchdog() {
	go func() {
		ticker := time.NewTicker(time.Minute * 30)
		defer ticker.Stop()
		for range ticker.C {
			PerfWatchdogTick()
		}
	}()
	logs.Info("perf watchdog start")
}

func PerfWatchdogTick() {
	o := orm.NewOrm()
	var total int64
	if err := o.Raw("select count(*) from futures_orders where status='FILLED' and realized_pnl != '0' and realized_pnl != ''").QueryRow(&total); err != nil {
		logs.Error("perf watchdog count:", err.Error())
		return
	}
	batch := int(total) / perfWatchdogWindow
	perfMu.Lock()
	shouldReport := batch > 0 && batch > perfLastReportedBatch
	if shouldReport {
		perfLastReportedBatch = batch
	}
	perfMu.Unlock()
	if !shouldReport {
		return
	}

	type pnlRow struct {
		Pnl float64
	}
	var rows []pnlRow
	if _, err := o.Raw("select cast(realized_pnl as real) as pnl from futures_orders where status='FILLED' and realized_pnl != '0' and realized_pnl != '' order by updateTime desc limit ?", perfWatchdogWindow).QueryRows(&rows); err != nil {
		logs.Error("perf watchdog rows:", err.Error())
		return
	}
	if len(rows) == 0 {
		return
	}
	var sum float64
	wins := 0
	for _, r := range rows {
		sum += r.Pnl
		if r.Pnl > 0 {
			wins++
		}
	}
	n := float64(len(rows))
	wr := float64(wins) / n * 100
	ev := sum / n

	var advice string
	switch {
	case ev >= perfGoodEv:
		advice = "期望达标, 可考虑上调单仓金额(建议幅度+50%, 并观察50笔)"
	case ev <= perfBadEv:
		advice = "期望为负, 建议降仓一半观察, 或暂停等待策略校准"
	default:
		advice = "期望为正但未达升仓标准, 维持当前仓位继续观察"
	}

	content := fmt.Sprintf(`
## 绩效报告(最近%d笔)
#### 胜率：<font color="#008000">%.1f%%</font>
#### 累计盈亏：<font color="#008000">%.2f USDT</font>
#### 期望/笔：<font color="#008000">%.3f%%</font>(名义金额百分比)
#### 建议：%s`, len(rows), wr, sum, ev, advice)

	alertPusher := notify.GetNotifyChannel().SetModuleName("futures_perf")
	switch p := alertPusher.(type) {
	case notify.DingDing:
		notify.DingDingApi(content, p)
	case notify.Slack:
		notify.SlackApi(content, p)
	default:
		alertPusher.FuturesPriceChangeNotice(notify.FuturesNoticeParams{
			Symbol:        "PERF",
			Title:         " 绩效报告",
			ChangePercent: ev,
			Price:         sum,
		})
	}
	logs.Info("perf watchdog reported: batch=%d wr=%.1f%% ev=%.3f%%", batch, wr, ev)
}
