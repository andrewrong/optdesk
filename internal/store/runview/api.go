// Package runview declares the readers of the runs and reasons of a session day and the writer of the reasons
// (docs/design/option-open-gap-jp.md, Interface). It holds declarations only: the implementation is
// internal/store/ops. It is a package of its own because internal/store/api.go is at its ceiling.
package runview

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// NotWiredDay is the session day under which the set of not wired jobs is stored: the date 1970-01-01, with the
// dataset NotWiredDataset. The scheduler replaces the whole set at each startup.
const NotWiredDay contracts.Day = "1970-01-01"

// NotWiredDataset is the dataset of the not_wired reasons stored under NotWiredDay.
const NotWiredDataset = "ops.not_wired"

// Row is one run of a market key (a row of ops.capture_run through the view rd.capture_run_r1).
type Row struct {
	SessionDay contracts.Day
	Dataset    string
	Key        string
	Status     string
	FinishedAt time.Time // zero while the run has not finished
	Returned   int
	Usable     int
	ErrorCode  string
	ReasonCode string
}

// Reason is one reason row of a run (ops.option_run_reason through rd.option_run_reason_r1). Scope names the
// part of the key it is about (an underlying, a job id); Detail is the producer's text, never a secret value.
type Reason struct {
	Day     contracts.Day
	Dataset string
	Subject string
	Scope   string
	Code    string
	Detail  string
	At      time.Time
}

// Reader reads the runs and the reasons of a session day.
type Reader interface {
	// RunsOn returns the runs of the session day ordered ascending by dataset, key and finished at; none gives an
	// empty non-nil slice.
	RunsOn(ctx context.Context, day contracts.Day) ([]Row, error)
	// ReasonsOn returns the reasons of the session day ascending by dataset, subject, scope and code; none gives an
	// empty non-nil slice.
	ReasonsOn(ctx context.Context, day contracts.Day) ([]Reason, error)
	// NotWired returns the reasons stored under NotWiredDay and the dataset NotWiredDataset, ascending by scope;
	// never nil.
	NotWired(ctx context.Context) ([]Reason, error)
}

// ReasonWriter writes the reasons of a run.
type ReasonWriter interface {
	// ReplaceReasons replaces the whole set of the day, dataset and subject with reasons in one transaction, so a
	// clean run (no reasons) clears an earlier reason.
	ReplaceReasons(ctx context.Context, day contracts.Day, dataset, subject string, reasons []Reason) error
}
