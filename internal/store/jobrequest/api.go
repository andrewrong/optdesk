// Package jobrequest declares the queue of job requests that the owner's page
// writes and the scheduler's worker runs (docs/design/job-console.md#Interface).
// It holds declarations only; the implementation is a later issue. The three
// interfaces are split by role: Reader for the pages role, Poster for the web
// role, Consumer for the runner role.
package jobrequest

import (
	"context"
	"time"
)

// State is the state of one request.
type State string

// The seven states of a request. A request is open while it is queued or running.
const (
	StateQueued      State = "queued"
	StateRunning     State = "running"
	StateDone        State = "done"
	StateRefused     State = "refused"
	StateFailed      State = "failed"
	StateInterrupted State = "interrupted"
	StateWithdrawn   State = "withdrawn"
)

// Request is what the page posts: one job, one day, forced or not. Day is
// YYYY-MM-DD.
type Request struct {
	JobID string
	Day   string
	Force bool
}

// KeyLine is the answer for one key of a finished request, the fields of the
// runner's per-key line as plain strings and numbers. DueAt is zero unless the
// key was not due.
type KeyLine struct {
	Dataset, Subject, Group, Status string
	ErrorCode, ReasonCode, Mark     string
	Returned, Usable, Attempt       int
	DueAt                           time.Time
}

// Row is one request as stored. StartedAt and FinishedAt are zero until the
// request starts or finishes. Reason is a code, never the text of an error.
// HoldJob and HoldAt say why a queued request waits (empty and zero when it
// does not). WithdrawnAt is zero unless a withdrawal was recorded. Lines is
// empty until the request finishes.
type Row struct {
	ID          int64
	Request     Request
	RequestedAt time.Time
	State       State
	StartedAt   time.Time
	FinishedAt  time.Time
	Reason      string
	HoldJob     string
	HoldAt      time.Time
	WithdrawnAt time.Time
	Lines       []KeyLine
}

// EnqueueOutcome says what Enqueue did.
type EnqueueOutcome int

// The outcomes of Enqueue.
const (
	// EnqueueCreated: a new request was stored; Enqueued.Row is it.
	EnqueueCreated EnqueueOutcome = iota + 1
	// EnqueueExisting: an open request with the same job, day and force exists;
	// Enqueued.Row is that one and nothing was stored.
	EnqueueExisting
	// EnqueueWithdrawing: the open request is being withdrawn; nothing was stored.
	EnqueueWithdrawing
	// EnqueueBusy: the open request finished between the insert and the read and
	// the insert tried again once; nothing was stored.
	EnqueueBusy
)

// Enqueued is the answer of Enqueue.
type Enqueued struct {
	Row     Row
	Outcome EnqueueOutcome
}

// Reader reads the requests for a page (the role ot_ro_pages).
type Reader interface {
	// Recent returns the newest limit requests, newest first.
	Recent(ctx context.Context, limit int) ([]Row, error)
	// Get returns one request; found is false when the id is unknown.
	Get(ctx context.Context, id int64) (row Row, found bool, err error)
	// Close releases the connection.
	Close() error
}

// Poster is what the web role may do: read and post, never edit a request.
type Poster interface {
	Reader
	// Enqueue stores a request unless an open one of the same job, day and force
	// exists; the outcome says which.
	Enqueue(ctx context.Context, r Request) (Enqueued, error)
	// Withdraw records the withdrawal of a request; the scheduler ends it
	// withdrawn if it has not started. An unknown id is an error.
	Withdraw(ctx context.Context, id int64) error
}

// Consumer is what the runner role does: take queued requests and write their
// outcome.
type Consumer interface {
	// Queued returns up to limit queued requests, oldest first.
	Queued(ctx context.Context, limit int) ([]Row, error)
	// Claim moves a queued request to running at now; claimed is false when it is
	// no longer queued (or withdrawn).
	Claim(ctx context.Context, id int64, now time.Time) (claimed bool, err error)
	// Hold records why a queued request waits: the job whose slot is near and
	// the time of it.
	Hold(ctx context.Context, id int64, job string, at time.Time) error
	// Finish ends a running request in s with a reason code and its key lines.
	Finish(ctx context.Context, id int64, s State, reason string, lines []KeyLine, now time.Time) error
	// FailRunning ends every running request as failed with reason (the start of
	// the host, after a crash).
	FailRunning(ctx context.Context, reason string, now time.Time) error
	// Close releases the connection.
	Close() error
}

// OpenReaderFunc opens a Reader from the DSN of the pages role.
type OpenReaderFunc func(ctx context.Context, dsn string) (Reader, error)

// OpenPosterFunc opens a Poster from the DSN of the web role.
type OpenPosterFunc func(ctx context.Context, dsn string) (Poster, error)

// OpenConsumerFunc opens a Consumer from the DSN of the runner role.
type OpenConsumerFunc func(ctx context.Context, dsn string) (Consumer, error)
