// zh_texts.go holds the Chinese strings of the running-now and already-running lines
// (docs/design/scheduler-per-job.md, Decisions 5 and 8): string constants only, the one door of the English
// guard of cmd/ot. The English meaning of each is beside it, as the owner asked.
package job_console

const (
	// zhAlreadyRunningFmt: "already running, since HH:MM UTC".
	zhAlreadyRunningFmt = "已在运行，自 %[1]s UTC (already running, since %[1]s UTC)"
	// zhRanUsuallyFmt: "ran X minutes, usually Y minutes".
	zhRanUsuallyFmt = "跑了 %[1]d 分钟，平常 %[2]d 分钟 (ran %[1]d minutes, usually %[2]d minutes)"
	// zhRanNoHistoryFmt: "ran X minutes, no history" (the job has no usual yet).
	zhRanNoHistoryFmt = "跑了 %[1]d 分钟，没有历史 (ran %[1]d minutes, no history)"
	// zhDoneOfFmt: "done 12 / 40".
	zhDoneOfFmt = "已完成 %[1]d / %[2]d (done %[1]d / %[2]d)"
	// zhSlowNote: "slow, a notice only".
	zhSlowNote = "偏慢，仅提示 (slow, a notice only)"
	// zhBrokerSilent: "the gateway did not answer, the keys were not fetched; run it again once the gateway answers".
	zhBrokerSilent = "网关没有应答，未取得这些键；网关应答后可再次运行 (the gateway did not answer, the keys were not fetched; run it again once the gateway answers)"
)
