package main

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

type fakeListReader struct {
	rows []underlying.Row
	err  error
}

func (f *fakeListReader) Current(context.Context) ([]underlying.Row, error) { return f.rows, f.err }
func (f *fakeListReader) History(context.Context, int) ([]underlying.Row, error) {
	return nil, nil
}

// fakeInner plans one key per entry symbol and records its runs.
type fakeInner struct {
	symbols []string
	runs    *[]*fakeInner
	tag     string
}

func (f *fakeInner) ID() string { return "fake" }
func (f *fakeInner) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	var keys []contracts.ExpectedKey
	for _, s := range f.symbols {
		keys = append(keys, contracts.ExpectedKey{DatasetID: "d", SubjectKey: s})
	}
	return keys, "h", nil
}
func (f *fakeInner) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	*f.runs = append(*f.runs, f)
	return contracts.JobResult{Returned: 1, Usable: 1}
}
func (f *fakeInner) Schedule() (string, string, string, time.Duration) {
	return "cal-" + f.tag, "anchor", "zone", time.Hour
}
func (f *fakeInner) RunBound() time.Duration { return 7 * time.Minute }
func (f *fakeInner) RestartSensitive() bool  { return true }

func activeRow(sym string) underlying.Row {
	return underlying.Row{Key: "US:" + sym, Symbol: sym, Market: "US", Status: underlying.StatusActive,
		Issuer: "cik:0000000001", ConID: 1}
}

type listedHarness struct {
	reader *fakeListReader
	built  int
	runs   []*fakeInner
	job    *listedJob
}

func newHarness(t *testing.T, group string, rows ...underlying.Row) *listedHarness {
	t.Helper()
	h := &listedHarness{reader: &fakeListReader{rows: rows}}
	build := func(es []jobs.WatchEntry) (contracts.Job, error) {
		h.built++
		in := &fakeInner{runs: &h.runs, tag: string(rune('a' + h.built))}
		for _, e := range es {
			in.symbols = append(in.symbols, e.Contract.Symbol)
		}
		return in, nil
	}
	held := func(context.Context) (map[string]bool, error) { return nil, nil }
	j, err := newListedJob(context.Background(), build, group, h.reader, held, clockwork.NewFakeClock())
	if err != nil {
		t.Fatal(err)
	}
	h.job = j
	return h
}

func subjects(keys []contracts.ExpectedKey) []string {
	var out []string
	for _, k := range keys {
		out = append(out, k.SubjectKey)
	}
	return out
}

func TestListedPlanReadsListAtCall(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := newHarness(t, groupBars, activeRow("AAA"))
	if h.built != 1 {
		t.Fatalf("built %d at start", h.built)
	}
	k1, _, err := h.job.Plan(ctx, "2026-11-11")
	if err != nil || !slices.Equal(subjects(k1), []string{"AAA"}) {
		t.Fatalf("plan 1 %v %v", k1, err)
	}
	h.reader.rows = []underlying.Row{activeRow("AAA"), activeRow("BBB")}
	k2, _, _ := h.job.Plan(ctx, "2026-11-11")
	if !slices.Equal(subjects(k2), []string{"AAA", "BBB"}) {
		t.Fatalf("plan 2 %v", k2)
	}
	paused := activeRow("AAA")
	paused.Status = underlying.StatusPaused
	h.reader.rows = []underlying.Row{paused, activeRow("BBB")}
	k3, _, _ := h.job.Plan(ctx, "2026-11-11")
	if !slices.Equal(subjects(k3), []string{"BBB"}) {
		t.Fatalf("plan 3 %v", k3)
	}
}

func TestListedRunInFlightRule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := newHarness(t, groupBars, activeRow("AAA"))
	k1, _, _ := h.job.Plan(ctx, "2026-11-11")
	first := h.job.current()
	h.reader.rows = []underlying.Row{activeRow("BBB")}
	if _, _, err := h.job.Plan(ctx, "2026-11-11"); err != nil {
		t.Fatal(err)
	}
	if h.job.current() == first {
		t.Fatal("newest did not move")
	}
	if res := h.job.Run(ctx, "2026-11-11", k1); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.runs) != 1 || h.runs[0] != first {
		t.Fatalf("run used %v, want plan 1's job", h.runs)
	}
	unknown := []contracts.ExpectedKey{{DatasetID: "d", SubjectKey: "ZZZ"}}
	res := h.job.Run(ctx, "2026-11-11", unknown)
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || res.Err.Error() != "no plan for these keys" {
		t.Fatalf("unknown keys: %v", res.Err)
	}
	// A Plan of a new day keeps the snapshots of the earlier day.
	if _, _, err := h.job.Plan(ctx, "2026-11-12"); err != nil {
		t.Fatal(err)
	}
	h.runs = nil
	if res := h.job.Run(ctx, "2026-11-11", k1); res.Err != nil {
		t.Fatalf("old day run: %v", res.Err)
	}
	if len(h.runs) != 1 || h.runs[0] != first {
		t.Fatalf("old day run used %v, want plan 1's job", h.runs)
	}
	if res := h.job.Run(ctx, "2026-11-13", k1); res.Err == nil || res.Err.Kind() != contracts.Permanent ||
		res.Err.Error() != "no plan for these keys" {
		t.Fatalf("never planned day: %v", res.Err)
	}
}

func TestListedKeepsDaysBounded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := newHarness(t, groupBars, activeRow("AAA"))
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	var days []contracts.Day
	var keys []contracts.ExpectedKey
	jobsOf := map[contracts.Day]contracts.Job{}
	for i := 0; i <= listedKeepDays; i++ {
		d := contracts.Day(start.AddDate(0, 0, i).Format("2006-01-02"))
		k, _, err := h.job.Plan(ctx, d)
		if err != nil {
			t.Fatal(err)
		}
		days, keys = append(days, d), k
		jobsOf[d] = h.job.current()
	}
	res := h.job.Run(ctx, days[0], keys)
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || res.Err.Error() != "no plan for these keys" {
		t.Fatalf("oldest day: %v", res.Err)
	}
	for _, d := range days[1:] {
		h.runs = nil
		if res := h.job.Run(ctx, d, keys); res.Err != nil {
			t.Fatalf("day %s: %v", d, res.Err)
		}
		if len(h.runs) != 1 || contracts.Job(h.runs[0]) != jobsOf[d] {
			t.Fatalf("day %s used another job", d)
		}
	}
}

func TestListedForwardsOptionalMethods(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := newHarness(t, groupBars, activeRow("AAA"))
	cal, anchor, zone, off := h.job.Schedule()
	if cal != "cal-b" || anchor != "anchor" || zone != "zone" || off != time.Hour {
		t.Fatalf("schedule %v %v %v %v", cal, anchor, zone, off)
	}
	if h.job.RunBound() != 7*time.Minute || !h.job.RestartSensitive() || h.job.ID() != "fake" {
		t.Fatal("forwarding")
	}
	if _, _, err := h.job.Plan(ctx, "2026-11-11"); err != nil {
		t.Fatal(err)
	}
	if cal, _, _, _ := h.job.Schedule(); cal != "cal-c" {
		t.Fatalf("schedule after plan: %v", cal)
	}
}

func TestListedEmptyStoreIsPermanent(t *testing.T) {
	t.Parallel()
	h := newHarness(t, groupBars, activeRow("AAA"))
	paused := activeRow("AAA")
	paused.Status = underlying.StatusPaused
	h.reader.rows = []underlying.Row{paused}
	keys, _, err := h.job.Plan(context.Background(), "2026-11-11")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent || err.Error() != "underlying list is empty" || len(keys) != 0 {
		t.Fatalf("keys %v err %v", keys, err)
	}
}

func TestListedReadErrorIsRetryableAndNeverStale(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := newHarness(t, groupBars, activeRow("AAA"))
	if _, _, err := h.job.Plan(ctx, "2026-11-11"); err != nil {
		t.Fatal(err)
	}
	h.reader.err = errors.New("db down")
	keys, _, err := h.job.Plan(ctx, "2026-11-11")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Retryable || len(keys) != 0 {
		t.Fatalf("keys %v err %v", keys, err)
	}
	h.reader.err = nil
	h.reader.rows = []underlying.Row{activeRow("BBB")}
	keys, _, err = h.job.Plan(ctx, "2026-11-11")
	if err != nil || !slices.Equal(subjects(keys), []string{"BBB"}) {
		t.Fatalf("after good read: %v %v", keys, err)
	}
}

func TestListedGroupOffPlansZeroKeys(t *testing.T) {
	t.Parallel()
	off := activeRow("AAA")
	off.Options = false
	h := newHarness(t, groupOptions, off)
	keys, _, err := h.job.Plan(context.Background(), "2026-11-11")
	if err != nil || len(keys) != 0 {
		t.Fatalf("keys %v err %v", keys, err)
	}
}

func TestListedJobHeldOptionOutsideGroup(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		options bool
	}{{"options off", false}, {"watched and held", true}} {
		row := activeRow("AAA")
		row.Options = c.options
		build := func([]jobs.WatchEntry) (contracts.Job, error) { return &fakeInner{}, nil }
		held := func(context.Context) (map[string]bool, error) { return map[string]bool{"option:US:AAA": true}, nil }
		j, err := newListedJob(context.Background(), build, groupOptions, &fakeListReader{rows: []underlying.Row{row}}, held,
			clockwork.NewFakeClock())
		if err != nil {
			t.Fatal(err)
		}
		entries, ok, err := j.entries(context.Background())
		if err != nil || !ok {
			t.Fatalf("%s: entries %v %v", c.name, ok, err)
		}
		if len(entries) != 1 || entries[0].Contract.Symbol != "AAA" {
			t.Errorf("%s: entries %+v", c.name, entries)
		}
	}
}
