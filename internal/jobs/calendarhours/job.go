// Package calendarhours holds the jobs calendar_hours_us and calendar_hours_jp: each compares the session days
// of master.session_day with the liquid hours IBKR reports for one reference contract of its market, logs one line
// per differing day and replaces the market's rows of master.session_hours_notice. It never writes
// master.session_day (docs/design/calendar-ibkr-hours.md, entry 6).
package calendarhours

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"
	// The zone database is embedded so that a zone name loads on every machine.
	_ "time/tzdata"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/hoursnotice"
)

//go:embed job_us.toml
var manifestUS []byte

//go:embed job_jp.toml
var manifestJP []byte

const (
	datasetID = "master.session_hours_notice"
	dayLayout = "2006-01-02"
	tolerance = time.Minute
)

var _ NewCalendarHoursFunc = New

// market is the fixed part of one job: its manifest and its reference contract.
type market struct {
	manifest []byte
	ref      broker.Contract
}

var markets = map[string]market{
	"XNYS": {manifestUS, broker.Contract{SecType: "STK", Symbol: "SPY", Exchange: "SMART", Currency: "USD"}},
	"XTKS": {manifestJP, broker.Contract{SecType: "STK", Symbol: "7203", Exchange: "TSEJ", Currency: "JPY"}},
}

type scheduleTable struct {
	Calendar         string `toml:"calendar"`
	Anchor           string `toml:"anchor"`
	Zone             string `toml:"zone"`
	Offset           string `toml:"offset"`
	RunBound         string `toml:"run_bound"`
	RestartSensitive bool   `toml:"restart_sensitive"`
}

type manifest struct {
	Version  int           `toml:"version"`
	ID       string        `toml:"id"`
	Schedule scheduleTable `toml:"schedule"`
}

type job struct {
	m        manifest
	market   string
	ref      broker.Contract
	loc      *time.Location
	offset   time.Duration
	runBound time.Duration
	br       broker.Broker
	src      clock.SessionSource
	hours    clock.IBKRHours
	st       hoursnotice.Store
	clk      clock.Clock
	log      *slog.Logger
}

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind: kind, err: err}
}

// New builds the job of market (XNYS or XTKS) from its embedded manifest. A nil logger is slog.Default().
func New(mkt string, b broker.Broker, src clock.SessionSource, hours clock.IBKRHours, st hoursnotice.Store,
	clk clock.Clock, log *slog.Logger) (contracts.Job, error) {
	mk, ok := markets[mkt]
	if !ok {
		return nil, errors.New("calendarhours: market must be XNYS or XTKS")
	}
	if b == nil || src == nil || hours == nil || st == nil || clk == nil {
		return nil, errors.New("calendarhours: broker, session source, hours parser, notice store and clock are required")
	}
	if log == nil {
		log = slog.Default()
	}
	var m manifest
	if err := toml.Unmarshal(mk.manifest, &m); err != nil {
		return nil, fmt.Errorf("calendarhours: manifest: %w", err)
	}
	s := m.Schedule
	loc, err := time.LoadLocation(s.Zone)
	if m.ID == "" || s.Calendar != mkt || s.Anchor != "session_open" || s.Zone == "" || err != nil {
		return nil, errors.New("calendarhours: manifest: id, calendar, anchor and zone are required")
	}
	off, err := time.ParseDuration(s.Offset)
	if err != nil {
		return nil, errors.New("calendarhours: manifest: schedule.offset is not a duration")
	}
	bound, err := time.ParseDuration(s.RunBound)
	if err != nil || bound < 0 {
		return nil, errors.New("calendarhours: manifest: schedule.run_bound is not a non-negative duration")
	}
	return &job{m: m, market: mkt, ref: mk.ref, loc: loc, offset: off, runBound: bound,
		br: b, src: src, hours: hours, st: st, clk: clk, log: log}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	return j.m.Schedule.Calendar, j.m.Schedule.Anchor, j.m.Schedule.Zone, j.offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.runBound }

// RestartSensitive is false: a run is one resolve and one replace of the market's rows.
func (j *job) RestartSensitive() bool { return false }

// Plan returns the one key {master.session_hours_notice, calendar}, never a master.session_day key.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	cal := j.m.Schedule.Calendar
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", j.m.ID, j.m.Version, day, cal)))
	return []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: cal}}, hex.EncodeToString(sum[:]), nil
}

// Postcondition is true: the rows are replaced as a whole, so a successful run is the proof.
func (j *job) Postcondition(context.Context, string, string, string) (bool, error) { return true, nil }

func (j *job) fail(step string, err error, kind contracts.ErrorKind) contracts.JobResult {
	te := asTyped(err, kind)
	j.log.Warn("run failed", "market", j.market, "step", step, "kind", string(te.Kind()))
	return contracts.JobResult{Err: te}
}

// Run makes one ResolveContract, compares the days IBKR lists with the stored rows and replaces the market's
// notice rows. A failure before the replace leaves the rows as they were.
func (j *job) Run(ctx context.Context, _ contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	res, err := j.br.ResolveContract(ctx, j.ref)
	if err != nil {
		kind, code := contracts.Retryable, "unknown"
		var te contracts.TypedError
		if errors.As(err, &te) {
			kind, code = te.Kind(), string(te.Kind())
		}
		return j.fail("resolve", &typedError{kind: kind,
			err: fmt.Errorf("calendarhours: %s: resolve failed, code %s: %w", j.market, code, err)}, kind)
	}
	if !res.Listed {
		return j.fail("resolve", &typedError{kind: contracts.Permanent,
			err: fmt.Errorf("calendarhours: %s: reference contract not listed, code %d", j.market, res.ErrorCode)}, contracts.Permanent)
	}
	days, err := j.hours.Parse(res.LiquidHours, res.TimeZoneID)
	if err != nil {
		return j.fail("parse", fmt.Errorf("calendarhours: %s: %w", j.market, err), contracts.Permanent)
	}
	out := contracts.JobResult{Returned: len(days), Usable: len(days)}
	j.log.Debug("calendar hours compared", "market", j.market, "days", len(days), "at", j.clk.Now())
	var notices []hoursnotice.Notice
	if len(days) > 0 {
		ours, err := j.ourRows(ctx, days)
		if err != nil {
			return j.fail("read_session_days", err, contracts.Retryable)
		}
		notices = j.compare(days, ours)
	}
	if err := j.st.Replace(ctx, j.market, notices); err != nil {
		out.Usable = 0
		r := j.fail("write", err, contracts.Retryable)
		out.Err = r.Err
	}
	return out
}

func (j *job) ourRows(ctx context.Context, days []clock.HoursDay) (map[string]contracts.SessionDay, error) {
	lo, hi := days[0].Day, days[0].Day
	for _, d := range days {
		if d.Day < lo {
			lo = d.Day
		}
		if d.Day > hi {
			hi = d.Day
		}
	}
	rows, err := j.src.SessionDays(ctx, j.market, lo, hi)
	if err != nil {
		return nil, err
	}
	m := make(map[string]contracts.SessionDay, len(rows))
	for _, r := range rows {
		m[r.SessionDay.Format(dayLayout)] = r
	}
	return m, nil
}

func near(a, b time.Time) bool {
	d := a.Sub(b)
	return d > -tolerance && d < tolerance
}

func (j *job) span(open, closeAt time.Time) string {
	return open.In(j.loc).Format("1504") + "-" + closeAt.In(j.loc).Format("1504")
}

// compare returns one notice per day IBKR lists on which our row differs; a day without a row is not compared.
func (j *job) compare(days []clock.HoursDay, ours map[string]contracts.SessionDay) []hoursnotice.Notice {
	var out []hoursnotice.Notice
	for _, d := range days {
		o, ok := ours[string(d.Day)]
		if !ok {
			continue
		}
		differ := o.IsSession != d.IsSession ||
			(o.IsSession && (!near(o.OpenAt, d.OpenAt) || !near(o.CloseAt, d.CloseAt)))
		if !differ {
			continue
		}
		n := hoursnotice.Notice{Market: j.market, Day: string(d.Day), Ours: "CLOSED", IBKR: "CLOSED"}
		if o.IsSession {
			n.Ours = j.span(o.OpenAt, o.CloseAt)
		}
		if d.IsSession {
			n.IBKR = j.span(d.OpenAt, d.CloseAt)
		}
		j.log.Info("calendar hours differ", "market", n.Market, "day", n.Day, "ours", n.Ours, "ibkr", n.IBKR)
		out = append(out, n)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Day < out[b].Day })
	return out
}
