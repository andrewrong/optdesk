package barsrefresh

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
)

// storedAdjusted is an adjusted_last winner row received on the given day (YYYY-MM-DD).
func storedAdjusted(id int, day, closePx string, epoch int, received string) contracts.PriceBar {
	b := stored(id, day, closePx, epoch)
	b.Basis = basisAdjusted
	b.ReceivedAt, _ = time.Parse(dayLayout, received)
	return b
}

// adjustedHistory answers trades with the one stored close and adjusted_last with adj.
func adjustedHistory(adj func() ([]broker.Bar, error)) *fakeHistory {
	return &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.Kind == broker.BarAdjustedLast {
			return adj()
		}
		return []broker.Bar{mkbar("2026-11-09", "10.0000")}, nil
	}}
}

// noRoute is a stock in a currency with no yfinance route: it keeps the weekly IB adjusted_last step.
func noRoute() jobs.WatchEntry {
	e := entry("cik:1", "AAA", 101)
	e.Contract.Currency = "EUR"
	return e
}

// runAdjusted runs one key with no yfinance route and the weekly step on (the shared build turns it off).
// A yfinance source is attached and must not be asked.
func runAdjusted(t *testing.T, h *fakeHistory, st *memBars) contracts.JobResult {
	t.Helper()
	j, _ := buildFB(t, closed, fakeCal{}, h, st, &fakeBarSource{fn: func(sources.BarRequest) (sources.BarResult, error) {
		t.Error("yfinance asked for a key with no route")
		return sources.BarResult{}, nil
	}}, noRoute())
	j.m.AdjustedCadenceDays = 7
	return j.Run(context.Background(), runDay, keysOf("cik:1"))
}

func adjustedQueries(h *fakeHistory) []broker.BarQuery {
	var out []broker.BarQuery
	for _, q := range h.queries {
		if q.Kind == broker.BarAdjustedLast {
			out = append(out, q)
		}
	}
	return out
}

func tradesRow() contracts.PriceBar { return stored(101, "2026-11-09", "10.0000", 1) }

func TestAdjustedPulledWhenNone(t *testing.T) {
	t.Parallel()
	h := adjustedHistory(func() ([]broker.Bar, error) {
		return []broker.Bar{mkbar("2026-11-06", "9.0000"), mkbar("2026-11-09", "9.5000")}, nil
	})
	st := &memBars{rows: []contracts.PriceBar{tradesRow()}}
	if res := runAdjusted(t, h, st); res.Err != nil {
		t.Fatal(res.Err)
	}
	qs := adjustedQueries(h)
	if len(qs) != 1 || qs[0].DurationDays != 400 || qs[0].End != "" || qs[0].Contract.ConID != 101 {
		t.Fatalf("queries %+v", qs)
	}
	got, _ := st.PriceBars(context.Background(), 101, "adjusted_last", firstDay, runDay)
	if len(got) != 2 || got[0].AdjustEpoch != 1 || got[0].Source != "ibkr" || got[0].Currency != "EUR" {
		t.Fatalf("rows %+v", got)
	}
}

func TestAdjustedPulledAtSevenDays(t *testing.T) {
	t.Parallel()
	h := adjustedHistory(func() ([]broker.Bar, error) { return []broker.Bar{mkbar("2026-11-09", "9.5000")}, nil })
	st := &memBars{rows: []contracts.PriceBar{tradesRow(), storedAdjusted(101, "2026-11-09", "9.0000", 1, "2026-11-02")}}
	if res := runAdjusted(t, h, st); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(adjustedQueries(h)) != 1 {
		t.Fatalf("queries %+v", h.queries)
	}
}

func TestAdjustedSkippedWhenFresh(t *testing.T) {
	t.Parallel()
	h := adjustedHistory(func() ([]broker.Bar, error) { return []broker.Bar{mkbar("2026-11-09", "9.5000")}, nil })
	st := &memBars{rows: []contracts.PriceBar{tradesRow(), storedAdjusted(101, "2026-11-09", "9.0000", 1, "2026-11-03")}}
	if res := runAdjusted(t, h, st); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(adjustedQueries(h)) != 0 || len(st.rows) != 2 {
		t.Fatalf("queries %+v rows %+v", h.queries, st.rows)
	}
}

func TestAdjustedOverwritesWholeWindowByKey(t *testing.T) {
	t.Parallel()
	old := func() []contracts.PriceBar {
		return []contracts.PriceBar{
			tradesRow(),
			storedAdjusted(101, "2026-11-05", "9.0000", 1, "2026-11-01"),
			storedAdjusted(101, "2026-11-06", "9.1000", 1, "2026-11-01"),
		}
	}
	cases := map[string][]broker.Bar{
		"every value restated by a dividend factor": {mkbar("2026-11-05", "8.8200"), mkbar("2026-11-06", "8.9180"), mkbar("2026-11-09", "9.5000")},
		"nothing differs": {mkbar("2026-11-05", "9.0000"), mkbar("2026-11-06", "9.1000")},
	}
	for name, answer := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := adjustedHistory(func() ([]broker.Bar, error) { return answer, nil })
			st := &memBars{rows: old()}
			if res := runAdjusted(t, h, st); res.Err != nil {
				t.Fatal(res.Err)
			}
			adj, _ := st.PriceBars(context.Background(), 101, basisAdjusted, firstDay, runDay)
			if len(adj) != len(answer) || len(adjustedQueries(h)) != 1 {
				t.Fatalf("adjusted rows %+v queries %+v", adj, h.queries)
			}
			for i, r := range adj {
				if r.SessionDay.Format(dayLayout) != answer[i].Day || r.Close != answer[i].Close {
					t.Fatalf("row %d %+v for bar %+v", i, r, answer[i])
				}
			}
			if len(st.rows) != 1+len(answer) {
				t.Fatalf("a key is stored twice: %+v", st.rows)
			}
		})
	}
}

func TestAdjustedFailureKeepsStoredRows(t *testing.T) {
	t.Parallel()
	h := adjustedHistory(func() ([]broker.Bar, error) { return nil, errors.New("boom") })
	st := &memBars{rows: []contracts.PriceBar{tradesRow(), storedAdjusted(101, "2026-11-09", "9.0000", 1, "2026-11-01")}}
	res := runAdjusted(t, h, st)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("err %v", res.Err)
	}
	if len(st.rows) != 2 || len(st.epoch(2)) != 0 {
		t.Fatalf("rows %+v", st.rows)
	}
}

func TestAdjustedStepNotTakenForRoutedStock(t *testing.T) {
	t.Parallel()
	h := adjustedHistory(func() ([]broker.Bar, error) {
		t.Error("IB adjusted_last asked for a stock with a route")
		return nil, nil
	})
	st := &memBars{rows: []contracts.PriceBar{tradesRow(), storedAdjusted(101, "2026-11-09", "9.0000", 1, "2026-10-01")}}
	src := &fakeBarSource{fn: answerFrom([]sources.RefBar{ybAdj("2026-11-09", "10.0000", "9.5000")})}
	j, _ := buildFB(t, closed, fakeCal{}, h, st, src, entry("cik:1", "AAA", 101))
	j.m.AdjustedCadenceDays = 7
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 0 {
		t.Fatalf("IB queries %+v", h.queries)
	}
}
