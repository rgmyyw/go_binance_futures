package coin

import (
	"go_binance_futures/models"
	"sort"
)

type TradeCoin1 struct {
}

// 策略: 从跌幅榜/涨幅榜各取动量最强的前2个(原为前6随机取2), 最近5min交易过的币不再交易
func (tradeCoin1 TradeCoin1) SelectCoins(allCoins []*models.Symbols) (coins []*models.Symbols) {
	exclude_symbols_map := getRecentOrderSymbols(5)
	sort.SliceStable(allCoins, func(i, j int) bool {
		return allCoins[i].PercentChange < allCoins[j].PercentChange // 涨幅从小到大排序
	})
	
	filterCoins := []*models.Symbols{}
	for _, coin := range allCoins {
		if _, exist := exclude_symbols_map[coin.Symbol]; exist {
			continue
		} // 最近交易过的排除
		if coin.Enable == 1 { // 只获取允许交易的币
			filterCoins = append(filterCoins, coin)
		}
	}
	sliceLength := 6
	if len(filterCoins) < sliceLength {
		sliceLength = len(filterCoins)
	}
	// 确定性选取动量最强的各 2 个(原为前 6 随机取 2, 会随机丢弃一半同强度信号)
	losers := filterCoins[:sliceLength]
	gainers := filterCoins[len(filterCoins)-sliceLength:]
	coins = append(coins, losers[:min(2, len(losers))]...)
	coins = append(coins, gainers[max(0, len(gainers)-2):]...)
	return coins
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
