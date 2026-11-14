package fundamentals

import (
	"context"
	"errors"
	"strings"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/fundkey"
)

var _ NewFundamentalsFunc = New

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
	skipped []FundamentalsSkip
}

var _ FundamentalsSkips = (*job)(nil)

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
			j.skipped = append(j.skipped, FundamentalsSkip{Subject: e.Issuer, Reason: reasonEdinet})
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
			j.skipped = append(j.skipped, FundamentalsSkip{Subject: subject, Reason: reasonNoCik})
		}
	}
	return j, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Skipped names the entries the job does not fetch, in watch-list order.
func (j *job) Skipped() []FundamentalsSkip {
	return append([]FundamentalsSkip(nil), j.skipped...)
}

// Run fetches the facts of the issuers of keys with no From bound (the whole document of each name, up to day)
// and writes them in one UpsertFacts call, so a fact read differently replaces the stored one.
// lookback_days is no longer read here; it stays only in the plan hash.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	up, ok := j.w.(fundkey.FactUpserter)
	if !ok {
		return contracts.JobResult{Err: asTyped(errors.New("fundamentals: the fact writer has no UpsertFacts"), contracts.Retryable)}
	}
	subjects := make([]string, 0, len(keys))
	for _, k := range keys {
		subjects = append(subjects, k.SubjectKey)
	}
	if len(subjects) == 0 {
		return contracts.JobResult{}
	}
	facts, err := j.src.Fetch(ctx, sources.FactPlan{Subjects: subjects, To: day})
	if err != nil {
		return contracts.JobResult{Err: asTyped(err, contracts.Permanent)}
	}
	res := contracts.JobResult{Returned: len(facts), Usable: len(facts)}
	if len(facts) == 0 {
		return res
	}
	if _, err := up.UpsertFacts(ctx, facts, "fundamentals:"+string(day)); err != nil {
		res.Usable = 0
		res.Err = asTyped(err, contracts.Retryable)
	}
	return res
}
