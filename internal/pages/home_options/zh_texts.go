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

// The Chinese sentences of the card: the subtitle, the say, the gap reasons and the tile captions.
const (
	subtitle      = "来自交易账本"
	sayNone       = "期权账本：暂无记录。"
	sayNoSold     = "账本：暂无已平仓的卖出记录。"
	sayPremium    = "已平仓卖出期权的权利金合计 "
	sayPremiumEnd = "。"
	textNoRows    = "账本：暂无记录"
	textNoReader  = "期权账本：尚未接入"
	textNoSold    = "账本：暂无已平仓的卖出记录"
	textNoDay     = "无账本日"
	textNotCents  = "账本金额不是整分，本页不做四舍五入"
	textMonthly   = "按平仓月份的已结算权利金：暂无数据"
	textNoClosed  = "本月没有平仓的卖出记录"
	textClosedMon = "本页不给出按月的平仓盈亏"
	textSoldCount = "本月卖出组数：账本汇总没有给出个数"
	textOpen      = "未平仓权利金：账本汇总没有给出未平仓权利金"

	textLots         = "本月新开手数"
	textRemaining    = "每手的剩余部分"
	textNoOpen       = "账本里没有未平仓的结构"
	textNetPaid      = "净支付，不显示"
	textNoFigure     = "账本没有给出数字"
	textAtLeast      = "至少：%d 个结构没有亏损数字"
	textOnlyOtherCcy = "只有非美元结构未平仓（%d 个），不换算"
)

// The Chinese sentences of the gap reasons of the figure tiles, the scope of the aliases and the caption of the closed series.
// Each format keeps its count first; textUncovered ends with the dollar figure.
const (
	textOtherCurrency = "%d 个非美元结构未计入"
	textUnreadable    = "%d 个结构因账本行无法读取未计入"
	textUncovered     = "%d 张卖出看涨期权在账本里没有对应的股数；其余结构至少 %s"
	textReview        = "%d 条账本记录需要复核，数字可能不完整"

	scopeNone    = "无账户"
	scopeOne     = "账户 %s"
	scopeMany    = "账户 %s 合并"
	scopeJoin    = "、"
	captionAll   = "所有已平仓的卖出持仓"
	captionReal  = "已实现 "
	captionKept  = "留存比例 "
	captionOneAc = "结果页只显示一个账户"
)
