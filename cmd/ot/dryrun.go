package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// dryRunJobs are the broker jobs `run-once --dry-run` can run, in the order it runs them.
var dryRunJobs = []string{jobOptionEod, jobOptionEodJP, jobQuoteSnapshot}

// Row status values of the option_eod and quote_snapshot rows: 1 ok, 2 no quote; the others (3 not listed,
// 4 stale, 5 error) count as partial.
const (
	rowOk     = 1
	rowNoData = 2
)

// dryRunTimes holds, per job, the zone and the local time of the fixed clock: after freeze_after and before
// stop_after of option_eod_jp (stop_after makes a run refuse). quote_snapshot takes the slot of each key.
var dryRunTimes = map[string]struct {
	zone string
	hhmm int
}{
	jobOptionEod:   {"America/New_York", 1630},
	jobOptionEodJP: {"Asia/Tokyo", 1600},
}

// setClock is a fixed clock the dry run moves before each key.
type setClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *setClock) Now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *setClock) set(t time.Time) { c.mu.Lock(); c.t = t; c.mu.Unlock() }

// memResolver gives ids from a counter; the same key gives the same id.
type memResolver struct {
	mu  sync.Mutex
	ids map[store.Identifier]int64
}

func (r *memResolver) ResolveInstrument(_ context.Context, key store.Identifier, _ store.InstrumentSpec, _ []store.Identifier) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.ids[key]; ok {
		return id, nil
	}
	if r.ids == nil {
		r.ids = map[store.Identifier]int64{}
	}
	id := int64(len(r.ids) + 1)
	r.ids[key] = id
	return id, nil
}

// memSink keeps the status of every row it took; it is both writers.
type memSink struct {
	mu       sync.Mutex
	statuses []int
}

func (s *memSink) take(statuses ...int) {
	s.mu.Lock()
	s.statuses = append(s.statuses, statuses...)
	s.mu.Unlock()
}

func (s *memSink) InsertOptionEod(_ context.Context, rows []contracts.OptionEod) error {
	for _, r := range rows {
		s.take(r.Status)
	}
	return nil
}

func (s *memSink) InsertQuoteSnapshots(_ context.Context, rows []contracts.QuoteSnapshot) error {
	for _, r := range rows {
		s.take(r.Status)
	}
	return nil
}

// countSince returns the ok, partial and no_data counts of the rows taken after the first n.
func (s *memSink) countSince(n int) (total, ok, partial, noData int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.statuses[n:] {
		switch st {
		case rowOk:
			ok++
		case rowNoData:
			noData++
		default:
			partial++
		}
	}
	return len(s.statuses) - n, ok, partial, noData
}

func (s *memSink) len() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.statuses) }

// runDryRun runs the capture of the named broker jobs for day through the live broker into memory: no fence,
// no store, no ledger, no window and no guard. The error is a flag or socket error, or the dial error.
func runDryRun(ctx context.Context, a *app, names []string, dayFlag string) error {
	if a.profile.Name == "demo" {
		return errors.New("run-once --dry-run needs a real profile")
	}
	home := a.getenv("OT_HOME")
	if home == "" {
		return errors.New("OT_HOME is not set")
	}
	socket := filepath.Join(home, "run", "ibkr.sock")
	if len(socket) >= maxSocketPath {
		return errors.New("socket path too long")
	}
	if len(names) == 0 {
		names = dryRunJobs
	}
	day, err := dayOrDefault(a.deps.newClock(), dayFlag)
	if err != nil {
		return err
	}
	brk, err := a.deps.dialBroker(ctx, socket, broker.Options{})
	if err != nil {
		return fmt.Errorf("dry-run: dial broker failed: %w", err)
	}
	defer func() { _ = brk.Close() }()
	var keys, failed int
	for _, name := range names {
		k, f, err := dryRunJob(ctx, a, brk, name, day)
		if err != nil {
			return err
		}
		keys, failed = keys+k, failed+f
	}
	status := "ok"
	if failed > 0 {
		status = "failed"
	}
	_, err = fmt.Fprintf(a.stdout, "dry-run %s "+
		"status=%s keys=%d failed=%d\n", day, status, keys, failed)
	return err
}

// dryRunJob builds one job over the in-memory sink and runs each planned key alone; it returns the keys run and
// the keys that ended with an error.
func dryRunJob(ctx context.Context, a *app, brk broker.Broker, name string, day contracts.Day) (keys, failed int, err error) {
	wall := a.deps.newClock()
	clk := &setClock{}
	sink := &memSink{}
	job, err := dryRunBuild(a.deps, name, brk, &memResolver{}, sink, clk)
	if err != nil {
		return 0, 0, err
	}
	planned, _, perr := job.Plan(ctx, day)
	if perr != nil {
		printKeyLine(a, name, day, "-", "-", 0, [3]int{}, 0, perr)
		return 1, 1, nil
	}
	for _, k := range planned {
		before := sink.len()
		start := wall.Now()
		res, err := dryRunKey(ctx, job, clk, name, day, k)
		if err != nil {
			return keys, failed, err
		}
		_, ok, partial, noData := sink.countSince(before)
		printKeyLine(a, name, day, k.DatasetID, k.SubjectKey, res.Returned, [3]int{ok, partial, noData}, wall.Now().Sub(start), res.Err)
		keys++
		if res.Err != nil {
			failed++
		}
	}
	return keys, failed, nil
}

func printKeyLine(a *app, job string, day contracts.Day, dataset, subject string, requested int, c [3]int, elapsed time.Duration, err error) {
	code := "none"
	if err != nil {
		code = string(contracts.Permanent)
		var te contracts.TypedError
		if errors.As(err, &te) {
			code = string(te.Kind())
		}
	}
	_, _ = fmt.Fprintf(a.stdout, "dry-run job=%s day=%s dataset=%s subject=%s "+
		"requested=%d ok=%d "+
		"partial=%d "+
		"no_data=%d elapsed=%s error=%s\n",
		job, day, dataset, subject, requested, c[0], c[1], c[2], elapsed.Round(time.Millisecond), code)
}

func dryRunBuild(d deps, name string, b broker.Broker, res store.InstrumentResolver, sink *memSink, clk clock.Clock) (contracts.Job, error) {
	switch name {
	case jobOptionEod:
		return d.newOptionEod(b, res, sink, clk)
	case jobOptionEodJP:
		return d.newOptionEodJP(b, res, sink, clk)
	case jobQuoteSnapshot:
		return d.newQuoteSnapshot(b, res, sink, clk)
	}
	return nil, fmt.Errorf("--job %q is not one of %v", name, dryRunJobs)
}

// dryRunKey runs one key with the clock set so that no guard refuses it. The manifest of quote_snapshot (zone,
// grace) is not readable from here, so its zone is found by trying the slot as local time at every UTC offset
// of a quarter hour, from -12:00 up, for each key. The answers run in this order: NoValue while the offset is
// behind the manifest zone (the guard sees a later local time or another day), then the run at the slot of the
// zone (inside its window for any grace, zero too), then an empty answer with no error once the offset is ahead
// (the guard's "not yet", which does nothing). The run is the first answer that is neither NoValue nor empty;
// when that answer is the empty one, the run is the NoValue before it, which came from inside the window (no
// quote). A run that is refused at every offset returns its last answer.
func dryRunKey(ctx context.Context, job contracts.Job, clk *setClock, name string, day contracts.Day,
	k contracts.ExpectedKey,
) (contracts.JobResult, error) {
	try := func(m int) (contracts.JobResult, error) {
		at, err := dryRunNow(name, day, k, m)
		if err != nil {
			return contracts.JobResult{}, err
		}
		clk.set(at)
		return job.Run(ctx, day, []contracts.ExpectedKey{k}), nil
	}
	if name != jobQuoteSnapshot {
		return try(0)
	}
	var prev contracts.JobResult
	for m := -12 * 60; m <= 14*60; m += 15 {
		r, err := try(m)
		if err != nil {
			return r, err
		}
		refused := r.Err != nil && r.Err.Kind() == contracts.NoValue
		empty := r.Err == nil && r.Returned == 0
		switch {
		case empty && m > -12*60:
			return prev, nil
		case !refused:
			return r, nil
		}
		prev = r
	}
	return prev, nil
}

// dryRunNow is the instant of the fixed clock for key on day: the local time of dryRunTimes, or the slot of a
// quote_snapshot key read as local time at the UTC offset offsetMin minutes.
func dryRunNow(job string, day contracts.Day, k contracts.ExpectedKey, offsetMin int) (time.Time, error) {
	d, err := time.Parse(time.DateOnly, string(day))
	if err != nil {
		return time.Time{}, fmt.Errorf("day %q is not YYYY-MM-DD", day)
	}
	t, ok := dryRunTimes[job]
	if !ok {
		var hhmm int
		if _, err := fmt.Sscanf(k.SubjectKey, "%04d", &hhmm); err != nil {
			return time.Time{}, fmt.Errorf("slot %q is not HHMM", k.SubjectKey)
		}
		loc := time.FixedZone("", offsetMin*60)
		return time.Date(d.Year(), d.Month(), d.Day(), hhmm/100, hhmm%100, 0, 0, loc), nil
	}
	loc, err := time.LoadLocation(t.zone)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(d.Year(), d.Month(), d.Day(), t.hhmm/100, t.hhmm%100, 0, 0, loc), nil
}
