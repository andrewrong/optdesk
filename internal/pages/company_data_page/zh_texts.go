package company_data_page

// Chinese texts of the page (owner rule 2026-11-17): constants only. Reason codes stay English.

// Column heads and glossary terms: Chinese first, English beside.
const (
	labelFiscalYearEnd = "财年结束月 Fiscal year end"
	labelPriceTo       = "价格更新至 Price up to"
	labelBarCount      = "日线根数 Daily bars"
	labelStatements    = "财报数据 Statements"
	labelLatestQuarter = "最新季度 Latest quarter"
	labelEPSQuarters   = "EPS 季度数 EPS quarters"
	labelNextReport    = "下次财报 Next report"
	labelMissing       = "缺失 Missing"
)

// Gloss under each column head.
const (
	glossFiscalYearEnd = "公司财政年度结束的月份"
	glossPriceTo       = "公司已存的最新一根日线所在的日期"
	glossBarCount      = "最近 400 天里已存的日线根数"
	glossStatements    = "是否至少有一个季度的已披露数据"
	glossLatestQuarter = "有数据的最新财季，以财年和季度命名"
	glossEPSQuarters   = "有稀释每股收益数据的季度数"
	glossNextReport    = "来源列出下一次财报日期时，显示该日期"
	glossMissing       = "公司缺哪些数据，以及原因"
)

// Text of each glossary entry.
const (
	textFiscalYearEnd   = "公司财政年度结束的月份，取自它的 12 个月数据。"
	textPriceTo         = "公司已存的最新一根日线所在的日期。"
	textDailyBars       = "最近 400 天里已存的日线（一天的开盘、最高、最低、收盘）根数。"
	textStatements      = "营收、净利润或每股收益至少有一个季度的数据时为“是”。"
	textLatestQuarter   = "有数据的最新财季，以财年和季度命名。"
	textEPSQuarters     = "有稀释每股收益（每股净利润）数据的季度数。"
	textNextReport      = "来源列出下一次财报日期时显示该日期，没有列出时为空。"
	textReasonJP        = "日本公司：没有接财报来源，这是有意不做的。"
	textReasonNoCIK     = "美国公司没有 SEC 编号（cik），请到标的页填写。"
	textReasonFundOff   = "公司的财报开关是关的，所以不抓取申报文件。"
	textReasonNoStmts   = "没有存任何 US-GAAP 数据：要么是 IFRS 申报公司，其季度数据有意不做，要么是每日任务还没存进来。"
	textReasonAnnual    = "该公司一年只申报一次（表格 20-F 或 40-F），所以没有季度数据。"
	textReasonNoInstr   = "券商还没有确认这家公司（没有 con_id），所以无法采集日线。"
	textReasonNoBars    = "公司已确认，但还没有存任何日线。"
	textReasonNoNextRep = "没有列出这家公司的财报日期，仅供参考。"
)

// Empty state and per-company gap.
const (
	textUnderlyings = "读不到公司列表，所以不显示任何行。"
	textReadFailed  = "：读不到已存的数值，所以这家公司没有行。"
)
