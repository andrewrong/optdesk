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
	// zhStartedFmt: "started 14:02 UTC".
	zhStartedFmt = "开始于 %[1]s UTC (started %[1]s UTC)"
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

// The lost reason and the late-source note of a cell: the English meaning beside each.
const (
	lostText = "已丢失：数据源只提供当天的数据 (lost: the source gives only the present)"
	lateText = "数据源晚一天发布：FRED（美联储数据服务）在次日才公布当天的点 (the source publishes a day later: FRED, the Federal Reserve data service, posts a day's point on the next day)"
)

// The data check block (docs/design/data-check.md, Decisions 12, 13): Chinese only; a rule id stands beside its
// label and a dataset id stands alone.
const (
	zhDataCheckTitle   = "数据检查"
	zhDataCheckNote    = "每天检查一次已保存的数据有没有错误的值和没有被发现的缺口；目前只报告，不修改任何数据。"
	zhDataCheckNone    = "还没有数据检查的结果。"
	zhDataCheckGapText = "无法读取数据检查的结果，所以不显示这一块，不代表没有问题；其余部分不受影响。"

	zhSevError   = "错误"
	zhSevWarn    = "缺口"
	zhSevUnknown = "无法判断"

	zhGlossSevError   = "错误：数据检查发现已保存的数据里有错误的值，却被当作好数据保存了。"
	zhGlossSevWarn    = "缺口：数据检查发现应该有的数据没有保存，或保存得不完整。"
	zhGlossSevUnknown = "无法判断：这条规则这次没有能力判断，例如读不到它需要的数据，不代表数据有问题。"
	zhTermNew         = "新"
	zhEvSep           = "，"
	zhGlossNew        = "新：这个问题在之前已经通知过的检查里没有出现过。"

	zhRuleWhyOther = "数据检查的一条规则发现了这个问题。"

	zhEvOKSame   = "%[1]d 个读数中 %[2]d 个与前一日相同"
	zhEvRowsLate = "%[1]d 行中 %[2]d 行在交易日之后才收到"
	zhEvOther    = "%d 个计数"
	zhEvNone     = "没有计数"
)

const (
	zhLabelReadingRepeat = "读数重复前一日"
	zhLabelOpenLate      = "期权开盘报价晚到"
	zhLabelCandidates    = "候选早于最新设置"
	zhLabelCaptureGap    = "采集缺口"
	zhLabelPlanAbsent    = "缺少计划"
	zhLabelGapUnclosed   = "缺口未补"

	zhWhyReadingRepeat = "一天的读数和前一天完全一样，通常说明源站没有更新，旧值被当作新值保存了。"
	zhWhyOpenLate      = "期权开盘报价在交易日之后才收到，用它算出的结果可能不是那一天的真实情况。"
	zhWhyCandidates    = "候选是在最新设置保存之前算出的，没有按现在的设置重算。"
	zhWhyCaptureGap    = "运行器记录里这一天有缺失或过期的键，这些数据没有保存。"
	zhWhyPlanAbsent    = "这个数据集在这个交易日没有任何计划记录，任务可能根本没有运行。"
	zhWhyGapUnclosed   = "任务记录了这个缺口，超过宽限时间仍然没有补上。"

	zhEvOK         = "%d 个读数正常"
	zhEvSame       = "%d 个与前一日相同"
	zhEvRows       = "共 %d 行"
	zhEvLate       = "%d 行晚到"
	zhEvHoursBehnd = "落后 %d 小时"
	zhEvKeys       = "共 %d 个键"
	zhEvMissing    = "缺失 %d 个"
	zhEvExpired    = "过期 %d 个"
	zhEvCells      = "%d 个格子"
	zhEvAgeHours   = "已经 %d 小时没有补上"
	zhEvReadFailed = "%d 次读取失败"
)
