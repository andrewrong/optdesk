package option_result_page

// Chinese texts of the page (docs/design/option-pages-r7.md#10).
const (
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
