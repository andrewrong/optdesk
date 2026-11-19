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
