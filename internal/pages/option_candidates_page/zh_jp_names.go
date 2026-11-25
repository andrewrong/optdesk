package option_candidates_page

// Chinese texts of the outcome of a Japanese name with no candidate row (sellput-jp-outcomes.md Decision 4):
// string constants only.
const (
	nameNoQuoteZh     = "交易所无报价（NO_QUOTE：没有人挂单，不是出错）"
	nameFailedZh      = "取数失败（可重试）"
	nameNoSpotHeadZh  = "没读到标的现价（"
	nameNoSpotTailZh  = "），所以没有询价"
	nameDeferredZh    = "未轮到取数（超出本次上限或时间，不是出错）"
	nameNoCandidateZh = "有报价，但没有合格的候选"
	nameNoRowsZh      = "没有这个名字的期权链行（不在今日取数计划里，或今日取数未运行）"
	nameOtherHeadZh   = "其他状态（"
	nameOtherTailZh   = "）"

	nameCountSepZh      = "；"
	nameCountNoQuoteZh  = "交易所无报价 "
	nameCountFailedZh   = "取数失败 "
	nameCountNoSpotZh   = "没读到现价 "
	nameCountDeferredZh = "未取 "
	nameCountOkZh       = "有报价 "
	nameCountOtherZh    = "其他 "
	nameCountUnitZh     = " 个合约"
	nameCountNoneZh     = "没有行"
	nameCodeSepZh       = "、"
)
