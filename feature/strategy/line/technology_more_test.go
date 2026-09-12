package line

import (
	"math"
	"testing"

	"github.com/adshao/go-binance/v2/futures"
)

// ===== MACD =====

func TestCalculateMACD(t *testing.T) {
	// 上升趋势: DIF 应为正
	closes := make([]float64, 50)
	for i := range closes {
		closes[49-i] = 100 + float64(i) // 新→旧: 149..100
	}
	dif, dea, hist, err := CalculateMACD(closes, 12, 26, 9)
	if err != nil {
		t.Fatal(err)
	}
	if len(dif) == 0 || dif[0] <= 0 {
		t.Fatalf("上升序列 DIF 应为正, got %v", dif[0])
	}
	if len(dif) != len(dea) || len(dea) != len(hist) {
		t.Fatal("MACD 三线长度应一致")
	}
	if _, _, _, err := CalculateMACD(closes, 26, 12, 9); err == nil {
		t.Fatal("fast>=slow 应报错")
	}
	if _, _, _, err := CalculateMACD(closes[:10], 12, 26, 9); err == nil {
		t.Fatal("数据不足应报错")
	}
}

// ===== 布林带 =====

func TestCalculateBollingerBands(t *testing.T) {
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = 100
	}
	upper, mid, lower, err := CalculateBollingerBands(closes, 20, 2)
	if err != nil {
		t.Fatal(err)
	}
	// 零方差: 上轨=中轨=下轨=均值
	if len(mid) == 0 || math.Abs(mid[0]-100) > 1e-9 || math.Abs(upper[0]-100) > 1e-9 || math.Abs(lower[0]-100) > 1e-9 {
		t.Fatalf("零方差布林带应收敛于均值: %v %v %v", upper[0], mid[0], lower[0])
	}
	if _, _, _, err := CalculateBollingerBands(closes, 20, -1); err == nil {
		t.Fatal("负标准差倍数应报错")
	}
}

// ===== ROC =====

func TestCalculateROC(t *testing.T) {
	closes := make([]float64, 15)
	for i := range closes {
		closes[i] = float64(86 + i) // 新→旧: 86..100 → 旧值100 → 新值86, 下跌
	}
	roc, err := CalculateROC(closes, 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(roc) != 1 || roc[0] >= 0 {
		t.Fatalf("下跌序列 ROC 应为负: %v", roc)
	}
	if _, err := CalculateROC(closes[:5], 14); err == nil {
		t.Fatal("数据不足应报错")
	}
	if _, err := CalculateROC(nil, 14); err == nil {
		t.Fatal("空输入应报错")
	}
}

// ===== MFI =====

func TestCalculateMFI(t *testing.T) {
	n := 20
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	amount := make([]float64, n)
	for i := 0; i < n; i++ {
		price := 100 + float64(n-i) // 新→旧: 119..100 → 反转后上涨
		high[i], low[i], close[i], amount[i] = price+1, price-1, price, 1000
	}
	mfi, err := CalculateMFI(high, low, close, amount, 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(mfi) == 0 || mfi[0] != 100 {
		t.Fatalf("单边上涨 MFI 应为100, got %v", mfi[0])
	}
	if _, err := CalculateMFI(high[:5], low[:5], close[:5], amount[:5], 14); err == nil {
		t.Fatal("数据不足应报错")
	}
	badAmount := append([]float64{}, amount...)
	badAmount[0] = -1
	if _, err := CalculateMFI(high, low, close, badAmount, 14); err == nil {
		t.Fatal("负成交额应报错")
	}
}

// ===== OBV =====

func TestCalculateOBV(t *testing.T) {
	close := []float64{103, 102, 101} // 新→旧: 上涨序列
	amount := []float64{100, 200, 300}
	obv, err := CalculateOBV(close, amount)
	if err != nil {
		t.Fatal(err)
	}
	// 反转为旧→新: 101(300), 102(200), 103(100) → obv: 0, +200, +300 → 新→旧: 300, 200, 0
	if len(obv) != 3 || obv[0] != 300 || obv[1] != 200 || obv[2] != 0 {
		t.Fatalf("OBV 结果错误: %v", obv)
	}
	if _, err := CalculateOBV(close[:2], amount); err == nil {
		t.Fatal("长度不一致应报错")
	}
}

// ===== CCI =====

func TestCalculateCCI(t *testing.T) {
	n := 30
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	for i := 0; i < n; i++ {
		c := 100 + float64(n-i)
		high[i], low[i], close[i] = c+1, c-1, c
	}
	cci, err := CalculateCCI(high, low, close, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(cci) == 0 {
		t.Fatal("CCI 不应为空")
	}
	if _, err := CalculateCCI(high[:5], low[:5], close[:5], 20); err == nil {
		t.Fatal("数据不足应报错")
	}
}

// ===== KDJ 完整版 =====

func TestCalculateKdj(t *testing.T) {
	n := 30
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	for i := 0; i < n; i++ {
		c := 100 + float64(n-i)
		high[i], low[i], close[i] = c+1, c-1, c
	}
	k, d, j, err := Kdj(high, low, close, 9, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	// 上涨序列: K 应偏高位
	if len(k) == 0 || k[0] <= 50 {
		t.Fatalf("上涨序列 K 应高于50, got %v", k[0])
	}
	if len(k) != len(d) || len(d) != len(j) {
		t.Fatal("KDJ 三线长度应一致")
	}
	if _, _, _, err := Kdj(high[:5], low[:5], close[:5], 9, 3, 3); err == nil {
		t.Fatal("数据不足应报错")
	}
}

func TestSum(t *testing.T) {
	if got := Sum([]float64{1.5, 2.5, 3.0}); math.Abs(got-7) > 1e-9 {
		t.Fatalf("Sum 应为7, got %v", got)
	}
	if Sum(nil) != 0 {
		t.Fatal("空切片和应为0")
	}
}

// ===== 肯纳特通道 =====

func TestCalculateKeltnerChannels(t *testing.T) {
	n := 60
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	for i := 0; i < n; i++ {
		c := 100 + float64(n-i)
		high[i], low[i], close[i] = c+1, c-1, c
	}
	upper, mid, lower, err := CalculateKeltnerChannels(high, low, close, 20, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(upper) == 0 || upper[0] <= mid[0] || mid[0] <= lower[0] {
		t.Fatalf("肯纳特通道应满足 upper>mid>lower: %v %v %v", upper[0], mid[0], lower[0])
	}
	if _, _, _, err := CalculateKeltnerChannels(high, low, close, 20, -1); err == nil {
		t.Fatal("负倍数应报错")
	}
}

// ===== 唐奇安通道 =====

func TestCalculateDonchianChannels(t *testing.T) {
	n := 30
	high := make([]float64, n)
	low := make([]float64, n)
	for i := 0; i < n; i++ {
		high[i], low[i] = 101+float64(n-i), 99+float64(n-i)
	}
	upper, mid, lower, err := CalculateDonchianChannels(high, low, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(upper) == 0 || upper[0] <= mid[0] || mid[0] <= lower[0] {
		t.Fatalf("唐奇安通道应满足 upper>mid>lower: %v %v %v", upper[0], mid[0], lower[0])
	}
	if _, _, _, err := CalculateDonchianChannels(high[:5], low[:5], 20); err == nil {
		t.Fatal("数据不足应报错")
	}
}

// ===== Supertrend / ADX 冒烟与错误路径 =====

func TestCalculateSupertrend(t *testing.T) {
	n := 60
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	for i := 0; i < n; i++ {
		c := 100 + float64(n-i)
		high[i], low[i], close[i] = c+1, c-1, c
	}
	data, trend, err := CalculateSupertrend(high, low, close, 10, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || len(data) != len(trend) {
		t.Fatal("Supertrend 输出长度应一致且非空")
	}
	if _, _, err := CalculateSupertrend(high, low, close, 10, 0); err == nil {
		t.Fatal("倍数为0应报错")
	}
}

func TestCalculateADX(t *testing.T) {
	n := 60
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	for i := 0; i < n; i++ {
		c := 100 + float64(n-i)
		high[i], low[i], close[i] = c+1, c-1, c
	}
	adx, plusDI, minusDI, err := CalculateADX(high, low, close, 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(adx) == 0 || len(adx) != len(plusDI) || len(plusDI) != len(minusDI) {
		t.Fatal("ADX 输出应非空且长度一致")
	}
	if _, _, _, err := CalculateADX(high[:10], low[:10], close[:10], 14); err == nil {
		t.Fatal("数据不足应报错")
	}
}

// ===== 乌云盖顶形态 =====

func TestIsDarkCloudCover(t *testing.T) {
	bullish := Candle{Open: 100, Close: 105, High: 106, Low: 99}
	perfect := Candle{Open: 106, Close: 101, High: 107, Low: 100}
	if !IsDarkCloudCover(bullish, perfect) {
		t.Fatal("标准乌云盖顶应判定为true")
	}
	// 第二根为阳线 → 不成立
	if IsDarkCloudCover(bullish, Candle{Open: 106, Close: 107, High: 108, Low: 100}) {
		t.Fatal("第二根阳线不应判定")
	}
	// 第二根收盘未低于第一根开盘 → 不成立
	if IsDarkCloudCover(bullish, Candle{Open: 106, Close: 105.5, High: 107, Low: 100}) {
		t.Fatal("未插入第一根实体不应判定")
	}
}

var _ = futures.Kline{}
