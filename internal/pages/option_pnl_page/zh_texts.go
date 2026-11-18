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
