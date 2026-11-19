// zh_texts.go holds the Chinese strings of the running-now and already-running lines
// (docs/design/scheduler-per-job.md, Decisions 5 and 8): string constants only, the one door of the English
// guard of cmd/ot. The English meaning of each is beside it, as the owner asked.
package job_console

const (
	// zhAlreadyRunningFmt: "already running, since HH:MM UTC".
	zhAlreadyRunningFmt = "已在运行，自 %s UTC (already running, since %s UTC)"
	// zhRanUsuallyFmt: "ran X minutes, usually Y minutes".
	zhRanUsuallyFmt = "跑了 %d 分钟，平常 %d 分钟 (ran %d minutes, usually %d minutes)"
	// zhRanNoHistoryFmt: "ran X minutes, no history" (the job has no usual yet).
	zhRanNoHistoryFmt = "跑了 %d 分钟，没有历史 (ran %d minutes, no history)"
	// zhDoneOfFmt: "done 12 / 40".
	zhDoneOfFmt = "已完成 %d / %d (done %d / %d)"
	// zhSlowNote: "slow, a notice only".
	zhSlowNote = "偏慢，仅提示 (slow, a notice only)"
)
