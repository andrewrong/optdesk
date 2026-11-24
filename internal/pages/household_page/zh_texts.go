package household_page

// The Chinese words of the family page.
const (
	// linkRisk is the header link to /risk, the risk page of the main account.
	linkRisk = "账户风险 Account risk"

	familyTitle    = "家庭合计"
	familySentence = "全家净值和现金是各账户最近一次采集的美元合计；没有计入的账户在表里留一行并写明原因，股票和期权请到单个账户页查看。"
	familyNetName  = "全家净值（计入 %d / %d 个账户）"
	familyCashRow  = "现金 %s，占合计净值 %s"
	viewFamily     = "全家"

	familyLabelOpen   = "账户（"
	familyLabelClose  = "）"
	familyWhoseMain   = "本人"
	familyWhoseOther  = "其他账户"
	familyCellNoData  = "未采集到"
	familyCellStale   = "采集过旧"
	familyCellNoRate  = "缺少汇率"
	familyCellError   = "读取失败"
	familyCellMissing = "数据不全"

	familyStaleText  = "最早数据日 "
	familyCashNote   = "现金分放在各账户，只担保本账户的卖出看跌；合计现金不是可共用的保证金"
	familyRateLine   = "日元账户按 USDJPY 汇率 %s（日期 %s，来源 %s）折成美元"
	familyDetailGap  = "家庭合计页不拆分这一块，请在单个账户页查看"
	familyCellDetail = "见单个账户"

	familyAccountsTitle   = "各账户"
	familyAccountsMeaning = "每个账户一行：净值和现金按美元，没有计入的账户写明原因；股票和期权见单个账户页。"
	familyHeadName        = "账户"
	familyHeadNet         = "净值"
	familyHeadCash        = "现金"
	familyHeadStock       = "股票"
	familyHeadOptions     = "期权"
	familyHeadDay         = "数据日"

	familyLineNoData     = "%s今天未采集到，合计不含"
	familyLineStale      = "%s今天未采集到，最近一次是 %s（超过 3 个交易日），合计不含"
	familyLineCurrency   = "%s的币种缺少美元汇率或暂不支持，合计不含"
	familyLineEarlierDay = "%s用的是 %s 的采集，早于最新交易日，已计入合计"
	familyLineReadError  = "%s读取失败，合计不含"
)

// textEarlierMark is the note of the household page for positions priced from an earlier mark day:
// the number of positions and the mark day.
const textEarlierMark = "有 %d 个持仓用的是 %s 的收盘标记，采集日尚无标记"

// The Chinese words of the held family page.
const (
	heldTitle        = "Held companies 持仓公司"
	heldIntro        = "把全家持有的股票按公司合并：哪些账户持有、值多少钱、占全家总资产几成、下次财报几号。"
	heldHintCompany  = "公司"
	heldHintAccounts = "持有账户"
	heldHintValue    = "市值（美元）"
	heldHintShare    = "占全家总资产"
	heldHintEarnings = "下次财报（距今天数）"
	heldNoteBasis    = "市值 = 持股数 x 最近一次收盘价（最多早 5 天，日元按全家同一个 USDJPY 折成美元）；总资产 = 各账户采集日的净值之和（含现金和期权），所以各行占比之和小于 100 %。"
	heldNoteShort    = "空头股票显示负数。"
	heldNoteOptions  = "只有期权、没有股票的公司不在此表。"
	heldNoteLeftOut  = "%s%s，其持仓不在上表"
	heldNoteReadFail = "%s的持仓读取失败，其股票不在上表"
	heldNoteOldMark  = "%d 行的收盘价早于采集日：%s"
	heldNoteNoDate   = "%d 行没有财报日期"
)

// The Chinese words of the family trend block.
const (
	familyTrendLabel      = "全家合计"
	familyTrendNoData     = "全家净值记录不足 2 天，画不出走势"
	familyTrendMany       = "账户超过 3 个，只画全家合计"
	familyTrendWindowText = "全家合计：%s 至 %s，共 %d 天。"
	familyTrendLeftOut    = "有 %s 天没有画点：当天至少一个账户没有净值记录。"
	familyTrendYen        = "日元账户按当天的美元兑日元汇率折成美元。"
)

// The Chinese words of the family risk blocks (design docs/design/household-risk.md). A term is explained
// next to it: 指派, 行权价, 缺口, 现金结算.
const (
	riskTitleObligations   = "家庭风险 · 期权义务 Option obligations"
	riskTitleCashAfter     = "家庭风险 · 指派后现金 Cash after assignment"
	riskTitleStress        = "家庭风险 · 压力测试 Stress test"
	riskTitleConcentration = "家庭风险 · 集中度 Concentration"

	riskMeaningObligations = "每张未平仓卖出看跌（short put）一旦被指派（对方要你按行权价买入股票），要付多少现金：行权价（约定的买入价）x 每张股数 x 张数，按公司合并，美元。"
	riskMeaningCashAfter   = "各账户现金减去本账户全部卖出看跌被指派所需现金，负数是缺口（现金不够的部分）。现金分放在各账户，不能互相担保。"
	riskMeaningStress      = "全部卖出看跌都被指派的情形；跌幅 -10 / -20 / -30 % 的档位还没有，因为没有存下标的最新价。"
	riskMeaningConc        = "若全部被指派，每家公司占全家净值几成：股票市值 + 被指派要买入的金额（行权价计）。注：现金结算（只收付现金、不交割股票）的指数期权仍按行权价全额计入，数字偏高。"

	riskNameSuffix   = " 卖出看跌"
	riskNoteAccounts = "；权利金 "
	riskNoteList     = "、"
	riskNoteCash     = "现金 %s − 需要 %s"
	riskNoteShort    = "；缺口：现金不够"
	riskNoteStock    = "股票 %s + 指派 %s"

	riskStressNeed  = "全部指派所需现金"
	riskStressCash  = "全家现金"
	riskStressAfter = "指派后全家现金"
	riskStressGap   = "压力测试：跌幅 -10 / -20 / -30 % 的档位还没有，因为没有存下标的最新价"

	riskGapRow      = "%s：数据不全，该行不显示"
	riskGapTotal    = "合计不显示：有一项数据不全"
	riskGapUnread   = "%s读取失败，不含在这一块的数字和合计里"
	riskGapNoMarks  = "没有行情读取器，算不出股票市值"
	riskGapNoAccts  = "没有计入的账户"
	riskGapSeparate = "；"

	riskLineNotCounted = "家庭风险各块不含 %d 个账户"
	riskLineShort      = "指派后现金：有 %d 个账户现金不够"
	riskLineHidden     = "家庭风险表只列前 60 行，另有 %d 行没有列出"
)

// The Chinese words of the account page: the sentence, the meaning line of each block and the texts of its gaps,
// warnings and empty states. A term of the block stays English; a verb keeps its type and order.
const (
	pageSentence = "净值、现金和期权义务取自最近一次采集；持仓市值需要价格，没有价格的持仓显示暂无数据。"

	meaningOverview      = "账户的净值（Net liquidation），取自最近一次采集。"
	meaningComposition   = "持有股票按采集日价格计的市值，及其占净值的比例。"
	meaningMovers        = "今天哪些持仓带动了变化；需要持仓价格。"
	meaningTrend         = "最近几个交易日的净值，每个交易日一个点。"
	meaningStress        = "全部卖出看跌被指派后的现金，以及标的下跌各档的情形。"
	meaningIncome        = "本月卖出看跌收到的权利金。"
	meaningMonthChange   = "净值从上月最后一个交易日到最近一次采集的变化。"
	meaningCashAfter     = "现金减去全部卖出看跌被指派所需的现金。"
	meaningObligations   = "每张未平仓卖出看跌被指派时要付的现金。"
	meaningConcentration = "单一标的占净值的比例，按采集日价格计。"
	meaningAllocation    = "持有股票按采集日价格计的市值。"
	meaningAccounts      = "本账户的净值、现金、股票和期权。"

	textStatementFrom   = "取自券商对账单，日期 "
	textStatementNote   = "；该账户没有实时网关。"
	textNoMovers        = "今天的变化需要两个交易日的价格：暂无数据"
	textNotBuilt        = "没有持有期来源：暂无数据"
	textMoversStaleLast = "最新价格日 %s 比本页日期早了 5 天以上"
	textMoversStalePrev = "较早价格日 %s 比最新价格日早了 5 天以上"
	textMoversNoRate    = "%s 当天或之前没有 USDJPY 汇率：净值变化需要它"
	textMissingQuote    = "需要持仓价格：暂无数据"
	textMonthSteps      = "交易结果需要持仓价格，所以各步骤暂无数据；只列出月初和月末"
	textNoRate          = "%s：%s 在 %s 当天或之前没有汇率"
	textRateDay         = " 日元金额按 %s 的 USDJPY 汇率折算。"
	textNoReader        = "没有读取器：暂无数据"
	textUnknown         = "未知账户"
	textNoCapture       = "该账户还没有采集"
	textStale           = "最近一次采集已超过 3 个交易日"
	textStaleLabel      = "账户数据日 "

	textNoMonthStart  = "%s 之前最后一个交易日没有净值采集：缺少月初"
	textNotInLedger   = "已存持仓里有 %d 张卖出看跌还不在期权账本中（没有 Flex 记录）：%s"
	textNotInSnapshot = "期权账本里有 %d 张卖出看跌不在持仓快照中：%s"
	textLedgerBoth    = "已存持仓里有 %d 张卖出看跌还不在期权账本中：%s；期权账本里有 %d 张不在持仓快照中：%s"

	noteNotInSnapshot = "不在持仓快照中"
	noteNotInLedger   = "有 %d 张卖出看跌还不在期权账本中：%s"
	noteStressSteps   = "-10%、-20%、-30% 的档位需要标的价格：暂无数据"

	labelAccount = "账户（"
	labelClose   = "）"
	labelDay     = "最近 "
	labelDays    = " 个交易日"

	heldTextNoRead = "没有读取器：暂无数据"
	heldTextNoEarn = "没有标的或财报读取器：每个下次财报日都缺"

	textNoStockHeld = "没有持有股票，无法按比例排序"

	textNoDataYet           = "暂无数据"
	textMissingInput        = "输入数据缺失：暂无数据"
	textUnsupportedCurrency = "缺少汇率或币种暂不支持：暂无数据"
	textNoData              = "没有数据：暂无数据"
)
