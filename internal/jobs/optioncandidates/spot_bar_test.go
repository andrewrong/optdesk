package optioncandidates

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeBars is a bar.PriceBarReader over fixed rows of basis trades; it keeps the arguments of the last read.
type fakeBars struct {
	rows     []contracts.PriceBar
	err      error
	basis    string
	from, to contracts.Day
}

func (b *fakeBars) PriceBars(_ context.Context, _ int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	b.basis, b.from, b.to = basis, from, to
	return b.rows, b.err
}

func barOf(d, closeText string) contracts.PriceBar {
	return contracts.PriceBar{SessionDay: day(d), Close: closeText}
}

// thanksgivingEnv is the test env with the sessions around Thanksgiving 2026 (the 26th is no session).
type thanksgivingEnv struct{ *srcEnv }

func (thanksgivingEnv) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	var out []contracts.SessionDay
	for _, d := range []string{"2026-11-23", "2026-11-24", "2026-11-25", "2026-11-27"} {
		out = append(out, contracts.SessionDay{CalendarID: "XNYS", SessionDay: sd(d), IsSession: true})
	}
	return out, nil
}

type afterThanksgiving struct{}

func (afterThanksgiving) Now() time.Time { return time.Date(2026, 11, 27, 21, 30, 0, 0, time.UTC) }

// runBarCase runs the job on 2026-11-27 on an eod chain of 2026-11-25 whose chain price is 363.83, with the given readers, and
// returns the stored spot and the spot marks.
func runBarCase(t *testing.T, quotes *fakeQuotes, bars *fakeBars) (spot, marks string) {
	t.Helper()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	e.catalog[7] = "2026-11-25"
	e.put(7, 11, "90")
	e.terms[11] = store.OptionTerms{InstrumentID: 11, Right: "P", Strike: "90", Multiplier: "100", ExpirationDate: "2027-01-08"}
	row := eodRowOf(7, 11, "2026-11-25", 1.7, 1.8, 40)
	row.UnderlyingPrice = new(363.83)
	e.eod = []contracts.OptionEod{row}
	st := e.stores(true)
	st.SessionDays = thanksgivingEnv{e}
	st.Quotes = quotes
	if bars != nil {
		st.Bars = bars
	}
	j, err := New(st, e, []jobs.WatchEntry{entryFor("ABC", "cik:1", "91.5")}, afterThanksgiving{})
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), "2026-11-27", nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	r := e.rowOf(t, 11)
	if r.Spot == nil {
		t.Fatal("no spot")
	}
	return *r.Spot, spotMarksOf(t, r.Marks)
}

func TestSpotFromBarWhenNoQuote(t *testing.T) {
	t.Parallel()
	bars := &fakeBars{rows: []contracts.PriceBar{barOf("2026-11-24", "375"), barOf("2026-11-25", "380.14")}}
	spot, marks := runBarCase(t, &fakeQuotes{}, bars)
	if spot != "380.14" || marks != "chain_day:2026-11-25 spot_quote:2026-11-25" {
		t.Fatalf("spot %s marks %s", spot, marks)
	}
	// the read is of basis trades, from the session before the chain day up to the run day
	if bars.basis != "trades" || bars.from != "2026-11-24" || bars.to != "2026-11-27" {
		t.Fatalf("read %s %s %s", bars.basis, bars.from, bars.to)
	}
}

func TestSpotQuoteBeatsBar(t *testing.T) {
	t.Parallel()
	q := &fakeQuotes{rows: []contracts.QuoteSnapshot{snap("2026-11-25", "2026-11-25T20:30:00Z", new(381.5), 1)}}
	spot, marks := runBarCase(t, q, &fakeBars{rows: []contracts.PriceBar{barOf("2026-11-25", "380.14")}})
	if spot != "381.5" || marks != "chain_day:2026-11-25 spot_quote:2026-11-25" {
		t.Fatalf("spot %s marks %s", spot, marks)
	}
}

func TestSpotNoBarKeepsChainPrice(t *testing.T) {
	t.Parallel()
	for name, bars := range map[string]*fakeBars{
		"no bar":     {},
		"read error": {err: errors.New("down")},
		"no reader":  nil,
	} {
		spot, marks := runBarCase(t, &fakeQuotes{}, bars)
		if spot != "363.83" || marks != "chain_day:2026-11-25 spot_chain" {
			t.Errorf("%s: spot %s marks %s", name, spot, marks)
		}
	}
}
