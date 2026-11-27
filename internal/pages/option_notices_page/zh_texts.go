package option_notices_page

// Chinese explanation texts of the page.
const (
	// noteHint explains the no-guard note.
	noteHint = "没有 guard 记录：持有 call 的 guard 在快照当天没有为该账户存储判定，因此没有检查任何空头 call 的提前行权或财报风险。"

	// cushionHintHead, cushionHintMid and cushionHintTail wrap the two thresholds of the cushion hint.
	cushionHintHead = "Cushion：券商存储的 Cushion 值。低于或等于 "
	cushionHintMid  = " 时提醒，低于或等于 "
	cushionHintTail = " 时需要处理。"

	// cashGlossZh explains the cash-due table.
	cashGlossZh = "如果当天到期的所有空头 put 都被行权（Assigned，即你卖出的 put 的买方要求你按行权价买入股票），这就是你买入股票需要支付的现金（行权价 strike × 合约乘数 multiplier × 数量 quantity）。"
)

// reasonTextsZh are the sentences of the known guard reason codes.
const (
	reasonEarningsZh   = "财报日在到期日之前"
	reasonAssignmentZh = "除息日之前有提前行权风险"
	reasonFloorZh      = "行权价低于卖出价格下限"
	reasonCashShortZh  = "剩余现金 = 可用现金 - 到这一天为止所有到期的空头 put 被行权（Assigned，买方要求你按行权价买入股票）需要的现金；是累计的，不是只算这一天。"
)

// Titles and labels read Chinese first with the English beside.
const (
	pageTitle           = "期权提醒 Option notices"
	titleExpiryToday    = "今日到期 Expires today"
	titleExpiryNear     = "即将到期 Expiring soon"
	titleCallGuard      = "持有的 call 被 guard 拦下 Held call blocked by the guard"
	titleEarningsInside = "财报日在到期日之前 Earnings date is before expiry"
	titleAssignmentSeen = "疑似被行权 Suspected assignment"
	titleCushionLow     = "Cushion 偏低 Cushion is low"
	titleCaptureStale   = "持仓快照过旧 Position snapshot is too old"
	titleCashShort      = "现金不足 Cash is short"

	labelEarnings    = "财报日 Earnings date"
	labelCaptureDay  = "快照日 Snapshot day"
	labelSessionDays = "相隔交易日 Trading days apart"
	labelCurrency    = "币种 Currency"
	labelAvailable   = "可用现金 Cash available"
	labelDue         = "所需现金 Cash needed"
	labelLeft        = "剩余现金 Cash left"
	labelShortDays   = "不足的天数 Days short"
)
