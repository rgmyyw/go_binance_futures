package line

import (
	"go_binance_futures/feature/api/binance"

	"github.com/adshao/go-binance/v2/futures"
)

// getKlineData 为可测试接缝: 生产路径直接走 binance REST,
// 单元测试通过替换该变量注入固定K线数据
var getKlineData = binance.GetKlineData

// binanceGetKlineData 保存生产实现, 供测试结束后恢复接缝
var binanceGetKlineData = binance.GetKlineData

var _ = futures.Kline{} // 保持 futures 导入(接缝签名使用处)
