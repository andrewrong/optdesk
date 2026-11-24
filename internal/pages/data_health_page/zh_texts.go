// zh_texts.go holds the Chinese hint texts of the repeated-failures table (docs/design/repeated-failures.md,
// Decision 8): string constants only, the one door of the English guard of cmd/ot.
package data_health_page

const (
	zhFailuresNote = "本表列出最近 14 个交易日中，至少 3 个不同交易日失败或返回了数据却一条也没保留的键，且该键最新一次完成的运行仍然是失败或空，不是成功：已经恢复的键不会出现。按天数、运行次数、最近日期从多到少排序，每个数据集最多列出 50 个键。"
	zhFailuresNone = "没有键符合规则。被中断或被截断的运行不在这里统计，它们在上面的网格里显示为 expired。"
	zhHiddenPrefix = "另有 "
	zhHiddenSuffix = " 个符合规则的键没有列出（每个数据集最多列出 50 个）。"
	zhGapText      = "无法读取重复失败的键，所以失败表是空的，不代表没有失败；覆盖网格不受影响。"

	zhClassRetryable  = "retryable：可重试的错误，例如源站暂时不可用或超时。同一个键在多个交易日反复出现，说明重试没有解决问题。"
	zhClassPermanent  = "permanent：永久性错误，重试不会改变结果，例如请求被源站拒绝或键本身无效。"
	zhClassUnknownCal = "unknown_calendar：任务使用的交易日历未知，运行无法确定交易日。"
	zhClassEmpty      = "empty：源站返回了数据行，但一条也没有保留。对没有新数据的源站，或自己检查结果的任务，这是正常现象；只有当该键同时在网格里是缺口时才值得查看。"
	zhClassOther      = "其他错误类别：运行完成时带有这个错误代码，详细原因只在日志里。"
)

// The gateway block (docs/design/gateway-health.md, Decision 12): the verdict key stays in each text with its
// meaning beside it. No text names an account or a port.
const (
	zhGatewaySessionUp   = "session_up：与网关的接口连接正常（session 指程序与网关之间的连接）。"
	zhGatewayRetrying    = "session_retrying：与网关的接口连接断了或暂时丢失，程序正在自动重连，不需要你操作。"
	zhGatewayFarmDown    = "farm_down：网关自己报告某个数据服务器（farm，即网关背后的行情或历史数据服务器）断了，程序与网关的连接仍然正常，所以重连没有用：请到这个网关重新登录。"
	zhGatewayUnreachable = "unreachable：这次没有从网关得到可用的状态（没有回答，或回答里的状态无法识别）；这不等于网关已经坏了。"
)

// The running lines above the grid (docs/design/scheduler-per-job.md, Decision 8): the same strings as the
// job console, the English meaning of each beside it.
const (
	// zhDoneOfFmt: "done 12 / 40".
	zhDoneOfFmt = "已完成 %[1]d / %[2]d (done %[1]d / %[2]d)"
	// zhRanUsuallyFmt: "ran X minutes, usually Y minutes".
	zhRanUsuallyFmt = "跑了 %[1]d 分钟，平常 %[2]d 分钟 (ran %[1]d minutes, usually %[2]d minutes)"
	// zhRanNoHistoryFmt: "ran X minutes, no history" (the job has no usual yet).
	zhRanNoHistoryFmt = "跑了 %[1]d 分钟，没有历史 (ran %[1]d minutes, no history)"
	// zhSlowNote: "slow, a notice only".
	zhSlowNote = "偏慢，仅提示 (slow, a notice only)"
)

// The glossary beside the legend of the grid (docs/design/data-health.md, Decision 12): each term stays English
// and its Chinese meaning follows it.
const (
	zhGlossOK         = "ok：运行器把这个键记为已保存；它不是数据行的数量。"
	zhGlossNoValue    = "no_value：源站回答说这个键没有数据。"
	zhGlossExpired    = "expired：运行失败、只完成一部分或中途崩溃后，运行器在窗口结束时关闭了这个键：这是真正的缺口，不是源站的回答。"
	zhGlossMissing    = "missing：运行器预期有这个键，但它不在。"
	zhGlossNotRun     = "not_run：运行器没有记录那一天的计划。"
	zhGlossNone       = "none：这个格子不提供补取。"
	zhGlossButton     = "button：这个格子可以用它的任务运行一次来补取；按钮上写明会启动哪些运行。"
	zhGlossNotRefetch = "not refetchable：没有按钮；格子下面的文字在源站只给出当前数据时写 lost（已丢失），其他情况写格子打印的其他原因文字。"
	zhGlossLedger     = "ledger：运行器的记录，按交易日列出它预期的键和已保存的键。"
	zhGlossSessionDay = "session day：该数据集的交易所日历开市的一天。"
	zhOverwriteNote   = "补取按键覆盖，不保留之前的值（a repair overwrites by key and keeps no earlier value）。"
	zhCoverageGapText = "无法读取运行器的覆盖记录（ledger），所以不显示任何数据集。"
)
