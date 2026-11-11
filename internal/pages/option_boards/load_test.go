package option_boards

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/atombender/go-jsonschema/pkg/types"
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
	mu      sync.Mutex
	calls   []int64
	windows [][2]contracts.Day
	rows    map[int64][]contracts.OptionEod
}

func (f *countingEod) OptionEod(_ context.Context, id int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, id)
	if from > to {
		return nil, errors.New("from is after to")
	}
	f.windows = append(f.windows, [2]contracts.Day{from, to})
	var out []contracts.OptionEod
	for _, e := range f.rows[id] {
		if e.SessionDay.IsZero() {
			out = append(out, e)
			continue
		}
		if d := contracts.Day(e.SessionDay.Format(time.DateOnly)); d >= from && d <= to {
			out = append(out, e)
		}
	}
	return out, nil
}

func sdate(s string) types.SerializableDate {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return types.SerializableDate{Time: t}
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
	if got := eodTextFor(m, 99).Volume; got != "n/a" {
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
	want := eodText{Volume: "1240", OpenInterest: "5310", Theta: "-0.0312", Gamma: "n/a", Vega: "0.1000"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := eodTextOf(nil); got != (eodText{"n/a", "n/a", "n/a", "n/a", "n/a"}) {
		t.Errorf("nil row: %+v", got)
	}
}

func TestEodTextGreeks(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   float64
		want string
	}{
		{"rounds to four decimals", 0.1234567, "0.1235"},
		{"tiny negative keeps its sign", -0.00001, "-0.0000"},
		{"large value", 1500, "1500.0000"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := eodTextOf(&contracts.OptionEod{ThetaPerCalendarDay: fp(c.in), Gamma: fp(c.in), VegaPerVolPoint: fp(c.in)})
			if got.Theta != c.want || got.Gamma != c.want || got.Vega != c.want {
				t.Errorf("got %+v, want %q in all three", got, c.want)
			}
		})
	}
}

func eodOf(day string, capture string, inst int, vol float64) contracts.OptionEod {
	return contracts.OptionEod{InstrumentID: inst, CaptureID: capture, SessionDay: sdate(day), Volume: fp(vol)}
}

func oneUnderlying() []store.Candidate { return []store.Candidate{{InstrumentID: 11, UnderlyingID: 1}} }

func TestEodChainDayBeforeDataDay(t *testing.T) {
	t.Parallel()
	f := &countingEod{rows: map[int64][]contracts.OptionEod{1: {eodOf("2026-11-06", "a", 11, 5)}}}
	m, err := loadEod(context.Background(), f, oneUnderlying(), "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if got := eodTextFor(m, 11).Volume; got != "5" {
		t.Errorf("volume = %q, want the day before's 5", got)
	}
	if got := chainDayNote(m, "2026-11-10"); got != "Volume from 2026-11-06 (chain day)" {
		t.Errorf("note = %q", got)
	}
	if got := chainDayNote(m, "2026-11-06"); got != "" {
		t.Errorf("note on the chain day = %q, want empty", got)
	}
}

func TestEodChainDayNewestWins(t *testing.T) {
	t.Parallel()
	f := &countingEod{rows: map[int64][]contracts.OptionEod{1: {
		eodOf("2026-11-05", "z", 11, 1), eodOf("2026-11-07", "a", 11, 2), eodOf("2026-11-05", "z", 12, 9),
	}}}
	m, err := loadEod(context.Background(), f, oneUnderlying(), "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if got := eodTextFor(m, 11).Volume; got != "2" {
		t.Errorf("volume = %q, want the newer day's 2", got)
	}
	if got := eodTextFor(m, 12).Volume; got != "n/a" {
		t.Errorf("instrument 12 only has an older day, got %q", got)
	}
}

func TestEodChainDayCaptureWins(t *testing.T) {
	t.Parallel()
	f := &countingEod{rows: map[int64][]contracts.OptionEod{1: {
		eodOf("2026-11-07", "b", 11, 2), eodOf("2026-11-07", "a", 11, 1),
	}}}
	m, err := loadEod(context.Background(), f, oneUnderlying(), "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if got := eodTextFor(m, 11).Volume; got != "2" {
		t.Errorf("volume = %q, want the greatest capture's 2", got)
	}
}

func TestEodNoRowInWindow(t *testing.T) {
	t.Parallel()
	f := &countingEod{rows: map[int64][]contracts.OptionEod{1: {eodOf("2026-10-20", "a", 11, 5)}}}
	m, err := loadEod(context.Background(), f, oneUnderlying(), "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if got := eodTextFor(m, 11).Volume; got != "n/a" {
		t.Errorf("volume = %q, want gap", got)
	}
	if got := chainDayNote(m, "2026-11-10"); got != "" {
		t.Errorf("note = %q, want empty", got)
	}
}

func TestEodWindow15Days(t *testing.T) {
	t.Parallel()
	f := &countingEod{rows: map[int64][]contracts.OptionEod{1: {eodOf("2026-10-27", "a", 11, 5)}}}
	rows := []store.Candidate{{InstrumentID: 11, UnderlyingID: 1}, {InstrumentID: 12, UnderlyingID: 1}}
	m, err := loadEod(context.Background(), f, rows, "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || f.windows[0] != [2]contracts.Day{"2026-10-27", "2026-11-10"} {
		t.Errorf("calls %v windows %v", f.calls, f.windows)
	}
	if got := eodTextFor(m, 11).Volume; got != "5" {
		t.Errorf("volume = %q, want the 14th day back read", got)
	}
}

func TestChainDayNoteEarliestAndZeroDay(t *testing.T) {
	t.Parallel()
	m := map[int]contracts.OptionEod{
		1: eodOf("2026-11-08", "a", 1, 1), 2: eodOf("2026-11-06", "a", 2, 1), 3: {InstrumentID: 3},
	}
	if got := chainDayNote(m, "2026-11-10"); got != "Volume from 2026-11-06 (chain day)" {
		t.Errorf("note = %q", got)
	}
	if got := chainDayNote(map[int]contracts.OptionEod{3: {InstrumentID: 3}}, "2026-11-10"); got != "" {
		t.Errorf("zero day counts as the data day, got %q", got)
	}
}
