package option_boards

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeCands struct {
	byDay map[contracts.Day][]store.Candidate
	err   error
	mu    sync.Mutex
	asked []contracts.Day
}

func (f *fakeCands) Candidates(_ context.Context, day contracts.Day, kind string) ([]store.Candidate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, day)
	if kind != "short_put" {
		return nil, errors.New("wrong kind " + kind)
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.byDay[day], nil
}

func TestLookbackFindsOlderDay(t *testing.T) {
	t.Parallel()
	f := &fakeCands{byDay: map[contracts.Day][]store.Candidate{
		"2026-11-04": {{InstrumentID: 1}},
	}}
	got, err := loadShortPuts(context.Background(), f, "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || got.Day != "2026-11-04" || len(got.Rows) != 1 {
		t.Fatalf("got %+v", got)
	}
	if len(f.asked) != 7 || f.asked[0] != "2026-11-10" || f.asked[6] != "2026-11-04" {
		t.Errorf("asked %v", f.asked)
	}
}

func TestLookbackStaleFlag(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	f := &fakeCands{byDay: map[contracts.Day][]store.Candidate{
		"2026-11-10": {{InstrumentID: 1, ComputedAt: at}},
		"2026-11-09": {{InstrumentID: 2}},
	}}
	fresh, err := loadShortPuts(context.Background(), f, "2026-11-10")
	if err != nil || fresh.Stale || fresh.Day != "2026-11-10" || !fresh.ComputedAt.Equal(at) {
		t.Fatalf("fresh = %+v, %v", fresh, err)
	}
	f2 := &fakeCands{byDay: map[contracts.Day][]store.Candidate{"2026-11-09": {{InstrumentID: 2}}}}
	stale, err := loadShortPuts(context.Background(), f2, "2026-11-10")
	if err != nil || !stale.Stale || stale.Day != "2026-11-09" {
		t.Fatalf("stale = %+v, %v", stale, err)
	}
}

func TestNoRowsInSevenDays(t *testing.T) {
	t.Parallel()
	f := &fakeCands{byDay: map[contracts.Day][]store.Candidate{"2026-11-02": {{InstrumentID: 1}}}}
	got, err := loadShortPuts(context.Background(), f, "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if got.Found || got.Day != "" || got.Stale || got.Rows == nil || len(got.Rows) != 0 {
		t.Errorf("got %+v", got)
	}
	if len(f.asked) != 8 {
		t.Errorf("asked %d days, want 8 (the build day and 7 back)", len(f.asked))
	}
	bad := &fakeCands{err: errors.New("down")}
	if _, err := loadShortPuts(context.Background(), bad, "2026-11-10"); err == nil {
		t.Error("reader error must fail")
	}
}

type countingEod struct {
	mu    sync.Mutex
	calls []int64
	rows  map[int64][]contracts.OptionEod
}

func (f *countingEod) OptionEod(_ context.Context, id int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, id)
	if from != to {
		return nil, errors.New("from must equal to")
	}
	return f.rows[id], nil
}

func fp(v float64) *float64 { return &v }

func TestEodOneCallPerUnderlying(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{
		{InstrumentID: 11, UnderlyingID: 1}, {InstrumentID: 12, UnderlyingID: 1}, {InstrumentID: 21, UnderlyingID: 2},
	}
	f := &countingEod{rows: map[int64][]contracts.OptionEod{
		1: {
			{InstrumentID: 11, CaptureID: "a", Volume: fp(1)},
			{InstrumentID: 11, CaptureID: "b", Volume: fp(2)},
			{InstrumentID: 12, CaptureID: "a", Volume: fp(3)},
		},
		2: {{InstrumentID: 21, CaptureID: "a", Volume: fp(4)}},
	}}
	m, err := loadEod(context.Background(), f, rows, "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 {
		t.Errorf("calls = %v, want 2", f.calls)
	}
	if got := eodTextFor(m, 11).Volume; got != "2" {
		t.Errorf("instrument 11 volume = %q, want the greatest capture's 2", got)
	}
	if got := eodTextFor(m, 12).Volume; got != "3" {
		t.Errorf("instrument 12 volume = %q", got)
	}
	if got := eodTextFor(m, 21).Volume; got != "4" {
		t.Errorf("instrument 21 volume = %q", got)
	}
	if got := eodTextFor(m, 99).Volume; got != "缺" {
		t.Errorf("unknown instrument volume = %q", got)
	}
}

func TestEodNilReaderAndError(t *testing.T) {
	t.Parallel()
	m, err := loadEod(context.Background(), nil, []store.Candidate{{InstrumentID: 1}}, "2026-11-10")
	if err != nil || len(m) != 0 {
		t.Errorf("nil reader: %v, %v", m, err)
	}
}

func TestEodTextGap(t *testing.T) {
	t.Parallel()
	e := &contracts.OptionEod{Volume: fp(1240), OpenInterest: fp(5310), ThetaPerCalendarDay: fp(-0.0312), Gamma: nil, VegaPerVolPoint: fp(0.1)}
	got := eodTextOf(e)
	want := eodText{Volume: "1240", OpenInterest: "5310", Theta: "-0.0312", Gamma: "缺", Vega: "0.1"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := eodTextOf(nil); got != (eodText{"缺", "缺", "缺", "缺", "缺"}) {
		t.Errorf("nil row: %+v", got)
	}
}
