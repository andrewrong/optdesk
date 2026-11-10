//nolint:unused // used by the page builders of entries 3 and 4 of docs/design/option-page-builders.md
package option_boards

// Texts of the shell of both pages. Option terms stay English, other text is Chinese (option-pages-v22
// Decision 6).
const (
	optionTitle      = "期权"
	stockTitle       = "单股期权"
	candidatesSource = "candidates"

	jpNote         = "日本期权:后续设计 (option-candidates-jp)"
	gapNoReader    = "没有候选读取器,无法显示"
	gapNoRows      = "没有候选行,候选任务尚未存入"
	staleDayPrefix = "数据日 "
	staleDaySuffix = ",早于今日"
	gapStale       = "候选行不是当日的"

	verdictWorth   = "值得看"
	verdictPending = "数据不全"
	verdictSkip    = "不够格"
	gatesPassed    = "各关卡通过"

	ivUnknown   = "缺"
	earnAbsent  = "E 无"
	earnMissing = "E 缺"

	noCardText = "没有该标的的候选行"
	footerText = "由存档收盘报价与候选表生成"
	policyText = "策略 sell put,买价线内才卖"
	trendText  = "趋势 缺"
)
