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

// The page title, sentence and gap texts.
const (
	// zhTitle: "Job console".
	zhTitle = "任务控制台 (Job console)"
	// zhSentence: "Run a job for one day; the scheduler does the work."
	zhSentence = "为某一天运行一个任务，由调度器完成工作 (Run a job for one day; the scheduler does the work.)"
	// zhNothingProducedFmt: "Request N (job, day): nothing was produced."
	zhNothingProducedFmt = "请求 %[1]d（%[2]s，%[3]s）：没有产出 (Request %[1]d (%[2]s, %[3]s): nothing was produced.)"
	// zhUnavailable: "The job requests cannot be read now."
	zhUnavailable = "现在无法读取任务请求 (The job requests cannot be read now.)"
	// zhNoSuch: "No request has this id."
	zhNoSuch = "没有请求使用这个编号 (No request has this id.)"
)

// The marks beside the outcome of a key line.
const (
	// zhPresentNow: "the key is present now".
	zhPresentNow = "键现在已存在 (the key is present now)"
	// zhPresentMissing: "the key is missing".
	zhPresentMissing = "键缺失 (the key is missing)"
	// zhPresentNoValue: "marked no value".
	zhPresentNoValue = "已标记为无值 (marked no value)"
)

// The Chinese names of the nine outcomes of a key line.
const (
	// zhOutAlreadyPresent: "already present".
	zhOutAlreadyPresent = "已存在 (already present)"
	// zhOutWaiting: "waiting".
	zhOutWaiting = "等待中 (waiting)"
	// zhOutSkipped: "skipped".
	zhOutSkipped = "已跳过 (skipped)"
	// zhOutNotRun: "not run".
	zhOutNotRun = "未运行 (not run)"
	// zhOutFailed: "failed".
	zhOutFailed = "失败 (failed)"
	// zhOutPartial: "partial".
	zhOutPartial = "部分完成 (partial)"
	// zhOutProduced: "produced".
	zhOutProduced = "已产出 (produced)"
	// zhOutUnconfirmed: "unconfirmed".
	zhOutUnconfirmed = "未确认 (unconfirmed)"
	// zhOutNothingFound: "nothing found".
	zhOutNothingFound = "未发现数据 (nothing found)"
)

// The sentences of a key line.
const (
	// zhLineAlreadyPresent: "the key was present before this run".
	zhLineAlreadyPresent = "本次运行前键已存在 (the key was present before this run)"
	// zhLineRunsAtSlot: "runs at its slot".
	zhLineRunsAtSlot = "到时段运行 (runs at its slot)"
	// zhLineRunsAtFmt: "runs at HH:MM UTC".
	zhLineRunsAtFmt = "将在 %[1]s UTC 运行 (runs at %[1]s UTC)"
	// zhLineInProgress: "another run of this key is in progress".
	zhLineInProgress = "此键的另一次运行正在进行 (another run of this key is in progress)"
	// zhLineNotReached: "not reached".
	zhLineNotReached = "尚未轮到 (not reached)"

	// zhRerunFailedFmt: "the re-run failed: CODE".
	zhRerunFailedFmt = "重跑失败：%[1]s (the re-run failed: %[1]s)"
	// zhRerunNoValue: "the job answered no value (for example outside its slot); the key stays present".
	zhRerunNoValue = "任务答复无值（例如在时段之外），键仍然存在 (the job answered no value (for example outside its slot); the key stays present)"
	// zhRerunUnknownFmt: "re-run returned N rows; the key was already present and the page cannot tell whether anything new was stored".
	zhRerunUnknownFmt = "重跑返回 %[1]d 行；键原本已存在，页面无法判断是否存入了新数据 (re-run returned %[1]d rows; the key was already present and the page cannot tell whether anything new was stored)" //nolint:lll // one string literal, as the English guard of cmd/ot reads it
	// zhRerunStays: "the key stays present from an earlier run".
	zhRerunStays = "键沿用更早一次运行的结果，仍然存在 (the key stays present from an earlier run)"

	// zhRanCut: "the run was cut before it finished".
	zhRanCut = "运行在完成前被中断 (the run was cut before it finished)"
	// zhRanPartialFmt: "cut after U of R rows; counts shown".
	zhRanPartialFmt = "在 %[1]d / %[2]d 行后中断，已显示计数 (cut after %[1]d of %[2]d rows; counts shown)"
	// zhRanFailedFmt: "the job failed: CODE".
	zhRanFailedFmt = "任务失败：%[1]s (the job failed: %[1]s)"
	// zhRanReturnedFmt: "returned R rows".
	zhRanReturnedFmt = "返回 %[1]d 行 (returned %[1]d rows)"
	// zhRanNotMarkedFmt: "returned R rows but the key is not marked present".
	zhRanNotMarkedFmt = "返回 %[1]d 行，但键未标记为存在 (returned %[1]d rows but the key is not marked present)"
	// zhRanEmptyBefore: "nothing yet, the scheduler tries again".
	zhRanEmptyBefore = "暂时没有数据，调度器会再试 (nothing yet, the scheduler tries again)"
	// zhRanNothing: "the job returned nothing".
	zhRanNothing = "任务没有返回数据 (the job returned nothing)"
	// zhRanNothingMarked: "the job returned nothing; marked no value".
	zhRanNothingMarked = "任务没有返回数据；已标记为无值 (the job returned nothing; marked no value)"
)

// The request-level texts.
const (
	// zhStartedFmt: "started HH:MM UTC".
	zhStartedFmt = "开始于 %[1]s UTC (started %[1]s UTC)"

	// zhRunningSinceFmt: "running since AT".
	zhRunningSinceFmt = "运行中，自 %[1]s 起 (running since %[1]s)"
	// zhStateDone: "done".
	zhStateDone = "已完成 (done)"
	// zhStateWithdrawn: "withdrawn".
	zhStateWithdrawn = "已撤回 (withdrawn)"
	// zhInterruptedStopped: "interrupted by a stop or a deploy; press Run again".
	zhInterruptedStopped = "因停止或部署而中断，请再次点击运行 (interrupted by a stop or a deploy; press Run again)"

	// The Chinese names of the states, for the fallback line.
	// zhStateQueued: "queued".
	zhStateQueued = "排队中 (queued)"
	// zhStateRunning: "running".
	zhStateRunning = "运行中 (running)"
	// zhStateRefused: "refused".
	zhStateRefused = "被拒绝 (refused)"
	// zhStateFailed: "failed".
	zhStateFailed = "失败 (failed)"
	// zhStateInterrupted: "interrupted".
	zhStateInterrupted = "已中断 (interrupted)"

	// zhQueuedWaits: "waits for the scheduler's worker; a running reconcile or console run goes first".
	zhQueuedWaits = "等待调度器的工作进程；正在运行的对账或控制台运行优先 (waits for the scheduler's worker; a running reconcile or console run goes first)"
	// zhQueuedStartsAfterFmt: "starts after HH:MM UTC (the slot of JOB)".
	zhQueuedStartsAfterFmt = "在 %[1]s UTC 之后开始（%[2]s 的时段）(starts after %[1]s UTC (the slot of %[2]s))"
	// zhQueuedHeld: "held: the schedule is not read yet".
	zhQueuedHeld = "已搁置：尚未读取时间表 (held: the schedule is not read yet)"
	// zhQueuedWithdrawing: "withdrawal pending: it takes effect when the scheduler's worker is next free (a running reconcile can take up to two hours)".
	zhQueuedWithdrawing = "撤回待处理：调度器的工作进程下次空闲时生效（正在运行的对账最多可能需要两小时）(withdrawal pending: it takes effect when the scheduler's worker is next free (a running reconcile can take up to two hours))" //nolint:lll // one string literal, as the English guard of cmd/ot reads it
	// zhQueuedStale: "still queued: the scheduler may have no console yet (it tries again on every turn), or its worker is busy".
	zhQueuedStale = "仍在排队：调度器可能还没有控制台（每一轮都会再试），或其工作进程正忙 (still queued: the scheduler may have no console yet (it tries again on every turn), or its worker is busy)" //nolint:lll // one string literal, as the English guard of cmd/ot reads it

	// zhRefusedNoKey: "not a session of this job's calendar".
	zhRefusedNoKey = "不是此任务日历上的交易日 (not a session of this job's calendar)"
	// zhRefusedNotDue: "the job's slot has not come".
	zhRefusedNotDue = "任务的时段还没到 (the job's slot has not come)"
	// zhRefusedNotDueAtFmt: "the job's slot has not come: runs at HH:MM UTC".
	zhRefusedNotDueAtFmt = "任务的时段还没到：将在 %[1]s UTC 运行 (the job's slot has not come: runs at %[1]s UTC)"
	// zhRefusedNoPlan: "no stored plan for this day (not a session, or the plan was not made)".
	zhRefusedNoPlan = "这一天没有已存的计划（不是交易日，或计划未生成）(no stored plan for this day (not a session, or the plan was not made))"
	// zhRefusedLeftOut: "the scheduler holds this job out".
	zhRefusedLeftOut = "调度器把这个任务排除在外 (the scheduler holds this job out)"
	// zhRefusedNotRunnable: "this job cannot be run from the page".
	zhRefusedNotRunnable = "此任务不能从页面运行 (this job cannot be run from the page)"
	// zhRefusedNotToday: "the day is not today any more in the job's zone".
	zhRefusedNotToday = "按任务所在时区，这一天已不是今天 (the day is not today any more in the job's zone)"
	// zhRefusedJobRunning: "this job is already running; press Run again when it ends".
	zhRefusedJobRunning = "此任务已在运行，结束后请再次点击运行 (this job is already running; press Run again when it ends)"

	// zhFailedSlotNear: "stopped before the next key because a slot starts soon; press Run again after it".
	zhFailedSlotNear = "因时段即将开始，在下一个键之前停止；之后请再次点击运行 (stopped before the next key because a slot starts soon; press Run again after it)"
	// zhFailedSlotNearAtFmt: "stopped before the next key because a slot starts at HH:MM UTC; press Run again after it".
	zhFailedSlotNearAtFmt = "因时段将在 %[1]s UTC 开始，在下一个键之前停止；之后请再次点击运行 (stopped before the next key because a slot starts at %[1]s UTC; press Run again after it)"
	// zhFailedLeaseHeld: "another run holds the lease".
	zhFailedLeaseHeld = "另一次运行持有租约 (another run holds the lease)"
	// zhFailedPlanFailed: "the plan of the day could not be read".
	zhFailedPlanFailed = "无法读取当天的计划 (the plan of the day could not be read)"
	// zhFailedError: "the run failed with an error; see the scheduler log".
	zhFailedError = "运行出错失败，请查看调度器日志 (the run failed with an error; see the scheduler log)"
	// zhFailedRestart: "the scheduler restarted while it ran".
	zhFailedRestart = "运行期间调度器重启了 (the scheduler restarted while it ran)"
)
