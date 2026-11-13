package barsrefresh

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

const pastDay = contracts.Day("2026-11-05")

func closeOf(t *testing.T, st *memBars, day string) string {
	t.Helper()
	got, _ := st.PriceBars(context.Background(), 101, basisTrades, firstDay, "2026-12-31")
	for _, r := range got {
		if r.SessionDay.Format(dayLayout) == day {
			return r.Close
		}
	}
	return ""
}

func TestDailyPullReplacesDifferingDay(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "10.000001"))} // inside the restatement tolerance
	st := &memBars{rows: []contracts.PriceBar{stored(101, "2026-11-09", "10.0000", 1)}}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || len(st.rows) != 1 || st.rows[0].Close != "10.000001" || st.by[0] != "bars_refresh:2026-11-09" {
		t.Fatalf("queries %+v rows %+v by %v", h.queries, st.rows, st.by)
	}
}

func TestEqualDayIsNotRewritten(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "100.50"))}
	st := &memBars{rows: []contracts.PriceBar{stored(101, "2026-11-09", "100.5", 1)}}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(st.rows) != 1 || st.rows[0].Close != "100.5" {
		t.Fatalf("rows %+v", st.rows)
	}
}

func TestPastDayAsksEndDayAnd400Days(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-04", "10.0"), mkbar("2026-11-05", "11.0"))}
	st := &memBars{rows: []contracts.PriceBar{stored(101, "2026-11-04", "10.0", 1)}}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), pastDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].End != "2026-11-05" || h.queries[0].DurationDays != 400 {
		t.Fatalf("queries %+v", h.queries)
	}
	if len(st.calls) != 1 || st.by[0] != "bars_refresh:2026-11-05" || len(st.rows) != 2 {
		t.Fatalf("calls %+v by %v", st.calls, st.by)
	}
}

func TestTodayAsksNoEnd(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "10.0"))}
	st := &memBars{rows: []contracts.PriceBar{stored(101, "2026-11-06", "10.0", 1)}}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].End != "" || h.queries[0].DurationDays != 5 {
		t.Fatalf("queries %+v", h.queries)
	}
}

func TestBarsAfterDayAreDropped(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-04", "10.0"), mkbar("2026-11-05", "11.0"), mkbar("2026-11-06", "12.0"))}
	st := &memBars{}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	res := j.Run(context.Background(), pastDay, keysOf("cik:1"))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(st.rows) != 2 || closeOf(t, st, "2026-11-06") != "" || res.Returned != 2 {
		t.Fatalf("rows %+v result %+v", st.rows, res)
	}
}

func TestPastDayRunsNeitherAdjustedNorDeepen(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		return []broker.Bar{mkbar(q.End, "10.0")}, nil
	}}
	st := &memBars{rows: []contracts.PriceBar{stored(101, dayOf(-404), "9.0", 1)}} // oldest stored day is recent: deepening would be due
	j := deepenJob(t, h, st, 1, nil)
	if res := j.Run(context.Background(), pastDay, allKeys(1)); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].Kind != broker.BarTrades || h.queries[0].End != string(pastDay) {
		t.Fatalf("queries %+v", h.queries)
	}
	if adj, _ := st.PriceBars(context.Background(), 101, basisAdjusted, firstDay, "2026-12-31"); len(adj) != 0 {
		t.Fatalf("adjusted rows %+v", adj)
	}
}

func TestEmptyAnswerNeverReplaces(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed()}
	st := &memBars{rows: []contracts.PriceBar{stored(101, "2026-11-05", "10.0", 1)}}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), pastDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(st.calls) != 0 || len(st.rows) != 1 || st.rows[0].Close != "10.0" {
		t.Fatalf("calls %d rows %+v", len(st.calls), st.rows)
	}
}

// splitHistory answers a split on the run day: the daily pull and the 400-day window give half the
// stored close, and the older pieces (each with an End) give the bars of piece.
func splitHistory(piece func(q broker.BarQuery) ([]broker.Bar, error)) *fakeHistory {
	return &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		switch {
		case q.End != "":
			return piece(q)
		case q.DurationDays == 5:
			return []broker.Bar{mkbar(dayOf(0), "50.0000")}, nil
		}
		return []broker.Bar{mkbar(dayOf(0), "50.0000"), mkbar(dayOf(-400), "49.0000")}, nil
	}}
}

func twoPieces(q broker.BarQuery) ([]broker.Bar, error) {
	if q.End == dayOf(-401) {
		return []broker.Bar{mkbar(dayOf(-1100), "48.0000")}, nil
	}
	return []broker.Bar{mkbar(dayOf(-1500), "47.0000")}, nil
}

func splitStored() []contracts.PriceBar {
	return []contracts.PriceBar{
		stored(101, dayOf(0), "100.0000", 1), stored(101, dayOf(-400), "98.0000", 1),
		stored(101, dayOf(-1100), "96.0000", 1), stored(101, dayOf(-1500), "94.0000", 1),
	}
}

func TestRestatementRepullsOlderStoredDays(t *testing.T) {
	t.Parallel()
	h := splitHistory(twoPieces)
	st := &memBars{rows: splitStored()}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	var pieces []broker.BarQuery
	for _, q := range h.queries {
		if q.End != "" {
			pieces = append(pieces, q)
		}
	}
	if len(pieces) != 2 || pieces[0].End != dayOf(-401) || pieces[0].DurationDays != 730 ||
		pieces[1].End != dayOf(-1101) || pieces[1].DurationDays != 400 {
		t.Fatalf("pieces %+v", pieces)
	}
	want := map[string]string{dayOf(0): "50.0000", dayOf(-400): "49.0000", dayOf(-1100): "48.0000", dayOf(-1500): "47.0000"}
	if len(st.rows) != 4 {
		t.Fatalf("rows %+v", st.rows)
	}
	for d, c := range want {
		if got := closeOf(t, st, d); got != c {
			t.Fatalf("day %s close %q, want %s (rows %+v)", d, got, c, st.rows)
		}
	}
}

func TestRestatementWritesOnceAfterAllPulls(t *testing.T) {
	t.Parallel()
	h := splitHistory(twoPieces)
	st := &memBars{rows: splitStored()}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(st.calls) != 1 || len(st.calls[0]) != 4 || st.by[0] != "bars_refresh:2026-11-09" {
		t.Fatalf("calls %+v by %v", st.calls, st.by)
	}
}

func TestRestatementPullFailureWritesNothing(t *testing.T) {
	t.Parallel()
	h := splitHistory(func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.End == dayOf(-401) {
			return []broker.Bar{mkbar(dayOf(-1100), "48.0000")}, nil
		}
		return nil, errors.New("boom")
	})
	st := &memBars{rows: splitStored()}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	res := j.Run(context.Background(), runDay, keysOf("cik:1"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("err %v", res.Err)
	}
	if len(st.calls) != 0 || closeOf(t, st, dayOf(0)) != "100.0000" || closeOf(t, st, dayOf(-1100)) != "96.0000" {
		t.Fatalf("calls %d rows %+v", len(st.calls), st.rows)
	}
}

func TestRestatementStopsAtEmptyPiece(t *testing.T) {
	t.Parallel()
	h := splitHistory(func(broker.BarQuery) ([]broker.Bar, error) { return nil, nil })
	st := &memBars{rows: splitStored()}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 3 || len(st.calls) != 1 || len(st.calls[0]) != 2 {
		t.Fatalf("queries %+v calls %+v", h.queries, st.calls)
	}
}

// insertOnly hides every method of a writer but InsertPriceBars.
type insertOnly struct{ bar.PriceBarWriter }

func TestRunWithoutUpserterIsRetryable(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "10.0"))}
	st := &memBars{}
	b := &fakeBroker{listed: true}
	j, err := New(b, h, fakeResolver{}, insertOnly{st}, st, []jobs.WatchEntry{entry("cik:1", "AAA", 0)}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), runDay, keysOf("cik:1"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || len(h.queries) != 0 || b.resolved != 0 || len(st.rows) != 0 {
		t.Fatalf("result %+v queries %d resolved %d", res, len(h.queries), b.resolved)
	}
}
