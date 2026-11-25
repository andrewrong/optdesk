package macro_page

// Design: docs/design/macro-page-v2.md#decisions 8.
//
// The Chinese hints of the page. This is the only file of the package with Chinese text;
// TestHintsOnlyInZhTexts fails on any other. The web prints a hint as it is.
const (
	hintTbill     = "三个月美国国债收益率，就是无风险现金利率；卖期权的收入要拿它来比，它高时现金本身就有回报。"
	hintHySpread  = "高收益债信用利差（ICE BofA，FRED BAMLH0A0HYM2）：越大说明市场越怕违约，常在股市下跌前后走阔；这里只提示，不挡单。"
	hintNextEvent = "期权窗口内的重大事件（CPI、非农、PCE、GDP、联储议息、日本央行议息）：合约跨过这一天，要把事件风险算进去。"
	hintIndexIV   = "指数隐含波动率（VIX、VXN、RVX，FRED收盘值，不是实时报价）：越高权利金越贵，卖方收入高，风险也高。"
	hintWindow    = "事件列和窗口提示用同一套说明：“窗口内”指到期日不超过期权最长到期天数（默认45天）。"

	hintDFF      = "联邦基金有效利率，美联储政策利率的实际水平，短期资金的价格。"
	hintDGS2     = "两年期美国国债收益率，反映市场对未来一两年政策利率的预期。"
	hintDGS10    = "十年期美国国债收益率，长期利率的基准，也影响估值。"
	hintT10Y2Y   = "十年减两年国债利差；为负是倒挂，常被当作衰退预警，只作参考。"
	hintT10Y3M   = "十年减三个月国债利差；同样是曲线形状，倒挂更受联储关注。"
	hintT10YIE   = "十年期盈亏平衡通胀率，市场隐含的未来十年通胀预期。"
	hintNFCI     = "芝加哥联储金融状况指数；0是平均，越高金融环境越紧。"
	hintDTWEXBGS = "美联储广义美元指数；美元走强常对应全球流动性收紧。"
	hintDEXJPUS  = "美元兑日元；日元急升常伴随套利交易平仓，波动放大。"
	hintIRLTJP   = "日本十年期国债收益率（OECD经FRED，月度）。"
)

// The hint of the event columns: previous and actual are the last published figure and the release, the figure kinds
// (y/y 同比, m/m 环比, NFP 非农月增, FOMC 目标区间), the latest vintage and the missing consensus.
const hintEventValues = "前值/公布值：CPI、PCE、PPI为同比，零售为环比，非农为月增（千人），联储为目标区间；均为FRED最新版本；共识预期没有免费可靠来源，不显示。"

// The lead note, the two page sentences of the liquidity block, the notes of two of its taps and the zero line of the NFCI
// sentence. The zero line follows an English clause of the sentence, so it carries no full stop of its own.
const (
	leadNote         = "仅供查看，没有任何规则读取这里的内容。"
	sentenceLiq      = "资金与信用背景，来自已存的序列；事件：暂无数据。"
	sentenceLiqEv    = "资金与信用背景，来自已存的序列。"
	noteNetLiquidity = "这是从业者自拟的指标，没有证据表明它领先价格。"
	noteStlfsi       = "0 是 1993 年末以来的平均水平；高于 0 表示压力高于平均。"
	zeroLineNFCI     = "高于 0 表示比平均更紧"
)

// The gap texts of the event calendar: the four prefixes of the kind lines, the endings of a line cut to the length limit,
// and the line of the events that are not shown (the day, the number shown, the total).
const (
	gapPrefixStale      = "日期已超过 10 天："
	gapPrefixNone       = "没有存放日期："
	gapPrefixHeld       = "已存的日期保留，来源这次给的更短："
	gapPrefixNotEntered = "未录入："
	gapMoreAfterPrefix  = "另有 %d 项"
	gapMoreAfterList    = "，另有 %d 项"
	gapEventsNotShown   = "%s 之后的事件没有全部显示：共 %[3]d 项，已显示 %[2]d 项"
)

// The gap texts of the event value cells and of the indicator unit. The first two take the id and the label.
const (
	gapValueNoNumber   = "%s %s 尚未公布（已存档但没有数值）"
	gapValueNotStored  = "%s %s 没有存档"
	gapEventDateUnread = "事件日期读不出来"
	gapEarlierEstimate = "更早的估计值没有存档（只保留最新版本）"
	gapNotYetReleased  = "尚未发布"
	gapNotYetPublished = "FRED 尚未发布"
	gapUnitUnknown     = "单位未知"
)

// The title of the event table and the release-time texts of its rows; the clock times and the zone codes stay as they are.
const (
	eventsTitleWithReader = "未来 60 天，日期按发布方所在时区（ET、JST）"
	whenET830             = "通常 8:30 ET"
	whenET1000            = "通常 10:00 ET"
	whenET1630            = "通常 4:30 pm ET"
	whenET1400            = "通常 2:00 pm ET"
	whenJSTMidday         = "通常中午前后 JST，日期按 JST"
	whenDateOnly          = "仅日期"
	whenHighSuffix        = "，重要"
	whenReadPrefix        = "，读取于 "
)

// The page title, the card titles and the two status labels of an indicator row: the Chinese word with the English beside it.
const (
	title           = "宏观 Macro"
	easeTitle       = "宽松度 Ease"
	eventsTitle     = "本周和下周 This week and next"
	indicatorsTitle = "指标 Indicators"
	notesTitle      = "说明 Notes"
	labelOnTime     = "按时 On time"
	labelLate       = "延迟 Late"
)
