// Package optioncandidates is the option_candidates job: once per XNYS session day, thirty minutes after the
// close, it evaluates the stored put chain of each watched US stock and writes one candidate row per contract
// (docs/design/option-candidates.md Decisions 1, 4, 5, 9, 13 and 15). It holds no dividend reader and no sender.
package optioncandidates

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

//go:embed job.toml
var manifestBytes []byte

const datasetID = "candidates.option_candidate"

var _ jobs.NewOptionCandidatesJobFunc = New

type job struct {
	m       manifest
	slot    slot
	params  compute.CandidateParams
	st      jobs.CandidateStores
	rule    compute.OptionCandidates
	entries []jobs.WatchEntry
	clk     clock.Clock

	mu      sync.Mutex
	skipped []string
}

type typedError struct {
	kind contracts.ErrorKind
	code string
	err  error
}

func (e *typedError) Error() string {
	if e.err == nil {
		return "optioncandidates: " + e.code
	}
	return "optioncandidates: " + e.code + ": " + e.err.Error()
}
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

// fail is a failed run with a named code; a typed error of the store keeps its kind, any other is Retryable.
func fail(code string, err error) contracts.JobResult {
	kind := contracts.Retryable
	var te contracts.TypedError
	if errors.As(err, &te) {
		kind = te.Kind()
	}
	return contracts.JobResult{Err: &typedError{kind: kind, code: code, err: err}}
}

// New builds the option_candidates job from the embedded job.toml. A nil Earnings, Accounts or Series reader
// gives unknown gates, never an error; the other readers, the writer, the rule and the clock are required.
func New(st jobs.CandidateStores, rule compute.OptionCandidates, entries []jobs.WatchEntry, clk clock.Clock) (contracts.Job, error) {
	if st.OptionEod == nil || st.Terms == nil || st.Names == nil || st.Catalog == nil || st.Candidates == nil ||
		st.SessionDays == nil || rule == nil || clk == nil {
		return nil, errors.New("optioncandidates: chain, terms, names, catalog, session day and candidate stores, the rule and the clock are required")
	}
	m, sl, params, err := parseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.Issuer == "" || seen[e.Issuer] {
			return nil, errors.New("optioncandidates: an entry needs a unique issuer")
		}
		seen[e.Issuer] = true
	}
	return &job{m: m, slot: sl, params: params, st: st, rule: rule, entries: entries, clk: clk}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	return j.slot.calendar, j.slot.anchor, j.slot.zone, j.slot.offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.slot.runBound }

// RestartSensitive is false: a rerun is a new run id and adds rows.
func (j *job) RestartSensitive() bool { return j.slot.restartSensitive }

// Skipped names, one line each, what the last run wrote no row for: a JP entry and a US entry with no chain.
// JobResult has no text, so this is where a skipped entry is named and never silent.
func (j *job) Skipped() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.skipped...)
}

// usEntries are the US stock entries, sorted by symbol; jp names the Japanese ones.
func (j *job) split() (us []jobs.WatchEntry, jp []string) {
	for _, e := range j.entries {
		switch {
		case e.Market == "JP":
			jp = append(jp, "JP entry "+e.Issuer+": no option chain yet, no row")
		case e.Market == "US" && e.Contract.SecType == "STK":
			us = append(us, e)
		}
	}
	sort.Slice(us, func(a, b int) bool { return us[a].Contract.Symbol < us[b].Contract.Symbol })
	return us, jp
}

// Plan returns one key per US stock entry and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	us, _ := j.split()
	keys := make([]contracts.ExpectedKey, 0, len(us))
	ids := make([]string, 0, len(us))
	for _, e := range us {
		keys = append(keys, contracts.ExpectedKey{DatasetID: datasetID, SubjectKey: e.Issuer})
		ids = append(ids, e.Issuer)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", j.m.ID, j.m.Version, day, strings.Join(ids, "\n"))))
	return keys, hex.EncodeToString(sum[:]), nil
}

// Run reads the stores, evaluates every watched US underlying and writes the rows of the day under a new run id
// (one UUID text per Run). Nothing is written when a read fails.
func (j *job) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	us, skipped := j.split()
	defer func() {
		j.mu.Lock()
		j.skipped = skipped
		j.mu.Unlock()
	}()
	days, err := sessionDays(ctx, j.st.SessionDays, day)
	if err != nil {
		return fail("read_session_days_failed", err)
	}
	ulist, missing, err := j.locate(ctx, day, us)
	if err != nil {
		return fail("read_chain_catalog_failed", err)
	}
	for _, s := range missing {
		skipped = append(skipped, "US entry "+s+": no chain day of option_eod, no row")
	}
	rate, rateDay, err := j.rate(ctx, day)
	if err != nil {
		return fail("read_rate_failed", err)
	}
	runID, now := uuid.NewString(), j.clk.Now().UTC()
	var rows []store.Candidate
	for _, ul := range ulist {
		entry := entryOf(us, ul.issuer)
		spot, quotes, err := j.quotes(ctx, ul)
		if err != nil {
			return fail("read_chain_failed", err)
		}
		info, ev, err := j.earnings(ctx, entry, ul.chainDay)
		if err != nil {
			return fail("read_earnings_failed", err)
		}
		accts, err := j.accounts(ctx, ul.symbol, days)
		if err != nil {
			return fail("read_account_failed", err)
		}
		inputs := []contracts.Day{ul.chainDay}
		if info != nil {
			inputs = append(inputs, info.RowDay)
		}
		for _, a := range accts {
			inputs = append(inputs, a.CaptureDay)
		}
		in := compute.CandidateInput{
			RunDay: day, ChainDay: ul.chainDay, SessionDays: trimSessionDays(days, inputs...), Underlying: ul.symbol,
			Spot: spot, Quotes: quotes, BuyPrice: entry.BuyPrice, SalePrice: entry.SalePrice, Earnings: info,
			Rate: rate, RateDay: rateDay, Accounts: accts, Kinds: []compute.CandidateKind{compute.KindShortPut},
		}
		for _, c := range j.rule.Evaluate(in, j.params) {
			rows = append(rows, toStoreRow(c, day, runID, ul, ev, now))
		}
	}
	rankRun(rows)
	if _, err := j.st.Candidates.InsertCandidates(ctx, rows); err != nil {
		return fail("write_candidates_failed", err)
	}
	res := contracts.JobResult{Returned: len(rows)}
	for _, r := range rows {
		if r.Status == "ok" {
			res.Usable++
		}
	}
	return res
}

func entryOf(us []jobs.WatchEntry, issuer string) jobs.WatchEntry {
	for _, e := range us {
		if e.Issuer == issuer {
			return e
		}
	}
	return jobs.WatchEntry{}
}
