package job_console

import (
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// Every visible string of the page is written here, in Chinese with the English glossed beside it
// (docs/design/job-console.md, Decision 12; docs/design/scheduler-per-job.md, Decision 8);
// the Chinese phrases are the constants of zh_texts.go.

const (
	titleText       = zhTitle
	sentenceText    = zhSentence
	unavailableText = zhUnavailable
	noSuchText      = zhNoSuch

	presentNow     = zhPresentNow
	presentMissing = zhPresentMissing
	presentNoValue = zhPresentNoValue
)

// The nine outcomes of a key line and the reason codes this file writes.
const (
	outAlreadyPresent = "already_present"
	outWaiting        = "waiting"
	outSkipped        = "skipped"
	outNotRun         = "not_run"
	outFailed         = "failed"
	outPartial        = "partial"
	outProduced       = "produced"
	outUnconfirmed    = "unconfirmed"
	outNothingFound   = "nothing_found"

	reasonNotDue         = "not_due"
	reasonInProgress     = "run_in_progress"
	reasonStopped        = "stopped"
	reasonCut            = "cut"
	reasonNoValue        = "no_value"
	reasonRerunUnknown   = "rerun_unknown"
	reasonNoValueOrEmpty = "no_value_or_empty"
	reasonPostcondition  = "postcondition"
	reasonEmptyBefore    = "empty_before_close"
	reasonSlotNear       = "slot_near"
	reasonBrokerSilent   = "broker_silent"
)

// The groups and statuses of a stored line, as the runner writes them.
const (
	groupAlreadyPresent = "already_present"
	groupNotDue         = "not_due"
	groupRerun          = "rerun"
	groupRan            = "ran"
	groupLeaseHeld      = "lease_held"
	groupNotReached     = "not_reached"

	statusDone      = "done"
	statusPartial   = "partial"
	statusAbandoned = "abandoned"
	markPresent     = "present"
	markNoValue     = "no_value"
)

const (
	atLayout    = "2006-01-02 15:04 UTC"
	clockLayout = "15:04"
)

func hhmm(t time.Time) string { return t.UTC().Format(clockLayout) }

// lineText is what one stored key line reads as.
type lineText struct {
	Outcome, Text, Present, Reason string
}

// lineOutcome maps a stored line to its outcome, outcome text, present text and reason. Every stored line falls in
// exactly one of the fifteen rows of the table of Decision 12; ok is false for a group or status outside it.
func lineOutcome(l jobrequest.KeyLine) (lineText, bool) {
	lt, ok := lineOutcomeBody(l)
	if ok {
		lt.Text = outcomeName(lt.Outcome) + ": " + lt.Text
	}
	return lt, ok
}

func lineOutcomeBody(l jobrequest.KeyLine) (lt lineText, ok bool) {
	lt.Present = presentFor(l)
	switch l.Group {
	case groupAlreadyPresent:
		lt.Outcome, lt.Text = outAlreadyPresent, zhLineAlreadyPresent
	case groupNotDue:
		lt.Outcome, lt.Reason = outWaiting, reasonNotDue
		lt.Text = zhLineRunsAtSlot
		if !l.DueAt.IsZero() {
			lt.Text = fmt.Sprintf(zhLineRunsAtFmt, hhmm(l.DueAt))
		}
	case groupLeaseHeld:
		lt.Outcome, lt.Reason = outSkipped, reasonInProgress
		lt.Text = zhLineInProgress
	case groupNotReached:
		lt.Outcome, lt.Text = outNotRun, zhLineNotReached
		lt.Reason = l.ReasonCode
		if lt.Reason == "" {
			lt.Reason = reasonStopped
		}
	case groupRerun:
		lt = rerunOutcome(l, lt)
	case groupRan:
		return ranOutcome(l, lt)
	default:
		return lt, false
	}
	return lt, true
}

// rerunOutcome covers the four rows of group rerun; none of them is produced, because a bare re-run of a present key
// cannot say whether anything new was stored.
func rerunOutcome(l jobrequest.KeyLine, lt lineText) lineText {
	switch {
	case l.ErrorCode != "":
		lt.Outcome, lt.Reason = outFailed, l.ErrorCode
		lt.Text = fmt.Sprintf(zhRerunFailedFmt, l.ErrorCode)
	case l.ReasonCode == reasonNoValue:
		lt.Outcome, lt.Reason = outNothingFound, reasonNoValue
		lt.Text = zhRerunNoValue
	case l.Usable > 0:
		lt.Outcome, lt.Reason = outUnconfirmed, reasonRerunUnknown
		lt.Text = fmt.Sprintf(zhRerunUnknownFmt, l.Returned)
	default:
		lt.Outcome, lt.Reason = outNothingFound, reasonNoValueOrEmpty
		lt.Text = zhRerunStays
	}
	return lt
}

// ranOutcome covers the seven rows of group ran.
func ranOutcome(l jobrequest.KeyLine, lt lineText) (lineText, bool) {
	switch l.Status {
	case statusAbandoned:
		lt.Outcome, lt.Reason = outFailed, reasonCut
		lt.Text = zhRanCut
	case statusPartial:
		lt.Outcome, lt.Reason = outPartial, reasonCut
		lt.Text = fmt.Sprintf(zhRanPartialFmt, l.Usable, l.Returned)
	case statusDone:
		switch {
		case l.ErrorCode != "":
			lt.Outcome, lt.Reason = outFailed, l.ErrorCode
			lt.Text = fmt.Sprintf(zhRanFailedFmt, l.ErrorCode)
		case l.Usable > 0 && l.Mark == markPresent:
			lt.Outcome = outProduced
			lt.Text = fmt.Sprintf(zhRanReturnedFmt, l.Returned)
		case l.Usable > 0:
			lt.Outcome, lt.Reason = outUnconfirmed, reasonPostcondition
			lt.Text = fmt.Sprintf(zhRanNotMarkedFmt, l.Returned)
		case l.ReasonCode == reasonEmptyBefore:
			lt.Outcome, lt.Reason = outNothingFound, reasonEmptyBefore
			lt.Text = zhRanEmptyBefore
		default:
			lt.Outcome, lt.Reason = outNothingFound, reasonNoValueOrEmpty
			lt.Text = zhRanNothing
			if l.Mark == markNoValue {
				lt.Text = zhRanNothingMarked
			}
		}
	default:
		return lt, false
	}
	return lt, true
}

// presentFor is the text of the mark beside the outcome; it is never the outcome. A present key stays present
// through a re-run.
func presentFor(l jobrequest.KeyLine) string {
	switch {
	case l.Group == groupAlreadyPresent, l.Group == groupRerun, l.Mark == markPresent:
		return presentNow
	case l.Mark == markNoValue:
		return presentNoValue
	}
	return presentMissing
}

// stateIn is what the request-level text depends on.
type stateIn struct {
	State       jobrequest.State
	Reason      string
	HoldJob     string
	HoldAt      time.Time
	Withdrawing bool
	Age         time.Duration // generated_at minus requested_at, for a queued request
	StartedAt   string        // at_text of the start, for a running request
	DueAt       time.Time     // the first instant a line of the request carries, zero when none
	Waiting     bool          // the request waits for its own job, which has an open run
}

const staleAfter = 2 * time.Minute

// stateText is the request-level text, a function of (state, reason, hold, withdrawal) and the facts above.
func stateText(in stateIn) string {
	switch in.State {
	case jobrequest.StateQueued:
		return queuedText(in)
	case jobrequest.StateRunning:
		return fmt.Sprintf(zhRunningSinceFmt, in.StartedAt)
	case jobrequest.StateDone:
		return zhStateDone
	case jobrequest.StateWithdrawn:
		return zhStateWithdrawn
	case jobrequest.StateInterrupted:
		if in.Reason == reasonStopped {
			return zhInterruptedStopped
		}
	case jobrequest.StateRefused:
		if t, ok := refusedText(in); ok {
			return t
		}
	case jobrequest.StateFailed:
		if t, ok := failedText(in); ok {
			return t
		}
	}
	if in.Reason != "" {
		return stateName(in.State) + ": " + in.Reason
	}
	return stateName(in.State)
}

func queuedText(in stateIn) string {
	switch {
	case in.Waiting:
		return waitingLine(in.HoldAt)
	case in.HoldJob != "":
		return fmt.Sprintf(zhQueuedStartsAfterFmt, hhmm(in.HoldAt), in.HoldJob)
	case !in.HoldAt.IsZero():
		return zhQueuedHeld
	case in.Withdrawing:
		return zhQueuedWithdrawing
	case in.Age > staleAfter:
		return zhQueuedStale
	}
	return zhQueuedWaits
}

func refusedText(in stateIn) (string, bool) {
	switch in.Reason {
	case "no_key":
		return zhRefusedNoKey, true
	case reasonNotDue:
		if in.DueAt.IsZero() {
			return zhRefusedNotDue, true
		}
		return fmt.Sprintf(zhRefusedNotDueAtFmt, hhmm(in.DueAt)), true
	case "no_plan":
		return zhRefusedNoPlan, true
	case "left_out":
		return zhRefusedLeftOut, true
	case "not_runnable":
		return zhRefusedNotRunnable, true
	case "not_today":
		return zhRefusedNotToday, true
	case "job_running":
		return zhRefusedJobRunning, true
	}
	return "", false
}

func failedText(in stateIn) (string, bool) {
	switch in.Reason {
	case reasonBrokerSilent:
		return zhBrokerSilent, true
	case reasonSlotNear:
		if in.DueAt.IsZero() {
			return zhFailedSlotNear, true
		}
		return fmt.Sprintf(zhFailedSlotNearAtFmt, hhmm(in.DueAt)), true
	case "lease_held":
		return zhFailedLeaseHeld, true
	case "plan_failed":
		return zhFailedPlanFailed, true
	case "error":
		return zhFailedError, true
	case "restart":
		return zhFailedRestart, true
	}
	return "", false
}

// The lines of the running jobs: their Chinese phrases are the constants of zh_texts.go (the one door of the
// English guard), each with its English beside it. The figures are copied from compute.FiguresFunc and hold_at;
// nothing is computed here.

// waitingLine is the line of a request that waits for its own job.
func waitingLine(since time.Time) string {
	return fmt.Sprintf(zhAlreadyRunningFmt, hhmm(since))
}

// runningLine is one line of "Running now".
func runningLine(f jobrunapi.Figure, startedAt time.Time) string {
	parts := []string{f.JobID, fmt.Sprintf(zhStartedFmt, hhmm(startedAt)), fmt.Sprintf(zhDoneOfFmt, f.KeysDone, f.KeysTotal)}
	if f.Step != "" {
		parts = append(parts, f.Step)
	}
	if f.HasUsual {
		parts = append(parts, fmt.Sprintf(zhRanUsuallyFmt, f.ElapsedMinutes, f.UsualMinutes))
	} else {
		parts = append(parts, fmt.Sprintf(zhRanNoHistoryFmt, f.ElapsedMinutes))
	}
	if f.Slow {
		parts = append(parts, zhSlowNote)
	}
	return strings.Join(parts, "; ")
}

// outcomeName is the Chinese name of an outcome code, with its English beside it.
func outcomeName(code string) string {
	switch code {
	case outAlreadyPresent:
		return zhOutAlreadyPresent
	case outWaiting:
		return zhOutWaiting
	case outSkipped:
		return zhOutSkipped
	case outNotRun:
		return zhOutNotRun
	case outFailed:
		return zhOutFailed
	case outPartial:
		return zhOutPartial
	case outProduced:
		return zhOutProduced
	case outUnconfirmed:
		return zhOutUnconfirmed
	case outNothingFound:
		return zhOutNothingFound
	}
	return code
}

// stateName is the Chinese name of a request state, with its English beside it.
func stateName(s jobrequest.State) string {
	switch s {
	case jobrequest.StateQueued:
		return zhStateQueued
	case jobrequest.StateRunning:
		return zhStateRunning
	case jobrequest.StateDone:
		return zhStateDone
	case jobrequest.StateRefused:
		return zhStateRefused
	case jobrequest.StateFailed:
		return zhStateFailed
	case jobrequest.StateInterrupted:
		return zhStateInterrupted
	case jobrequest.StateWithdrawn:
		return zhStateWithdrawn
	}
	return string(s)
}
