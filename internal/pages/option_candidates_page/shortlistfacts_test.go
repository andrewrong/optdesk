package option_candidates_page

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRowsCarryStoredFields(t *testing.T) {
	t.Parallel()
	rows := shortRows()
	f := &fakeShortlister{res: shortResult(rows)}
	buildShort(t, shortConfig(rows, f, true))
	if f.calls() != 1 {
		t.Fatalf("Select called %d times", f.calls())
	}
	got := f.in[0].Rows
	if len(got) != 3 {
		t.Fatalf("rows = %d, want one per stored row of every status", len(got))
	}
	r := got[0]
	if r.Strike != "85.00" || r.Spot != "96.50" || r.Annualized != "0.123456" || r.Delta != "-0.19" || r.Bid != "0.42" || r.DTE != "39" {
		t.Errorf("row 0 = %+v", r)
	}
	if r.Key != keyOf(rows, 0) || r.Ticker != "AAA" || got[2].Ticker != "BBB" || got[2].Spot != "41.20" {
		t.Errorf("keys or tickers: %+v %+v", r, got[2])
	}
}

func TestRowsCarryGateReasons(t *testing.T) {
	t.Parallel()
	rows := shortRows()
	rows[0].Gates = `[{"gate":"quote","status":"pass"},{"gate":"quote","status":"block","reason":"wide_spread"},` +
		`{"gate":"events","status":"unknown","reason":"stale_chain"}]`
	rows[0].Marks = `["no_open_interest"]`
	f := &fakeShortlister{res: shortResult(rows)}
	buildShort(t, shortConfig(rows, f, false))
	r := f.in[0].Rows[0]
	if r.QuoteReason != "wide_spread" || r.EventsReason != "stale_chain" || len(r.Marks) != 1 || r.Marks[0] != "no_open_interest" {
		t.Errorf("row = %+v", r)
	}
}

func TestUnreadableGatesPassedWithMark(t *testing.T) {
	t.Parallel()
	rows := shortRows()
	rows[1].Gates = "not json"
	f := &fakeShortlister{res: shortResult(rows)}
	buildShort(t, shortConfig(rows, f, false))
	got := f.in[0].Rows
	if len(got) != 3 {
		t.Fatalf("the page dropped a row: %d", len(got))
	}
	if len(got[1].Marks) != 1 || got[1].Marks[0] != "unreadable_gates" || got[1].QuoteReason != "" {
		t.Errorf("row = %+v", got[1])
	}
}

func TestShortlistReaderErrorReturned(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	rows := shortRows()
	_, err := Build(context.Background(), shortConfig(rows, &fakeShortlister{err: boom}, false), nil, buildDay, testNow)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "shortlist") {
		t.Errorf("err = %v", err)
	}
}
