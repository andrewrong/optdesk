//go:build pg

package optionopenpg

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestOptionOpenSlotReadsOnlyItsSlotAndIds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := openWriter(t, newDatabase(t))
	t0 := time.Date(2026, 11, 11, 14, 45, 0, 0, time.UTC)
	other := openRow("0945", 2, t0, 9)
	other.SessionDay = day("2026-11-10")
	rows := []contracts.OptionOpen{
		openRow("0945", 3, t0, 3.3), openRow("0945", 2, t0, 2.2), openRow("0945", 4, t0, 4.4),
		openRow("1000", 2, t0.Add(time.Minute), 5.5), openRow("1000", 3, t0.Add(time.Minute), 6.6), other,
	}
	if err := w.InsertOptionOpen(ctx, rows); err != nil {
		t.Fatal(err)
	}
	r, ok := w.(interface {
		OptionOpenSlot(context.Context, contracts.Day, string, []int64) ([]contracts.OptionOpen, error)
	})
	if !ok {
		t.Fatal("the store has no OptionOpenSlot")
	}
	got, err := r.OptionOpenSlot(ctx, "2026-11-11", "0945", []int64{3, 2, 9})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d rows, %v", len(got), err)
	}
	if got[0].InstrumentID != 2 || *got[0].Bid != 2.2 || got[1].InstrumentID != 3 || *got[1].Bid != 3.3 {
		t.Errorf("rows = %+v", got)
	}
	for _, g := range got {
		if g.Slot != "0945" || g.SessionDay.Format(time.DateOnly) != "2026-11-11" || g.ReceivedAt.Location() != time.UTC {
			t.Errorf("row = %+v", g)
		}
	}
}

func TestOptionOpenSlotSeesRowWrittenByInsert(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := openWriter(t, newDatabase(t))
	t0 := time.Date(2026, 11, 11, 14, 45, 0, 0, time.UTC)
	okRow := func(inst int) contracts.OptionOpen { return openRow("0945", inst, t0, 1.1) }
	notOK := func(inst int) contracts.OptionOpen {
		r := openRow("0945", inst, t0.Add(time.Minute), 0)
		r.Status, r.Bid, r.Delta, r.IvDecimal = 3, nil, nil, nil
		return r
	}
	// Instrument 2: ok then not ok. Instrument 3: not ok then ok.
	for _, batch := range [][]contracts.OptionOpen{{okRow(2)}, {notOK(2)}, {notOK(3)}, {okRow(3)}} {
		if err := w.InsertOptionOpen(ctx, batch); err != nil {
			t.Fatal(err)
		}
	}
	r := w.(interface {
		OptionOpenSlot(context.Context, contracts.Day, string, []int64) ([]contracts.OptionOpen, error)
	})
	got, err := r.OptionOpenSlot(ctx, "2026-11-11", "0945", []int64{2, 3})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d rows, %v", len(got), err)
	}
	for _, g := range got {
		if g.Status != 1 || g.Bid == nil || *g.Bid != 1.1 {
			t.Errorf("instrument %d: %+v", g.InstrumentID, g)
		}
	}
}
