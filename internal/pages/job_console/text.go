package job_console

import (
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// Every visible string of the page is written here, in English (docs/design/job-console.md, Decision 12).

const (
	titleText    = "Job console"
	sentenceText = "Run a job for one day; the scheduler does the work."

	nothingProducedText = "nothing was produced"

	unavailableText = "The job requests cannot be read now."
	noSuchText      = "No request has this id."

	presentNow     = "the key is present now"
	presentMissing = "the key is missing"
	presentNoValue = "marked no value"
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
func lineOutcome(l jobrequest.KeyLine) (lt lineText, ok bool) {
	lt.Present = presentFor(l)
	switch l.Group {
	case groupAlreadyPresent:
		lt.Outcome, lt.Text = outAlreadyPresent, "the key was present before this run"
	case groupNotDue:
		lt.Outcome, lt.Reason = outWaiting, reasonNotDue
		lt.Text = "runs at its slot"
		if !l.DueAt.IsZero() {
			lt.Text = "runs at " + hhmm(l.DueAt) + " UTC"
		}
	case groupLeaseHeld:
		lt.Outcome, lt.Reason = outSkipped, reasonInProgress
		lt.Text = "another run of this key is in progress"
	case groupNotReached:
		lt.Outcome, lt.Text = outNotRun, "not reached"
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
		lt.Text = "the re-run failed: " + l.ErrorCode
	case l.ReasonCode == reasonNoValue:
		lt.Outcome, lt.Reason = outNothingFound, reasonNoValue
		lt.Text = "the job answered no value (for example outside its slot); the key stays present"
	case l.Usable > 0:
		lt.Outcome, lt.Reason = outUnconfirmed, reasonRerunUnknown
		lt.Text = fmt.Sprintf("re-run returned %d rows; the key was already present and the page cannot tell whether anything new was stored", l.Returned)
	default:
		lt.Outcome, lt.Reason = outNothingFound, reasonNoValueOrEmpty
		lt.Text = "the key stays present from an earlier run"
	}
	return lt
}

// ranOutcome covers the seven rows of group ran.
func ranOutcome(l jobrequest.KeyLine, lt lineText) (lineText, bool) {
	switch l.Status {
	case statusAbandoned:
		lt.Outcome, lt.Reason = outFailed, reasonCut
		lt.Text = "the run was cut before it finished"
	case statusPartial:
		lt.Outcome, lt.Reason = outPartial, reasonCut
		lt.Text = fmt.Sprintf("cut after %d of %d rows; counts shown", l.Usable, l.Returned)
	case statusDone:
		switch {
		case l.ErrorCode != "":
			lt.Outcome, lt.Reason = outFailed, l.ErrorCode
			lt.Text = "the job failed: " + l.ErrorCode
		case l.Usable > 0 && l.Mark == markPresent:
			lt.Outcome = outProduced
			lt.Text = fmt.Sprintf("returned %d rows", l.Returned)
		case l.Usable > 0:
			lt.Outcome, lt.Reason = outUnconfirmed, reasonPostcondition
			lt.Text = fmt.Sprintf("returned %d rows but the key is not marked present", l.Returned)
		case l.ReasonCode == reasonEmptyBefore:
			lt.Outcome, lt.Reason = outNothingFound, reasonEmptyBefore
			lt.Text = "nothing yet, the scheduler tries again"
		default:
			lt.Outcome, lt.Reason = outNothingFound, reasonNoValueOrEmpty
			lt.Text = "the job returned nothing"
			if l.Mark == markNoValue {
				lt.Text += "; marked no value"
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
}

const staleAfter = 2 * time.Minute

// stateText is the request-level text, a function of (state, reason, hold, withdrawal) and the facts above.
func stateText(in stateIn) string {
	switch in.State {
	case jobrequest.StateQueued:
		return queuedText(in)
	case jobrequest.StateRunning:
		return "running since " + in.StartedAt
	case jobrequest.StateDone:
		return "done"
	case jobrequest.StateWithdrawn:
		return "withdrawn"
	case jobrequest.StateInterrupted:
		if in.Reason == reasonStopped {
			return "interrupted by a stop or a deploy; press Run again"
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
		return string(in.State) + ": " + in.Reason
	}
	return string(in.State)
}

func queuedText(in stateIn) string {
	switch {
	case in.HoldJob != "":
		return "starts after " + hhmm(in.HoldAt) + " UTC (the slot of " + in.HoldJob + ")"
	case !in.HoldAt.IsZero():
		return "held: the schedule is not read yet"
	case in.Withdrawing:
		return "withdrawal pending: it takes effect when the scheduler's worker is next free (a running reconcile can take up to two hours)"
	case in.Age > staleAfter:
		return "still queued: the scheduler may have no console yet (it tries again on every turn), or its worker is busy"
	}
	return "waits for the scheduler's worker; a running reconcile or console run goes first"
}

func refusedText(in stateIn) (string, bool) {
	switch in.Reason {
	case "no_key":
		return "not a session of this job's calendar", true
	case reasonNotDue:
		if in.DueAt.IsZero() {
			return "the job's slot has not come", true
		}
		return "the job's slot has not come: runs at " + hhmm(in.DueAt) + " UTC", true
	case "no_plan":
		return "no stored plan for this day (not a session, or the plan was not made)", true
	case "left_out":
		return "the scheduler holds this job out", true
	case "not_runnable":
		return "this job cannot be run from the page", true
	case "not_today":
		return "the day is not today any more in the job's zone", true
	}
	return "", false
}

func failedText(in stateIn) (string, bool) {
	switch in.Reason {
	case reasonSlotNear:
		if in.DueAt.IsZero() {
			return "stopped before the next key because a slot starts soon; press Run again after it", true
		}
		return "stopped before the next key because a slot starts at " + hhmm(in.DueAt) + " UTC; press Run again after it", true
	case "lease_held":
		return "another run holds the lease", true
	case "plan_failed":
		return "the plan of the day could not be read", true
	case "error":
		return "the run failed with an error; see the scheduler log", true
	case "restart":
		return "the scheduler restarted while it ran", true
	}
	return "", false
}
