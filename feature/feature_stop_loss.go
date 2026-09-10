package feature

import (
	"go_binance_futures/feature/api/binance"
	"go_binance_futures/models"
	"go_binance_futures/utils"
	"math"
	"strconv"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
)

// 交易所原生止损: 开仓成功后在币安挂 STOP_MARKET 全平单(closePosition),
// 程序内止损失效(机器人重启/断网/卡死)时由交易所兜底触发。
// 触发价 = 开仓价 × (1 ∓ loss%/100/杠杆), loss 取币种配置的止损 ROI 百分比。

// PlaceSymbolStopLoss 为该仓位在交易所挂止损单
func PlaceSymbolStopLoss(coin *models.Symbols, entryText string, leverage int64, positionSide futures.PositionSideType) {
	if coin == nil || positionSide == "" {
		return
	}
	loss, err := strconv.ParseFloat(coin.Loss, 64)
	if err != nil || loss <= 0 {
		return // 未配置止损阈值, 不挂原生止损
	}
	if leverage <= 0 {
		leverage = int64(coin.Leverage)
	}
	entry, _ := strconv.ParseFloat(entryText, 64)
	if entry <= 0 || leverage <= 0 {
		return
	}
	move := loss / 100 / float64(leverage) // ROI 阈值换算为价格波动幅度
	stopPrice := entry * (1 - move)
	side := futures.SideTypeSell
	if positionSide == futures.PositionSideTypeShort {
		stopPrice = entry * (1 + move)
		side = futures.SideTypeBuy
	}
	stopPrice = utils.GetTradePrecision(stopPrice, coin.TickSize)
	if stopPrice <= 0 {
		return
	}
	if _, err := binance.OrderStopLoss(coin.Symbol, stopPrice, side, positionSide); err != nil {
		logs.Error("%s:place exchange stop loss(%s@%v) failed: %s", coin.Symbol, positionSide, stopPrice, err.Error())
		return
	}
	logs.Info("%s:exchange stop loss placed at %v (%s, roi -%v%%)", coin.Symbol, stopPrice, positionSide, loss)
}

// CancelSymbolStopOrders 仓位平掉后撤销该币种残留的止损单
func CancelSymbolStopOrders(symbol string) {
	orders, err := binance.GetOpenOrder(symbol)
	if err != nil {
		return
	}
	for _, order := range orders {
		if order.Type != "STOP_MARKET" {
			continue
		}
		if _, err := binance.CancelOrder(symbol, order.OrderID); err == nil {
			logs.Info("%s:exchange stop loss cancelled", symbol)
		}
	}
}

// SyncStopOrders 启动/定期对账: 仓位已消失则撤销残留止损单; 仓位缺止损单则按开仓价补挂
func SyncStopOrders() {
	var positions []models.FuturesPosition
	if _, err := orm.NewOrm().QueryTable("futures_positions").All(&positions); err != nil {
		return
	}
	openOrders, err := binance.GetOpenOrder()
	if err != nil {
		logs.Error("SyncStopOrders get open orders failed: %s", err.Error())
		return
	}
	liveSymbols := map[string]bool{}
	for _, p := range positions {
		amt, _ := strconv.ParseFloat(p.Amount, 64)
		if math.Abs(amt) < 0.0000001 {
			continue
		}
		liveSymbols[p.Symbol] = true
	}
	for _, order := range openOrders {
		if order.Type != "STOP_MARKET" {
			continue
		}
		if !liveSymbols[order.Symbol] {
			if _, err := binance.CancelOrder(order.Symbol, order.OrderID); err == nil {
				logs.Info("%s:stale exchange stop loss cancelled", order.Symbol)
			}
		}
	}
	var coins []*models.Symbols
	if _, err := orm.NewOrm().QueryTable("symbols").All(&coins); err != nil {
		return
	}
	coinMap := map[string]*models.Symbols{}
	for _, c := range coins {
		coinMap[c.Symbol] = c
	}
	for _, p := range positions {
		amt, _ := strconv.ParseFloat(p.Amount, 64)
		if math.Abs(amt) < 0.0000001 {
			continue
		}
		side := futures.PositionSideType(p.Side)
		if stopOrderExists(openOrders, p.Symbol, side) {
			continue
		}
		PlaceSymbolStopLoss(coinMap[p.Symbol], p.EntryPrice, p.Leverage, side)
	}
}

func stopOrderExists(orders []*futures.Order, symbol string, side futures.PositionSideType) bool {
	for _, order := range orders {
		if order.Type == "STOP_MARKET" && order.Symbol == symbol && order.PositionSide == side {
			return true
		}
	}
	return false
}
