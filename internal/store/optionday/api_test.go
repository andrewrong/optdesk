package optionday_test

import (
	"context"
	"sort"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

// fakeReader keeps the promises of DayReader: session_day then instrument_id
// order, an empty result is an empty slice and no error.
type fakeReader struct{ rows []optionday.Day }

var (
	_ optionday.DayReader         = (*fakeReader)(nil)
	_ optionday.OpenDayReaderFunc = func(context.Context, string) (optionday.DayReader, error) { return &fakeReader{}, nil }
)

func (f *fakeReader) pick(keep func(optionday.Day) bool) []optionday.Day {
	out := []optionday.Day{}
	for _, r := range f.rows {
		if keep(r) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SessionDay != out[j].SessionDay {
			return out[i].SessionDay < out[j].SessionDay
		}
		return out[i].InstrumentID < out[j].InstrumentID
	})
	return out
}

func (f *fakeReader) Days(_ context.Context, ids []int64, from, to contracts.Day) ([]optionday.Day, error) {
	want := map[int64]bool{}
	for _, id := range ids {
		want[id] = true
	}
	return f.pick(func(r optionday.Day) bool {
		return want[r.InstrumentID] && r.SessionDay >= from && r.SessionDay <= to
	}), nil
}

func (f *fakeReader) ChainDay(_ context.Context, underlyingID int64, day contracts.Day) ([]optionday.Day, error) {
	return f.pick(func(r optionday.Day) bool { return r.UnderlyingID == underlyingID && r.SessionDay == day }), nil
}

func (f *fakeReader) Close() error { return nil }

func row(inst, und int64, day contracts.Day) optionday.Day {
	return optionday.Day{InstrumentID: inst, UnderlyingID: und, SessionDay: day, Source: "option_eod", Status: 1}
}

func TestDayReaderContract(t *testing.T) {
	t.Parallel()
	r := &fakeReader{rows: []optionday.Day{
		row(2, 9, "2026-11-13"), row(1, 9, "2026-11-13"), row(2, 9, "2026-11-12"), row(1, 9, "2026-11-12"), row(3, 9, "2026-11-12"),
	}}
	got, err := r.Days(context.Background(), []int64{1, 2}, "2026-11-12", "2026-11-13")
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]interface{}{{contracts.Day("2026-11-12"), int64(1)}, {contracts.Day("2026-11-12"), int64(2)},
		{contracts.Day("2026-11-13"), int64(1)}, {contracts.Day("2026-11-13"), int64(2)}}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].SessionDay != w[0] || got[i].InstrumentID != w[1] {
			t.Errorf("row %d = %s/%d, want %v", i, got[i].SessionDay, got[i].InstrumentID, w)
		}
	}
	empty, err := r.Days(context.Background(), []int64{1, 2}, "2026-01-01", "2026-01-02")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty range = %v, %v; want empty non-nil slice and no error", empty, err)
	}
}

func TestChainDayContract(t *testing.T) {
	t.Parallel()
	r := &fakeReader{rows: []optionday.Day{
		row(3, 9, "2026-11-13"), row(1, 9, "2026-11-13"), row(2, 9, "2026-11-12"), row(4, 8, "2026-11-13"),
	}}
	got, err := r.ChainDay(context.Background(), 9, "2026-11-13")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].InstrumentID != 1 || got[1].InstrumentID != 3 {
		t.Fatalf("chain = %+v, want contracts 1 and 3 of underlying 9", got)
	}
}
