package option_result_page

// Chinese texts of the page (docs/design/option-pages-r7.md#10).
const (
	nothingText = "没有已平仓的期权头寸"
	noReader    = "没有读取器，无法显示"
	// leadMid sits between the heading and the alias of a lead; leadEnd ends it.
	leadMid        = "，账户 "
	leadEnd        = "：已平仓的期权，各币种分别列出，不相加"
	booksNote      = "券商数字取自 Flex 的 fifoPnlRealized，只作比较，不计入任何数字"
	noKeptNote     = "Retained ratio 没有数值：没有 sell-to-open 权利金可作除数"
	noRealizedNote = "Realized 或 Premium 没有数值"
	// assignment: premium, then shares, then the cost note.
	assignPremium  = "被行权权利金 "
	assignShares   = "，股数 "
	assignEnd      = "；成本仍为行权价，不扣除权利金"
	noneText       = "无"
	unpairedHead   = "未配对的平仓（不计入任何数字）"
	reviewHead     = "待核对的撤销或更正（不计入任何数字）"
	sameLine       = "与券商 realized 一致："
	differLine     = "与券商 realized 不同："
	noBrokerLine   = "没有券商数字："
	tradeUnit      = " 笔"
	emptyMonths    = "没有已平仓的月份"
	emptyRunning   = "没有累计结果"
	emptyEndings   = "没有已结束的头寸"
	endingRealized = "，realized "
	// reconcileGloss says what the two reconciliation figures compare; it names no cause.
	reconcileGloss = "对账说明：Same 表示该笔已实现盈亏与券商 Flex 的 fifoPnlRealized 相等；Differs 表示两者不相等。两者只作比较，不计入任何数字。"
	// scopeNote is followed by the account alias.
	scopeNote = "本页只含账户 "
	// countGloss says what a counted trade is on this page and that the pnl page counts something else.
	countGloss = "计数说明：本页的一笔指账户的一个已平仓头寸，认购与认沽合计，按方向和币种分列；期权盈亏页只计已结束的卖出认沽批次，两者口径不同，笔数可能不同。"
	// otherAccounts joins the scope note to the other aliases, which are joined by otherSep.
	otherAccounts = "；其他账户："
	otherSep      = "、"
)
