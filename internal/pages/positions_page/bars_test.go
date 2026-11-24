package positions_page_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/positions_page"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeBars struct {
	byID  map[int64][]contracts.PriceBar
	calls atomic.Int32
	err   error
}

func (f *fakeBars) PriceBars(_ context.Context, id int64, _ string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	var out []contracts.PriceBar
	for _, b := range f.byID[id] {
		if d := contracts.Day(b.SessionDay.Format("2006-01-02")); d >= from && d <= to {
			out = append(out, b)
		}
	}
	return out, nil
}

type fakeStockIDs struct {
	rows  []store.InstrumentName
	calls atomic.Int32
	names []string
	err   error
}

func (f *fakeStockIDs) StockIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	f.calls.Add(1)
	f.names = names
	return f.rows, f.err
}

func bar(id int64, d, closeP, cur string) contracts.PriceBar {
	t, err := types.SerializableDate{}, error(nil)
	if err = t.UnmarshalJSON([]byte(`"` + d + `"`)); err != nil {
		panic(err)
	}
	return contracts.PriceBar{InstrumentID: int(id), SessionDay: t, Close: closeP, Currency: cur, Basis: "trades"}
}

func idOf(id int64, name string) store.InstrumentName {
	return store.InstrumentName{InstrumentID: id, Type: "stock", Name: name}
}

// noMarks is positionsConfig with no mark of any position.
func noMarks(ps ...store.Position) (positions_page.Config, map[int64]string) {
	c, m := positionsConfig(ps...)
	return marksOf(c), m
}

func withBars(c positions_page.Config, ids *fakeStockIDs, bars *fakeBars) positions_page.Config {
	c.StockIDs, c.Bars = ids, bars
	return c
}

func cellText(t *testing.T, m map[string]any, row int, field string) string {
	t.Helper()
	s, _ := at0(t, m, "views", 0, "stocks", "rows", row, field, "text").(string)
	return s
}

func TestPriceFallsBackToBarClose(t *testing.T) {
	t.Parallel()
	c, _ := noMarks(
		pos("2026-11-10", "AAA", "stock", "USD", 1, "10", "100"),
		pos("2026-11-10", "BBB", "stock", "USD", 2, "10", "100"),
		option(pos("2026-11-10", "XYZ", "option", "USD", 3, "-1", "260"), "put", "2026-11-20", "100"),
	)
	ids := &fakeStockIDs{rows: []store.InstrumentName{idOf(11, "AAA"), idOf(12, "BBB")}}
	bars := &fakeBars{byID: map[int64][]contracts.PriceBar{
		11: {bar(11, "2026-11-06", "100", "USD"), bar(11, "2026-11-09", "110", "USD")},
		12: {bar(12, "2026-11-06", "50", "USD"), bar(12, "2026-11-09", "60", "USD")},
	}}
	m, b := build(t, withBars(marksOf(c, midMark(2, "65")), ids, bars), "main")
	validate(t, b)
	if got := cellText(t, m, 0, "price"); got != "$110.00" {
		t.Fatalf("bar price = %q", got)
	}
	if got := cellText(t, m, 1, "price"); got != "$65.00" {
		t.Fatalf("marked price = %q, the mark is kept", got)
	}
	if got := cellText(t, m, 1, "day"); got != "+$50.00" {
		t.Fatalf("marked day = %q, want 10 x (65 - 60)", got)
	}
	if n := ids.calls.Load(); n != 1 {
		t.Fatalf("StockIDs calls = %d", n)
	}
	if strings.Join(ids.names, ",") != "AAA,BBB" {
		t.Fatalf("names = %v", ids.names)
	}
	if n := bars.calls.Load(); n != 2 {
		t.Fatalf("PriceBars calls = %d, want one per resolved stock row", n)
	}
}

func TestPreviousCloseIsTheBarBeforeThePriceDay(t *testing.T) {
	t.Parallel()
	c, _ := noMarks(
		pos("2026-11-10", "AAA", "stock", "USD", 1, "10", "100"),
		pos("2026-11-10", "7203", "stock", "JPY", 2, "100", "2800"),
		pos("2026-11-10", "CCC", "stock", "USD", 3, "5", "10"),
	)
	ids := &fakeStockIDs{rows: []store.InstrumentName{idOf(11, "AAA"), idOf(12, "7203"), idOf(13, "CCC")}}
	bars := &fakeBars{byID: map[int64][]contracts.PriceBar{
		// the bar of the capture day is not the previous close of a mark
		11: {bar(11, "2026-11-06", "100", "USD"), bar(11, "2026-11-09", "110", "USD"), bar(11, "2026-11-10", "999", "USD")},
		// a JP name whose last bar is older than the capture day
		12: {bar(12, "2026-11-05", "2900", "JPY"), bar(12, "2026-11-06", "3000", "JPY")},
		13: {bar(13, "2026-11-09", "20", "USD")},
	}}
	m, b := build(t, withBars(marksOf(c, midMark(1, "112")), ids, bars), "main")
	validate(t, b)
	if got := cellText(t, m, 0, "day"); got != "+$20.00" {
		t.Fatalf("US day = %q, want 10 x (112 - 110)", got)
	}
	if got := cellText(t, m, 1, "price"); got != "¥3,000.00" {
		t.Fatalf("JP price = %q", got)
	}
	if got := cellText(t, m, 1, "day"); got != "+¥10,000.00" {
		t.Fatalf("JP day = %q, want 100 x (3000 - 2900)", got)
	}
	if got := cellText(t, m, 2, "price"); got != "$20.00" {
		t.Fatalf("one-bar price = %q", got)
	}
	if got := cellText(t, m, 2, "day"); got != "n/a" {
		t.Fatalf("one-bar day = %q, want the gap", got)
	}
}

func TestSharedSymbolAndCurrencyMismatchAreGaps(t *testing.T) {
	t.Parallel()
	c, _ := noMarks(
		pos("2026-11-10", "AAA", "stock", "USD", 1, "10", "100"),
		pos("2026-11-10", "BBB", "stock", "USD", 2, "10", "100"),
		pos("2026-11-10", "CCC", "stock", "USD", 3, "10", "100"),
		pos("2026-11-10", "DDD", "stock", "USD", 4, "10", "100"),
	)
	ids := &fakeStockIDs{rows: []store.InstrumentName{
		idOf(11, "AAA"), idOf(21, "AAA"), // shared
		idOf(12, "BBB"), idOf(13, "CCC"), // DDD has no id
	}}
	bars := &fakeBars{byID: map[int64][]contracts.PriceBar{
		11: {bar(11, "2026-11-08", "1", "USD"), bar(11, "2026-11-09", "2", "USD")},
		12: {bar(12, "2026-11-08", "1", "JPY"), bar(12, "2026-11-09", "2", "JPY")}, // price bar currency differs
		13: {bar(13, "2026-11-08", "1", "JPY"), bar(13, "2026-11-09", "2", "USD")}, // only the previous differs
	}}
	m, b := build(t, withBars(c, ids, bars), "main")
	validate(t, b)
	for row, want := range map[int]string{0: "No price: no data yet", 1: "No price: no data yet", 3: "No price: no data yet", 2: "$2.00"} {
		if got := cellText(t, m, row, "price"); got != want {
			t.Fatalf("row %d price = %q, want %q", row, got, want)
		}
	}
	for row := 0; row < 4; row++ {
		if got := cellText(t, m, row, "day"); got != "n/a" {
			t.Fatalf("row %d day = %q, want the gap", row, got)
		}
	}
	if n := bars.calls.Load(); n != 2 {
		t.Fatalf("PriceBars calls = %d, shared and unknown symbols read none", n)
	}
}

func TestBarPricedRowFillsFigures(t *testing.T) {
	t.Parallel()
	c, _ := noMarks(pos("2026-11-10", "AAA", "stock", "USD", 1, "10", "100"))
	ids := &fakeStockIDs{rows: []store.InstrumentName{idOf(11, "AAA")}}
	bars := &fakeBars{byID: map[int64][]contracts.PriceBar{
		11: {bar(11, "2026-11-06", "120", "USD"), bar(11, "2026-11-09", "130", "USD")},
	}}
	m, _ := build(t, withBars(c, ids, bars), "main")
	if got := cellText(t, m, 0, "unrealised"); got != "+$300.00" {
		t.Fatalf("unrealised = %q, want 10 x (130 - 100)", got)
	}
	if got := cellText(t, m, 0, "day"); got != "+$100.00" {
		t.Fatalf("day = %q", got)
	}
	gap := at0(t, m, "gaps", 0, "text")
	if gap != "Options: Day result needs the previous trading day price: no data yet" {
		t.Fatalf("gap = %v", gap)
	}
}

func TestFallbackNoteWithinNotesCap(t *testing.T) {
	t.Parallel()
	var ps []store.Position
	var found []store.InstrumentName
	byID := map[int64][]contracts.PriceBar{}
	for i := 1; i <= 25; i++ {
		name := fmt.Sprintf("STK%02d", i)
		ps = append(ps, pos("2026-11-10", name, "stock", "USD", int64(i), "10", "10"))
		id := int64(100 + i)
		found = append(found, idOf(id, name))
		byID[id] = []contracts.PriceBar{bar(id, "2026-11-06", "5", "USD"), bar(id, fmt.Sprintf("2026-11-%02d", 7+i%3), "6", "USD")}
	}
	ps = append(ps, pos("2026-11-10", "FRAC", "option", "USD", 900, "1.5", "1"))
	c, _ := noMarks(ps...)
	c.Statement = []string{"main"}
	c.Ledgers = func() compute.OptionLedger { return openLegs(nil) }
	m, b := build(t, withBars(c, &fakeStockIDs{rows: found}, &fakeBars{byID: byID}), "main")
	validate(t, b)
	notes := notesOf(t, m)
	if len(notes) > 20 || len(notes) != 3 {
		t.Fatalf("notes = %d: %v", len(notes), notes)
	}
	var fb string
	for _, n := range notes {
		s := n.(string)
		if l := len([]rune(s)); l > 200 {
			t.Fatalf("note of %d characters: %s", l, s)
		}
		if strings.Contains(s, "last stored close") {
			fb = s
		}
	}
	if !strings.Contains(fb, "STK01") || !strings.Contains(fb, " more is the last stored close") || strings.Contains(fb, "STK09") {
		t.Fatalf("fallback note = %q", fb)
	}
}

func TestBarReaderErrorsAreErrors(t *testing.T) {
	t.Parallel()
	c, _ := noMarks(pos("2026-11-10", "AAA", "stock", "USD", 1, "10", "100"))
	for name, cfg := range map[string]positions_page.Config{
		"stock ids": withBars(c, &fakeStockIDs{err: errors.New("boom")}, &fakeBars{}),
		"bars": withBars(c, &fakeStockIDs{rows: []store.InstrumentName{idOf(11, "AAA")}},
			&fakeBars{err: errors.New("boom")}),
	} {
		if _, err := positions_page.NewPage(cfg, "main").Build(context.Background(), nil, day, at); err == nil {
			t.Fatalf("%s: want an error", name)
		}
	}
}

func TestGoldenBars(t *testing.T) {
	t.Parallel()
	c, _ := noMarks(
		pos("2026-11-10", "AAA", "stock", "USD", 1, "10", "100"),
		pos("2026-11-10", "7203", "stock", "JPY", 2, "100", "2800"),
		pos("2026-11-10", "CCC", "stock", "USD", 3, "5", "10"),
		option(pos("2026-11-10", "XYZ", "option", "USD", 4, "-1", "260"), "put", "2026-11-20", "100"),
	)
	ids := &fakeStockIDs{rows: []store.InstrumentName{idOf(11, "AAA"), idOf(12, "7203"), idOf(13, "CCC")}}
	bars := &fakeBars{byID: map[int64][]contracts.PriceBar{
		11: {bar(11, "2026-11-06", "100", "USD"), bar(11, "2026-11-09", "110", "USD")},
		12: {bar(12, "2026-11-05", "2900", "JPY"), bar(12, "2026-11-06", "3000", "JPY")},
		13: {bar(13, "2026-11-09", "20", "USD")},
	}}
	_, b := build(t, withBars(marksOf(c, midMark(4, "3.15")), ids, bars), "main")
	validate(t, b)
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		t.Fatal(err)
	}
	out.WriteByte('\n')
	golden := filepath.Join("testdata", "bars.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, out.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, out.Bytes()) {
		t.Fatalf("output differs from %s:\n%s", golden, out.Bytes())
	}
}

func TestBarFallbackUsesPickedBar(t *testing.T) {
	t.Parallel()
	c, _ := noMarks(pos("2026-11-10", "AAA", "stock", "USD", 1, "10", "100"))
	ids := &fakeStockIDs{rows: []store.InstrumentName{idOf(11, "AAA")}}
	bars := &fakeBars{byID: map[int64][]contracts.PriceBar{
		11: {bar(11, "2026-11-06", "100", "USD"), bar(11, "2026-11-09", "110", "USD"), bar(11, "2026-11-10", "0", "USD")},
	}}
	m, b := build(t, withBars(c, ids, bars), "main")
	validate(t, b)
	if got := cellText(t, m, 0, "price"); got != "$110.00" {
		t.Fatalf("price = %q, want the newest bar with a positive close", got)
	}
}

// closedSessions says whether calendar XTKS has a session on the capture day; any other calendar is open.
type closedSessions struct {
	fakeSessions
	rows []contracts.SessionDay
	err  error
}

func (f closedSessions) SessionDays(ctx context.Context, cal string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	if cal != "XTKS" {
		return f.fakeSessions.SessionDays(ctx, cal, from, to)
	}
	return f.rows, f.err
}

func tokyoDay(isSession bool) []contracts.SessionDay {
	return []contracts.SessionDay{{CalendarID: "XTKS", SessionDay: types.SerializableDate{Time: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)}, IsSession: isSession}}
}

// jpStock is a held TSEJ stock marked 14.9 over its last bar close of 4840.
func jpStock(sessions closedSessions) positions_page.Config {
	p := pos("2026-11-10", "TKY1", "stock", "JPY", 1, "500", "4000")
	p.Exchange = "TSEJ"
	c, _ := noMarks(p)
	c.Sessions = sessions
	ids := &fakeStockIDs{rows: []store.InstrumentName{idOf(11, "TKY1")}}
	bars := &fakeBars{byID: map[int64][]contracts.PriceBar{11: {bar(11, "2026-11-06", "4830", "JPY"), bar(11, "2026-11-09", "4840", "JPY")}}}
	return withBars(marksOf(c, midMark(1, "4854.9")), ids, bars)
}

func TestClosedMarketGivesNoDayResult(t *testing.T) {
	t.Parallel()
	m, b := build(t, jpStock(closedSessions{rows: tokyoDay(false)}), "main")
	validate(t, b)
	if got := cellText(t, m, 0, "day"); got != "n/a" {
		t.Fatalf("day = %q, want the gap", got)
	}
	if r := at0(t, m, "views", 0, "stocks", "rows", 0, "day", "reason"); r != "no_data" {
		t.Fatalf("reason = %v", r)
	}
	if got := cellText(t, m, 0, "price"); got != "¥4,854.90" {
		t.Fatalf("price = %q, unchanged", got)
	}
	var note string
	for _, n := range notesOf(t, m) {
		if s := n.(string); strings.Contains(s, "closed") {
			note = s
		}
	}
	if !strings.Contains(note, "TKY1") || !strings.Contains(note, "2026-11-10") || len([]rune(note)) > 200 {
		t.Fatalf("note = %q", note)
	}
}

func TestOpenMarketKeepsDayResult(t *testing.T) {
	t.Parallel()
	for name, s := range map[string]closedSessions{"session": {rows: tokyoDay(true)}, "no row": {}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m, _ := build(t, jpStock(s), "main")
			if got := cellText(t, m, 0, "day"); got != "+¥7,450.00" {
				t.Fatalf("day = %q, want 500 x (4854.9 - 4840)", got)
			}
			for _, n := range notesOf(t, m) {
				if strings.Contains(n.(string), "closed") {
					t.Fatalf("note %q", n)
				}
			}
		})
	}
}

func TestClosedMarketReaderErrorIsAnError(t *testing.T) {
	t.Parallel()
	c := jpStock(closedSessions{err: errors.New("boom")})
	if _, err := positions_page.NewPage(c, "main").Build(context.Background(), nil, day, at); err == nil {
		t.Fatal("want the reader error")
	}
}
