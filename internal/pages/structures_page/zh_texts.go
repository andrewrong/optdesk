package structures_page

// The reasons a multi-leg unit of kind other was made, one line each, written in the note of the page.
const (
	// reasonPutCallOneExpiry: a put and a call at one expiry.
	reasonPutCallOneExpiry = "同一到期日同时持有认沽和认购"
)

// The texts of the Ex-dividend cell of the list (Decision 7 of docs/design/option-pages-r8.md).
const (
	exNoReaderText = "没有除息数据来源"
	exPassedText   = "除息日已过，等待下次采集"
	exNoRowText    = "没有除息预期记录（该股可能不派息，或尚未采集）"
)

// The line of the page note that names the close behind each distance.
const (
	closeNotePrefix = "距现价所用价格："
	closeWord       = "收盘"
	printWord       = "最近成交"
	closeNoteSep    = "；"
)

// The sentence under the title of the page: the counts and the alias are put between the parts.
const (
	totalSentenceHead = "共 "
	totalSentenceMid  = " 个持仓结构，本周待决策："
	emptySentenceHead = "账户 "
	emptySentenceTail = " 当前没有持仓结构"
)

// The parts of the note line written by noteOf; the option term Flex stays English.
const (
	otherNote         = "未识别的结构，按单腿逐条列出"
	noReaderNote      = "账本或交易日读取器未连接（no_reader）"
	noCaptureNote     = "没有持仓快照，未与 Flex 比对"
	oldCaptureNote    = "持仓快照已超过 3 个交易日，未与 Flex 比对"
	noAccountNote     = "持仓快照读取器未连接（no_reader），未与 Flex 比对"
	captureOnlyPrefix = "尚未进入 Flex 的持仓："
	ledgerOnlyPrefix  = "Flex 中有而持仓快照缺失的持仓："
)

// The gloss of each recognised kind: what the legs are, nothing on premium. detail.go maps them by kind;
// the door allows const declarations only.
const (
	glossPutDebitSpread    = "买入较高行权价认沽、卖出较低行权价认沽，同一到期日"
	glossCallDebitSpread   = "买入较低行权价认购、卖出较高行权价认购，同一到期日"
	glossCalendarSpread    = "同一行权价、同一权利方向，买卖两腿到期日不同"
	glossDiagonalSpread    = "同一权利方向，买卖两腿行权价和到期日都不同"
	glossLongStraddle      = "同一到期日、同一行权价，同时买入认购和认沽"
	glossShortStraddle     = "同一到期日、同一行权价，同时卖出认购和认沽"
	glossLongStrangle      = "同一到期日，买入较低行权价认沽和较高行权价认购"
	glossShortStrangle     = "同一到期日，卖出较低行权价认沽和较高行权价认购"
	glossIronCondor        = "同一到期日，卖出内侧认沽和认购、买入外侧认沽和认购，共四腿"
	glossReverseIronCondor = "同一到期日，买入内侧认沽和认购、卖出外侧认沽和认购，共四腿"
	glossCollar            = "持有正股，买入认沽并卖出认购"
)

// The texts of the spread legs page (docs/design/spread-legs.md, Decision 10).
const (
	spreadsTitle        = "价差腿 Spread legs"
	spreadsGroupSpreads = "价差组合 Spreads"
	spreadsGroupOther   = "未识别组合 Unidentified"
	spreadsStaleHead    = "数据日 "
	spreadsNoReader     = "价差腿的账本、交易日读取器或计算器未连接（no_reader）"
	spreadsUnknownAcct  = "未知账户"
	spreadsEmptyHead    = "账户 "
	spreadsEmptyTail    = " 当前没有价差组合"
	spreadsSentenceHead = "共 "
	spreadsSentenceMid  = " 个组合，另有 "
	spreadsSentenceTail = " 个单腿持仓不在此页（见“持仓结构”页）；按到期天数由近到远排列。"
	spreadsNote         = "净收入 = 各腿（-数量 x 开仓价 x 合约乘数）之和，只含开仓价，不含手续费和平仓。"
	spreadsLegHeads     = "腿列：数量 Qty（负数为卖出）、行权价 Strike、开仓价 Open price（成交时的价格）。"
	spreadsExpiryHead   = "到期 "
	spreadsDaysOpen     = "（"
	spreadsDaysClose    = " 天）"
	spreadsCreditUnit   = " 美元"
	spreadsBreakJoin    = " 和 "
	spreadsNoOpenPrice  = "无开仓价"
	spreadsUnnamedLine  = "匹配器未能命名这组腿"
	spreadsReasonOpen   = "（"
	spreadsReasonClose  = "）"
)

// The gap texts of a figure cell, by reason.
const (
	gapTextMissingInput = "有一条腿缺开仓价"
	gapTextUnsupported  = "非美元，此页不算"
	gapTextNoRule       = "此结构没有这项规则"
	gapTextNoData       = "暂无数据"
	gapTextCallMaxLoss  = "无上限规则（含股票，此页不算）"
	gapTextCallBreak    = "没有盈亏平衡价（含股票，此页不算）"
)

// The reason sentences of gapTexts, by reason code (cells.go).
const (
	gapTextMissingQuote = "没有报价"
	gapTextStaleQuote   = "报价已过期"
	gapTextForeign      = "非美元合约，未换算"
	gapTextNoRuleFigure = "此结构没有这项数据"
	gapTextNoDataReason = "暂无数据"
	gapTextMissingIn    = "缺少输入"
	gapTextNoReader     = "读取器未连接"
	gapTextNoLimit      = "无上限"
)

// The captions of the tiles of the detail page and the break-even sentences of breakEvenCaption.
const (
	captionPnL             = "按中间价估值"
	captionAtExpiry        = "到期时"
	captionExpiryPrice     = "到期时标的价格"
	captionLosesBelow      = "到期价低于此价时亏损"
	captionLosesAbove      = "到期价高于此价时亏损"
	captionProfitsBelow    = "到期价低于此价时盈利"
	captionProfitsAbove    = "到期价高于此价时盈利"
	captionNoBreakEven     = "没有单一盈亏平衡价"
	captionLegByLeg        = "逐腿列出，没有单一盈亏平衡价"
	hintStrike             = "合约的行权价"
	hintShortStrike        = "卖出腿的行权价"
	hintLongStrike         = "买入腿的行权价"
	hintBreakEven          = "到期盈亏为零时的标的价格"
	hintOtherLevel         = "关键价位"
	notOpenText            = "该结构已不在持仓中"
	cannotReadText         = "无法读取持仓结构"
	capNoteHead            = "只显示前 "
	legsNoteTail           = " 条腿"
	fillsNoteTail          = " 条成交记录"
	fillsCannotBeShownText = "部分成交记录无法显示"
)

// The sides of a leg.
const (
	sideShortPut  = "卖出认沽 Short put"
	sideLongPut   = "买入认沽 Long put"
	sideShortCall = "卖出认购 Short call"
	sideLongCall  = "买入认购 Long call"
	sideOther     = "腿 Leg"
)

// The kind names with their gloss.
const (
	kindPutSpread         = "认沽价差 Put spread"
	glossPutSpread        = "卖出价格较高的认沽、买入价格较低的认沽"
	kindCallSpread        = "认购价差 Call spread"
	glossCallSpread       = "卖出价格较低的认购、买入价格较高的认购"
	kindPutDebitSpread    = "认沽借方价差 Put debit spread"
	kindCallDebitSpread   = "认购借方价差 Call debit spread"
	kindCalendarSpread    = "日历价差 Calendar spread"
	kindDiagonalSpread    = "对角价差 Diagonal spread"
	kindLongStraddle      = "买入跨式 Long straddle"
	kindShortStraddle     = "卖出跨式 Short straddle"
	kindLongStrangle      = "买入宽跨式 Long strangle"
	kindShortStrangle     = "卖出宽跨式 Short strangle"
	kindIronCondor        = "铁鹰 Iron condor"
	kindReverseIronCondor = "反向铁鹰 Reverse iron condor"
	kindCollar            = "领口 Collar"
	kindCoveredCall       = "备兑认购 Covered call"
	glossCoveredCall      = "持有正股、卖出认购"
	kindOther             = "未识别组合 Unidentified"
	kindShortPut          = "卖出认沽 Short put"
	kindNakedCall         = "裸卖认购 Naked call"
	kindLongPut           = "买入认沽 Long put"
	kindLongCall          = "买入认购 Long call"
	moneyIn               = "价内 In the money"
	moneyOut              = "价外 Out of the money"
	moneyAt               = "平值 At the money"
	glossOther            = "匹配器未能命名的一组期权腿"
	kindUnknown           = "其他结构"
	glossUnknown          = "此类型没有说明"
)

// The Chinese parts of the titles of the list and detail pages; each is written with the English phrase beside it.
const (
	zhPageTitle   = "持仓结构"
	zhDecideTitle = "本周待决策"
	zhOthersTitle = "其他结构"
	zhLegsTitle   = "腿明细"
	zhPayoffTitle = "到期盈亏"
	zhGreeksTitle = "希腊值"
	zhLevelsTitle = "关键价位"
)
