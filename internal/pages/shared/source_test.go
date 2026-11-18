package shared

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

type fakeBars struct {
	rows           []contracts.PriceBar
	err            error
	gotID          int64
	gotBasis       string
	gotFrom, gotTo contracts.Day
}

func (f *fakeBars) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	f.gotID, f.gotBasis, f.gotFrom, f.gotTo = id, basis, from, to
	return f.rows, f.err
}

func TestSourceOfCarriesRowsAndBars(t *testing.T) {
	t.Parallel()
	px := 367.71
	ts := time.Date(2026, 11, 18, 20, 0, 0, 0, time.UTC)
	rows := []contracts.OptionEod{
		{SessionDay: types.SerializableDate{Time: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)}, UnderlyingPrice: &px, UnderlyingEventTs: &ts},
		{SessionDay: types.SerializableDate{Time: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)}},
	}
	fb := &fakeBars{rows: []contracts.PriceBar{{SessionDay: types.SerializableDate{Time: time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)}, Close: "370.1"}}}
	s, err := SourceOf(context.Background(), fb, 7, rows, "2026-11-18")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Prints) != 1 || s.Prints[0] != (Print{Day: "2026-11-18", Price: "367.71", At: &ts}) {
		t.Fatalf("prints = %+v", s.Prints)
	}
	if len(s.Bars) != 1 || s.Bars[0] != (Bar{Day: "2026-11-17", Close: "370.1"}) {
		t.Fatalf("bars = %+v", s.Bars)
	}
	if fb.gotID != 7 || fb.gotBasis != "trades" || fb.gotFrom != "2026-11-04" || fb.gotTo != "2026-11-18" {
		t.Fatalf("read %d %s %s %s", fb.gotID, fb.gotBasis, fb.gotFrom, fb.gotTo)
	}
}

func TestSourceOfNilReaderGivesPrintsOnly(t *testing.T) {
	t.Parallel()
	px := 10.0
	s, err := SourceOf(context.Background(), nil, 1, []contracts.OptionEod{{UnderlyingPrice: &px}}, "2026-11-18")
	if err != nil || len(s.Bars) != 0 || len(s.Prints) != 1 {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestSourceOfReaderError(t *testing.T) {
	t.Parallel()
	_, err := SourceOf(context.Background(), &fakeBars{err: errors.New("boom")}, 1, nil, "2026-11-18")
	if err == nil {
		t.Fatal("want error")
	}
}
