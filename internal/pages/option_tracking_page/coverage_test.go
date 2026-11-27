package option_tracking_page

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

// tenSessions is ten consecutive session days, 2026-11-01 to 2026-11-10 (the as-of day last).
type tenSessions struct{}

func (tenSessions) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	var out []contracts.SessionDay
	for d := 1; d <= 10; d++ {
		t := time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC)
		out = append(out, contracts.SessionDay{SessionDay: types.SerializableDate{Time: t}, IsSession: true})
	}
	return out, nil
}

type fakeDays struct {
	optionday.DayReader
	rows []optionday.Day
	err  error
}

func (f fakeDays) Days(_ context.Context, ids []int64, from, to contracts.Day) ([]optionday.Day, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []optionday.Day
	for _, r := range f.rows {
		for _, id := range ids {
			if r.InstrumentID == id && r.SessionDay >= from && r.SessionDay <= to {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

func dayRow(id int64, day string, bid, ask string) optionday.Day {
	return optionday.Day{InstrumentID: id, SessionDay: contracts.Day(day), Bid: new(bid), Ask: new(ask)}
}

// covConfig is a page Config whose replay holds one open position per opened day, contract ids 10, 11, ...
func covConfig(opened ...string) (Config, []opt) {
	var pos []compute.LedgerPosition
	var open []opt
	for i, o := range opened {
		id := "p" + strconv.Itoa(i)
		pos = append(pos, compute.LedgerPosition{PositionID: id, Underlying: "AAPL", Contract: strconv.Itoa(10 + i), Status: "open"})
		r := sampleOpen(id)
		r.OpenedOn = contracts.Day(o)
		open = append(open, r)
	}
	c := cfgFor(map[string]trackapi.TrackResult{"main": {Open: open}}, "main")
	c.Replay = &fakeReplay{out: compute.LedgerResult{Positions: pos}}
	c.Sessions = tenSessions{}
	c.Catalog, c.Eod = fakeCatalog{ids: []int64{1}}, fakeEod{rows: nil}
	return c, open
}

func jsonMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// cellOf is the coverage cell of open row i of the first account.
func cellOf(t *testing.T, m map[string]any, i int) map[string]any {
	t.Helper()
	acc := m["accounts"].([]any)[0].(map[string]any)
	row := acc["open"].(map[string]any)["rows"].([]any)[i].(map[string]any)
	return row["cells"].(map[string]any)[coverageID].(map[string]any)
}

// notesWith counts the notes that contain text.
func notesWith(m map[string]any, text string) int {
	n := 0
	for _, v := range m["notes"].([]any) {
		if strings.Contains(v.(string), text) {
			n++
		}
	}
	return n
}

func buildMap(t *testing.T, c Config) map[string]any {
	t.Helper()
	return jsonMap(t, buildRaw(t, c))
}

func TestCoverageNoReaderIsGap(t *testing.T) {
	t.Parallel()
	c, _ := covConfig("2026-11-06")
	m := buildMap(t, c)
	cell := cellOf(t, m, 0)
	if cell["kind"] != "gap" || cell["reason"] != "no_reader" {
		t.Fatalf("cell = %v", cell)
	}
	if notesWith(m, coverageNoteMidZh) != 0 {
		t.Fatalf("notes = %v", m["notes"])
	}
}

func TestCoverageCountsTwoSidedDays(t *testing.T) {
	t.Parallel()
	c, _ := covConfig("2026-11-08")
	c.Days = fakeDays{rows: []optionday.Day{dayRow(10, "2026-11-08", "1", "2"), dayRow(10, "2026-11-09", "1", "2"), dayRow(10, "2026-11-10", "1", "2")}}
	cell := cellOf(t, buildMap(t, c), 0)
	if cell["text"] != "3 of 3 days" || cell["tone"] != "none" || len(cell["badges"].([]any)) != 0 {
		t.Fatalf("cell = %v", cell)
	}
	c.Days = fakeDays{rows: []optionday.Day{dayRow(10, "2026-11-08", "1", "2"), dayRow(10, "2026-11-09", "1", "0"), dayRow(10, "2026-11-10", "1", "2")}}
	cell = cellOf(t, buildMap(t, c), 0)
	if cell["text"] != "2 of 3 days" || cell["tone"] != "down" || len(cell["badges"].([]any)) != 0 {
		t.Fatalf("cell = %v", cell)
	}
}

func TestCoverageGapReasons(t *testing.T) {
	t.Parallel()
	c, _ := covConfig("2026-11-08")
	// 11-08 no row anywhere (before the first stored quote, not counted); 11-09 no option_day row
	// but an option_eod row with both sides; 11-10 stored.
	c.Days = fakeDays{rows: []optionday.Day{dayRow(10, "2026-11-10", "1", "2")}}
	c.Eod = fakeEod{rows: map[int64][]contracts.OptionEod{1: {eodRow(10, "2026-11-09", 1, 2)}}}
	cell := cellOf(t, buildMap(t, c), 0)
	if cell["text"] != "2 of 2 days" {
		t.Fatalf("cell = %v", cell)
	}

	c, _ = covConfig("2026-11-01")
	c.Days = fakeDays{rows: []optionday.Day{dayRow(10, "2026-11-08", "1", "2"), dayRow(10, "2026-11-09", "1", "2"), dayRow(10, "2026-11-10", "1", "2")}}
	cell = cellOf(t, buildMap(t, c), 0)
	if cell["text"] != "3 of 3 days" {
		t.Fatalf("cell = %v", cell)
	}
}

func TestCoverageCountsFromFirstStoredQuote(t *testing.T) {
	t.Parallel()
	// Opened 2026-11-01; the first stored quote is 11-08, with a gap on 11-09: seven earlier sessions are not counted.
	c, _ := covConfig("2026-11-01")
	c.Days = fakeDays{rows: []optionday.Day{dayRow(10, "2026-11-08", "1", "2"), dayRow(10, "2026-11-10", "1", "2")}}
	m := buildMap(t, c)
	cell := cellOf(t, m, 0)
	if cell["text"] != "2 of 3 days" || len(cell["badges"].([]any)) != 0 {
		t.Fatalf("cell = %v", cell)
	}
	if notesWith(m, "main: "+coverageNoteHeadZh+"3"+coverageNoteMidZh+"1"+coverageNoteTailZh) != 1 {
		t.Fatalf("notes = %v", m["notes"])
	}
}

func TestCoverageNoteTotals(t *testing.T) {
	t.Parallel()
	c, _ := covConfig("2026-11-01", "2026-11-06")
	// row 10 (10 days) lacks 11-02; row 11 (5 days) lacks 11-07 and 11-08: 3 gaps of 15.
	var rows []optionday.Day
	for _, d := range []string{"01", "03", "04", "05", "06", "07", "08", "09", "10"} {
		rows = append(rows, dayRow(10, "2026-11-"+d, "1", "2"))
	}
	for _, d := range []string{"06", "09", "10"} {
		rows = append(rows, dayRow(11, "2026-11-"+d, "1", "2"))
	}
	c.Days = fakeDays{rows: rows}
	m := buildMap(t, c)
	if notesWith(m, "main: "+coverageNoteHeadZh+"15"+coverageNoteMidZh+"3"+coverageNoteTailZh) != 1 {
		t.Fatalf("notes = %v", m["notes"])
	}
	// an alias with no open row has no note
	c2 := cfgFor(map[string]trackapi.TrackResult{"main": {Recent: []opt{sampleClosed("p0")}}}, "main")
	c2.Days = fakeDays{}
	if m := buildMap(t, c2); notesWith(m, coverageNoteMidZh) != 0 {
		t.Fatalf("notes = %v", m["notes"])
	}
}

func TestCoverageReaderErrorFailsBuild(t *testing.T) {
	t.Parallel()
	c, _ := covConfig("2026-11-06")
	c.Days = fakeDays{err: errors.New("boom")}
	_, err := Build(context.Background(), c, nil, buildDay, time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestCoverageColumnIsLast(t *testing.T) {
	t.Parallel()
	last := openColumns("", nil)[len(openColumns("", nil))-1]
	if last.ID != "coverage" || last.Label != "Quote coverage" || last.Hint == nil || *last.Hint != coverageHint {
		t.Fatalf("last column = %+v", last.column)
	}
}

func TestCoverageEodNoSidesIsNoQuote(t *testing.T) {
	t.Parallel()
	c, _ := covConfig("2026-11-09")
	c.Days = fakeDays{rows: []optionday.Day{dayRow(10, "2026-11-10", "1", "2")}}
	c.Eod = fakeEod{rows: map[int64][]contracts.OptionEod{1: {eodRow(10, "2026-11-09", 0, 0)}}}
	if cell := cellOf(t, buildMap(t, c), 0); cell["text"] != "1 of 1 days" {
		t.Fatalf("cell = %v", cell)
	}
}

func TestCoverageHintNamesFirstStoredQuote(t *testing.T) {
	t.Parallel()
	last := openColumns("", nil)[len(openColumns("", nil))-1]
	if last.Hint == nil || strings.Contains(*last.Hint, "开仓后") {
		t.Fatalf("hint = %v, counts from the first stored quote now", last.Hint)
	}
}
