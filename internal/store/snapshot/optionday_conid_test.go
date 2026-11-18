package snapshot

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

type fakeConidSource struct {
	ids   map[int64]int64
	err   error
	calls [][]int64
}

func (f *fakeConidSource) InstrumentIDs(_ context.Context, conids []int64) (map[int64]int64, error) {
	f.calls = append(f.calls, conids)
	return f.ids, f.err
}

type fakeInner struct {
	rows      []optionday.Day
	askedIDs  [][]int64
	chainCall int64
	closed    bool
}

func (f *fakeInner) Days(_ context.Context, ids []int64, _, _ contracts.Day) ([]optionday.Day, error) {
	f.askedIDs = append(f.askedIDs, ids)
	return f.rows, nil
}

func (f *fakeInner) ChainDay(_ context.Context, u int64, _ contracts.Day) ([]optionday.Day, error) {
	f.chainCall = u
	return f.rows, nil
}

func (f *fakeInner) Close() error { f.closed = true; return nil }

func TestConidDaysAsksInnerWithInstrumentIDs(t *testing.T) {
	t.Parallel()
	src := &fakeConidSource{ids: map[int64]int64{5001: 900}}
	inner := &fakeInner{rows: []optionday.Day{
		{InstrumentID: 900, SessionDay: "2026-11-06"},
		{InstrumentID: 777, SessionDay: "2026-11-06"}, // not in the map: skipped
	}}
	got, err := newConidDays(src, inner).Days(context.Background(), []int64{5001}, "2026-11-01", "2026-11-30")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].InstrumentID != 5001 || got[0].SessionDay != "2026-11-06" {
		t.Fatalf("rows %+v, want one row with InstrumentID 5001", got)
	}
	if want := [][]int64{{900}}; !reflect.DeepEqual(inner.askedIDs, want) {
		t.Fatalf("inner asked %v, want %v", inner.askedIDs, want)
	}
}

func TestConidDaysUnknownConidNoRow(t *testing.T) {
	t.Parallel()
	src := &fakeConidSource{ids: map[int64]int64{}}
	inner := &fakeInner{}
	got, err := newConidDays(src, inner).Days(context.Background(), []int64{4040}, "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 0 {
		t.Fatalf("rows %+v, err %v, want none", got, err)
	}
}

func TestConidDaysTwoInstrumentsOneConidFails(t *testing.T) {
	t.Parallel()
	db := &fakeDB{rows: [][]any{{int64(900), int64(5001)}, {int64(901), int64(5001)}, {int64(902), int64(5002)}}}
	inner := &fakeInner{}
	_, err := newConidDays(&conidDB{db: db}, inner).Days(context.Background(), []int64{5002, 5001}, "2026-11-01", "2026-11-30")
	assertKind(t, err, contracts.Permanent, nil)
	if err == nil || !strings.Contains(err.Error(), "5001") {
		t.Fatalf("error %v does not name conid 5001", err)
	}
	if len(inner.askedIDs) != 0 {
		t.Fatal("inner reader was called")
	}
}

func TestConidDaysEmptyConidsNoCall(t *testing.T) {
	t.Parallel()
	src := &fakeConidSource{}
	inner := &fakeInner{}
	got, err := newConidDays(src, inner).Days(context.Background(), nil, "2026-11-01", "2026-11-30")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("rows %#v, err %v, want an empty non-nil slice", got, err)
	}
	if len(src.calls) != 0 || len(inner.askedIDs) != 0 {
		t.Fatal("source or inner reader was called")
	}
}

func TestConidDaysChainDayPassesThrough(t *testing.T) {
	t.Parallel()
	inner := &fakeInner{rows: []optionday.Day{{InstrumentID: 900}}}
	r := newConidDays(&fakeConidSource{}, inner)
	got, err := r.ChainDay(context.Background(), 8, "2026-11-06")
	if err != nil || len(got) != 1 || got[0].InstrumentID != 900 || inner.chainCall != 8 {
		t.Fatalf("chain %+v, err %v, underlying %d", got, err, inner.chainCall)
	}
	if err := r.Close(); err != nil || !inner.closed {
		t.Fatalf("close: err %v, closed %v", err, inner.closed)
	}
}

func TestConidDaysSourceErrorPassesUp(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := newConidDays(&fakeConidSource{err: boom}, &fakeInner{}).Days(context.Background(), []int64{1}, "2026-11-01", "2026-11-30")
	if !errors.Is(err, boom) {
		t.Fatalf("error %v, want boom", err)
	}
}
