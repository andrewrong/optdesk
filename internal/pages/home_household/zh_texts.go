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
