package line

import (
	"testing"

	"github.com/adshao/go-binance/v2/futures"
)

func TestGetLineClosePrices(t *testing.T) {
	bars := []*futures.Kline{
		{Close: "100.5"},
		{Close: "101"},
		{Close: "99.9"},
	}
	got := GetLineClosePrices(bars)
	if len(got) != 3 || got[0] != 100.5 || got[2] != 99.9 {
		t.Fatalf("收盘价序列错误: %v", got)
	}
}

func TestCalculateSimpleMovingAverage(t *testing.T) {
	// 输入新→旧; 内部反转为旧→新计算后再次反转为新→旧输出
	// 序列(旧→新): 1,2,3,4 → 新→旧输入: 4,3,2,1; period=2
	// 旧→新的SMA: (1+2)/2=1.5, (2+3)/2=2.5, (3+4)/2=3.5 → 反转输出: 3.5,2.5,1.5
	sma, err := CalculateSimpleMovingAverage([]float64{4, 3, 2, 1}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(sma) != 3 || sma[0] != 3.5 || sma[1] != 2.5 || sma[2] != 1.5 {
		t.Fatalf("SMA 结果错误: %v", sma)
	}
	if _, err := CalculateSimpleMovingAverage([]float64{1}, 2); err == nil {
		t.Fatal("数据不足应返回错误")
	}
	if _, err := CalculateSimpleMovingAverage(nil, 0); err == nil {
		t.Fatal("非法周期应返回错误")
	}
}

func TestCalculateRSI(t *testing.T) {
	// 全涨: RSI=100 (输入新→旧: 119..100, 反转后旧→新递增)
	allUp := make([]float64, 20)
	for i := range allUp {
		allUp[i] = float64(119 - i)
	}
	rsi, err := CalculateRSI(allUp, 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(rsi) == 0 || rsi[0] != 100 {
		t.Fatalf("单边上涨 RSI 应为100, got %v", rsi[0])
	}

	// 全跌: RSI=0 (输入新→旧: 100..119, 反转后旧→新递减)
	allDown := make([]float64, 20)
	for i := range allDown {
		allDown[i] = float64(100 + i)
	}
	rsi, err = CalculateRSI(allDown, 14)
	if err != nil {
		t.Fatal(err)
	}
	if rsi[0] != 0 {
		t.Fatalf("单边下跌 RSI 应为0, got %v", rsi[0])
	}

	if _, err := CalculateRSI(allUp[:5], 14); err == nil {
		t.Fatal("数据不足应返回错误")
	}
	if _, err := CalculateRSI(allUp, -1); err == nil {
		t.Fatal("非法周期应返回错误")
	}
}

func TestKdjSimple(t *testing.T) {
	// 最近发生金叉: 短线最新在上, num 窗口内早期在下
	maFast := []float64{5, 5, 1, 1}
	maSlow := []float64{4, 6, 2, 2}
	if !KdjSimple(maFast, maSlow, 4) {
		t.Fatal("金叉应判定为true")
	}
	// 短线一直在上(无交叉)
	if KdjSimple([]float64{5, 5, 5, 5}, []float64{4, 4, 4, 4}, 4) {
		t.Fatal("无交叉应判定为false")
	}
	// 最新短线在下
	if KdjSimple([]float64{1, 5, 5, 5}, []float64{4, 4, 4, 4}, 4) {
		t.Fatal("最新数据短线在下应判定为false")
	}
	// 数据不足
	if KdjSimple([]float64{1}, []float64{2}, 4) {
		t.Fatal("数据不足应判定为false")
	}
	if KdjSimple(nil, maSlow, 4) {
		t.Fatal("nil 输入应判定为false")
	}
}
