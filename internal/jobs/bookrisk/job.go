// Package bookrisk is the book_risk job: once per XNYS session day, forty-five minutes after the close, it loads
// the open book of each account alias, has it stressed over the FINRA ladder and writes one book_risk row per alias
// and currency, one per household and currency, and one fit row per ok put candidate and alias with funds
// (docs/design/strategy-risk.md Decisions 4, 5, 8, 9, 11 and 12). It imports the root package of strategy only: the
// stresser and the fitter are handed in. It reads stores, calls no broker and no network source.
package bookrisk

import (
	"context"
	"crypto/sha1" //nolint:gosec // an identity hash of parameter lines, as strategy/shared does, not a secret
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
	"github.com/andrewrong/optdesk/internal/strategy"
)

//go:embed job.toml
var manifestBytes []byte

const (
	datasetID       = "strategy.result"
	strategyBook    = "book_risk"
	strategyFit     = "fit"
	strategyPut     = "short_put"
	ruleBook        = "risk_v1"
	ruleFit         = "fit_v1"
	defaultCurrency = "USD" // the currency of a row that records a failed read: no holding tells another
)

// Results is the store of strategy results: the job reads the candidate rows and writes its own.
type Results interface {
	strategyresult.Writer
	strategyresult.Reader
}

// Stores is what the job reads and writes. Aliases are the account labels in order. OptionEod, Terms, Marks and
// Series may be nil: the book then has no option quotes (no option mark), no share marks, no rate. Every other field
// is required. Stresser and Fitter are the implementations cmd/ot injects (design Decision 11).
type Stores struct {
	Aliases     []string
	Account     store.AccountReader
	Ledger      store.OptionLedgerReader
	SessionDays store.SessionDayReader
	Ledgers     compute.NewOptionLedgerFunc
	Structures  compute.NewOptionStructuresFunc
	OptionEod   store.OptionEodReader
	Terms       store.OptionTermsReader
	Marks       mark.Reader
	Series      store.SeriesReader
	Results     Results
	Stresser    strategy.RiskStresser
	Fitter      strategy.RiskFitter
}

type manifest struct {
	Version  int    `toml:"version"`
	ID       string `toml:"id"`
	Schedule struct {
		Calendar         string `toml:"calendar"`
		Anchor           string `toml:"anchor"`
		Zone             string `toml:"zone"`
		Offset           string `toml:"offset"`
		RunBound         string `toml:"run_bound"`
		RestartSensitive bool   `toml:"restart_sensitive"`
	} `toml:"schedule"`
	Params struct {
		MaxMarkAgeSessions int `toml:"max_mark_age_sessions"`
	} `toml:"params"`
	Class []struct {
		Symbol string `toml:"symbol"`
		Class  string `toml:"class"`
	} `toml:"class"`
}

type job struct {
	m                      manifest
	calendar, anchor, zone string
	offset, bound          time.Duration
	classes                map[string]strategy.RiskClass
	paramHash              string
	st                     Stores
	clk                    clock.Clock
}

type typedError struct {
	kind contracts.ErrorKind
	code string
	err  error
}

func (e *typedError) Error() string {
	if e.err == nil {
		return "bookrisk: " + e.code
	}
	return "bookrisk: " + e.code + ": " + e.err.Error()
}
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func fail(code string, err error) contracts.JobResult {
	kind := contracts.Retryable
	var te contracts.TypedError
	if errors.As(err, &te) {
		kind = te.Kind()
	}
	return contracts.JobResult{Err: &typedError{kind: kind, code: code, err: err}}
}

// New builds the book_risk job from the embedded job.toml. An error names a key, never a value.
func New(st Stores, clk clock.Clock) (contracts.Job, error) {
	if st.Account == nil || st.Ledger == nil || st.SessionDays == nil || st.Ledgers == nil || st.Structures == nil ||
		st.Results == nil || st.Stresser == nil || st.Fitter == nil || clk == nil {
		return nil, errors.New("bookrisk: account, ledger, session day and result stores, the replay and structure rules, " +
			"the stresser, the fitter and the clock are required")
	}
	var m manifest
	if err := toml.Unmarshal(manifestBytes, &m); err != nil || m.ID == "" {
		return nil, errors.New("bookrisk: manifest is invalid")
	}
	off, err1 := time.ParseDuration(m.Schedule.Offset)
	bound, err2 := time.ParseDuration(m.Schedule.RunBound)
	if err1 != nil || err2 != nil || m.Schedule.Calendar == "" || m.Schedule.Anchor == "" || m.Params.MaxMarkAgeSessions < 0 {
		return nil, errors.New("bookrisk: manifest: schedule or params are invalid")
	}
	classes := map[string]strategy.RiskClass{}
	lines := []string{fmt.Sprintf("max_mark_age_sessions=%d", m.Params.MaxMarkAgeSessions)}
	for _, c := range m.Class {
		switch strategy.RiskClass(c.Class) {
		case classHigh, classOther, classEquity:
		default:
			return nil, errors.New("bookrisk: manifest: class is invalid")
		}
		if c.Symbol == "" {
			return nil, errors.New("bookrisk: manifest: class symbol is empty")
		}
		classes[c.Symbol] = strategy.RiskClass(c.Class)
		lines = append(lines, "class."+c.Symbol+"="+c.Class)
	}
	return &job{m: m, calendar: m.Schedule.Calendar, anchor: m.Schedule.Anchor, zone: m.Schedule.Zone, offset: off,
		bound: bound, classes: classes, paramHash: hashLines(lines), st: st, clk: clk}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	return j.calendar, j.anchor, j.zone, j.offset
}

// After is the job whose put rows of the day the fit rows read.
func (j *job) After() []string { return []string{"option_candidates"} }

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.bound }

// RestartSensitive is false: a rerun is a new run id and adds rows.
func (j *job) RestartSensitive() bool { return j.m.Schedule.RestartSensitive }

// Plan returns one key per alias and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := make([]contracts.ExpectedKey, 0, len(j.st.Aliases))
	for _, a := range j.st.Aliases {
		keys = append(keys, contracts.ExpectedKey{DatasetID: datasetID, SubjectKey: a})
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", j.m.ID, j.m.Version, day, strings.Join(j.st.Aliases, "\n"))))
	return keys, hex.EncodeToString(sum[:]), nil
}

// aliasRun is what one alias gave: its book and the outcome of the stress, or a failed read.
type aliasRun struct {
	alias  string
	failed bool
	book   strategy.RiskBook
	out    strategy.RiskOutcome
}

// Run loads and stresses every alias, reads the ok put candidates of the day and inserts all rows under one new
// run id. A failed read of one alias is a row of its own and the others go on; a failed read of the candidates or
// of the insert fails the run with nothing written.
func (j *job) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	var runs []aliasRun
	for _, alias := range j.st.Aliases {
		l, err := j.load(ctx, alias, day)
		if err != nil {
			slog.Warn("book not read", "component", "jobs", "job", strategyBook, "alias", alias, "status", "read_failed", "err", err)
			runs = append(runs, aliasRun{alias: alias, failed: true})
			continue
		}
		if l.empty {
			continue
		}
		out := j.st.Stresser.Stress(l.book)
		mergeExcluded(&out, l.notInLedger, l.book.Currency)
		runs = append(runs, aliasRun{alias: alias, book: l.book, out: out})
	}
	cands, err := j.st.Results.Results(ctx, day, strategyPut)
	if err != nil {
		return fail("read_candidates_failed", err)
	}
	h := header{day: day, runID: uuid.NewString(), at: j.clk.Now().UTC(), paramHash: j.paramHash}
	rows := bookRows(h, runs)
	fits := j.fitRows(h, runs, cands)
	rows = append(rows, fits...)
	if len(rows) == 0 {
		return contracts.JobResult{}
	}
	if _, err := j.st.Results.InsertResults(ctx, rows); err != nil {
		return fail("write_results_failed", err)
	}
	res := contracts.JobResult{Returned: len(rows)}
	for _, r := range rows {
		if r.Status == "ok" {
			res.Usable++
		}
	}
	return res
}

func hashLines(lines []string) string {
	sorted := append([]string(nil), lines...)
	slices.Sort(sorted)
	sum := sha1.Sum([]byte(strings.Join(sorted, "\n"))) //nolint:gosec // see the import
	return hex.EncodeToString(sum[:])
}
