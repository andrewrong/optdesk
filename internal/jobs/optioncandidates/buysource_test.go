package optioncandidates

import (
	"context"
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/jobs"
)

func marked(e jobs.WatchEntry, marks ...string) []string {
	c := compute.Candidate{Marks: marks}
	addBuySourceMark(&c, e)
	return c.Marks
}

func entryWith(price, source string) jobs.WatchEntry {
	e := usEntry()
	e.BuyPrice, e.BuySource = price, source
	return e
}

func TestBuySourceMarkValuation(t *testing.T) {
	t.Parallel()
	got := marked(entryWith("91.2", buypriceapi.SourceValuation))
	if !slices.Equal(got, []string{buypriceapi.MarkValuation}) {
		t.Fatalf("marks %v", got)
	}
}

func TestBuySourceMarkHand(t *testing.T) {
	t.Parallel()
	got := marked(entryWith("91.2", buypriceapi.SourceHand))
	if !slices.Equal(got, []string{buypriceapi.MarkHand}) {
		t.Fatalf("marks %v", got)
	}
}

func TestBuySourceEmptySourceIsHand(t *testing.T) {
	t.Parallel()
	got := marked(entryWith("91.2", ""))
	if !slices.Equal(got, []string{buypriceapi.MarkHand}) {
		t.Fatalf("marks %v", got)
	}
}

func TestBuySourceNoMarkWithoutPrice(t *testing.T) {
	t.Parallel()
	for _, src := range []string{"", buypriceapi.SourceNone, buypriceapi.SourceHand, buypriceapi.SourceValuation} {
		if got := marked(entryWith("", src)); len(got) != 0 {
			t.Fatalf("source %q marks %v", src, got)
		}
	}
}

func TestBuySourceKeepsOtherMarks(t *testing.T) {
	t.Parallel()
	got := marked(entryWith("91.2", buypriceapi.SourceValuation), markEodFallback)
	if !slices.Equal(got, []string{markEodFallback, buypriceapi.MarkValuation}) {
		t.Fatalf("marks %v", got)
	}
}

func TestBuySourceOnCoveredCalls(t *testing.T) {
	t.Parallel()
	f := newFakes()
	div := &fakeDividends{}
	st := f.stores()
	e := entryWith("91.5", buypriceapi.SourceValuation)
	j, err := NewWithCalls(st, div, kindsRule{}, []jobs.WatchEntry{e}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), runDay, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	rows := f.written[0]
	if len(rows) != 2 {
		t.Fatalf("%d rows", len(rows))
	}
	for _, r := range rows {
		if r.Marks != `["buy_from_valuation"]` {
			t.Fatalf("kind %s marks %s", r.Kind, r.Marks)
		}
	}
}

func TestBuySourceStoredOnPutRun(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ src, price, marks string }{
		{buypriceapi.SourceValuation, "91.5", `["buy_from_valuation"]`},
		{buypriceapi.SourceHand, "91.5", `["buy_from_hand"]`},
		{"", "91.5", `["buy_from_hand"]`},
		{"", "", `[]`},
	} {
		f := newFakes()
		j := build(t, f.stores(), f, entryWith(c.price, c.src))
		for range 2 {
			if res := j.Run(context.Background(), runDay, nil); res.Err != nil {
				t.Fatal(res.Err)
			}
		}
		a, b := f.written[0][0], f.written[1][0]
		if a.Marks != c.marks || a.RunID != b.RunID {
			t.Fatalf("%+v: marks %s run ids %s %s", c, a.Marks, a.RunID, b.RunID)
		}
		if c.price == "" && a.BuyPrice != nil {
			t.Fatalf("buy price %v", *a.BuyPrice)
		}
	}
}
