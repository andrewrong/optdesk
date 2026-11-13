package barsrefresh

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
)

// dayOf is runDay (2026-11-09) shifted by n days, as YYYY-MM-DD.
func dayOf(n int) string {
	d, _ := time.Parse(dayLayout, string(runDay))
	return d.AddDate(0, 0, n).Format(dayLayout)
}

// pieceHistory answers the daily and backfill pulls with one close on the run day, and a piece (a
// query with an End) with a bar on End and one on its first day, so the piece spans its whole Days.
func pieceHistory() *fakeHistory {
	return &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.End == "" {
			return []broker.Bar{mkbar(string(runDay), "10.0000")}, nil
		}
		end, _ := time.Parse(dayLayout, q.End)
		first := end.AddDate(0, 0, 1-q.DurationDays).Format(dayLayout)
		return []broker.Bar{mkbar(first, "5.0000"), mkbar(q.End, "6.0000")}, nil
	}}
}

// deepenJob builds a job for the names (con ids 101, 102, ...) with the weekly step on.
func deepenJob(t *testing.T, h *fakeHistory, st *memBars, names int, tune func(*Manifest)) *job {
	t.Helper()
	var es []jobs.WatchEntry
	for i := 0; i < names; i++ {
		es = append(es, entry("cik:"+string(rune('1'+i)), "N"+string(rune('A'+i)), int64(101+i)))
	}
	j, err := New(&fakeBroker{listed: true}, h, fakeResolver{}, st, st, es, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	if tune != nil {
		tune(&j.(*job).m)
	}
	return j.(*job)
}

func allKeys(names int) []contracts.ExpectedKey {
	var subjects []string
	for i := 0; i < names; i++ {
		subjects = append(subjects, "cik:"+string(rune('1'+i)))
	}
	return keysOf(subjects...)
}

// storedFor is a name's trades rows: the run day and the oldest day, which is age days before it.
func storedFor(id, age, epoch int) []contracts.PriceBar {
	return []contracts.PriceBar{stored(id, string(runDay), "10.0000", epoch), stored(id, dayOf(-age), "9.0000", epoch)}
}

func pieces(h *fakeHistory) []broker.BarQuery {
	var out []broker.BarQuery
	for _, q := range h.queries {
		if q.End != "" {
			out = append(out, q)
		}
	}
	return out
}

func TestDeepenWhenOldestIsNewer(t *testing.T) {
	t.Parallel()
	h := pieceHistory()
	st := &memBars{rows: storedFor(101, 400, 3)}
	j := deepenJob(t, h, st, 1, nil)
	if res := j.Run(context.Background(), runDay, allKeys(1)); res.Err != nil {
		t.Fatal(res.Err)
	}
	ps := pieces(h)
	if len(ps) == 0 || ps[0].Kind != broker.BarTrades || ps[0].End != dayOf(-401) || ps[0].Contract.ConID != 101 {
		t.Fatalf("queries %+v", h.queries)
	}
	got, _ := st.PriceBars(context.Background(), 101, "trades", firstDay, runDay)
	var older int
	for _, r := range got {
		if r.SessionDay.Format(dayLayout) < dayOf(-400) {
			older++
			if r.Basis != "trades" {
				t.Fatalf("row %+v", r)
			}
		}
	}
	if older == 0 {
		t.Fatalf("no older rows in %+v", got)
	}
}

func TestDeepenSkippedWhenDeepEnough(t *testing.T) {
	t.Parallel()
	h := pieceHistory()
	st := &memBars{rows: storedFor(101, 1800, 1)} // older than 1825 - 30 days
	j := deepenJob(t, h, st, 1, nil)
	if res := j.Run(context.Background(), runDay, allKeys(1)); res.Err != nil {
		t.Fatal(res.Err)
	}
	if ps := pieces(h); len(ps) != 0 {
		t.Fatalf("pieces %+v", ps)
	}
}

func TestDeepenSkippedBeforeFirstBackfill(t *testing.T) {
	t.Parallel()
	h := pieceHistory()
	st := &memBars{}
	j := deepenJob(t, h, st, 1, nil)
	if res := j.Run(context.Background(), runDay, allKeys(1)); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) < 1 || h.queries[0].DurationDays != 400 || len(pieces(h)) != 0 {
		t.Fatalf("queries %+v", h.queries)
	}
}

func TestDeepenInsertsOnlyOlderDays(t *testing.T) {
	t.Parallel()
	oldest := dayOf(-400)
	h := &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.End == "" {
			return []broker.Bar{mkbar(string(runDay), "10.0000")}, nil
		}
		if q.End <= dayOf(-1001) { // the second piece ends the loop
			return []broker.Bar{mkbar(dayOf(-1900), "4.0000")}, nil
		}
		return []broker.Bar{mkbar(dayOf(-1000), "5.0000"), mkbar(oldest, "99.0000"), mkbar(dayOf(-300), "99.0000")}, nil
	}}
	st := &memBars{rows: storedFor(101, 400, 1)}
	j := deepenJob(t, h, st, 1, nil)
	if res := j.Run(context.Background(), runDay, allKeys(1)); res.Err != nil {
		t.Fatal(res.Err)
	}
	var added []contracts.PriceBar
	for _, r := range st.rows {
		if r.Basis == "trades" && r.SessionDay.Format(dayLayout) < oldest {
			added = append(added, r)
		}
	}
	if len(added) != 2 {
		t.Fatalf("added %+v", added)
	}
	for _, r := range st.rows {
		if r.Basis == "trades" && r.Close == "99.0000" {
			t.Fatalf("row not older than the oldest stored day was inserted: %+v", r)
		}
	}
}

func TestDeepenPiecesAreAtMost730Days(t *testing.T) {
	t.Parallel()
	h := pieceHistory()
	st := &memBars{rows: storedFor(101, 400, 1)}
	j := deepenJob(t, h, st, 1, nil)
	if res := j.Run(context.Background(), runDay, allKeys(1)); res.Err != nil {
		t.Fatal(res.Err)
	}
	ps := pieces(h)
	if len(ps) != 2 {
		t.Fatalf("pieces %+v", ps)
	}
	if ps[0].DurationDays > 730 || ps[1].DurationDays > 730 {
		t.Fatalf("pieces %+v", ps)
	}
	if ps[0].End != dayOf(-401) {
		t.Fatalf("first piece %+v", ps[0])
	}
	// The first piece's oldest row is its End minus its days plus one; the next ends the day before.
	first, _ := time.Parse(dayLayout, ps[0].End)
	want := first.AddDate(0, 0, -ps[0].DurationDays).Format(dayLayout)
	if ps[1].End != want {
		t.Fatalf("second piece ends %s, want %s", ps[1].End, want)
	}
}

func TestDeepenZeroCapIsUnlimited(t *testing.T) {
	t.Parallel()
	h := pieceHistory()
	st := &memBars{}
	for i := 0; i < 3; i++ {
		st.rows = append(st.rows, storedFor(101+i, 400, 1)...)
	}
	j := deepenJob(t, h, st, 3, func(m *Manifest) { m.DeepenNamesPerRun = 0 })
	if res := j.Run(context.Background(), runDay, allKeys(3)); res.Err != nil {
		t.Fatal(res.Err)
	}
	seen := map[int64]bool{}
	for _, q := range pieces(h) {
		seen[q.Contract.ConID] = true
	}
	if len(seen) != 3 {
		t.Fatalf("names deepened %v", seen)
	}
}

func TestDeepenCapDefersWithoutRequest(t *testing.T) {
	t.Parallel()
	h := pieceHistory()
	st := &memBars{}
	for i := 0; i < 2; i++ {
		st.rows = append(st.rows, storedFor(101+i, 400, 1)...)
	}
	j := deepenJob(t, h, st, 2, func(m *Manifest) { m.DeepenNamesPerRun = 1 })
	if res := j.Run(context.Background(), runDay, allKeys(2)); res.Err != nil {
		t.Fatal(res.Err)
	}
	for _, q := range pieces(h) {
		if q.Contract.ConID != 101 {
			t.Fatalf("name past the cap asked: %+v", q)
		}
	}
	if len(pieces(h)) == 0 {
		t.Fatal("the name within the cap was not deepened")
	}
	// The deferred name is not marked done for the week: its weekly step is still due the next day.
	if adj, _ := st.PriceBars(context.Background(), 102, "adjusted_last", firstDay, runDay); len(adj) != 0 {
		t.Fatalf("deferred name got its weekly pull: %+v", adj)
	}
	if res := j.Run(context.Background(), "2026-11-10", allKeys(2)); res.Err != nil {
		t.Fatal(res.Err)
	}
	var second bool
	for _, q := range pieces(h) {
		second = second || q.Contract.ConID == 102
	}
	if !second {
		t.Fatal("deferred name not deepened the next day")
	}
}

func TestYoungListingAskedOncePerWeek(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.End != "" {
			return nil, nil // listed for less than 5 years: no older day
		}
		return []broker.Bar{mkbar(string(runDay), "10.0000")}, nil
	}}
	st := &memBars{rows: storedFor(101, 400, 1)}
	j := deepenJob(t, h, st, 1, nil)
	for _, day := range []contracts.Day{runDay, "2026-11-10", "2026-11-15", "2026-11-16"} {
		if res := j.Run(context.Background(), day, allKeys(1)); res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	if ps := pieces(h); len(ps) != 2 {
		t.Fatalf("a young listing was asked %d times in 8 days: %+v", len(ps), ps)
	}
}

func TestYoungListingWithEmptyAdjustedAskedOncePerWeek(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.End != "" || q.Kind == broker.BarAdjustedLast {
			return nil, nil // no older day, and no adjusted_last answer
		}
		return []broker.Bar{mkbar(string(runDay), "10.0000")}, nil
	}}
	st := &memBars{rows: storedFor(101, 400, 1)}
	j := deepenJob(t, h, st, 1, nil)
	for _, day := range []contracts.Day{runDay, "2026-11-10"} {
		if res := j.Run(context.Background(), day, allKeys(1)); res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	if ps := pieces(h); len(ps) != 1 {
		t.Fatalf("asked %d times in 2 days: %+v", len(ps), ps)
	}
}
