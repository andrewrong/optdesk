package option_boards

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var testZone = time.FixedZone("+08", 8*3600)

// The stored issuer texts of the watch list; the display names are the tickers of nameFake.
const (
	issuerApple = "cik:0000320193"
	issuerMsft  = "cik:0000789019"
)

const buildDay = contracts.Day("2026-11-10")

// buildAt is 09:30 on the build day in the owner zone, given in UTC.
var buildAt = time.Date(2026, 11, 10, 1, 30, 0, 0, time.UTC)

func ip(n int) *int { return &n }

func dayp(d contracts.Day) *contracts.Day { return &d }

// row builds a stored short put; tests change the fields they care about.
func candRow(id int, issuer string, under int, status string) store.Candidate {
	return store.Candidate{
		SessionDay: buildDay, Kind: "short_put", InstrumentID: id, Issuer: issuer, UnderlyingID: under,
		Expiry: "2026-11-13", DTE: 3, Right: "put", Strike: "62", Multiplier: 100, BidEod: "1.20", Ask: sp("1.30"),
		Spot: sp("67.5"), BreakEven: "60.80", AnnualizedYield: sp("0.123456"), Delta: sp("-0.15"), Iv: sp("0.38"),
		Status: status, Gates: `[{"gate":"events","status":"pass"}]`, Marks: "[]", Fits: "{}",
		NextEarnings: dayp("2026-12-01"), EarningsState: "listed",
		ComputedAt: time.Date(2026, 11, 10, 0, 40, 0, 0, time.UTC),
	}
}

func okRow(id int, issuer string, under, rank, inU int) store.Candidate {
	c := candRow(id, issuer, under, "ok")
	c.Rank, c.RankInUnderlying = ip(rank), ip(inU)
	return c
}

// fixtureRows are ok, blocked and unknown rows of two companies and one row without ask.
func fixtureRows() []store.Candidate {
	a1 := okRow(11, issuerApple, 1, 1, 1)
	a2 := okRow(12, issuerApple, 1, 2, 2)
	a2.Expiry, a2.DTE, a2.Strike, a2.BidEod, a2.Ask, a2.Delta = "2026-11-20", 10, "60", "0.95", sp("1.05"), sp("-0.1")
	b1 := okRow(21, issuerMsft, 2, 3, 1)
	b1.Strike, b1.Spot, b1.BidEod, b1.Ask = "30.5", sp("33"), "0.40", sp("0.44")
	b1.Marks, b1.EarningsState, b1.NextEarnings = `["earnings_unconfirmed"]`, "absent", nil
	b2 := candRow(22, issuerMsft, 2, "blocked")
	b2.Gates, b2.Strike, b2.BidEod, b2.Ask = `[{"gate":"events","status":"block","reason":"earnings_in_term"}]`, "31", "0.50", sp("0.60")
	b3 := candRow(23, issuerMsft, 2, "unknown")
	b3.Gates = `[{"gate":"quote","status":"unknown","reason":"no_ask"}]`
	b3.Ask, b3.EarningsState, b3.NextEarnings, b3.Delta, b3.Strike = nil, "unknown", nil, nil, "29"
	return []store.Candidate{a1, a2, b1, b2, b3}
}

func fixtureEod() *countingEod {
	return &countingEod{rows: map[int64][]contracts.OptionEod{
		1: {{InstrumentID: 11, SessionDay: sdate("2026-11-09"), CaptureID: "a", Volume: fp(1240), OpenInterest: fp(5310)}},
		2: {{InstrumentID: 21, SessionDay: sdate("2026-11-09"), CaptureID: "a", Volume: fp(10)}},
	}}
}

func buildOpt(t *testing.T, cands store.CandidateReader, eod store.OptionEodReader, day contracts.Day) []byte {
	t.Helper()
	c := OptionConfig{RunID: "run-1", BuildID: "build-1", Scope: "options", Zone: testZone, Candidates: cands, Eod: eod, Names: tickerNames()}
	b, err := BuildOption(context.Background(), c, nil, day, buildAt)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decodeMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func pane(m map[string]any) map[string]any { return m["markets"].([]any)[0].(map[string]any) }

func cardsOf(m map[string]any) []any { return pane(m)["cards"].([]any) }

func tableRowsOf(m map[string]any) []any {
	return pane(m)["candidates"].(map[string]any)["rows"].([]any)
}

// cellOf is the cell of a column in a decoded row.
func cellOf(t *testing.T, r any, col string) map[string]any {
	t.Helper()
	for _, c := range r.(map[string]any)["cells"].([]any) {
		if cm := c.(map[string]any); cm["column"] == col {
			return cm["cell"].(map[string]any)
		}
	}
	t.Fatalf("no column %s", col)
	return nil
}

func goldenCheck(t *testing.T, name string, got []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Indent(&buf, got, "", "  "); err != nil {
		t.Fatal(err)
	}
	buf.WriteByte('\n')
	path := "testdata/" + name + ".golden.json"
	if *updateGolden {
		if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Errorf("%s differs from the golden file:\n%s", name, buf.String())
	}
}

func TestOptionPageGolden(t *testing.T) {
	t.Parallel()
	cands := &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: fixtureRows()}}
	goldenCheck(t, "option_page_ok", buildOpt(t, cands, fixtureEod(), buildDay))
}

func TestOptionPageNoRowsGolden(t *testing.T) {
	t.Parallel()
	goldenCheck(t, "option_page_no_rows", buildOpt(t, &fakeCands{}, fixtureEod(), buildDay))
}

func TestCardsBestPerCompany(t *testing.T) {
	t.Parallel()
	m := decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: fixtureRows()}}, fixtureEod(), buildDay))
	cards := cardsOf(m)
	if len(cards) != 2 || cards[0].(map[string]any)["symbol"] != "AAPL" || cards[1].(map[string]any)["symbol"] != "MSFT" {
		t.Fatalf("cards = %v", cards)
	}
	if got := cards[0].(map[string]any)["contract_line"]; got != "sell put 62 · 11-13 Fri · 3 days · Δ −0.15" {
		t.Errorf("contract_line = %v", got)
	}
	if got := cards[0].(map[string]any)["iv_line"]; got != "IV 38% · HV30 n/a · IVP n/a" {
		t.Errorf("iv_line = %v", got)
	}
	if got := cards[0].(map[string]any)["liquidity_line"]; got != "Volume 1240 · Open interest 5310" {
		t.Errorf("liquidity_line = %v", got)
	}
	if got := pane(m)["headline"]; got != "2 worth a look." {
		t.Errorf("headline = %v", got)
	}

	var many []store.Candidate
	for i := 1; i <= 8; i++ {
		many = append(many, okRow(i, fmt.Sprintf("S%d", i), i, i, 1))
	}
	m = decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: many}}, nil, buildDay))
	if n := len(cardsOf(m)); n != 6 {
		t.Errorf("cards = %d, want at most 6", n)
	}
	if got := pane(m)["all_link"].(map[string]any)["text"]; got != "All 8 opportunities" {
		t.Errorf("all_link = %v", got)
	}
}

func TestVerdictFromStatus(t *testing.T) {
	t.Parallel()
	for status, want := range map[string]verdict{
		"ok":      {"worth", "Worth a look", "good"},
		"unknown": {"pending", "Incomplete data", "warn"},
		"blocked": {"skip", "Not eligible", "muted"},
	} {
		if got := verdictOf(status); got != want {
			t.Errorf("%s: %+v, want %+v", status, got, want)
		}
	}
}

func TestUnknownGateIsPending(t *testing.T) {
	t.Parallel()
	m := decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: fixtureRows()}}, nil, buildDay))
	var got []string
	for _, r := range tableRowsOf(m) {
		got = append(got, cellOf(t, r, "verdict")["text"].(string))
	}
	if want := "Worth a look Worth a look Worth a look Not eligible Incomplete data"; strings.Join(got, " ") != want {
		t.Errorf("verdicts = %v", got)
	}
	// the unknown row never becomes a card
	if n := len(cardsOf(m)); n != 2 {
		t.Errorf("cards = %d", n)
	}
	// a blocked row with a blocking events gate says so in the E cell
	if got := cellOf(t, tableRowsOf(m)[3], "earnings")["text"]; got != "E 12-01 in window" {
		t.Errorf("E cell = %v", got)
	}
}

func TestStrikeSpreadCells(t *testing.T) {
	t.Parallel()
	m := decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: fixtureRows()}}, fixtureEod(), buildDay))
	rows := tableRowsOf(m)
	if c := cellOf(t, rows[0], "strike_pct"); c["text"] != "−8.1%" || c["value"] != float64(-815) {
		t.Errorf("strike_pct = %v", c)
	}
	if c := cellOf(t, rows[0], "spread_pct"); c["text"] != "8.0%" || c["value"] != float64(800) {
		t.Errorf("spread_pct = %v", c)
	}
	c := cellOf(t, rows[4], "spread_pct")
	if c["reason"] != "missing_input" || c["text"] != "missing ask" {
		t.Errorf("row without ask: %v", c)
	}
	if c := cellOf(t, rows[0], "ivp"); c["reason"] != "no_data" {
		t.Errorf("ivp = %v", c)
	}
	if c := cellOf(t, rows[0], "volume"); c["value"] != float64(1240) || c["unit"] != "contracts" {
		t.Errorf("volume = %v", c)
	}
	if c := cellOf(t, rows[1], "volume"); c["reason"] != "no_data" {
		t.Errorf("volume without eod = %v", c)
	}
	// a missing spot gives a gap, never a guess
	one := candRow(1, "NOS", 1, "ok")
	one.Spot = nil
	m = decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: {one}}}, nil, buildDay))
	if c := cellOf(t, tableRowsOf(m)[0], "strike_pct"); c["reason"] != "missing_input" || c["text"] != "missing spot" {
		t.Errorf("no spot: %v", c)
	}
}

func TestChainGapCell(t *testing.T) {
	t.Parallel()
	m := decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: fixtureRows()}}, nil, buildDay))
	ch := pane(m)["chain"].(map[string]any)
	if got := ch["summary"]; got != "Selected: AAPL 11/13 strike $62 Put" {
		t.Errorf("summary = %v", got)
	}
	rows := ch["table"].(map[string]any)["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
	// strikes ascending: 60 first, which has no stored row on 11-13
	if c := cellOf(t, rows[0], "strike"); c["text"] != "60" {
		t.Errorf("first strike = %v", c)
	}
	if c := cellOf(t, rows[0], "e2026-11-13"); c["reason"] != "no_data" {
		t.Errorf("missing pair = %v", c)
	}
	if c := cellOf(t, rows[1], "e2026-11-13"); c["text"] != "$1.20" || c["unit"] != "usd" {
		t.Errorf("stored bid = %v", c)
	}
	// no card, no chain
	m = decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: {candRow(5, "ZZZ", 5, "blocked")}}}, nil, buildDay))
	if pane(m)["chain"] != nil {
		t.Errorf("chain = %v", pane(m)["chain"])
	}
}

func TestTableCutAt200(t *testing.T) {
	t.Parallel()
	var many []store.Candidate
	for i := 1; i <= 205; i++ {
		many = append(many, candRow(i, "SYM", 1, "blocked"))
	}
	m := decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: many}}, nil, buildDay))
	if n := len(tableRowsOf(m)); n != 200 {
		t.Errorf("rows = %d", n)
	}
	notes := m["notes"].([]any)
	if len(notes) != 2 || notes[1] != "Candidate table shows the first 200 of 205" {
		t.Errorf("notes = %v", notes)
	}
	if got := pane(m)["all_link"].(map[string]any)["text"]; got != "All 205 opportunities" {
		t.Errorf("all_link = %v", got)
	}
}

func TestNoReaderGap(t *testing.T) {
	t.Parallel()
	m := decodeMap(t, buildOpt(t, nil, nil, buildDay))
	gaps := m["gaps"].([]any)
	if len(gaps) != 1 || gaps[0].(map[string]any)["reason"] != "no_reader" || m["run_status"] != "partial" {
		t.Errorf("gaps = %v, status %v", gaps, m["run_status"])
	}
	if _, err := BuildOption(context.Background(), OptionConfig{}, nil, buildDay, buildAt); err == nil {
		t.Error("nil zone gives no error")
	}
	boom := errors.New("boom")
	c := OptionConfig{Zone: testZone, Candidates: &fakeCands{err: boom}}
	if _, err := BuildOption(context.Background(), c, nil, buildDay, buildAt); !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}

func optStaleOf(m map[string]any) map[string]any { return m["stale"].(map[string]any) }

func TestStaleLabelFreshDay(t *testing.T) {
	t.Parallel()
	m := decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: fixtureRows()}}, nil, buildDay))
	s := optStaleOf(m)
	if s["data_time"] != "2026-11-10T08:40:00+08:00" || s["stale_after"] != "2026-11-10T23:59:59+08:00" || s["text"] != "Data day 2026-11-10" {
		t.Errorf("stale = %v", s)
	}
	if len(m["gaps"].([]any)) != 0 || m["run_status"] != "ok" {
		t.Errorf("gaps %v status %v", m["gaps"], m["run_status"])
	}
}

func TestStaleLabelStaleDay(t *testing.T) {
	t.Parallel()
	row := okRow(1, "AAA", 1, 1, 1)
	row.SessionDay = "2026-11-08"
	m := decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{"2026-11-08": {row}}}, nil, buildDay))
	s := optStaleOf(m)
	if s["data_time"] != "2026-11-10T08:40:00+08:00" || s["stale_after"] != "2026-11-08T23:59:59+08:00" || s["text"] != "Data day 2026-11-08, before today" {
		t.Errorf("stale = %v", s)
	}
	gaps := m["gaps"].([]any)
	if len(gaps) != 1 || gaps[0].(map[string]any)["reason"] != "stale" || m["run_status"] != "partial" {
		t.Errorf("gaps = %v", gaps)
	}
}

func TestStaleLabelNoRows(t *testing.T) {
	t.Parallel()
	m := decodeMap(t, buildOpt(t, &fakeCands{}, nil, buildDay))
	s := optStaleOf(m)
	if s["data_time"] != "2026-11-10T00:00:00+08:00" || s["stale_after"] != s["data_time"] ||
		s["text"] != "No candidate rows: the candidate job has not stored any yet" {
		t.Errorf("stale = %v", s)
	}
	gaps := m["gaps"].([]any)
	if len(gaps) != 1 || gaps[0].(map[string]any)["reason"] != "no_data" || gaps[0].(map[string]any)["text"] != s["text"] {
		t.Errorf("gaps = %v", gaps)
	}
	if len(m["data_times"].([]any)) != 0 {
		t.Errorf("data_times = %v", m["data_times"])
	}
}

func TestOptionTableMissingBidIsGap(t *testing.T) {
	t.Parallel()
	one := candRow(1, "NOB", 1, "ok")
	one.BidEod = ""
	one.Ask = nil
	one.AnnualizedYield = sPtr("0")
	m := decodeMap(t, buildOpt(t, &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: {one}}}, nil, buildDay))
	row := tableRowsOf(m)[0]
	if c := cellOf(t, row, "annualised"); c["reason"] != "missing_input" || c["text"] != "missing bid" {
		t.Errorf("annualised = %v", c)
	}
	if c := cellOf(t, row, "bid_ask"); c["text"] != "n/a / n/a" {
		t.Errorf("bid_ask = %v", c)
	}
}
