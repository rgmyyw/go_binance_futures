## 语言
1. 请始终使用中文描述和回答问题，代码注释使用英文

## 输出要求
1. 先给当前结论/下一步动作
2. 多步骤任务显示 Step X/Y
3. 每次完成修改明确说明“现在什么已经可用”
4. 报错统一输出：位置 / 原因 / 修复
5. 避免与当前任务无关的扩展建议
6. 长任务持续维护当前进度

## 策略经验(2026-09-12 实盘复盘, 43笔/-3.14U 换来的)
1. 优化目标是期望值(EV), 不是胜率: TP1% 把胜率抬到67%但削掉肥尾, 当日唯一大盈利(+4.34)死于宽止盈被改窄
2. 出场结构: 宽止盈(TP20 ROI)让利润奔跑 + 紧止损(SL5 ROI)有界风险, 已回退生效
3. line6 均值回归实证无边际(29笔 -5.57U 多空双向皆亏), 已由 line6Enabled() 硬禁用, 重启用需先多周期回测
4. 震荡/分化行情下双向开仓=被whipsaw鞭打; 单日多次改动导致无法归因, 一次只改一个变量
5. 回测/实盘差距: 市价单滑点+点差 0.1~0.3%/笔, meme币更宽; 回测胜率67%实盘仅40%, 汇报时优先讲EV而非胜率
6. 交易所侧止损会先于软止损成交, 属正常; 统计必须走 RealizedClose 事件而非软止损分支

## 系统机制(当前有效)
1. 熔断/降级统计: ws ORDER_TRADE_UPDATE 发布 RealizedClose 事件 → feature 订阅计数(按订单号去重, 止损级阈值 -1.5U)
2. line6 降级护栏: 连续3笔ROI止损 → 自动降回 line5 + 冷却12h(RecordStopLossForStrategy)
3. 行情切换: LLM 每小时判 market_condition(历史表 market_condition_histories), 成功时不打日志, 查表确认; 启动即对齐策略
4. 绩效看门狗: 每50笔平仓推送胜率/期望(StartPerfWatchdog); VM101 cron gbf-watchdog.sh 每10分钟监控错误风暴/封禁/大亏
5. 价格变动提醒(ws_futures_price_change_limit)已实现但当前阈值0=关闭
6. K线获取统一走接缝 getKlineData(测试可注入), 生产实现在 kline_fetch.go

## 测试约定
1. 改策略/参数必须跑: go test ./feature/strategy/... ./feature ./utils (当前137+, 全绿)
2. 策略层函数覆盖100%; 夹具由 Python 复刻算法搜索生成, 避开浮点精确边界(ParseFloat 下 0.3% 恰好不触发)
3. 测试发现过的真实bug: line6/line7/line_custom 恒真条件三处, coin1 重复选币, 乌云盖顶永假——新增策略必须配测试

## 运维要点
1. 部署链: patches→master→push→VM101 src reset→compose build v1.0.9-pN→up; DB版本提升需先备份 coin.db 再 sync db
2. sqlite 直接写需 sudo(文件属主 docker); 改动前 .backup
3. OpenClash 币安专线(Fallback组)会漂回被封节点; 封禁告警后用 API PUT /proxies 切换, 密钥见 clash-switch-notify.sh
4. 恢复/暂停开仓: config 表 future_allow_long/short; 策略错位可置 line5 后重启对齐
5. 风控红线: 单笔止损≤账户0.7%; 实盘不满50笔样本不下结论不升仓
