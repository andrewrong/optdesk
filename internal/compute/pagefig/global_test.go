package pagefig

import (
	"fmt"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// friday is the page day of most tests (2026-11-13 is a Friday).
const friday = contracts.Day("2026-11-13")

// weekdays returns n weekdays newest first, the first being day (a weekday).
func weekdays(day contracts.Day, n int) []contracts.Day {
	t, _ := time.Parse("2006-01-02", string(day))
	out := make([]contracts.Day, 0, n)
	for len(out) < n {
		if wd := t.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, contracts.Day(t.Format("2006-01-02")))
		}
		t = t.AddDate(0, 0, -1)
	}
	return out
}

func ser(sym string, c0 contracts.Day, closes ...string) blockapi.CloseIn {
	return blockapi.CloseIn{Name: sym, Symbol: sym, Closes: closes, Days: weekdays(c0, len(closes))}
}

func flat(n int, first string) []string {
	out := []string{first}
	for len(out) < n {
		out = append(out, "100")
	}
	return out
}

func glob(day contracts.Day, spy blockapi.CloseIn, proxies, sectors []blockapi.CloseIn) blockapi.GlobalResult {
	return NewGlobal().Global(blockapi.GlobalIn{Day: day, Benchmark: spy, Proxies: proxies, Sectors: sectors})
}

func TestProxyFormulaIsMarketMove(t *testing.T) {
	t.Parallel()
	s := ser("SPY", friday, "110", "100", "100", "100", "100", "125")
	want := New().Moves([]blockapi.CloseIn{s})[0]
	got := glob(friday, s, []blockapi.CloseIn{s}, nil).Proxies[0]
	if got.D1 != want.D1 || got.D5 != want.D5 || got.D1.Value != "10.00" || got.D5.Value != "-12.00" {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestProxyOneDayBeforeDayGivesFigures(t *testing.T) {
	t.Parallel()
	c0 := weekdays(friday, 2)[1]
	s := ser("SPY", c0, "110", "100", "100", "100", "100", "100")
	got := glob(friday, s, []blockapi.CloseIn{s}, nil)
	if got.DataDay != c0 || got.Proxies[0].D1.Value != "10.00" {
		t.Fatalf("got %+v", got)
	}
}

func TestWeekendDayShowsFridayClose(t *testing.T) {
	t.Parallel()
	s := ser("SPY", friday, "110", "100", "100", "100", "100", "100")
	got := glob("2026-11-14", s, []blockapi.CloseIn{s}, nil)
	if got.DataDay != friday || got.Proxies[0].D1.Value != "10.00" {
		t.Fatalf("got %+v", got)
	}
}

func TestProxyStaleAfterThreeWeekdays(t *testing.T) {
	t.Parallel()
	c0 := weekdays(friday, 5)[4]
	s := ser("SPY", c0, flat(6, "110")...)
	got := glob(friday, s, []blockapi.CloseIn{s}, []blockapi.CloseIn{s})
	if got.Proxies[0].D1.Gap != "stale_input" || got.Proxies[0].D5.Gap != "stale_input" {
		t.Fatalf("proxy %+v", got.Proxies[0])
	}
	// Three weekdays behind is still fine.
	ok := ser("SPY", weekdays(friday, 4)[3], flat(6, "110")...)
	if g := glob(friday, ok, []blockapi.CloseIn{ok}, nil); g.Proxies[0].D1.Gap != "" {
		t.Fatalf("three behind: %+v", g.Proxies[0])
	}
}

func TestProxyDifferentC0IsMissingInput(t *testing.T) {
	t.Parallel()
	spy := ser("SPY", friday, flat(6, "100")...)
	p := ser("TLT", weekdays(friday, 2)[1], flat(6, "100")...)
	got := glob(friday, spy, []blockapi.CloseIn{p}, nil).Proxies[0]
	if got.D1.Gap != "missing_input" || got.D5.Gap != "missing_input" {
		t.Fatalf("got %+v", got)
	}
}

func TestSectorExcessUsesUnroundedChanges(t *testing.T) {
	t.Parallel()
	spy := ser("SPY", friday, "100.004", "100")
	etf := ser("XLK", friday, "101.004", "100")
	if d1 := glob(friday, spy, nil, []blockapi.CloseIn{etf}).Sectors[0].D1; d1.Value != "1.00" {
		t.Fatalf("d1 %+v", d1)
	}
	// Rounded first: 1.01 - 0.00 = 1.01; exact: 1.006 - 0.004 = 1.002 -> 1.00.
	etf = ser("XLK", friday, "101.006", "100")
	if d1 := glob(friday, spy, nil, []blockapi.CloseIn{etf}).Sectors[0].D1; d1.Value != "1.00" {
		t.Fatalf("d1 %+v", d1)
	}
}

func TestSectorNeedsSameDays(t *testing.T) {
	t.Parallel()
	spy := ser("SPY", friday, flat(6, "100")...)
	etf := ser("XLK", friday, flat(6, "100")...)
	etf.Days[5] = "2020-01-01"
	got := glob(friday, spy, nil, []blockapi.CloseIn{etf}).Sectors[0]
	if got.D5.Gap != "missing_input" || got.D1.Gap != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestSectorStaleBenchmark(t *testing.T) {
	t.Parallel()
	c0 := weekdays(friday, 5)[4]
	spy := ser("SPY", c0, flat(21, "100")...)
	rows := []blockapi.CloseIn{ser("XLK", c0, flat(21, "100")...), ser("XLE", c0, flat(21, "100")...)}
	for _, r := range glob(friday, spy, nil, rows).Sectors {
		if r.D1.Gap != "stale_input" || r.D5.Gap != "stale_input" || r.D20.Gap != "stale_input" {
			t.Fatalf("row %+v", r)
		}
	}
}

func TestSectorC0DiffersFromSpy(t *testing.T) {
	t.Parallel()
	spy := ser("SPY", friday, flat(21, "100")...)
	etf := ser("XLK", weekdays(friday, 2)[1], flat(21, "100")...)
	r := glob(friday, spy, nil, []blockapi.CloseIn{etf}).Sectors[0]
	if r.D1.Gap != "missing_input" || r.D5.Gap != "missing_input" || r.D20.Gap != "missing_input" {
		t.Fatalf("row %+v", r)
	}
}

func TestNoSpyIsNoDataEverywhere(t *testing.T) {
	t.Parallel()
	p := ser("TLT", friday, flat(21, "100")...)
	got := glob(friday, blockapi.CloseIn{Name: "SPY", Symbol: "SPY"}, []blockapi.CloseIn{p}, []blockapi.CloseIn{p})
	r := got.Sectors[0]
	if got.DataDay != "" || got.Proxies[0].D1.Gap != "no_data" || got.Proxies[0].D5.Gap != "no_data" ||
		r.D1.Gap != "no_data" || r.D5.Gap != "no_data" || r.D20.Gap != "no_data" || got.Rotation != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestSectorTwentyNeeds21Closes(t *testing.T) {
	t.Parallel()
	spy := ser("SPY", friday, flat(20, "101")...)
	etf := ser("XLK", friday, flat(20, "102")...)
	r := glob(friday, spy, nil, []blockapi.CloseIn{etf}).Sectors[0]
	if r.D20.Gap != "no_data" || r.D1.Value == "" || r.D5.Value == "" {
		t.Fatalf("row %+v", r)
	}
	spy, etf = ser("SPY", friday, flat(21, "101")...), ser("XLK", friday, flat(21, "102")...)
	if r := glob(friday, spy, nil, []blockapi.CloseIn{etf}).Sectors[0]; r.D20.Value != "1.00" {
		t.Fatalf("d20 %+v", r.D20)
	}
}

func TestEmptySeriesIsNoData(t *testing.T) {
	t.Parallel()
	spy := ser("SPY", friday, flat(21, "100")...)
	e := blockapi.CloseIn{Name: "XLK", Symbol: "XLK"}
	got := glob(friday, spy, []blockapi.CloseIn{e}, []blockapi.CloseIn{e})
	if got.Proxies[0].D1.Gap != "no_data" || got.Sectors[0].D5.Gap != "no_data" {
		t.Fatalf("got %+v", got)
	}
}

func TestNonPositiveCloseIsMissingInput(t *testing.T) {
	t.Parallel()
	spy := ser("SPY", friday, flat(6, "100")...)
	zero := ser("TLT", friday, "100", "0", "100", "100", "100", "x")
	got := glob(friday, spy, []blockapi.CloseIn{zero}, []blockapi.CloseIn{zero})
	if got.Proxies[0].D1.Gap != "missing_input" || got.Proxies[0].D5.Gap != "missing_input" ||
		got.Sectors[0].D1.Gap != "missing_input" || got.Sectors[0].D5.Gap != "missing_input" {
		t.Fatalf("got %+v", got)
	}
}

func TestSectorsSortedByD5GapsLast(t *testing.T) {
	t.Parallel()
	spy := ser("SPY", friday, flat(6, "100")...)
	rows := []blockapi.CloseIn{
		ser("A", friday, flat(6, "101")...),
		blockapi.CloseIn{Name: "B", Symbol: "B"},
		ser("C", friday, flat(6, "103")...),
		ser("D", friday, flat(6, "101")...),
		ser("E", friday, flat(6, "99")...),
	}
	var names []string
	for _, r := range glob(friday, spy, nil, rows).Sectors {
		names = append(names, r.Name)
	}
	if fmt.Sprint(names) != "[C A D E B]" {
		t.Fatalf("order %v", names)
	}
}

// elevenSectors has d5 (and d20) excess of 11 - i for row i against a flat SPY, rows named S0..S10.
func elevenSectors(n int) (blockapi.CloseIn, []blockapi.CloseIn) {
	spy := ser("SPY", friday, flat(n, "100")...)
	var rows []blockapi.CloseIn
	for i := 0; i < 11; i++ {
		rows = append(rows, ser(fmt.Sprintf("S%d", i), friday, flat(n, fmt.Sprint(111-i))...))
	}
	return spy, rows
}

func TestRotationFacts(t *testing.T) {
	t.Parallel()
	spy, rows := elevenSectors(21)
	// Shuffle the input: the first row gets the lowest value.
	rows[0], rows[10] = rows[10], rows[0]
	rot := glob(friday, spy, nil, rows).Rotation
	if rot == nil || fmt.Sprint(rot.Leaders5) != "[S0 S1 S2]" || rot.Laggard != "S10" || rot.LaggardD5 != "1.00" ||
		rot.Leader20 != "S0" {
		t.Fatalf("rotation %+v", rot)
	}
	// d20 tie with S0 (input index 10): T at input index 3 comes first in list order.
	rows = append([]blockapi.CloseIn(nil), rows...)
	rows[3] = ser("T", friday, flat(21, "111")...)
	if rot = glob(friday, spy, nil, rows).Rotation; rot.Leader20 != "T" {
		t.Fatalf("tie not by list order: %+v", rot)
	}
}

func TestRotationOmittedBelowSixRows(t *testing.T) {
	t.Parallel()
	spy, rows := elevenSectors(21)
	if r := glob(friday, spy, nil, rows[:5]).Rotation; r != nil {
		t.Fatalf("five rows: %+v", r)
	}
	rows[5] = blockapi.CloseIn{Name: "gone"}
	if r := glob(friday, spy, nil, rows[:6]).Rotation; r != nil {
		t.Fatalf("five with d5: %+v", r)
	}
	if r := glob(friday, spy, nil, rows[:7]).Rotation; r == nil {
		t.Fatal("six with d5 should have rotation")
	}
}

func TestRotationOmittedWithoutD20(t *testing.T) {
	t.Parallel()
	spy, rows := elevenSectors(6)
	if r := glob(friday, spy, nil, rows).Rotation; r != nil {
		t.Fatalf("rotation %+v", r)
	}
}
