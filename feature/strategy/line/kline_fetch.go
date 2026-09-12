package line

import (
	"go_binance_futures/feature/api/binance"

	"github.com/adshao/go-binance/v2/futures"
)

// getKlineData 为可测试接缝: 生产路径直接走 binance REST,
// 单元测试通过 line.GetKlineData = ... 替换为固定数据
var getKlineData = binance.GetKlineData

var _ = futures.Kline{} // 保持 futures 导入(接缝签名使用处)
