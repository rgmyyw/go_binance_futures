package utils

import (
	"math"
	"testing"
)

func TestMaNAndMaNList(t *testing.T) {
	closes := []float64{10, 12, 14, 16, 18} // 新→旧
	if got := MaN(closes, 3); math.Abs(got-12) > 1e-9 {
		t.Fatalf("MaN 前3项均值应12, got %v", got)
	}
	list := MaNList(closes, 3, 3)
	// list[0]=mean(10,12,14)=12; list[1]=mean(12,14,16)=14; list[2]=mean(14,16,18)=16
	if len(list) != 3 || math.Abs(list[0]-12) > 1e-9 || math.Abs(list[1]-14) > 1e-9 || math.Abs(list[2]-16) > 1e-9 {
		t.Fatalf("MaNList 结果错误: %v", list)
	}
}
