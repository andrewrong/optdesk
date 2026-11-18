package option_candidates_page

// Texts of the tier view (candidates-tiers.md Decision 11): labels and every number are English, the
// explanations and glosses are Chinese. A value in braces is filled by the page from the answer.
const (
	tierFitsWord  = "Fits"
	tierCheckWord = "Check"
	tierGreyWord  = "Not judged"

	tierFitsID  = "fits"
	tierCheckID = "check"
	greyIDHead  = "grey_"

	iconCheck = "check"
	iconAlert = "alert"
	iconMinus = "minus"

	titleFitsTail  = " fit your conditions"
	titleCheckTail = " to check"

	explainFitsHead  = "满足你的全部条件：已设买入价；保本点（行权价减 bid）不高于买入价；价外；|delta| 不高于 "
	explainFitsMid   = "（"
	explainFitsClose = "）；"
	explainFitsTail  = "。这不是买入建议。"
	deltaSourceSet   = "试用值，可在设置里改"
	deltaSourceFixed = "默认值"
	cashClauseBuilt  = "现金够付全额行权价（已扣除已卖出的 put 占用的行权价）"
	cashClauseMark   = "现金只做标记，现有数据算不出全额行权价覆盖"
	explainCheck     = "已设买入价、保本点不高于买入价、价外这三项都满足，但有需要再看一眼的徽标。"
	explainGrey      = "未评判，已折叠，只显示数量；点开可看每一行为什么在这里。"

	thinSentenceHead = "|delta| 低于 "
	thinSentenceTail = " 时权利金薄，仍然满足条件。"
	cashBasisHead    = "现金：每一行单独对照一个账户的可用现金（已扣除已卖出的 put 占用的行权价），截至 "
	cashBasisTail    = "；不在各行之间扣减，也不替你选账户。"
	cashAgeHead      = "现金记录最多早 "
	cashAgeTail      = " 天。"
	cashNoFigure     = "现金：现有数据算不出全额行权价覆盖，只做标记（cash_unknown）。"

	commonHead = "本次运行的每一行都有："
	commonMid  = "。"
	commonTail = "这不会改变任何一行的档位。"

	keptHead      = "未进入 Fits 的原因（行数）："
	keptSeparator = "；"
	keptEvery     = "每一行"
	keptEllipsis  = "…"
	keptMaxChars  = 200

	legendFitsTitle  = "✓ Fits"
	legendCheckTitle = "! Check"
	legendGreyTitle  = "– Not judged"
	legendCashTitle  = "Account cash"
	legendCashTier   = "cash"
	legendGreyTier   = "grey"
	legendCountSep   = "："
	legendCountTail  = " 行。"

	dayHint = "；数据日 "

	notSet          = "not set"
	earningsUnknown = "unknown"
	noSpotText      = "no spot"
	countFit        = " fit, "
	countCheck      = " check, "
	countCollapsed  = " collapsed"
	unreadableGates = "unreadable_gates"
	dteSuffix       = "d"
	noValue         = "not formed"
	movesSuffix     = " moves*"
	yieldSuffix     = "%/yr"
	beHead          = "BE "
	beVs            = " vs "
	lineSep         = "; "
	lineBid         = "Bid "
	lineYield       = "yield "
	lineIntrinsic   = "intrinsic "
	lineTimeValue   = ", time value "
	lineBEGap       = "BE: "
	gapNoIV         = "no IV"
	gapNoBid        = "no bid"
	gapNoYield      = "no yield"
	fitsPrefix      = "fits: "
	fitsNone        = "fits: none"
	aliasSep        = ": "
	cashEnough      = "enough, "
	cashShort       = "short, "
	cashFree        = " free after "
	cashCommitted   = " committed ("
	cashNoFig       = "no figure: "
	gapNoAccount    = "No account cash figure"
	earningsLabelSp = " "
	badgeToneWarn   = "warn"
	badgeToneInfo   = "info"
	badgeToneNone   = "none"
	evBadgeCode     = "evidence"
	cashBadgeHead   = "cash_"
	cashBadgeGloss  = "各账户的现金情况：每一行单独对照，不在各行之间扣减。"
	unknownGloss    = "未收录的标记："
)

// Column and field labels of the tier view.
const (
	colArrowID, colArrowLabel       = "arrow", "Details"
	colTierID, colTierLabel         = "tier", "Tier"
	colContractID, colContractLabel = "contract", "Contract"
	colMarginID, colMarginLabel     = "margin", "Margin of safety"
	colBelowSpotID, colBelowLabel   = "below_spot", "Below spot"
	colPremiumID, colPremiumLabel   = "premium", "Premium / yield"
	colBadgesID, colBadgesLabel     = "badges", "Badges"

	hintTier     = "档位：满足条件、需要再看，或未评判。"
	hintContract = "合约：标的、行权价、到期日和剩余天数。"
	hintMargin   = "安全边际：保本点（行权价减 bid）比你的买入价低多少，占买入价的百分比。"
	hintBelow    = "距现价：保本点比现价低多少；moves* = (现价 − 保本点) / (现价 × 本合约 IV × sqrt(年数))，即几个预期波动。"
	hintPremium  = "权利金：bid 乘以乘数得每份合约的钱；年化收益只按时间价值除以行权价计算。"
	hintBadges   = "徽标：需要再看一眼的地方；琥珀色的会让这一行落到 Check。"

	fieldIVID, fieldIVLabel           = "iv", "IV (this contract)"
	fieldDeltaID, fieldDeltaLabel     = "delta", "Delta (rough chance of assignment)"
	fieldITMID, fieldITMLabel         = "itm_probability", "ITM probability"
	fieldVsCashID, fieldVsCashLabel   = "vs_cash", "Vs cash"
	fieldCashID, fieldCashLabel       = "account_cash", "Account cash"
	fieldBidID, fieldBidLabel         = "bid_per_share", "Bid per share"
	hintIV                            = "本合约自己的隐含波动率，不是公司的整体 IV"
	hintDelta                         = "|delta| 约等于被指派的粗略概率，只是粗略估计"
	hintITM                           = "模型估计的到期价内概率，不是被指派的概率"
	hintVsCash                        = "存入的年化收益与短期国债利率的差，试用口径"
	hintCash                          = "每个账户的可用现金，已扣除已卖出的 put 占用的行权价"
	hintBid                           = "收盘 bid，下一个交易日卖出的估计价，不是成交价"
	deltaOutsideBand                  = "delta_outside_band"
	reasonStaleCash, reasonNoCash     = "stale cash", "no cash"
	reasonNotUSD, reasonOtherCurrency = "not USD", "other currency"
)

// greySpec is the title tail, the word of the kept line and the explanation of one grey reason.
type greySpec struct{ titleTail, keptLabel, gloss string }

var greySpecs = map[string]greySpec{
	"no_buy_price":    {" not judged: no buy price", "没有买入价", "没有设买入价，无法判断保本点是否合适；不是被隐藏，只是没评判。"},
	"no_spot":         {" not judged: no spot", "没有现价", "没有现价，无法判断是否价外。"},
	"in_the_money":    {" in the money", "价内", "价内：行权价高于现价，卖出后几乎一定被指派（按行权价买入股票）；权利金里含内在价值，收益只按时间价值算。"},
	"no_bid":          {" not judged: no bid", "没有 bid", "没有 bid，无法算保本点。"},
	"above_buy_price": {" break-even above buy price", "保本点高于买入价", "保本点（行权价减 bid）高于你的买入价：被指派后的成本高于你愿意付的价。"},
	"bad_input":       {" not judged: unreadable numbers", "数据读不出来", "数据读不出来，没有评判。"},
}

const fallbackGrey = "bad_input"

// badgeTexts are the English label and the Chinese gloss of each badge, note and info mark.
var badgeTexts = map[string]struct{ label, gloss string }{
	"no_open_interest":        {"No open interest", "未平仓量没有记录或为零，可能难以成交。"},
	"low_open_interest":       {"Low open interest", "未平仓量低于设置的最小值，成交可能困难。"},
	"one_sided_quote":         {"One-sided quote", "只有单边报价，看到的价格不一定能成交。"},
	"wide_quote":              {"Wide quote", "买卖价差过大，实际成交价可能比 bid 差。"},
	"stale_quote":             {"Stale quote", "报价延迟或不是最新交易日，数字可能已变。"},
	"earnings_before_expiry":  {"Earnings before expiry", "到期前有财报，财报日股价可能大幅跳空；存的行不区分日期是否已人工确认。"},
	"cash_short":              {"Cash short", "现金不够付全额行权价（已扣除已卖出的 put 占用的行权价）。"},
	"delta_above_bound":       {"Delta above bound", "delta 高于你设的上限，被指派的概率较高（粗略等于 |delta|）。"},
	"delta_unknown":           {"Delta unknown", "没有存 delta，无法核对上限。"},
	"thin_premium":            {"Thin premium", "|delta| 低于 0.15，权利金薄，仍然满足条件。"},
	"earnings_unconfirmed":    {"Earnings unconfirmed", "财报日来自数据商，没有人工核实，不改变档位。"},
	"earnings_beyond_horizon": {"Earnings beyond horizon", "近期刚报过财报，到期前不会再报，不改变档位。"},
	"earnings_unknown":        {"Earnings unknown", "没有可用的财报日，不改变档位。"},
	"cash_unknown":            {"Cash unknown", "没有现金数字，现金只做标记。"},
}

// cashWhyTexts are the words of the reason an alias has no cash figure.
var cashWhyTexts = map[string]string{
	"stale_cash": reasonStaleCash, "no_cash": reasonNoCash, "not_usd": reasonNotUSD, "other_currency": reasonOtherCurrency,
}

// whyTexts are the gap texts of a figure the answer could not form, by its reason code.
var whyTexts = map[string]string{
	"no_buy_price": gapNoBuy, "no_iv": gapNoIV, "no_spot": noSpotText, "no_bid": gapNoBid, "in_the_money": "in the money",
}
