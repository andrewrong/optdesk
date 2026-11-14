package fredseries

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store/seriesdef"
)

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Unix(0, 0) }

type fakeSource struct {
	results []fetchResult
	plans   []sources.FetchPlan
}

type fetchResult struct {
	points []contracts.SeriesPoint
	err    error
}

func (s *fakeSource) Fetch(_ context.Context, p sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	s.plans = append(s.plans, p)
	if len(s.results) == 0 {
		return nil, nil
	}
	r := s.results[0]
	s.results = s.results[1:]
	return r.points, r.err
}

// fakeWriter is a store.SeriesWriter with the optional upsert method.
type fakeWriter struct {
	calls     [][]contracts.SeriesPoint
	writtenBy []string
}

func (w *fakeWriter) InsertSeriesPoints(context.Context, []contracts.SeriesPoint) error {
	return errors.New("fakeWriter: insert must not be used")
}

func (w *fakeWriter) UpsertSeriesPoints(_ context.Context, rows []contracts.SeriesPoint, writtenBy string) (seriesdef.WriteCount, error) {
	w.calls = append(w.calls, rows)
	w.writtenBy = append(w.writtenBy, writtenBy)
	return seriesdef.WriteCount{Written: len(rows)}, nil
}

// insertOnlyWriter has no upsert method.
type insertOnlyWriter struct{ calls int }

func (w *insertOnlyWriter) InsertSeriesPoints(context.Context, []contracts.SeriesPoint) error {
	w.calls++
	return nil
}

type kindErr struct{ kind contracts.ErrorKind }

func (e *kindErr) Error() string             { return "boom" }
func (e *kindErr) Kind() contracts.ErrorKind { return e.kind }

func val(v float64) contracts.SeriesPoint {
	return contracts.SeriesPoint{SeriesID: "DGS10", Status: "present", Value: &v}
}

var keys = []contracts.ExpectedKey{{DatasetID: "obs.series_point", SubjectKey: "DGS10"}}

func TestRunWritesOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	day := contracts.Day("2026-11-05")
	points := []contracts.SeriesPoint{val(4.1), {SeriesID: "DGS10", Status: "no_value"}}

	src := &fakeSource{results: []fetchResult{{points: points}}}
	w := &fakeWriter{}
	j, err := New(src, w, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(ctx, day, keys)
	if res.Err != nil || res.Returned != 2 || res.Usable != 1 {
		t.Fatalf("result = %+v", res)
	}
	if len(w.calls) != 1 || len(w.calls[0]) != 2 {
		t.Fatalf("calls = %v", w.calls)
	}
	got := w.calls[0][1]
	if got.Value != nil || got.Status != "no_value" {
		t.Fatalf("no_value point = %+v", got)
	}
	if *w.calls[0][0].Value != 4.1 {
		t.Fatal("value changed")
	}
	p := src.plans[0]
	if len(p.SeriesIDs) != 1 || p.SeriesIDs[0] != "DGS10" || p.From != day || p.To != day {
		t.Fatalf("plan = %+v", p)
	}

	// A Retryable error comes back as the same value and nothing is written.
	rerr := &kindErr{kind: contracts.Retryable}
	src2 := &fakeSource{results: []fetchResult{{err: rerr}}}
	w2 := &fakeWriter{}
	j2, _ := New(src2, w2, fakeClock{})
	res = j2.Run(ctx, day, keys)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || !errors.Is(res.Err, rerr) {
		t.Fatalf("err = %v", res.Err)
	}
	if res.Err != contracts.TypedError(rerr) {
		t.Fatal("error is not the same value")
	}
	if len(w2.calls) != 0 {
		t.Fatal("written after error")
	}
}

func TestPlanLateRunRecovery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	day := contracts.Day("2026-11-05")
	fc := clockwork.NewFakeClock()
	var clk clock.Clock = fc

	src := &fakeSource{results: []fetchResult{
		{err: &kindErr{kind: contracts.Retryable}},
		{points: []contracts.SeriesPoint{val(4.2)}},
	}}
	w := &fakeWriter{}
	j, err := New(src, w, clk)
	if err != nil {
		t.Fatal(err)
	}
	ks, _, err := j.Plan(ctx, day)
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(ctx, day, ks); res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("first run = %+v", res)
	}
	fc.Advance(24 * time.Hour)
	if res := j.Run(ctx, day, ks); res.Err != nil || res.Returned != 1 || res.Usable != 1 {
		t.Fatalf("second run = %+v", res)
	}
	if len(w.calls) != 1 {
		t.Fatalf("writes = %d", len(w.calls))
	}
}

func keysOf(ids ...string) []contracts.ExpectedKey {
	out := make([]contracts.ExpectedKey, 0, len(ids))
	for _, id := range ids {
		out = append(out, contracts.ExpectedKey{DatasetID: "obs.series_point", SubjectKey: id})
	}
	return out
}

var (
	plainIDs    = []string{"DGS10", "DGS3MO", "RVXCLS", "SP500", "VIXCLS", "VXNCLS", "VXVCLS"}
	windowedIDs = []string{"BAA10Y", "BAMLC0A0CM", "BAMLH0A0HYM2", "DFF", "IORB", "NFCI", "RRPONTSYD", "SOFR", "SOFR99", "STLFSI4", "TLAACBW027SBOG", "WALCL", "WDTGAL", "WLCFLPCL", "WRESBAL"}
)

func TestRunFetchWindowPerGroup(t *testing.T) {
	t.Parallel()
	day := contracts.Day("2026-11-06")
	src := &fakeSource{}
	j, err := New(src, &fakeWriter{}, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	ks, _, err := j.Plan(context.Background(), day)
	if err != nil {
		t.Fatal(err)
	}
	if len(ks) != 22 {
		t.Fatalf("keys = %d", len(ks))
	}
	j.Run(context.Background(), day, ks)
	if len(src.plans) != 2 {
		t.Fatalf("fetch calls = %d", len(src.plans))
	}
	for _, p := range src.plans {
		if p.To != day {
			t.Fatalf("To = %s", p.To)
		}
		from := contracts.Day("2026-11-06")
		if slices.Contains(p.SeriesIDs, "WALCL") {
			from = "2026-10-23"
		}
		if p.From != from {
			t.Fatalf("ids %v From = %s, want %s", p.SeriesIDs, p.From, from)
		}
	}
	ids := func(p sources.FetchPlan) []string { s := slices.Clone(p.SeriesIDs); slices.Sort(s); return s }
	got := [][]string{ids(src.plans[0]), ids(src.plans[1])}
	if !slices.Equal(got[0], plainIDs) || !slices.Equal(got[1], windowedIDs) {
		t.Fatalf("groups = %v", got)
	}
}

func TestRunOnlyWindowedKeysMakeOneCall(t *testing.T) {
	t.Parallel()
	day := contracts.Day("2026-11-06")
	for name, c := range map[string]struct {
		ids  []string
		from contracts.Day
	}{
		"windowed": {windowedIDs, "2026-10-23"},
		"plain":    {plainIDs, day},
	} {
		src := &fakeSource{}
		j, err := New(src, &fakeWriter{}, fakeClock{})
		if err != nil {
			t.Fatal(err)
		}
		j.Run(context.Background(), day, keysOf(c.ids...))
		if len(src.plans) != 1 {
			t.Fatalf("%s: fetch calls = %d", name, len(src.plans))
		}
		if p := src.plans[0]; p.From != c.from || p.To != day || len(p.SeriesIDs) != len(c.ids) {
			t.Fatalf("%s: plan = %+v", name, p)
		}
	}
}

func TestRunWithoutUpsertIsRetryable(t *testing.T) {
	t.Parallel()
	w := &insertOnlyWriter{}
	src := &fakeSource{results: []fetchResult{{points: []contracts.SeriesPoint{val(4.1)}}}}
	j, err := New(src, w, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), "2026-11-05", keys)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("err = %v", res.Err)
	}
	if !strings.Contains(res.Err.Error(), "UpsertSeriesPoints") {
		t.Fatalf("error does not name the missing upsert: %v", res.Err)
	}
	if res.Usable != 0 || w.calls != 0 {
		t.Fatalf("usable %d, insert calls %d", res.Usable, w.calls)
	}
}

func TestRunUpsertsOnceWithWrittenBy(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	src := &fakeSource{results: []fetchResult{{points: []contracts.SeriesPoint{val(4.1)}}, {points: []contracts.SeriesPoint{val(5.2)}}}}
	j, err := New(src, w, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), "2026-11-05", keysOf("DGS10", "WALCL"))
	if res.Err != nil || res.Returned != 2 || res.Usable != 2 {
		t.Fatalf("result = %+v", res)
	}
	if len(w.calls) != 1 || len(w.calls[0]) != 2 {
		t.Fatalf("calls = %v", w.calls)
	}
	if w.writtenBy[0] != "fred_series:2026-11-05" {
		t.Fatalf("writtenBy = %q", w.writtenBy[0])
	}
}

func TestRunEmptyFetchMakesNoUpsert(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	j, err := New(&fakeSource{}, w, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), "2026-11-05", keys)
	if res.Err != nil || res.Returned != 0 || len(w.calls) != 0 {
		t.Fatalf("result = %+v, calls %v", res, w.calls)
	}
}
