package option_tracking_page

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optiontrack"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

type fakeMarks struct {
	rows []mark.PositionMark
	err  error
}

func (f fakeMarks) Marks(context.Context, string, string) ([]mark.PositionMark, error) {
	return f.rows, f.err
}

func marksRig(contract, status string, rows ...mark.PositionMark) Config {
	r := newRig()
	r.replay.out = compute.LedgerResult{Positions: []compute.LedgerPosition{{PositionID: "p1", Contract: contract, Underlying: "AAPL",
		Status: status, Right: "P", Expiry: "2026-12-18", Strike: "150", Quantity: "-1"}}}
	c := r.config()
	c.Marks = fakeMarks{rows: rows}
	return c
}

func TestMarkWithBidAskBecomesQuoteAndRowIsOk(t *testing.T) {
	t.Parallel()
	c := marksRig("123", "open", mark.PositionMark{ConID: 123, SessionDay: "2026-11-10", Price: "1.5", Basis: mark.BasisMid, Bid: "1.4", Ask: "1.6"})
	in, _, err := gather(context.Background(), c, "main", buildDay)
	if err != nil {
		t.Fatal(err)
	}
	want := []trackapi.TrackQuote{{Underlying: "AAPL", Expiry: "2026-12-18", Strike: "150", Right: "P", Day: "2026-11-10", Bid: "1.4", Ask: "1.6"}}
	if !reflect.DeepEqual(in.Quotes, want) {
		t.Fatalf("quotes = %+v", in.Quotes)
	}
	res := optiontrack.New().Track(in, trackapi.TrackParams{MarkMaxAgeSessions: 3, RecentDays: 30})
	if len(res.Open) != 1 || res.Open[0].MarkStatus != "ok" || res.Open[0].MarkDay != "2026-11-10" {
		t.Fatalf("open = %+v", res.Open)
	}
}

func TestMarkWithPriceOnlyGivesBidAskEqualPrice(t *testing.T) {
	t.Parallel()
	c := marksRig("123", "open", mark.PositionMark{ConID: 123, SessionDay: "2026-11-10", Price: "2.25", Basis: mark.BasisLast})
	in, _, err := gather(context.Background(), c, "main", buildDay)
	if err != nil || len(in.Quotes) != 1 || in.Quotes[0].Bid != "2.25" || in.Quotes[0].Ask != "2.25" {
		t.Fatalf("err = %v quotes = %+v", err, in.Quotes)
	}
}

func TestMarkBasisNoneGivesNoQuoteAndNoMark(t *testing.T) {
	t.Parallel()
	c := marksRig("123", "open", mark.PositionMark{ConID: 123, SessionDay: "2026-11-10", Basis: mark.BasisNone})
	in, _, err := gather(context.Background(), c, "main", buildDay)
	if err != nil || len(in.Quotes) != 0 {
		t.Fatalf("err = %v quotes = %+v", err, in.Quotes)
	}
	res := optiontrack.New().Track(in, trackapi.TrackParams{MarkMaxAgeSessions: 3, RecentDays: 30})
	if len(res.Open) != 1 || res.Open[0].MarkStatus != "no_mark" {
		t.Fatalf("open = %+v", res.Open)
	}
}

func TestMarkNotMatchedToCompositeOrClosedPosition(t *testing.T) {
	t.Parallel()
	row := mark.PositionMark{ConID: 123, SessionDay: "2026-11-10", Price: "1", Basis: mark.BasisLast}
	for name, c := range map[string]Config{
		"composite": marksRig("AAPL:2026-12-18:150:P", "open", row),
		"closed":    marksRig("123", "closed", row),
	} {
		in, _, err := gather(context.Background(), c, "main", buildDay)
		if err != nil || len(in.Quotes) != 0 {
			t.Fatalf("%s: err = %v quotes = %+v", name, err, in.Quotes)
		}
	}
}

func TestNilMarksKeepsInputAndMarksReadWithoutEodReaders(t *testing.T) {
	t.Parallel()
	c := marksRig("123", "open", mark.PositionMark{ConID: 123, SessionDay: "2026-11-10", Price: "1", Basis: mark.BasisLast})
	c.Marks = nil
	in, _, err := gather(context.Background(), c, "main", buildDay)
	if err != nil || len(in.Quotes) != 0 {
		t.Fatalf("err = %v quotes = %+v", err, in.Quotes)
	}
	c = marksRig("123", "open", mark.PositionMark{ConID: 123, SessionDay: "2026-11-10", Price: "1", Basis: mark.BasisLast})
	c.Eod = nil
	in, gs, err := gather(context.Background(), c, "main", buildDay)
	if err != nil || len(in.Quotes) != 1 || len(gs) != 1 || gs[0] != gapNoEodReaders {
		t.Fatalf("err = %v quotes = %+v gaps = %v", err, in.Quotes, gs)
	}
}

func TestMarksReaderErrorIsReturned(t *testing.T) {
	t.Parallel()
	c := marksRig("123", "open")
	c.Marks = fakeMarks{err: errors.New("boom")}
	if _, _, err := gather(context.Background(), c, "main", buildDay); err == nil {
		t.Fatal("want error")
	}
}
