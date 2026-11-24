package option_pnl_page

// Explanation texts in Chinese (owner request 2026-11-17); labels and numbers stay English.
// Door for Chinese in page code: string constants only. Design: docs/design/candidates-tiers.md#Decisions.
const (
	noQuotesZh        = "该合约在仓位持有期间没有存档报价（报价从 2026-11-07 起才开始存档），所以无法拆分，只显示账本数字。"
	leftOutNoQuotesZh = "多为 2026-11-07 前已结束的仓位，当时还没有报价存档"
	longPutSkippedZh  = "本页只显示卖出的看跌期权，已跳过买入的看跌期权仓位"
)

// Ledger block texts. Design: docs/design/options-pnl-ledger.md#Decisions (7).
const (
	ledgerNoteZh           = "只统计已结束的卖出看跌期权，按结束的月份归月；未结束的仓位不在其中"
	noConclusionZh         = "没有结论：样本太小，不能据此判断这种做法是否赚钱"
	noEndedMonthZh         = "本月没有已结束的仓位"
	ledgerHintRentZh       = "卖保险赚到的钱：时间价值减去波动带来的成本，可与你的预期对比"
	ledgerHintLedgerAllZh  = "已拆分的仓位加上没有存档报价的仓位，按账本数字合计；不受 20 个仓位规则限制"
	ledgerHintTimeZh       = "随着日子过去，期权价值下降；卖方把这部分留在手里"
	ledgerHintMoveCostZh   = "凸性：股价波动越大，卖出的看跌期权对你越不利"
	ledgerHintDirectionZh  = "赌股价不跌；这是你的估值判断，不是波动率优势"
	ledgerHintIVRemarkZh   = "隐含波动率（市场对未来波动的定价）变动带来的盈亏；它会回归"
	ledgerHintEntryZh      = "成交价与存档的买卖中间价之差"
	ledgerHintExitZh       = "买回成交价与存档的买卖中间价之差"
	ledgerHintTotalZh      = "期权这一侧的盈亏；下面的行权内在价值不包含在内"
	ledgerHintAssignedZh   = "看跌期权被行权时转移到股票上的价值"
	ledgerHintLedgerSaysZh = "账本给出的数字：合计加行权内在价值加差额"
)

// Late entry and folded months. Design: docs/design/option-pages-r7.md#4. {N} and {M} are written by the
// page with strconv.
const (
	lateEntryLotZh  = "这是从成交到第一条存档报价之间的市场波动，不是滑点"
	lateEntryBookZh = "{M} 个仓位中有 {N} 个在第一条存档报价之前成交（报价从 2026-11-07 起存档）"
	foldedMonthsZh  = "无拆分的月份"
	foldedNoteZh    = "2026-11-07 之前没有存档报价，这些仓位只有账本数字，无法拆分"
)

// Hints of the lot cards and the account card that have no ledger twin. The other card hints reuse the
// ledgerHint...Zh constants, so one term has one gloss on the page.
const (
	cardHintUnsplitZh    = "无法求解隐含波动率的步骤；原因写在仓位上"
	cardHintLedgerZh     = "账本给出的该仓位的数字"
	cardHintDifferenceZh = "账本数字减去合计，再减去行权内在价值"
	cardHintAssignmentZh = "合计加行权内在价值等于账本"
)

// Page notes, sample line, gap sentences and short card lines. Design: docs/design/options-pnl-ledger.md#Decisions (7).
// Option terms (Black-Scholes, IV, mid, Flex, gamma) and numbers stay as they are.
const (
	modelNoteZh    = "模型：欧式 Black-Scholes，一年按 365 天，不含股息；除息造成的下跌会落在求解出的 IV 里；不模拟提前行权的溢价"
	quoteDayNoteZh = "报价日按账本的算法：卖价大于 0，买价不小于 0，并且有标的价格"
	callsTailZh    = " 个看涨期权仓位已跳过：本页只显示卖出的看跌期权"

	sampleFewZh  = "拆分的仓位不足 20 个：没有结论，不能据此判断这种做法是否赚钱"
	sampleEnufZh = "拆分的仓位有 20 个或更多"

	gapNoReaderZh      = "没有账本读取器"
	gapNoFlexRowsZh    = "该账户没有 Flex 记录"
	gapNoSessionsZh    = "没有交易日读取器，页面无法判断交易日"
	gapNoQuoteReaderZh = "没有存档报价读取器，页面无法拆分盈亏"

	circularNoteZh  = "未结束仓位的核对是循环的：两边用的都是我们存档的 mid，所以它核对的是成交、佣金和费用，不是市值"
	noBookLineZh    = "这个币种没有可以拆分的仓位"
	noQuoteDaysZh   = "没有存档报价日"
	notComputedZh   = "未计算"
	brokerDiffersZh = "不一致（原因未解释）"

	classNoMarkZh   = "没有存档报价"
	classNoCloseZh  = "没有存档收盘价"
	classNoLedgerZh = "没有存档账本盈亏"
	classRefusedZh  = "输入被拒绝"

	leftOutHeadZh = " 个未计入合计："
	endedLotsZh   = " 个已结束的仓位，"
	splitLotsZh   = " 个已拆分的仓位："
	leftOutOfZh   = "共 "
	leftOutLotsZh = " 个仓位，其中 "

	ledgerSepZh   = "，账本 "
	countUnitZh   = " 个"
	lotsOpenZh    = "的仓位（"
	semiZh        = "；"
	closeZh       = "）"
	noQuoteLotsZh = " 个没有存档报价的仓位（账本 "
	unreconLotsZh = " 个未对上的仓位（账本 "
)
