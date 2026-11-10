package fundamentals

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ jobs.NewFundamentalsFunc = New

const (
	reasonNoCik   = "no_cik"
	reasonEdinet  = "edinet_not_fetched"
	marketUS      = "US"
	cikPrefix     = "cik:"
	edinetPrefix  = "edinet:"
	dayLayoutDate = "2006-01-02"
)

type job struct {
	m       manifest
	src     sources.FactSource
	w       store.FactWriter
	clk     clock.Clock
	issuers []string
	skipped []jobs.FundamentalsSkip
}

var _ jobs.FundamentalsSkips = (*job)(nil)

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

// asTyped returns err unchanged when it is a TypedError, else wraps it with kind.
func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind: kind, err: err}
}

// New builds the fundamentals job from the embedded job.toml and the watch list. An entry of Market US with a
// cik: issuer is planned; one with an empty or other issuer is skipped as no_cik (an empty issuer of any
// market too), an edinet: one of any market as edinet_not_fetched. No CIK is guessed.
func New(src sources.FactSource, w store.FactWriter, entries []jobs.WatchEntry, clk clock.Clock) (contracts.Job, error) {
	if src == nil || w == nil || clk == nil {
		return nil, errors.New("fundamentals: source, writer and clock are required")
	}
	m, err := parseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	j := &job{m: m, src: src, w: w, clk: clk}
	seen := map[string]bool{}
	for _, e := range entries {
		switch {
		case strings.HasPrefix(e.Issuer, edinetPrefix):
			// A real edinet: entry is Japanese; it is named whatever its market.
			j.skipped = append(j.skipped, jobs.FundamentalsSkip{Subject: e.Issuer, Reason: reasonEdinet})
		case e.Market != marketUS && e.Issuer != "":
			// Not a US company and not EDINET: not this job's, not named.
		case strings.HasPrefix(e.Issuer, cikPrefix):
			if !seen[e.Issuer] {
				seen[e.Issuer] = true
				j.issuers = append(j.issuers, e.Issuer)
			}
		default:
			subject := e.Issuer
			if subject == "" {
				subject = e.Contract.Symbol
			}
			j.skipped = append(j.skipped, jobs.FundamentalsSkip{Subject: subject, Reason: reasonNoCik})
		}
	}
	return j, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Skipped names the entries the job does not fetch, in watch-list order.
func (j *job) Skipped() []jobs.FundamentalsSkip {
	return append([]jobs.FundamentalsSkip(nil), j.skipped...)
}

// Run fetches the facts of the issuers of keys with filing days in [day - lookback_days, day] and writes
// them in one call.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	subjects := make([]string, 0, len(keys))
	for _, k := range keys {
		subjects = append(subjects, k.SubjectKey)
	}
	if len(subjects) == 0 {
		return contracts.JobResult{}
	}
	facts, err := j.src.Fetch(ctx, sources.FactPlan{Subjects: subjects, From: daysBefore(day, j.m.LookbackDays), To: day})
	if err != nil {
		return contracts.JobResult{Err: asTyped(err, contracts.Permanent)}
	}
	res := contracts.JobResult{Returned: len(facts), Usable: len(facts)}
	if len(facts) == 0 {
		return res
	}
	if err := j.w.InsertFacts(ctx, facts); err != nil {
		res.Usable = 0
		res.Err = asTyped(err, contracts.Retryable)
	}
	return res
}

// daysBefore is the calendar day n days before day; an unparsable day is returned as it is.
func daysBefore(day contracts.Day, n int) contracts.Day {
	t, err := time.Parse(dayLayoutDate, string(day))
	if err != nil {
		return day
	}
	return contracts.Day(t.AddDate(0, 0, -n).Format(dayLayoutDate))
}
