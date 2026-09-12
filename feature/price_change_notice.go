package feature

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	agentevent "go_binance_futures/agent/event"
	"go_binance_futures/models"
	"go_binance_futures/notify"
	"go_binance_futures/utils"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
)

// ws_futures_price_change_limit 的实现(原设置只有配置项没有逻辑):
// 基于 futures 全市场 ws 价格流, 对启用交易的币种监控自上次提醒以来的累计涨跌幅,
// 超过阈值(config.ws_futures_price_change_limit, 百分比, 0=关闭)推送通知;
// 同币种冷却 10 分钟, 触发后参考价重置为当前价

const priceChangeNoticeCooldown = 10 * time.Minute
const priceChangeConfigRefresh = time.Minute

type priceChangeEntry struct {
	mu          sync.Mutex
	refPrice    float64
	lastAlertAt int64 // unix ms
}

type priceChangeSnapshot struct {
	wsEnabled bool
	limit     float64
	watch     map[string]bool
}

var priceChangeStates sync.Map   // symbol -> *priceChangeEntry
var priceChangeSnap atomic.Value // priceChangeSnapshot

// StartPriceChangeNotice 订阅 ws 价格事件流, 启动价格变动提醒
func StartPriceChangeNotice() {
	priceChangeSnap.Store(priceChangeSnapshot{watch: map[string]bool{}})
	err := agentevent.DefaultBus().Subscribe(agentevent.TypePriceTick, handlePriceChangeTick)
	if err != nil {
		logs.Error("subscribe price tick for price change notice:", err)
		return
	}
	refreshPriceChangeSnapshot()
	go func() {
		for {
			time.Sleep(priceChangeConfigRefresh)
			refreshPriceChangeSnapshot()
		}
	}()
	logs.Info("price change notice bot start")
}

func handlePriceChangeTick(evt agentevent.Event) {
	tick, ok := evt.(agentevent.PriceTickEvent)
	if !ok {
		return
	}
	snap := priceChangeSnap.Load().(priceChangeSnapshot)
	if !snap.wsEnabled || snap.limit <= 0 {
		return
	}
	symbol := strings.TrimSpace(tick.Meta.Symbol)
	if symbol == "" || !snap.watch[symbol] {
		return
	}
	nowMs := time.Now().UnixMilli()
	value, _ := priceChangeStates.LoadOrStore(symbol, &priceChangeEntry{refPrice: tick.Price})
	entry := value.(*priceChangeEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if nowMs-entry.lastAlertAt < priceChangeNoticeCooldown.Milliseconds() {
		return
	}
	if entry.refPrice <= 0 {
		entry.refPrice = tick.Price
		return
	}
	changePct := (tick.Price - entry.refPrice) / entry.refPrice * 100
	if changePct < snap.limit && changePct > -snap.limit {
		return
	}
	pusher.SetModuleName("futures").FuturesPriceChangeNotice(notify.FuturesNoticeParams{
		Title:         " 价格变动提醒",
		Symbol:        symbol,
		Price:         tick.Price,
		ChangePercent: changePct,
		Status:        "success",
	})
	entry.lastAlertAt = nowMs
	entry.refPrice = tick.Price
}

// 每 1 分钟刷新: 开关/阈值 + 启用币种集合(改配置无需重启)
func refreshPriceChangeSnapshot() {
	systemConfig, err := utils.GetSystemConfig()
	if err != nil {
		logs.Error("price change notice load config:", err)
		return
	}
	snap := priceChangeSnapshot{
		wsEnabled: systemConfig.WsFuturesEnable == 1,
		limit:     float64(systemConfig.WsFuturesPriceChangeLimit),
		watch:     map[string]bool{},
	}
	var coins []*models.Symbols
	if _, err := orm.NewOrm().QueryTable("symbols").Filter("enable", 1).All(&coins); err != nil {
		logs.Error("load enabled symbols for price change notice:", err)
		return
	}
	for _, coin := range coins {
		if s := strings.TrimSpace(coin.Symbol); s != "" {
			snap.watch[s] = true
		}
	}
	priceChangeSnap.Store(snap)
}
