package household_page

// The Chinese words of the family page.
const (
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
	heldNoteOldMark  = "%d 行的收盘价早于采集日"
	heldNoteNoDate   = "%d 行没有财报日期"
)
