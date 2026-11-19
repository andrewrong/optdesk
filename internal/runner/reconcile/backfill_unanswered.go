package reconcile

import (
	"log/slog"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
)

const (
	// unansweredStop is the count of keys of one job that end retryable in a row after which a console backfill
	// runs no more keys of that job. One failed key costs 20 to 30 s, so three cost at most 90 s; one or two
	// failed keys can be the key's own, while three in a row of one job are the broker not answering.
	unansweredStop = 3
	// reasonBrokerSilent is the reason of a key not reached because its job's last keys ended retryable.
	reasonBrokerSilent = "broker_silent"
)

// unanswered counts, per job id, the keys that ended retryable in a row.
type unanswered map[string]int

// silent reports whether the job has reached the stop count.
func (u unanswered) silent(id string) bool { return u[id] >= unansweredStop }

// note reads the line of a key of the job: a ran or rerun line that ended retryable adds one, one with any other
// ending resets the count, a line of another group changes nothing. It reports true when this line made the job
// reach the stop count.
func (u unanswered) note(id string, line runner.KeyRun) bool {
	if line.Group != runner.GroupRan && line.Group != runner.GroupRerun {
		return false
	}
	if line.ErrorCode != string(contracts.Retryable) {
		u[id] = 0
		return false
	}
	u[id]++
	return u[id] == unansweredStop
}

// logSilent logs the stop of a job: its id, the day and the number of keys left (no ticker names).
func (r *reconciler) logSilent(id string, day contracts.Day, left int) {
	r.log.Warn("backfill stopped a job after keys ended retryable in a row",
		slog.String("job", id), slog.String("day", string(day)), slog.Int("keys_left", left))
}

// keysLeft counts the keys of the job among keys.
func (r *reconciler) keysLeft(keys []contracts.ExpectedKey, p plan, id string) int {
	return len(keysOfJob(keys, p, id))
}
