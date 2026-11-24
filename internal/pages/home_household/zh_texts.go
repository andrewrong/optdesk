package home_household

// The Chinese words of the family card.
const (
	familySubtitle = "全家合计 · 计入 %d / %d 个账户 · 最早数据日 %s"
	familyNoDay    = "无"

	familyLineNoData      = "%s今天未采集到，合计不含"
	familyLineStale       = "%s今天未采集到，最近一次是 %s（超过 3 个交易日），合计不含"
	familyLineCurrency    = "%s的币种缺少美元汇率或暂不支持，合计不含"
	familyLineEarlierDay  = "%s用的是 %s 的采集，早于最新交易日，已计入合计"
	familyLineReadError   = "%s读取失败，合计不含"
	familyRateLine        = "日元账户按 USDJPY 汇率 %s（日期 %s，来源 %s）折成美元"
	familyCashLine        = "现金分放在各账户，只担保本账户的卖出看跌；合计现金不是可共用的保证金"
	familyAccountLabel    = "账户（%s）"
	familyNeedsPrior      = "没有更早的采集：%s"
	familyNeedsMonthStart = "没有上月末的采集：%s"
	familyListSeparator   = "、"

	cardDayMixed = "各账户数据日不同，不显示单日变化"
)

// The Chinese words of the member card.
const (
	textSubtitle   = "主账户 · 数据日 "
	textDataDay    = "数据日 "
	textStaleLabel = "账户数据日 "
	textNoReader   = "未接入读取器，暂无数据"
	textNoData     = "该账户还没有采集"
	textNoCapture  = "无采集"
	textStale      = "最近一次采集已超过 3 个交易日"
	textBorrowing  = "借款"

	explainNetWorth    = "主账户的净清算值（券商 NetLiquidation，美元），取自最近一次采集。"
	explainCash        = "主账户的现金（TotalCashValue，美元）。大数字是占净值的比例，小数字是金额；现金为负表示融资借款。"
	explainDayChange   = "最近一次采集的净清算值减去上一个有采集的交易日，美元；含出入金，不是纯收益。百分比是这一变化除以前一日。"
	explainMonthToDate = "最近一次采集的净清算值减去上月最后一个交易日，美元；含出入金，不是时间加权收益。"

	textNeedsPrior      = "需要第二个有采集的交易日；首次采集 "
	textNeedsMonthStart = "需要上月最后一个交易日的采集；首次采集 "
)
