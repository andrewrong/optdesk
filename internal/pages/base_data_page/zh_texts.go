package base_data_page

// Chinese texts of the page (docs/design/base-data-page.md, Decision 8): constants only. Builder.Hint looks them up.

// Hints of the rules, by rule name; a rule name is the same on every row that has it.
const (
	hintRows            = "已检查的数据行数；为 0 表示这一项没有任何可用数据，判为失败。"
	hintNewest          = "最新一条数据的值、日期和对象；只是展示，不参与判定。"
	hintCoverageGap     = "最近 30 个交易日里，台账记录“应有但缺失”的天数。缺失的天数越多越需要补采，到「采集记录」页修复。"
	hintCoverageLatest  = "最近一个已收盘交易日里，缺失或过期的键占全部键的比例。"
	hintAge             = "最新数据距上一个已收盘交易日过了几个交易日（美股按纽约、日股按东京的交易日历）；没有交易日历的数据按自然日计。"
	hintOhlcOrder       = "日线的最高价低于最低价，或最高、最低价没有包住开盘、收盘价：这样的 K 线不可能存在，说明数据损坏。"
	hintBarJump         = "收盘价与上一根相比变动超过 40%。系统里没有存储拆股信息，所以拆股（split）看起来正是这个样子；只提醒，不判失败。"
	hintCrossed         = "买价高于卖价的行占比。少量属于取快照瞬间的正常现象，占比过高说明报价有问题。"
	hintEmpty           = "整天都没有取到报价的合约占比（远离平价的行权价常常没有报价）。"
	hintErrors          = "取数出错、整天没有成功报价的合约占比。"
	hintNonLive         = "不是实时报价（冻结或延迟）的行占比。收盘后取的快照按设计就是冻结的，请结合时间阅读。"
	hintBadIV           = "隐含波动率小于等于 0 或大于 500% 的行，数值不合理。"
	hintSeriesAge       = "FRED 序列里，最新观测日期比其更新频率允许的天数更旧的序列数。"
	hintFxBand          = "美元兑日元汇率落在 80 到 250 之外，超出合理范围。"
	hintFxJump          = "汇率相对上一个日期变动超过 5%，可能是数据错误。"
	hintEarningsPast    = "财报日期已经过去、却还作为“即将发布”存放的公司数，说明日期没有更新。"
	hintDividendPast    = "预计除息日已经过去、却没有更新的公司数。"
	hintMacroExhausted  = "某类宏观事件已经没有今天及以后的日期，说明日历用完了需要补充。"
	hintCalendarHorizon = "交易日历只存到不足 30 天之后，需要补充未来的交易日。"
	hintPeriodOrder     = "财报期间的结束日早于开始日，数据矛盾。"
	hintSubjectStale    = "最新一份财报提交日期过旧（超过 135 天）的公司占比。"
	hintCaptureAge      = "账户快照距上一个已收盘交易日过了几个交易日。"
	hintNoNetliq        = "最新一次账户快照里没有净清算价值（NetLiquidation）的账户数。"
	hintPositionExpired = "已经过了到期日、仍然记作持仓的期权，说明持仓没有更新。"
	hintFlexAge         = "券商 Flex 对账单最新记录距今的自然日数。"
	hintFlexSection     = "Flex 对账单里第二版需要的部分在最近一次下载中缺失；只是设置提示，不参与判定。"
	hintNotRead         = "这一项所在的数据视图读取失败或超时，没有被判定。"
	hintFallback        = "这一项没有中文说明。"
)

// Hints of the rows: a short name and what the dataset is.
const (
	rowBars      = "日线：美股和日股的每日开高低收价。"
	rowQuotes    = "报价快照：期权合约在取数时刻的买卖报价。"
	rowChains    = "期权链（收盘）：每个交易日收盘后存下的期权链，含美股、日股和指数。"
	rowOpen      = "开盘期权报价：开盘时取的期权报价。"
	rowContracts = "期权合约表：每个标的有哪些合约的清单与条款。"
	rowRates     = "利率序列：来自 FRED 的利率与流动性序列。"
	rowFx        = "汇率：美元兑日元的每日汇率。"
	rowEvents    = "事件：财报日期、预计除息日和宏观事件日历。"
	rowCalendar  = "交易日历：美股和日股的开市日。"
	rowFacts     = "公司基本面：财报里取出的财务数据。"
	rowAccount   = "账户快照：券商账户的持仓、现金和净值。"
	rowFlex      = "Flex 对账单：券商出具的成交记录。"
)

// Top line and gaps.
const (
	topTextFormat   = "基础数据共 %d 项：%d 项正常，%d 项警告，%d 项失败，%d 项未知。"
	topTextNoFacts  = "基础数据暂时无法读取。"
	derivedNoteText = "派生数据（读数、候选、策略、计划）不在这里检查：请到「采集记录」页查看。"
)

// Glossary: a term and its explanation.
const (
	termPass     = "pass"
	textPass     = "正常：所有检查通过。"
	termWarn     = "warn"
	textWarn     = "警告：有检查越过了提醒线，但还没到失败。"
	termFail     = "fail"
	textFail     = "失败：至少一项检查越过了失败线，请到「采集记录」页修复。"
	termUnknown  = "unknown"
	textUnknown  = "未知：没有可用的事实来判定，或者存放它的数据视图读取失败。"
	termSession  = "session"
	textSession  = "交易日：交易所开市的一天。年龄里的 s 就是交易日数。"
	termLedger   = "ledger"
	textLedger   = "台账：运行器记录的每个交易日应当采集的键和已经存下的键。"
	termTrial    = "trial"
	textTrial    = "试用值：我们自己定的阈值，读到真实数据后再调整；def 表示定义上就不能违反。"
	termShare    = "share"
	textShare    = "占比：出问题的行数除以检查的行数。"
	termSplit    = "split"
	textSplit    = "拆股：一股拆成几股，价格会一次性大幅下跌，看起来像是数据跳变。"
	termCrossed  = "crossed"
	textCrossed  = "买卖倒挂：买价高于卖价。"
	termNonLive  = "non_live"
	textNonLive  = "非实时：行情是冻结或延迟的，不是当时的实时报价。"
	termCoverage = "coverage"
	textCoverage = "覆盖：计划采集的键里实际存下了多少。"
)
