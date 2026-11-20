package option_tracking_page

// Chinese explanation texts of the page (owner ask optdesk-4802): string
// constants only; labels and numbers stay English, option terms stay English.
const (
	markOnlyZh = "只有账户标记价，没有该合约的存档行情（无正股收盘价、Delta、IV）"

	flexNoteZh    = "Flex 次日才到；今天新开或被指派的仓位可能还没显示，见核对表"
	markNoteZh    = "mid 是收盘 bid/ask 的中间价，不是成交价"
	leftOutTailZh = " 条 long 腿未列出，见 Held options > Structures"
	staleTextZh   = "部分未平仓合约没有最新收盘价，相关单元格显示为空缺"

	moneynessHintZh  = "占收盘价的百分比，正数表示价外"
	coverageHintZh   = "从开仓日与该合约首条存档行情日两者中较晚的一天起，统计存有双边报价（bid 与 ask 都大于零）的交易日"
	netPremiumHintZh = "开仓交易与平仓交易的现金合计，未扣佣金；买回价高于卖出价时为负"

	coverageNoteHeadZh = "自首条存档行情起，共 "
	coverageNoteMidZh  = " 个 lot-day，其中 "
	coverageNoteTailZh = " 个没有双边报价"

	noMarkZh     = "没有最新收盘价"
	noEventRowZh = "近 7 天候选表中没有该公司的记录，财报日和除息日未知"
	noGuardRowZh = "没有持仓 call 的保护行，不按安全处理"
	noFlexRowsZh = "该账户没有 Flex 记录"
	noAccountZh  = "没有账户持仓记录，核对表未生成"
	noValueZh    = "没有该数值"
	noCloseZh    = "没有正股收盘价"
	noReaderZh   = "没有读取器"
	unreadableZh = "无法读取该数值"

	noSessionsZh   = "没有交易日读取器，页面无法判断交易日"
	noEodReaderZh  = "没有日终价读取器，财报日和除息日未知"
	noCandidatesZh = "没有候选读取器，财报日和除息日未知"
	noNamesZh      = "没有标的读取器，财报日和除息日未知"

	earlyAssignZh   = "除息日前有提前被指派的风险"
	noDividendZh    = "没有股息信息"
	staleDividendZh = "股息信息已过期"

	notInFlexZh    = "账户里有持仓而 Flex 里没有：可能今天新开，Flex 次日才到"
	notInAccountZh = "Flex 里是未平仓而账户里已没有：可能今天被指派或到期，Flex 还没到"
)
