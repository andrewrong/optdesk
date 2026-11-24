package price_lines_page

// The explanation, hint and gap texts of the page are Chinese (owner rule 2026-11-17); the title, the labels and
// the option terms stay English.
const (
	sentenceText = "为每个持有的标的手填买入价。只有没有估值买入区间的标的，卖出认沽的闸门才会读取这个价格。"
	introText    = "保存的价格在下一次期权候选运行时生效；清除手填价格，只会取消没有估值的标的的检查。"
	addHintText  = "请输入标的清单中某个标的的代码和市场。"
	footnoteText = "该价格即标的页面上该标的的 buy_price；备注与标的页面共用。"
	noReaderText = "此处无法读取标的清单。"
)
