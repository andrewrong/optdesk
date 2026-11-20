package structures_page

// The reasons a multi-leg unit of kind other was made, one line each, written in the note of the page.
const (
	// reasonPutCallOneExpiry: a put and a call at one expiry.
	reasonPutCallOneExpiry = "同一到期日同时持有认沽和认购"
)

// The texts of the Ex-dividend cell of the list (Decision 7 of docs/design/option-pages-r8.md), set on the gap
// directly: gapTexts holds English.
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
