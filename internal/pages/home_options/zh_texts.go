package home_options

// The Chinese sentences of the options card: notes, the series caption and two gap reasons.
const (
	textMaxLoss  = "最大亏损需要组合结构，账本没有给出"
	textOtherCcy = "账本含非美元持仓，本页不计入"

	seriesCaption = "美元，按平仓月份：到期、行权、被指派和买回平仓都计入，不扣手续费；取平仓日的年月；不做时区换算；按账户别名分列，不合并"

	noteCash        = "权利金按开仓成交现金计，不含手续费；卖出为正，买入为负。"
	noteClosed      = "权利金和已实现盈亏只计已平仓的卖出持仓，所有账户合并。"
	noteClosedEmpty = "平仓盈亏暂无数据：本页不给出按月的平仓盈亏。"
)
