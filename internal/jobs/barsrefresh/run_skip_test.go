package barsrefresh

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// nextDay is the day after runDay: a key whose newest stored row is runDay is complete for runDay and is not asked.
const nextDay = contracts.Day("2026-11-10")

func TestRunKeySkipsCompleteKey(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "11.0"))}
	st := &memBars{rows: []contracts.PriceBar{
		stored(101, "2026-11-09", "11.0", 1), // complete for the run day
		stored(102, "2026-11-08", "11.0", 1), // newest row a day earlier
	}}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101), entry("cik:2", "BBB", 102))
	for i := 0; i < 2; i++ { // a second run of the same day
		if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	if len(h.queries) != 0 || len(st.calls) != 0 {
		t.Fatalf("complete key: queries %+v, upserts %d", h.queries, len(st.calls))
	}
	if res := j.Run(context.Background(), runDay, keysOf("cik:2")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].DurationDays != 5 {
		t.Fatalf("incomplete key: queries %+v", h.queries)
	}
}

func TestSkippedKeyStillRunsAdjustedByItsOwnGate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		received string
		want     int // adjusted requests, and deepen pieces (a trades query with an End)
	}{
		{"adjusted due", "2026-11-02", 1},
		{"adjusted fresh", "2026-11-08", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := adjustedHistory(func() ([]broker.Bar, error) { return []broker.Bar{mkbar("2026-11-09", "9.5000")}, nil })
			st := &memBars{rows: []contracts.PriceBar{tradesRow(), storedAdjusted(101, "2026-11-09", "9.0000", 1, c.received)}}
			if res := runAdjusted(t, h, st); res.Err != nil {
				t.Fatal(res.Err)
			}
			if got := len(adjustedQueries(h)); got != c.want {
				t.Fatalf("adjusted requests %d, want %d", got, c.want)
			}
			var daily, deepen int
			for _, q := range h.queries {
				switch {
				case q.Kind != broker.BarTrades:
				case q.End == "":
					daily++
				default:
					deepen++
				}
			}
			if daily != 0 || deepen != c.want {
				t.Fatalf("daily requests %d, deepen pieces %d, want 0 and %d", daily, deepen, c.want)
			}
		})
	}
}

func TestWithinToleranceIsNotRestated(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "100.0005"))}
	st := &memBars{rows: []contracts.PriceBar{stored(101, "2026-11-09", "100.0000", 1)}}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), nextDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || len(st.epoch(2)) != 0 || len(st.rows) != 1 {
		t.Fatalf("queries %d rows %+v", len(h.queries), st.rows)
	}
}

func TestIndexConIDResolvedOnce(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.DurationDays == 5 {
			return []broker.Bar{mkbar("2026-11-09", "50.0000")}, nil
		}
		return []broker.Bar{mkbar("2026-11-09", "50.0000")}, nil
	}}
	// stored 100 against pulled 50 forces a second pull of the same key in one run
	st := &memBars{rows: []contracts.PriceBar{stored(7001, "2026-11-09", "100.0000", 1)}}
	j, b := build(t, h, st)
	res := j.Run(context.Background(), nextDay, keysOf("index:SPX"))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 2 || h.queries[0].Contract.ConID != 7001 || h.queries[1].Contract.ConID != 7001 || b.resolved != 1 {
		t.Fatalf("queries %+v resolved %d", h.queries, b.resolved)
	}
}
