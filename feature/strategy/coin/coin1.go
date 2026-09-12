package coin

import (
	"go_binance_futures/models"
	"sort"
)

type TradeCoin1 struct {
}

// 策略: 从跌幅榜/涨幅榜各取动量最强的前4个(每侧扩容提升候选多样性, 原为前6随机取2→前2定值),
// 最近5min交易过的币不再交易
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
	// 币种不足时两侧切片会重叠, 用 seen 去重防止同一币入选两次
	seen := map[string]bool{}
	addStrongest := func(candidates []*models.Symbols, fromEnd bool, n int) {
		if fromEnd {
			for i := len(candidates) - 1; i >= 0 && n > 0; i-- {
				if seen[candidates[i].Symbol] {
					continue
				}
				seen[candidates[i].Symbol] = true
				coins = append(coins, candidates[i])
				n--
			}
			return
		}
		for i := 0; i < len(candidates) && n > 0; i++ {
			if seen[candidates[i].Symbol] {
				continue
			}
			seen[candidates[i].Symbol] = true
			coins = append(coins, candidates[i])
			n--
		}
	}
	addStrongest(filterCoins[:sliceLength], false, 4) // 跌幅最强
	addStrongest(filterCoins, true, 4)                // 涨幅最强
	return coins
}
