package option_pnl_page

// Explanation texts in Chinese (owner request 2026-11-17); labels and numbers stay English.
// Door for Chinese in page code: string constants only. Design: docs/design/candidates-tiers.md#Decisions.
const (
	noQuotesZh        = "该合约在仓位持有期间没有存档报价（报价从 2026-11-07 起才开始存档），所以无法拆分，只显示账本数字。"
	leftOutNoQuotesZh = "多为 2026-11-07 前已结束的仓位，当时还没有报价存档"
	longPutSkippedZh  = "本页只显示卖出的看跌期权，已跳过买入的看跌期权仓位"
)
