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
