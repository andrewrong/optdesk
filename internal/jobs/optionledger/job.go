package optionledger

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
	// The zone database is embedded so that a zone name loads on every machine.
	_ "time/tzdata"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

//go:embed job.toml
var manifestBytes []byte

const (
	datasetID     = "optionledger.flex_trade_row"
	parserVersion = "1"
	statusOK      = "ok"
	statusFailed  = "failed"
	nyZone        = "America/New_York"
)

var _ jobs.NewOptionLedgerJobFunc = New

type manifest struct {
	Version     int    `toml:"version"`
	ID          string `toml:"id"`
	HistoryFrom string `toml:"history_from"`
	Schedule    struct {
		Calendar         string `toml:"calendar"`
		Anchor           string `toml:"anchor"`
		Offset           string `toml:"offset"`
		RunBound         string `toml:"run_bound"`
		RestartSensitive bool   `toml:"restart_sensitive"`
	} `toml:"schedule"`
}

type job struct {
	m        manifest
	offset   time.Duration
	runBound time.Duration
	stores   jobs.OptionLedgerStores
	cfg      jobs.OptionLedgerConfig
	clk      clock.Clock
	loc      *time.Location
	log      *slog.Logger

	// The runner calls Run once per key of Plan; Flex is fetched once per day, so the first complete Run of a
	// day is remembered and the calls that follow, for the other keys or a retry of the key, return it.
	mu   sync.Mutex
	done map[contracts.Day]contracts.JobResult
}

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

// New builds the option_ledger job from the embedded job.toml. A login with a nil Source is accepted.
func New(s jobs.OptionLedgerStores, cfg jobs.OptionLedgerConfig, clk clock.Clock) (contracts.Job, error) {
	return newJob(s, cfg, clk, slog.Default())
}

func newJob(s jobs.OptionLedgerStores, cfg jobs.OptionLedgerConfig, clk clock.Clock, log *slog.Logger) (*job, error) {
	if s.Ledger == nil || s.Read == nil || clk == nil {
		return nil, errors.New("optionledger: ledger, reader and clock are required")
	}
	var m manifest
	if err := toml.Unmarshal(manifestBytes, &m); err != nil {
		return nil, fmt.Errorf("optionledger: manifest: %w", err)
	}
	if m.ID == "" {
		return nil, errors.New("optionledger: manifest needs an id")
	}
	offset, err := time.ParseDuration(m.Schedule.Offset)
	if err != nil {
		return nil, errors.New("optionledger: manifest: schedule.offset is not a duration")
	}
	bound, err := time.ParseDuration(m.Schedule.RunBound)
	if err != nil {
		return nil, errors.New("optionledger: manifest: schedule.run_bound is not a duration")
	}
	loc, err := time.LoadLocation(nyZone)
	if err != nil {
		return nil, err
	}
	if cfg.HistoryFrom == "" {
		cfg.HistoryFrom = m.HistoryFrom
	}
	if _, err := time.Parse(dayLayout, cfg.HistoryFrom); err != nil {
		return nil, errors.New("optionledger: history_from is not a day")
	}
	return &job{m: m, offset: offset, runBound: bound, stores: s, cfg: cfg, clk: clk, loc: loc, log: log, done: map[contracts.Day]contracts.JobResult{}}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest: session_close plus 60 minutes.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	return j.m.Schedule.Calendar, j.m.Schedule.Anchor, "", j.offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.runBound }

// RestartSensitive is false: a rerun fetches the same windows and the store ignores rows it holds.
func (j *job) RestartSensitive() bool { return j.m.Schedule.RestartSensitive }

// Plan returns one key per alias of the allow-list, in alias order, and the hash of the plan. A key carries
// an alias only.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	seen := map[string]bool{}
	var aliases []string
	for _, a := range j.cfg.Accounts {
		if a != "" && !seen[a] {
			seen[a] = true
			aliases = append(aliases, a)
		}
	}
	sort.Strings(aliases)
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%d\n%s", j.m.ID, j.m.Version, day)
	keys := make([]contracts.ExpectedKey, 0, len(aliases))
	for _, a := range aliases {
		keys = append(keys, contracts.ExpectedKey{DatasetID: datasetID, SubjectKey: a})
		fmt.Fprintf(h, "\n%s", a)
	}
	return keys, hex.EncodeToString(h.Sum(nil)), nil
}

// run is the state of one Run call.
type run struct {
	j        *job
	last     contracts.Day
	stored   map[string]bool
	storeErr error
	degraded map[string]bool
	// failedRequired holds the names of the required logins that failed.
	failedRequired []string
	res            contracts.JobResult
}

// Run fetches every query of every login once per window, once per day whatever the keys: the keys are
// the aliases of Plan and every one of them gets the result of the day's one fetch, a retry of a key
// included, so a failed window is not retried that day. A failed window stops that query for the run. A
// failure of a login without Required is one degraded log record and leaves the result ok; a failure of a
// Required login fails the result. A run cut by ctx is not remembered.
func (j *job) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	j.mu.Lock()
	defer j.mu.Unlock()
	if res, ok := j.done[day]; ok {
		return res
	}
	res := j.fetchAll(ctx)
	if ctx.Err() == nil {
		j.done[day] = res
	}
	return res
}

// Postcondition is true when the day's fetch has run to its end without failing the result: a day whose
// rows were all unchanged, or whose only failures were of logins without Required, leaves no key missing.
func (j *job) Postcondition(_ context.Context, day, _, _ string) (bool, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	res, ok := j.done[contracts.Day(day)]
	return ok && res.Err == nil, nil
}

func (j *job) fetchAll(ctx context.Context) contracts.JobResult {
	now := j.clk.Now().In(j.loc)
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
	r := &run{j: j, last: contracts.Day(yesterday.Format(dayLayout)), degraded: map[string]bool{}}
	aliases, err := j.stores.Read.StoredAliases(ctx)
	r.storeErr = err
	r.stored = make(map[string]bool, len(aliases))
	for _, a := range aliases {
		r.stored[a] = true
	}
	for _, l := range j.cfg.Logins {
		if l.Source == nil {
			r.fail(l, "not_configured")
			continue
		}
		for _, q := range l.QueryIDs {
			r.query(ctx, l, q)
		}
	}
	if len(r.failedRequired) > 0 {
		r.res.Err = &typedError{kind: contracts.Permanent, err: fmt.Errorf("optionledger: required login %s failed", r.failedRequired[0])}
	}
	return r.res
}

// fail records the failure of a login: the degraded log record (login and status only) for a login
// without Required, the failed result for a Required one.
func (r *run) fail(l jobs.FlexRun, status string) {
	if l.Required {
		r.failedRequired = append(r.failedRequired, l.Name)
		return
	}
	if !r.degraded[l.Name] {
		r.degraded[l.Name] = true
		r.j.log.Warn("optionledger: degraded", "login", l.Name, "status", status)
	}
}

func (r *run) rowsForAll() bool {
	if r.storeErr != nil {
		return false
	}
	for _, a := range r.j.cfg.Accounts {
		if !r.stored[a] {
			return false
		}
	}
	return true
}

func (r *run) query(ctx context.Context, l jobs.FlexRun, queryID string) {
	rd := r.j.stores.Read
	rec, has, err := rd.LatestOkFetch(ctx, l.Name, queryID)
	if err != nil {
		r.fail(l, "store_error")
		return
	}
	var okLast contracts.Day
	if has {
		okLast = rec.LastDay
	}
	wins, err := planWindows(contracts.Day(r.j.cfg.HistoryFrom), r.last, okLast, r.rowsForAll())
	if err != nil {
		r.fail(l, "bad_window")
		return
	}
	for _, w := range wins {
		if status := r.fetch(ctx, l, queryID, w); status != statusOK {
			r.fail(l, status)
			return
		}
	}
}

// fetch makes the one Fetch of a window and records it; it returns ok or the status of the failure.
func (r *run) fetch(ctx context.Context, l jobs.FlexRun, queryID string, w window) string {
	started := r.j.clk.Now()
	rec := store.FlexFetchRecord{Login: l.Name, QueryID: queryID, FirstDay: w.first, LastDay: w.last, StartedAt: started, ParserVersion: parserVersion}
	rep, err := l.Source.Fetch(ctx, sources.FlexRequest{QueryID: queryID, First: w.first, Last: w.last})
	var rows []store.FlexTradeRow
	if err == nil {
		rec.RawSHA256 = rep.RawSHA256
		rec.Statements = len(rep.Statements)
		for _, st := range rep.Statements {
			sr, ok, merr := mapStatement(r.j.cfg.Accounts, st)
			if merr != nil {
				err = merr
				break
			}
			if !ok {
				rec.UnmappedStatement++
				continue
			}
			rows = append(rows, sr...)
		}
	}
	rec.FinishedAt = r.j.clk.Now()
	if err != nil {
		rec.Status = statusFailed
		var te contracts.TypedError
		if errors.As(err, &te) {
			rec.ErrorKind = string(te.Kind())
		} else {
			rec.ErrorKind = "bad_report"
		}
		rec.ErrorCode = errorCode(err)
		if _, rerr := r.j.stores.Ledger.RecordFlexFetch(ctx, rec, nil); rerr != nil {
			return "store_error"
		}
		return statusFailed
	}
	rec.Status = statusOK
	rec.RowsParsed = len(rows)
	ins, err := r.j.stores.Ledger.RecordFlexFetch(ctx, rec, rows)
	if err != nil {
		return "store_error"
	}
	r.res.Returned += len(rows)
	r.res.Usable += ins.New + ins.Revised
	if r.j.stores.Sections != nil {
		if err := r.recordSections(ctx, l, sources.FlexRequest{QueryID: queryID, First: w.first, Last: w.last}, rep); err != nil {
			return "store_error"
		}
	}
	return statusOK
}

// errorCode is the short text of an error, at most 64 bytes; sources errors hold no token, query id or address.
func errorCode(err error) string {
	s := err.Error()
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}
