package utils

import (
	"math"
	"testing"
)

func TestFuturesLeveragedROI(t *testing.T) {
	cases := []struct {
		name                          string
		profit, qty, price            float64
		leverage                      int64
		want                          float64
	}{
		{"3倍做多浮盈3U", 3, 0.3, 100, 3, 30},
		{"亏损为负ROI", -3, 0.3, 100, 3, -30},
		{"数量为0返回0", 3, 0, 100, 3, 0},
		{"价格为0返回0", 3, 0.3, 0, 3, 0},
		{"杠杆为0返回0", 3, 0.3, 100, 0, 0},
		{"1倍杠杆等于价格涨跌百分比", 1, 1, 100, 1, 1},
	}
	for _, c := range cases {
		if got := FuturesLeveragedROI(c.profit, c.qty, c.price, c.leverage); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGetPowAndTradePrecision(t *testing.T) {
	cases := []struct {
		lotsize string
		pow     int
	}{
		{"1", 0}, {"0.1", 1}, {"0.01", 2}, {"0.001", 3}, {"0.0001", 4},
	}
	for _, c := range cases {
		if got := GetPow(c.lotsize); got != c.pow {
			t.Errorf("GetPow(%q)=%d, want %d", c.lotsize, got, c.pow)
		}
	}
	if got := GetTradePrecision(0.123456789, "0.001"); math.Abs(got-0.123) > 1e-9 {
		t.Errorf("GetTradePrecision 3位=%v", got)
	}
	if got := GetTradePrecision(12345.6789, "1"); got != 12346 {
		t.Errorf("GetTradePrecision 整数位=%v, want 12346", got)
	}
}

func TestIsAscIsDescReverse(t *testing.T) {
	if !IsAsc([]float64{1, 2, 3}) {
		t.Error("升序应判定为true")
	}
	if IsAsc([]float64{3, 2, 1}) {
		t.Error("降序应判定为false")
	}
	if IsAsc([]float64{1, 1, 2}) {
		t.Error("相等不算严格升序")
	}
	if !IsDesc([]float64{3, 2, 1}) {
		t.Error("降序应判定为true")
	}
	if !IsDesc([]float64{2, 1}) {
		t.Error("两元素降序应为true")
	}
	if !IsDesc([]float64{1}) || !IsAsc([]float64{1}) {
		t.Error("单元素应平凡判定为true")
	}
	rev := ReverseArray([]float64{1, 2, 3})
	if rev[0] != 3 || rev[2] != 1 {
		t.Errorf("ReverseArray 结果错误: %v", rev)
	}
}
