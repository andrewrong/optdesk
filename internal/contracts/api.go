// Package contracts owns the records, the job contract and the typed errors that the modules share.
package contracts

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts/gen"
)

// SessionDay is the record of dataset master.session_day r1, generated from contracts/datasets.
type SessionDay = gen.SessionDay

// SeriesPoint is the record of dataset obs.series_point r1, generated from contracts/datasets.
type SeriesPoint = gen.SeriesPoint

// OptionEod is the record of dataset obs.option_eod r1, generated from contracts/datasets.
type OptionEod = gen.OptionEod

// QuoteSnapshot is the record of dataset obs.quote_snapshot r1, generated from contracts/datasets.
type QuoteSnapshot = gen.QuoteSnapshot

// SeriesUnit is the unit enum of obs.series_point r1.
type SeriesUnit = gen.SeriesUnit

// Day is a calendar date in the form YYYY-MM-DD; no zone is implied.
type Day string

// ErrorKind classifies a TypedError.
type ErrorKind string

// The kinds of a TypedError.
const (
	Retryable       ErrorKind = "retryable"
	Permanent       ErrorKind = "permanent"
	UnknownCalendar ErrorKind = "unknown_calendar"
	NoValue         ErrorKind = "no_value"
)

// TypedError is an error with a kind; modules return only these across their interfaces.
type TypedError interface {
	error
	Kind() ErrorKind
}

// ExpectedKey names one record a job is expected to deliver; runner treats both fields as opaque.
type ExpectedKey struct {
	DatasetID  string
	SubjectKey string
}

// JobResult is what Job.Run returns: records returned, records usable, and an error or nil.
type JobResult struct {
	Returned int
	Usable   int
	Err      TypedError
}

// Job is one unit of scheduled work for a session day.
type Job interface {
	// ID is the stable name of the job.
	ID() string
	// Plan returns the keys the job expects to deliver for day and a hash of that plan.
	Plan(ctx context.Context, day Day) (keys []ExpectedKey, planHash string, err error)
	// Run fetches and stores the records of keys for day.
	Run(ctx context.Context, day Day, keys []ExpectedKey) JobResult
}

// ReadingRecord is the record of dataset readings.reading r1, generated from contracts/datasets.
type ReadingRecord = gen.ReadingRecord
