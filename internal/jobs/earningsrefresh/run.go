// Package earningsrefresh is the earnings_refresh job: once per XNYS session day it writes one
// obs.earnings_date row per watch entry (docs/design/event-sources.md Decisions 1, 4, 5, 7 and 12).
package earningsrefresh

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/fundkey"
	"github.com/andrewrong/optdesk/internal/symbol"
)

//go:embed job.toml
var manifestBytes []byte

const (
	datasetID = "obs.earnings_date"
	dayLayout = "2006-01-02"
)

var _ NewEarningsRefreshFunc = New

type manifest struct {
	Version  int `toml:"version"`
	ID       string
	Schedule struct {
		Calendar         string `toml:"calendar"`
		Anchor           string `toml:"anchor"`
		Zone             string `toml:"zone"`
		Offset           string `toml:"offset"`
		RunBound         string `toml:"run_bound"`
		RestartSensitive bool   `toml:"restart_sensitive"`
	} `toml:"schedule"`
}

type job struct {
	m        manifest
	offset   time.Duration
	runBound time.Duration
	srcs     map[string]sources.EarningsSource
	w        store.EarningsDateWriter
	r        store.EarningsDateReader
	entries  []jobs.WatchEntry
	clk      clock.Clock
	loc      *time.Location
	tr       symbol.Translator
	log      *slog.Logger
	// memo holds, per day, what the first Run of this instance did: one Plan makes one Fetch per source.
	mu   sync.Mutex
	memo map[contracts.Day]outcome
}

type typedError struct {
	kind contracts.ErrorKind
	msg  string
}

func (e *typedError) Error() string             { return e.msg }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind, err.Error()}
}

// logFailure logs the step and the error kind of a failed run; the kind is unknown when err has none. It
// never logs the error text, which can carry a key or an address.
func (j *job) logFailure(step string, err error) {
	kind := "unknown"
	var k interface{ Kind() contracts.ErrorKind }
	if errors.As(err, &k) {
		kind = string(k.Kind())
	}
	j.log.Warn("run failed", "step", step, "kind", kind)
}

// vendor is the row source of a market's EarningsSource.
var vendor = map[string]contracts.EarningsDate{
	"US": {Market: "US", Source: "alphavantage"},
	"JP": {Market: "JP", Source: "jquants"},
}

var _ NewEarningsRefreshSymbolsFunc = NewWithSymbols

// New builds the earnings_refresh job from the embedded job.toml. srcs holds one source per market (US, JP).
// It matches symbols by sameSymbol, as NewWithSymbols does with a nil translator.
func New(
	srcs map[string]sources.EarningsSource, w store.EarningsDateWriter, r store.EarningsDateReader,
	entries []jobs.WatchEntry, clk clock.Clock,
) (contracts.Job, error) {
	return NewWithSymbols(srcs, w, r, entries, clk, nil)
}

// NewWithSymbols is New with a symbol.Translator: an entry matches a listing when both give the same
// ToVendor(symbol, symbol.AlphaVantage). A nil tr falls back to sameSymbol.
func NewWithSymbols(
	srcs map[string]sources.EarningsSource, w store.EarningsDateWriter, r store.EarningsDateReader,
	entries []jobs.WatchEntry, clk clock.Clock, tr symbol.Translator,
) (contracts.Job, error) {
	if w == nil || r == nil || clk == nil {
		return nil, errors.New("earningsrefresh: writer, reader and clock are required")
	}
	var m manifest
	if err := toml.Unmarshal(manifestBytes, &m); err != nil || m.ID == "" {
		return nil, errors.New("earningsrefresh: manifest is invalid")
	}
	off, err1 := time.ParseDuration(m.Schedule.Offset)
	bound, err2 := time.ParseDuration(m.Schedule.RunBound)
	if err1 != nil || err2 != nil || m.Schedule.Calendar == "" {
		return nil, errors.New("earningsrefresh: manifest: schedule is invalid")
	}
	loc, err := time.LoadLocation(m.Schedule.Zone)
	if err != nil {
		return nil, errors.New("earningsrefresh: manifest: zone is invalid")
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.Issuer == "" || seen[e.Issuer] {
			return nil, errors.New("earningsrefresh: an entry needs a unique issuer")
		}
		if _, ok := vendor[e.Market]; !ok {
			return nil, errors.New("earningsrefresh: market must be US or JP")
		}
		seen[e.Issuer] = true
	}
	return &job{m: m, offset: off, runBound: bound, srcs: srcs, w: w, r: r, entries: entries, clk: clk, loc: loc, tr: tr,
		log: slog.Default(), memo: map[contracts.Day]outcome{}}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	s := j.m.Schedule
	return s.Calendar, s.Anchor, s.Zone, j.offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.runBound }

// RestartSensitive is false: a later Run of one instance and day returns the first result and makes no call.
func (j *job) RestartSensitive() bool { return false }

// Plan returns one key per watch entry and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := make([]contracts.ExpectedKey, 0, len(j.entries))
	ids := make([]string, 0, len(j.entries))
	for _, e := range j.entries {
		keys = append(keys, contracts.ExpectedKey{DatasetID: datasetID, SubjectKey: e.Issuer})
		ids = append(ids, e.Issuer)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", j.m.ID, j.m.Version, day, strings.Join(ids, "\n"))))
	return keys, hex.EncodeToString(sum[:]), nil
}

func date(s string) (*types.SerializableDate, error) {
	t, err := time.Parse(dayLayout, s)
	if err != nil {
		return nil, err
	}
	return &types.SerializableDate{Time: t}, nil
}

// sameSymbol matches a vendor symbol with a watch symbol; share classes (BRK.B, BRK-B, BRK B) are one company.
func sameSymbol(a, b string) bool {
	norm := func(s string) string {
		return strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(s, "-", "."), " ", "."))
	}
	return norm(a) == norm(b)
}

// Postcondition is true when a row exists for the day and the subject key; Run writes the rows of every entry
// on the first key's run, so a later key finds its row already stored.
func (j *job) Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error) {
	if datasetID != "obs.earnings_date" {
		return false, fmt.Errorf("earningsrefresh: unknown dataset %q", datasetID)
	}
	rows, err := j.r.EarningsDates(ctx, subjectKey, contracts.Day(day), contracts.Day(day))
	if err != nil {
		return false, fmt.Errorf("earningsrefresh: earnings dates: %w", err)
	}
	return len(rows) > 0, nil
}

// outcome is what the first Run of an instance and day did: its result and the market that failed to fetch.
type outcome struct {
	res    contracts.JobResult
	failed map[string]string
}

// Run writes the rows of day by key: it makes at most one Fetch per source and upserts all rows of the run in one
// call, so a rerun replaces what the source now says. A day before the clock's date in the manifest zone is a
// replay. The first Run of an instance and day does the work and every later Run of the same instance and day
// returns that result, a failure included, without a fetch. A new instance fetches again.
//
// With one key that names a watch entry, only a failure of that entry's market is an error; otherwise any failed
// market is.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	observed, err := date(string(day))
	if err != nil {
		te := &typedError{contracts.Permanent, "earningsrefresh: bad day"}
		j.logFailure("bad_day", te)
		return contracts.JobResult{Err: te}
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	o, ok := j.memo[day]
	if !ok {
		o = j.fetchAndWrite(ctx, day, observed)
		j.memo[day] = o
	}
	res := o.res
	if res.Err != nil {
		return res
	}
	if counted := j.countedFailures(keys, o.failed); len(counted) > 0 {
		var texts, markets, codes []string
		for _, m := range counted {
			lm := strings.ToLower(m)
			texts = append(texts, lm+": "+o.failed[m])
			markets = append(markets, lm)
			codes = append(codes, o.failed[m])
		}
		te := &typedError{contracts.Permanent, "earningsrefresh: " + strings.Join(texts, "; ")}
		j.log.Warn("run failed", "step", "source_failed", "kind", string(te.kind),
			"market", strings.Join(markets, ","), "code", strings.Join(codes, ","))
		res.Err = te
	}
	return res
}

func (j *job) fetchAndWrite(ctx context.Context, day contracts.Day, observed *types.SerializableDate) outcome {
	up, ok := j.w.(fundkey.EarningsUpserter)
	if !ok {
		te := &typedError{contracts.Retryable, "earningsrefresh: the earnings store has no UpsertEarnings"}
		j.logFailure("no_upsert", te)
		return outcome{res: contracts.JobResult{Err: te}}
	}
	now := j.clk.Now().UTC()
	replay := string(day) < now.In(j.loc).Format(dayLayout)
	base := func(e jobs.WatchEntry) contracts.EarningsDate {
		return contracts.EarningsDate{
			Attempt: 1, Issuer: e.Issuer, Market: vendor[e.Market].Market, ObservedOn: *observed,
			ReceivedAt: now, Symbol: e.Contract.Symbol,
		}
	}
	var rows []contracts.EarningsDate
	pending := map[string][]jobs.WatchEntry{}
	for _, e := range j.entries {
		row := base(e)
		switch {
		case e.ReportDateOverride != "":
			d, err := date(e.ReportDateOverride)
			if err != nil {
				te := &typedError{contracts.Permanent, "earningsrefresh: bad report_date_override of " + e.Issuer}
				j.logFailure("override_date", te)
				return outcome{res: contracts.JobResult{Err: te}}
			}
			row.Status, row.Source, row.ReportDate = "listed", "owner", d
			rows = append(rows, row)
		case j.srcs[e.Market] == nil:
			row.Status, row.Source = "unsupported", "none"
			rows = append(rows, row)
		default:
			pending[e.Market] = append(pending[e.Market], e)
		}
	}
	fetched, failed := j.fetchPending(ctx, day, pending, base)
	rows = append(rows, fetched...)
	return outcome{res: j.write(ctx, up, replay, "earnings_refresh:"+string(day), rows), failed: failed}
}

// countedFailures returns the failed markets (US, then JP) that fail this run: the market of the one watch entry
// named by keys, or every failed market when keys is empty or names no entry.
func (j *job) countedFailures(keys []contracts.ExpectedKey, failed map[string]string) []string {
	only := ""
	if len(keys) == 1 {
		for _, e := range j.entries {
			if e.Issuer == keys[0].SubjectKey {
				only = e.Market
			}
		}
	}
	var out []string
	for _, m := range []string{"US", "JP"} {
		if _, ok := failed[m]; ok && (only == "" || only == m) {
			out = append(out, m)
		}
	}
	return out
}

// fetchPending makes the one Fetch of each market that has pending entries and returns their rows and the error code
// of each market whose Fetch failed.
func (j *job) fetchPending(
	ctx context.Context, day contracts.Day, pending map[string][]jobs.WatchEntry,
	base func(jobs.WatchEntry) contracts.EarningsDate,
) (rows []contracts.EarningsDate, failed map[string]string) {
	failed = map[string]string{}
	for _, market := range []string{"US", "JP"} {
		entries := pending[market]
		if len(entries) == 0 {
			continue
		}
		listings, err := j.srcs[market].Fetch(ctx)
		for _, e := range entries {
			row := base(e)
			row.Source = vendor[market].Source
			if err != nil {
				row.Status, row.ErrorCode = "error", "unknown"
				var ce sources.CodedError
				if errors.As(err, &ce) && ce.ErrorCode() != "" {
					row.ErrorCode = ce.ErrorCode()
				}
				failed[market] = row.ErrorCode
			} else {
				row = listed(row, e, listings, string(day), j.tr)
			}
			rows = append(rows, row)
		}
	}
	return rows, failed
}

// write upserts all rows in one call; a store error is Retryable unless already typed.
func (j *job) write(ctx context.Context, up fundkey.EarningsUpserter, replay bool, writtenBy string, rows []contracts.EarningsDate) contracts.JobResult {
	if len(rows) == 0 {
		return contracts.JobResult{}
	}
	put := up.UpsertEarnings
	if replay {
		put = up.ReplayEarnings
	}
	if _, err := put(ctx, rows, writtenBy); err != nil {
		j.logFailure("write", err)
		return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
	}
	res := contracts.JobResult{Returned: len(rows)}
	for _, row := range rows {
		if row.Status != "error" {
			res.Usable++
		}
	}
	return res
}

// listed fills row with the earliest listing of the entry on or after day, else status absent.
func listed(row contracts.EarningsDate, e jobs.WatchEntry, listings []sources.EarningsListing, day string, tr symbol.Translator) contracts.EarningsDate {
	var want string
	if tr != nil {
		var err error
		if want, err = tr.ToVendor(e.Contract.Symbol, symbol.AlphaVantage); err != nil {
			slog.Warn("earningsrefresh: entry symbol not translated", "symbol", e.Contract.Symbol, "error", err.Error())
			row.Status = "absent"
			return row
		}
	}
	var best *sources.EarningsListing
	for i := range listings {
		l := &listings[i]
		if l.ReportDate == "" || string(l.ReportDate) < day {
			continue
		}
		if tr == nil {
			if !sameSymbol(l.Symbol, e.Contract.Symbol) {
				continue
			}
		} else if got, err := tr.ToVendor(l.Symbol, symbol.AlphaVantage); err != nil || got != want {
			continue
		}
		if best == nil || l.ReportDate < best.ReportDate {
			best = l
		}
	}
	if best == nil {
		row.Status = "absent"
		return row
	}
	d, err := date(string(best.ReportDate))
	if err != nil {
		row.Status = "absent"
		return row
	}
	row.Status, row.ReportDate, row.Symbol, row.TimeOfDay = "listed", d, best.Symbol, best.TimeOfDay
	if f, err := date(string(best.FiscalPeriodEnd)); err == nil {
		row.FiscalPeriodEnd = f
	}
	if best.Estimate != "" {
		est := best.Estimate
		row.Estimate = &est
	}
	return row
}
