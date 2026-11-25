package home_companies

// The Chinese sentences of the companies card.
const (
	noteFirstRows = "只显示前 %d 家，共 %d 家公司。"
	sharpNoteText = "急剧波动：暂无读数。"

	noReaderText    = "未设置读取器"
	notComputedText = "尚未计算，随下一次数据运行给出"
	// missingInputText takes the stored reason word.
	missingInputText = "缺少输入（%s）"
	bandFewYearsText = "需要 5 年基本面数据，已存数据不足；载入 5 年后显示"
	noStoredText     = "无已存读数"
	notNumberText    = "读数不是数字"
	bandNotNumber    = "区间读数不是数字"
	// noCurrencyText takes the symbol and the market.
	noCurrencyText = "%s: 市场 %s 无币种"
	sharpGapText   = "暂无急剧波动读数"

	ivpNoHistoryText = "没有已存的自有 IV30 历史"
	ivpNoInputText   = "无输入，当日没有 IV30 读数"
	// ivpShortText takes the observation count.
	ivpShortText = "历史不足，自有 IV30 交易日 %d/252"
	ivpStaleText = "输入过期，自有 IV30 已过时"
	ivpBadText   = "输入有误，自有 IV30 历史不可用"
	// ivpOtherText takes the reason code.
	ivpOtherText = "自有读数缺失（原因 %d）"

	earningsNoIssuerText   = "无发行人"
	earningsNoneListedText = "暂无列出的日期"

	// sayText takes the row count and the count of rows with a price line.
	sayText = "持有公司：%d 家 · 其中 %d 家有价格线。"
)
