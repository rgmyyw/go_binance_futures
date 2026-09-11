package line

import (
	"go_binance_futures/feature/api/binance"
	"go_binance_futures/feature/strategy"
	"go_binance_futures/utils"
	"fmt"
	"math"
	"strconv"

	"github.com/adshao/go-binance/v2/futures"
)

type TradeLine4 struct {
}

// 交易逻辑: 看的是 6h k线 和 2h k线
// 做多逻辑
// 1. 在4个line线之内，6小时线产生金叉
// 2. 2小时线最低点在 11个之内，最低点到最低点+8个line里面至少6个是红线，最低点事红线，(下影线长度 / 实体长度) > 0.5
// 3. rsi6 < 80, rsi14 < 75
// 4. 基本盘逻辑: btc 的跌幅大于 5%，当前所有币种跌的数量>= 75% 时禁止做多，反之禁止做空
// 做空相反
func (TradeLine4 TradeLine4) GetCanLongOrShort(openParams strategy.OpenParams) (openResult strategy.OpenResult) {
	symbols := openParams.Symbols
	openResult.CanLong = false
	openResult.CanShort = false
	
	kline_6h, err1 := binance.GetKlineData(symbols.Symbol, "6h", 50)
	kline_1h, err2 := binance.GetKlineData(symbols.Symbol, "2h", 24)
	if err1 != nil || err2 != nil {
		openResult.Reason = fmt.Sprintf("no trading strategy conditions passed | K线获取失败(6h:%v, 2h:%v)", err1, err2)
		return openResult
	}
	kline_6h_close := GetLineClosePrices(kline_6h)

	ma6h_3, _ := CalculateSimpleMovingAverage(kline_6h_close, 3) // ma3
	ma6h_7, _ := CalculateSimpleMovingAverage(kline_6h_close, 7) // ma7
	rsi6, _ := CalculateRSI(kline_6h_close, 6) // rsi6
	rsi14, _ := CalculateRSI(kline_6h_close, 14) // rsi14
	if (rsi6 == nil || rsi14 == nil || len(rsi6) < 2 || len(rsi14) < 2) {
		// 开盘小于 4.5 天
		openResult.Reason = "no trading strategy conditions passed | 上市时间短, K线数据不足, 跳过判定"
		return openResult
	}
	baseCanLong, baseCanShort := BaseCheckCanLongOrShort() // 基本盘
	isRsi := rsi6[0] < 80 && rsi6[0] > 30 && rsi14[0] < 75 && rsi14[0] > 28
	longCross := KdjSimple(ma6h_3, ma6h_7, 4) // 1天之内发生过金叉, rsi 没有超买
	longLine, longLineDbg := TradeLine4.checkLongLine(kline_1h)
	shortCross := KdjSimple(ma6h_7, ma6h_3, 4)
	shortLine, shortLineDbg := TradeLine4.checkShortLine(kline_1h)

	if longCross && longLine && isRsi && baseCanLong {
		// 短线穿越长线金叉
		openResult.CanLong = true
		return openResult
	}
	if shortCross && shortLine && isRsi && baseCanShort {
		openResult.CanShort = true
		return openResult
	}
	openResult.Reason = fmt.Sprintf(
		"no trading strategy conditions passed | 多[金叉%v(%s) 形态%v(%s) 基本盘%v] 空[金叉%v 形态%v(%s) 基本盘%v] RSI6=%.1f RSI14=%.1f",
		longCross, crossState(ma6h_3, ma6h_7), longLine, longLineDbg, baseCanLong,
		shortCross, shortLine, shortLineDbg, baseCanShort,
		rsi6[0], rsi14[0],
	)
	return openResult
}

// ma1(短线)相对 ma2(长线)的交叉状态, 数组第0位为最新
func crossState(ma1 []float64, ma2 []float64) string {
	if len(ma1) == 0 || len(ma2) == 0 {
		return "无数据"
	}
	if ma1[0] >= ma2[0] {
		for i := 1; i < len(ma1) && i < len(ma2); i++ {
			if ma1[i] < ma2[i] {
				return fmt.Sprintf("金叉%d根K线前", i)
			}
		}
		return "持续在上"
	}
	for i := 1; i < len(ma1) && i < len(ma2); i++ {
		if ma1[i] >= ma2[i] {
			return fmt.Sprintf("死叉%d根K线前", i)
		}
	}
	return "持续在下"
}

// 达到止盈或止损后判断是否可以平仓
// 5min 最新价格是否跌破前一个5min的收盘价
func (TradeLine4 TradeLine4) CanOrderComplete(closeParams strategy.CloseParams) (closeResult strategy.CloseResult) {
	symbols := closeParams.Symbols // 交易对
	position := closeParams.Position // 当前仓位
	closeResult.Complete = false
	
	lines, err := binance.GetKlineData(symbols.Symbol, "5m", 2)
	if err != nil {
		closeResult.Complete = true
		return closeResult
	}
	close0, _ := strconv.ParseFloat(lines[0].Close, 64)
	close1, _ := strconv.ParseFloat(lines[1].Close, 64)
	if position.Side == "LONG" {
		closeResult.Complete = close0 < close1 // 价格在下跌中
	} else if position.Side == "SHORT" {
		closeResult.Complete = close0 > close1 // 价格在上涨中
	} else {
		closeResult.Complete = true
	}
	return closeResult
}

// 达到止盈或止损前判定是否可以平仓
// 1. 1天的kline线，ma7和ma3金叉，ma15和ma3金叉，ma3线3连跌
func (TradeLine4 TradeLine4) AutoStopOrder(closeParams strategy.CloseParams) (closeResult strategy.CloseResult) {
	position := closeParams.Position // 当前仓位
	closeResult.Complete = false
	
	if closeParams.NowProfit > 3 || closeParams.NowProfit < -3 {
		// 已超出 ±3% 区间, 交给常规止盈止损逻辑
		closeResult.Complete = false
		return closeResult
	}
	closeResult.Complete = TradeLine4.MarketReversal(position.Symbol, position.Side)
	return closeResult
}

func (TradeLine4 TradeLine4) MarketReversal(symbol string, positionSide string) (isReversal bool) {
	kline_1d, err1 := binance.GetKlineData(symbol, "1d", 50)
	if err1 != nil {
		return false
	}
	kline_1d_close := GetLineClosePrices(kline_1d)
	
	ma1d_3, _ := CalculateSimpleMovingAverage(kline_1d_close, 3) // ma3
	ma1d_7, _ := CalculateSimpleMovingAverage(kline_1d_close, 7) // ma7
	ma1d_15, _ := CalculateSimpleMovingAverage(kline_1d_close, 15) // ma15
	
	if positionSide== "LONG" {
		if KdjSimple(ma1d_7, ma1d_3, 4) && KdjSimple(ma1d_15, ma1d_3, 4) && utils.IsAsc(ma1d_3[0:3]) {
			return true
		}
	}
	if positionSide == "SHORT" {
		if KdjSimple(ma1d_3, ma1d_7, 4) && KdjSimple(ma1d_3, ma1d_15, 4) && utils.IsDesc(ma1d_3[0:3]) {
			return true
		}
	}
	return false
}

// 2h线见底形态: 最近11根内最低点为长下影阴线, 其后8根内至少6根阴线
// 返回值 dbg 为形态明细, 供日志排查
func (TradeLine4 TradeLine4) checkLongLine(klines []*futures.Kline) (can bool, dbg string) {
	lineData := normalizationLineData(klines) // 24条线
	minIndex := lineData.MinIndex
	line := lineData.Line
	if minIndex < 1 || minIndex > 11 || minIndex+8 > len(line) {
		return false, fmt.Sprintf("低点在第%d根(需1~11)", minIndex)
	}
	linePoint := line[minIndex] // 最低的那个line
	underLength := math.Abs(linePoint.Close - linePoint.Low) // 下影线长度
	entityLength := math.Abs(linePoint.Open - linePoint.Close) // 实体长度
	fallCount := 0
	for _, item := range line[minIndex : minIndex+8] {
		if item.Position == "SHORT" {
			fallCount++
		}
	}
	pointColor := "阳线"
	if linePoint.Position == "SHORT" {
		pointColor = "阴线"
	}
	can = getRightLine(line[minIndex:minIndex+8], "SHORT") && // 最低点到最低点+8个line里面至少6个是红线
		linePoint.Position == "SHORT" && // 最低点的line是跌
		(underLength / entityLength) > 0.5 // 下影线长度  实体长度
	dbg = fmt.Sprintf("低点第%d根,%s,影/实%.2f,8根内阴线%d", minIndex, pointColor, underLength/entityLength, fallCount)
	return can, dbg
}

// 2h线见顶形态: 最近11根内最高点为长上影阳线, 其后8根内至少6根阳线
func (TradeLine4 TradeLine4) checkShortLine(klines []*futures.Kline) (can bool, dbg string) {
	lineData := normalizationLineData(klines) // 24条线
	maxIndex := lineData.MaxIndex
	line := lineData.Line
	if maxIndex < 1 || maxIndex > 11 || maxIndex+8 > len(line) {
		return false, fmt.Sprintf("高点在第%d根(需1~11)", maxIndex)
	}
	linePoint := line[maxIndex] // 最高的那个line
	upperLength := math.Abs(linePoint.High - linePoint.Close) // 上影线长度
	entityLength := math.Abs(linePoint.Open - linePoint.Close) // 实体长度
	riseCount := 0
	for _, item := range line[maxIndex : maxIndex+8] {
		if item.Position == "LONG" {
			riseCount++
		}
	}
	pointColor := "阳线"
	if linePoint.Position == "SHORT" {
		pointColor = "阴线"
	}
	can = getRightLine(line[maxIndex:maxIndex+8], "LONG") && // 最高点到最高点+8个line里面至少6个是绿线
		linePoint.Position == "LONG" && // 最高点的line是涨
		(upperLength / entityLength) > 0.5 // 上影线长度 > 实体长度
	dbg = fmt.Sprintf("高点第%d根,%s,影/实%.2f,8根内阳线%d", maxIndex, pointColor, upperLength/entityLength, riseCount)
	return can, dbg
}
