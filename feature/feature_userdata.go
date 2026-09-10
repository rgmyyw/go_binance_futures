package feature

import (
	"go_binance_futures/feature/api/binance"
	"go_binance_futures/models"
	"math"
	"strconv"
	"time"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
)


func SyncUserData() {
	deleteOldUserData()
	getNowUserData()
	go SyncStopOrders() // 对账交易所原生止损单
	go func() {
		binance.WsUserData()
	}()
	go func() {
		// 10分钟同步一次用户数据
		for {
			time.Sleep(time.Minute * 30) // 30分钟间隔
			getNowUserData()
			SyncStopOrders()
		}
	}()
}

// 删除数据表旧数据
func deleteOldUserData() {
	o := orm.NewOrm()
	o.Raw("DELETE FROM futures_orders where 1=1 and (status = 'NEW' or status = 'PARTIALLY_FILLED')").Exec() // 只删除未成交的订单
	o.Raw("DELETE FROM futures_positions where 1=1").Exec()
}

// 查询 api 接口获取最新数据
func getNowUserData() {
	o := orm.NewOrm()
	nowTime := time.Now().Unix() * 1000
	
	// positions
	allPositions, err := binance.GetPosition(binance.PositionParams{})
	if err != nil {
		logs.Error("GetPosition err in feature_userdata:", err.Error())
		return
	}
	for _, position := range allPositions {
		positionAmt, _ := strconv.ParseFloat(position.PositionAmt, 64)
		positionAmtFloatAbs := math.Abs(positionAmt) // 空单为负数,纠正为绝对值
		if positionAmtFloatAbs < 0.0000001 {
			continue
		}
		leverage, _ := strconv.ParseInt(position.Leverage, 10, 64)
		
		var positionModel models.FuturesPosition
		o.QueryTable("futures_positions").Filter("symbol", position.Symbol).Filter("side", position.PositionSide).One(&positionModel)
		positionModel.Symbol = position.Symbol
		positionModel.Side = position.PositionSide
		positionModel.Amount = position.PositionAmt
		positionModel.Leverage = leverage
		positionModel.MarginType = position.MarginType
		positionModel.IsolatedWallet = position.IsolatedWallet
		positionModel.EntryPrice = position.EntryPrice
		positionModel.MarkPrice = position.MarkPrice
		positionModel.UnrealizedProfit = position.UnRealizedProfit
		positionModel.AccumulatedRealized = "0"
		positionModel.MaintenanceMarginRequired = "0"
		positionModel.CreateTime = nowTime
		positionModel.UpdateTime = nowTime
		if positionModel.ID == 0 {
			o.Insert(&positionModel)
		} else {
			o.Update(&positionModel)
		}
	}

	// remove local position rows that no longer exist on the exchange,
	// otherwise the trade loop keeps closing a ghost position and gets -2022
	livePositions := map[string]bool{}
	for _, position := range allPositions {
		amt, _ := strconv.ParseFloat(position.PositionAmt, 64)
		if math.Abs(amt) < 0.0000001 {
			continue // 没有持仓的
		}
		livePositions[position.Symbol+"|"+position.PositionSide] = true
	}
	var localPositions []models.FuturesPosition
	if cnt, _ := o.QueryTable("futures_positions").All(&localPositions); cnt > 0 {
		for _, lp := range localPositions {
			if !livePositions[lp.Symbol+"|"+lp.Side] {
				logs.Info("position %s %s not found on exchange, remove stale local record", lp.Symbol, lp.Side)
				o.Delete(&lp)
			}
		}
	}
	
	// open orders
	allOpenOrders, err := binance.GetOpenOrder()
	if err != nil {
		logs.Error("GetOpenOrder err in feature_userdata:", err.Error())
		return
	}
	for _, order := range allOpenOrders {
		var orderModel models.FuturesOrder
		o.QueryTable("futures_orders").Filter("order_id", order.OrderID).One(&orderModel)
		orderModel.Symbol = order.Symbol
		orderModel.ClientOrderId = order.ClientOrderID
		orderModel.OrderId = strconv.FormatInt(order.OrderID, 10)
		orderModel.Side = string(order.Side)
		orderModel.PositionSide = string(order.PositionSide)
		orderModel.Type = string(order.Type)
		orderModel.Status = string(order.Status) // NEW
		orderModel.Price = string(order.Price)
		orderModel.OrigQty = order.OrigQuantity
		orderModel.ExecutedQty = order.ExecutedQuantity
		orderModel.CreateTime = nowTime
		orderModel.UpdateTime = nowTime
		if orderModel.ID == 0 {
			o.Insert(&orderModel)
		} else {
			o.Update(&orderModel)
		}
	}
}
