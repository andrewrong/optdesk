package main

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// importWriter is a valuation.Writer over memory: it records each call in order and fails the keys it is told to.
type importWriter struct {
	calls     []string
	closed    map[string]bool // review ids that answer ErrClosed
	failKey   string          // a company key whose profile write fails
	reviews   []valuation.Review
	rversions []valuation.RVersion
}

func (w *importWriter) PutProfile(_ context.Context, p valuation.Profile) error {
	if p.CompanyKey == w.failKey {
		return errors.New("db password=secret")
	}
	w.calls = append(w.calls, "profile:"+p.CompanyKey)
	return nil
}

func (w *importWriter) PutReview(_ context.Context, r valuation.Review) error {
	if w.closed[r.ReviewID] {
		return valuation.ErrClosed
	}
	w.calls = append(w.calls, "review:"+r.CompanyKey)
	w.reviews = append(w.reviews, r)
	return nil
}

func (w *importWriter) PutRVersion(_ context.Context, v valuation.RVersion) error {
	w.calls = append(w.calls, "rversion")
	w.rversions = append(w.rversions, v)
	return nil
}

func (*importWriter) PutRTable(context.Context, valuation.RTableVersion) error { return nil }

func (w *importWriter) PutMarginTable(context.Context, valuation.MarginTableVersion) error {
	w.calls = append(w.calls, "margin")
	return nil
}

func insertRow(ticker, key string) valueimportapi.RowPlan {
	return valueimportapi.RowPlan{Key: key, Ticker: ticker, Name: ticker + " Corp", Symbol: ticker, Currency: "USD", RMarket: "US",
		Pool: "in_pool", NewProfile: true, Action: valueimportapi.ActionInsert, ReviewType: "first", ReviewID: "imp-x-" + ticker,
		V: "100", P: "4", VersionDate: "2026-11-01", ChangeMemo: "note: invented", ReportText: "invented report"}
}

func importPlan(rows ...valueimportapi.RowPlan) valueimportapi.Plan {
	return valueimportapi.Plan{
		AsOf: "2026-11-05",
		R:    valueimportapi.RPlan{Action: valueimportapi.ActionInsert, US: "0.9", JP: "0.8", HKCN: "0.7", AISwitch: true, AIExpiry: "later"},
		Margin: valueimportapi.MarginPlan{Action: valueimportapi.ActionInsert, EffectiveDate: "2026-11-05",
			Rows: []valuelinesapi.MarginRow{{PFrom: "0", M: "0.1", W: "0.1", H: "0.1", S: "0.1"}}},
		Rows: rows,
	}
}

func outcomeList(t *testing.T, w *importWriter, p valueimportapi.Plan, ticks map[string]bool) []string {
	t.Helper()
	var got []string
	for _, o := range applyImport(t.Context(), w, p, ticks) {
		got = append(got, o.Key+"="+o.Outcome)
	}
	return got
}

func TestApplyOrderMarginRThenProfileThenReview(t *testing.T) {
	t.Parallel()
	w := &importWriter{}
	got := outcomeList(t, w, importPlan(insertRow("AAA", "US:AAA"), insertRow("BBB", "US:BBB")), nil)
	want := []string{"margin", "rversion", "profile:US:AAA", "review:US:AAA", "profile:US:BBB", "review:US:BBB"}
	if !slices.Equal(w.calls, want) {
		t.Fatalf("calls %v, want %v", w.calls, want)
	}
	if len(got) != 4 {
		t.Fatalf("outcomes %v", got)
	}
	if v := w.rversions[0]; v.YieldUsed != "" || !v.AISwitch || v.AIExpiry != "later" || v.EffectiveDate != "2026-11-05" {
		t.Fatalf("r version %+v", v)
	}
	r := w.reviews[0]
	if r.Type != "first" || r.Status != "effective" || r.Source != "owner" || r.ReviewID != "imp-x-AAA" || r.VersionDate != "2026-11-01" {
		t.Fatalf("review %+v", r)
	}
}

func TestApplySecondConfirmWritesOnlyMissing(t *testing.T) {
	t.Parallel()
	w := &importWriter{}
	p := importPlan(insertRow("AAA", "US:AAA"))
	p.R.Action, p.Margin.Action = valueimportapi.ActionSame, valueimportapi.ActionSame
	same := insertRow("BBB", "US:BBB")
	same.Action = valueimportapi.ActionSame
	older := insertRow("CCC", "US:CCC")
	older.Action = valueimportapi.ActionOlder
	refused := insertRow("DDD", "US:DDD")
	refused.Action = valueimportapi.ActionRefused
	p.Rows = append(p.Rows, same, older, refused)
	got := outcomeList(t, w, p, nil)
	want := []string{"margin_table=same", "r_version=same", "US:AAA=inserted", "US:BBB=same", "US:CCC=older", "US:DDD=refused"}
	if !slices.Equal(got, want) {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	if !slices.Equal(w.calls, []string{"profile:US:AAA", "review:US:AAA"}) {
		t.Fatalf("calls %v", w.calls)
	}
}

func TestApplyClosedReviewIsWritten(t *testing.T) {
	t.Parallel()
	w := &importWriter{closed: map[string]bool{"imp-x-AAA": true}}
	got := outcomeList(t, w, importPlan(insertRow("AAA", "US:AAA"), insertRow("BBB", "US:BBB")), nil)
	if got[2] != "US:AAA=same" || got[3] != "US:BBB=inserted" {
		t.Fatalf("outcomes %v: a closed review is read as already written and the next row goes on", got)
	}
}

func TestApplyUnmatchedNeedsTick(t *testing.T) {
	t.Parallel()
	row := insertRow("ZZZ", "US:ZZZ")
	row.NeedsTick, row.Unmatched = true, true
	w := &importWriter{}
	got := outcomeList(t, w, importPlan(row), nil)
	if got[2] != "US:ZZZ=not_confirmed" || slices.ContainsFunc(w.calls, func(c string) bool { return c == "profile:US:ZZZ" || c == "review:US:ZZZ" }) {
		t.Fatalf("unticked: %v %v", got, w.calls)
	}
	w = &importWriter{}
	got = outcomeList(t, w, importPlan(row), map[string]bool{"ZZZ": true})
	if got[2] != "US:ZZZ=inserted" || !slices.Contains(w.calls, "review:US:ZZZ") {
		t.Fatalf("ticked: %v %v", got, w.calls)
	}
	// ticked, but no key was picked: refused, nothing written for it
	row.Key = ""
	w = &importWriter{}
	got = outcomeList(t, w, importPlan(row), map[string]bool{"ZZZ": true})
	if got[2] != "ZZZ=refused" || len(w.calls) != 2 {
		t.Fatalf("no key: %v %v", got, w.calls)
	}
}

func TestApplyFailureStopsOneRowOnly(t *testing.T) {
	t.Parallel()
	w := &importWriter{failKey: "US:AAA"}
	out := applyImport(t.Context(), w, importPlan(insertRow("AAA", "US:AAA"), insertRow("BBB", "US:BBB")), nil)
	if out[2].Outcome != "failed" || out[3].Outcome != "inserted" {
		t.Fatalf("outcomes %+v", out)
	}
	if out[2].Text != textWriteFailed {
		t.Fatalf("failed text %q holds the error", out[2].Text)
	}
	if slices.Contains(w.calls, "review:US:AAA") {
		t.Fatalf("the review of a row whose profile failed was written: %v", w.calls)
	}
}
