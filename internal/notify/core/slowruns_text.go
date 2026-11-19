package core

import "fmt"

// slowNoUsual is the usual-duration word of a job with no history ("no history").
const slowNoUsual = "没有历史"

// slowRunText is the text of one slow run. It carries the job id, the minutes and the key counts,
// never the key in hand.
//
//	任务 %s 运行偏慢（仅提醒，未中断）：已运行 %d 分钟，通常 %s，进度 %d/%d
//	(job %s is running slow — a notice only, nothing stopped: %d minutes so far, usual %s, progress %d/%d)
func slowRunText(job string, elapsedMin int, usual string, done, total int) string {
	return fmt.Sprintf("任务 %s 运行偏慢（仅提醒，未中断）：已运行 %d 分钟，通常 %s，进度 %d/%d", job, elapsedMin, usual, done, total)
}

// slowUsualText is "usual N minutes" ("N 分钟"), or "没有历史" when the job has none.
func slowUsualText(has bool, minutes int) string {
	if !has {
		return slowNoUsual
	}
	return fmt.Sprintf("%d 分钟", minutes)
}
