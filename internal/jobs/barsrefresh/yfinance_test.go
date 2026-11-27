package barsrefresh

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
)

// ybAdj is a yfinance bar with an adjusted close.
func ybAdj(day, closePx, adj string) sources.RefBar {
	b := yb(day, closePx)
	b.AdjClose = adj
	return b
}

// answerFrom answers every pair with the bars on or after the From day of the request and the given actions.
func answerFrom(bars []sources.RefBar, actions ...sources.RefAction) func(sources.BarRequest) (sources.BarResult, error) {
	return func(req sources.BarRequest) (sources.BarResult, error) {
		var out sources.BarResult
		for _, b := range bars {
			if b.Day >= req.From && b.Day <= req.To {
				b.Pair = req.Pairs[0]
				out.Bars = append(out.Bars, b)
			}
		}
		for _, a := range actions {
			a.Pair = req.Pairs[0]
			out.Actions = append(out.Actions, a)
		}
		return out, nil
	}
}

func historyThatFails(t *testing.T) *fakeHistory {
	return &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		t.Errorf("IB history asked: %+v", q)
		return nil, errors.New("not asked")
	}}
}

func rowsOf(st *memBars, id int64, basis string) []contracts.PriceBar {
	got, _ := st.PriceBars(context.Background(), id, basis, firstDay, runDay)
	return got
}

func TestNoIbHistoryForRoutedStock(t *testing.T) {
	t.Parallel()
	bars := []sources.RefBar{ybAdj("2026-11-06", "10.0000", "9.0000"), ybAdj("2026-11-09", "11.0000", "10.0000")}
	jp := jobs.WatchEntry{Issuer: "cik:2", Market: "JP", Contract: broker.Contract{SecType: "STK", Symbol: "4063", Exchange: "TSEJ", Currency: "JPY", ConID: 102}}
	t.Run("usd and jpy stocks ask IB for no bar, whatever is stored", func(t *testing.T) {
		t.Parallel()
		h := historyThatFails(t)
		// stale adjusted rows and a shallow history: every IB step (window, backfill, deepen, adjusted) would be due
		st := &memBars{rows: []contracts.PriceBar{
			stored(101, "2026-11-05", "10.0000", 1), storedAdjusted(101, "2026-11-05", "9.0000", 1, "2026-10-01"),
		}}
		src := &fakeBarSource{fn: answerFrom(bars)}
		j, _ := buildFB(t, closed, fakeCal{}, h, st, src, entry("cik:1", "AAA", 101), jp)
		j.m.AdjustedCadenceDays = 7
		if res := j.Run(context.Background(), runDay, keysOf("cik:1", "cik:2")); res.Err != nil {
			t.Fatal(res.Err)
		}
		if len(h.queries) != 0 {
			t.Fatalf("IB queries %+v", h.queries)
		}
		if got := rowsOf(st, 101, basisTrades); len(got) != 3 {
			t.Fatalf("trades rows %+v", got)
		}
		if len(src.calls) != 2 || src.calls[0].Pairs[0] != (sources.BarPair{Symbol: "AAA", Currency: "USD"}) ||
			src.calls[1].Pairs[0] != (sources.BarPair{Symbol: "4063", Currency: "JPY"}) {
			t.Fatalf("source calls %+v", src.calls)
		}
	})
	t.Run("an index key keeps the IB path", func(t *testing.T) {
		t.Parallel()
		h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "5000.0000"))}
		st := &memBars{}
		src := &fakeBarSource{fn: answerFrom(bars)}
		j, _ := buildFB(t, closed, fakeCal{}, h, st, src)
		j.m.AdjustedCadenceDays = 7
		if res := j.Run(context.Background(), runDay, keysOf("index:SPX")); res.Err != nil {
			t.Fatal(res.Err)
		}
		var kinds []broker.BarKind
		for _, q := range h.queries {
			kinds = append(kinds, q.Kind)
		}
		if len(src.calls) != 0 || len(kinds) < 2 || kinds[0] != broker.BarTrades || kinds[len(kinds)-1] != broker.BarAdjustedLast {
			t.Fatalf("source calls %+v, IB kinds %v", src.calls, kinds)
		}
	})
	t.Run("a stock in another currency keeps the IB path", func(t *testing.T) {
		t.Parallel()
		h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "50.0000"))}
		src := &fakeBarSource{fn: answerFrom(bars)}
		eur := jobs.WatchEntry{Issuer: "cik:3", Market: "EU", Contract: broker.Contract{SecType: "STK", Symbol: "SAP", Exchange: "IBIS", Currency: "EUR", ConID: 103}}
		j, _ := buildFB(t, closed, fakeCal{}, h, &memBars{}, src, eur)
		if res := j.Run(context.Background(), runDay, keysOf("cik:3")); res.Err != nil {
			t.Fatal(res.Err)
		}
		if len(src.calls) != 0 || countQueries(h, 103) == 0 {
			t.Fatalf("source calls %+v, IB queries %+v", src.calls, h.queries)
		}
	})
}

// yfStored is a name with a yfinance pull up to 2026-11-06.
func yfStored() []contracts.PriceBar {
	adj := yfRow(101, "2026-11-06", "9.0000")
	adj.Basis = basisAdjusted
	return []contracts.PriceBar{yfRow(101, "2026-11-06", "10.0000"), adj}
}

func TestNoEventNoHistoryRequest(t *testing.T) {
	t.Parallel()
	h := historyThatFails(t)
	st := &memBars{rows: yfStored()}
	src := &fakeBarSource{fn: answerFrom([]sources.RefBar{
		ybAdj("2026-11-05", "9.9000", "8.9000"), ybAdj("2026-11-06", "10.0000", "9.0000"), ybAdj("2026-11-09", "11.0000", "10.0000"),
	})}
	j, _ := buildFB(t, closed, fakeCal{}, h, st, src, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(src.calls) != 1 {
		t.Fatalf("source calls %+v", src.calls)
	}
	if c := src.calls[0]; c.From != "2026-11-06" || c.To != runDay || !c.Actions || !c.Adjusted {
		t.Fatalf("call %+v", c)
	}
	if len(st.replaced) != 0 || len(st.calls) != 1 {
		t.Fatalf("replaced %d upserts %d", len(st.replaced), len(st.calls))
	}
	tr, adj := rowsOf(st, 101, basisTrades), rowsOf(st, 101, basisAdjusted)
	if len(tr) != 2 || len(adj) != 2 || tr[1].Close != "11.0000" || adj[1].Close != "10.0000" || adj[1].Open != "10.0000" || adj[1].Source != "yfinance" {
		t.Fatalf("trades %+v adjusted %+v", tr, adj)
	}
	if st.by[0] != "bars_refresh:2026-11-09" {
		t.Fatalf("writtenBy %v", st.by)
	}
}

func TestEventFullPullReplaces(t *testing.T) {
	t.Parallel()
	all := []sources.RefBar{
		ybAdj("2026-11-04", "9.8000", "8.0000"), ybAdj("2026-11-05", "9.9000", "8.1000"),
		ybAdj("2026-11-06", "10.0000", "8.2000"), ybAdj("2026-11-09", "11.0000", "9.0000"),
	}
	event := sources.RefAction{Day: "2026-11-09", Kind: "dividend", Value: "0.5"}
	for name, rows := range map[string][]contracts.PriceBar{
		"a dividend after the last pull": append([]contracts.PriceBar{stored(101, "2026-11-04", "9.8000", 1), stored(101, "2026-11-05", "9.9000", 1)}, yfStored()...),
		"no yfinance row":                {stored(101, "2026-11-04", "9.8000", 1), stored(101, "2026-11-05", "9.9000", 1)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := historyThatFails(t)
			st := &memBars{rows: rows}
			src := &fakeBarSource{fn: answerFrom(all, event)}
			j, _ := buildFB(t, closed, fakeCal{}, h, st, src, entry("cik:1", "AAA", 101))
			if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
				t.Fatal(res.Err)
			}
			if len(st.replaced) != 1 || len(st.calls) != 0 || len(st.replaced[0]) != 8 {
				t.Fatalf("replaced %+v upserts %d", st.replaced, len(st.calls))
			}
			if last := src.calls[len(src.calls)-1]; last.From != firstDay || !last.Adjusted {
				t.Fatalf("calls %+v", src.calls)
			}
			tr, adj := rowsOf(st, 101, basisTrades), rowsOf(st, 101, basisAdjusted)
			if len(tr) != 4 || len(adj) != 4 {
				t.Fatalf("trades %+v adjusted %+v", tr, adj)
			}
			for i, r := range tr {
				if r.Source != "yfinance" || r.Close != all[i].Close || r.Volume == nil || *r.Volume != "1000" {
					t.Fatalf("trades row %+v", r)
				}
				a := adj[i]
				if a.Source != "yfinance" || a.Close != all[i].AdjClose || a.Open != a.Close || a.High != a.Close || a.Low != a.Close {
					t.Fatalf("adjusted row %+v", a)
				}
			}
		})
	}
}

func TestYfinanceFailureKeepsRows(t *testing.T) {
	t.Parallel()
	for _, code := range []string{sources.BarCodeRateLimited, sources.BarCodeNoData, sources.BarCodeTimeout, sources.BarCodeNetwork} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			h := historyThatFails(t)
			st := &memBars{rows: yfStored()}
			before := append([]contracts.PriceBar(nil), st.rows...)
			var failing atomic.Bool
			failing.Store(true)
			ok := answerFrom([]sources.RefBar{ybAdj("2026-11-06", "10.0000", "9.0000"), ybAdj("2026-11-09", "11.0000", "10.0000")})
			src := &fakeBarSource{fn: func(req sources.BarRequest) (sources.BarResult, error) {
				if failing.Load() {
					return sources.BarResult{Unanswered: []sources.BarUnanswered{{Pair: req.Pairs[0], Code: code}}}, nil
				}
				return ok(req)
			}}
			j, buf := buildFB(t, closed, fakeCal{}, h, st, src, entry("cik:1", "AAA", 101), entry("cik:2", "BBB", 102))
			res := j.Run(context.Background(), runDay, keysOf("cik:1", "cik:2"))
			if res.Err == nil || res.Err.Kind() != contracts.Retryable {
				t.Fatalf("err %v", res.Err)
			}
			if !reflect.DeepEqual(before, st.rows) || len(st.calls) != 0 || len(st.replaced) != 0 {
				t.Fatalf("rows changed: %+v", st.rows)
			}
			if !strings.Contains(buf.String(), "cik:1") || !strings.Contains(buf.String(), code) {
				t.Fatalf("log %s", buf.String())
			}
			if code == sources.BarCodeRateLimited && len(src.calls) != 1 {
				t.Fatalf("a rate limited run asked %d times", len(src.calls))
			}
			failing.Store(false) // the next run asks the key again
			if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil || len(rowsOf(st, 101, basisTrades)) != 2 {
				t.Fatalf("next run: %v rows %+v", res.Err, st.rows)
			}
		})
	}
	t.Run("an error of the source", func(t *testing.T) {
		t.Parallel()
		st := &memBars{rows: yfStored()}
		src := &fakeBarSource{fn: func(sources.BarRequest) (sources.BarResult, error) { return sources.BarResult{}, errors.New("boom") }}
		j, _ := buildFB(t, closed, fakeCal{}, historyThatFails(t), st, src, entry("cik:1", "AAA", 101))
		res := j.Run(context.Background(), runDay, keysOf("cik:1"))
		if res.Err == nil || res.Err.Kind() != contracts.Retryable || len(st.rows) != 2 {
			t.Fatalf("err %v rows %+v", res.Err, st.rows)
		}
	})
}

func TestYfinanceAdjustedRowScalesOpenHighLow(t *testing.T) {
	t.Parallel()
	full := sources.RefBar{Day: "2026-11-06", Open: "10.4", High: "10.8", Low: "9.6", Close: "10", AdjClose: "9.5", Volume: "1000"}
	noOpen := sources.RefBar{Day: "2026-11-09", High: "11", Low: "10", Close: "10.5", AdjClose: "10"}
	noAdj := yb("2026-11-05", "9.9000")
	st := &memBars{rows: yfStored()}
	src := &fakeBarSource{fn: answerFrom([]sources.RefBar{noAdj, full, noOpen})}
	j, buf := buildFB(t, closed, fakeCal{}, historyThatFails(t), st, src, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	adj := rowsOf(st, 101, basisAdjusted)
	if len(adj) != 1 {
		t.Fatalf("adjusted rows %+v", adj)
	}
	if a := adj[0]; a.Open != "9.8800" || a.High != "10.2600" || a.Low != "9.1200" || a.Close != "9.5" {
		t.Fatalf("adjusted row %+v", a)
	}
	for _, r := range st.rows {
		if r.Basis == basisAdjusted && (r.Open == "" || r.High == "" || r.Low == "") {
			t.Fatalf("empty price in %+v", r)
		}
	}
	if tr := rowsOf(st, 101, basisTrades); len(tr) != 1 || tr[0].Open != "10.4" || tr[0].Close != "10" {
		t.Fatalf("trades %+v", tr)
	}
	if !strings.Contains(buf.String(), "left as a gap") {
		t.Fatalf("log %s", buf.String())
	}
}
