package macro_page

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/liquidity-view.md, Decisions 2, 3, 7, 9 and 11.
//
// This file reads the fourteen series of liquidityapi.Input, calls the Func it is given once and writes the
// Result as four sentences and four taps. The only arithmetic is rounding for display.

const (
	liquidityTitle      = "Funding and credit background"
	liquidityBackground = "Background only: nothing here changes a put decision."
	// liquidityLookbackDays is how far before day the series are read; the high-yield series is read from
	// highYieldFloor so its percentile window is whatever is stored.
	liquidityLookbackDays = 400
	highYieldFloor        = contracts.Day("2000-01-01")
	highYieldID           = "BAMLH0A0HYM2"

	// With Liquidity set the sentence is the text of Decision 4 of the design.
	sentenceLiq   = "Funding and credit background from stored series; events: no data yet."
	sentenceLiqEv = "Funding and credit background from stored series."

	noteNetLiquidity = "Practitioner construct, no evidence it leads prices."
	noteStlfsi       = "Zero is the average since late 1993; above 0 = above-average stress."
	zeroLineNFCI     = "above 0 means tighter than average"
)

var liquidityIDs = []string{
	"WRESBAL", "RRPONTSYD", "WALCL", "WDTGAL", "TLAACBW027SBOG", "WLCFLPCL",
	"SOFR", "IORB", "SOFR99",
	highYieldID, "BAMLC0A0CM", "BAA10Y",
	"NFCI", "STLFSI4",
}

// gapReason maps a reason of a figure to its gap reason (Decision 11); an unknown reason is missing_input.
func gapReason(r compute.Reason) string {
	switch r {
	case compute.NoInput:
		return "no_data"
	case compute.StaleInput:
		return "stale_input"
	case liquidityapi.MissingInput, compute.ShortHistory, compute.ZeroRange, compute.BadInput:
		return "missing_input"
	default:
		return "missing_input"
	}
}

var gapWords = map[string]string{
	"no_data":       "no data yet",
	"missing_input": "an input is missing",
	"stale_input":   "the stored data is too old",
}

func gapOf(what string, r compute.Reason) gap {
	reason := gapReason(r)
	return gap{Reason: reason, Text: what + ": " + gapWords[reason]}
}

type liquidityLine struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	Sentence   any    `json:"sentence"`
	SourceText string `json:"source_text"`
}

type liquidityRow struct {
	Label   string  `json:"label"`
	Text    any     `json:"text"`
	DataDay *string `json:"data_day"`
}

type liquidityTap struct {
	Key   string         `json:"key"`
	Label string         `json:"label"`
	Rows  []liquidityRow `json:"rows"`
	Note  *string        `json:"note"`
}

type liquidityBlock struct {
	BackgroundText string          `json:"background_text"`
	Lines          []liquidityLine `json:"lines"`
	Taps           []liquidityTap  `json:"taps"`
}

// withLiquidity reads the series, calls c.Liquidity once and turns the page into its v2 form. An error of a
// read is an error, never a gap.
func withLiquidity(ctx context.Context, r store.SeriesReader, c Config, d time.Time, vm *viewModel) error {
	if c.Liquidity == nil {
		return nil
	}
	in, err := liquidityInput(ctx, r, c, d)
	if err != nil {
		return err
	}
	vm.SchemaVersion = 2
	vm.LiquidityTitle = liquidityTitle
	vm.Liquidity = liquidityFrom(c.Liquidity(in))
	vm.Sentence = sentenceLiq
	if c.Events != nil {
		vm.Sentence = sentenceLiqEv
	}
	return nil
}

func liquidityInput(ctx context.Context, r store.SeriesReader, c Config, d time.Time) (liquidityapi.Input, error) {
	day := contracts.Day(d.Format(dayLayout))
	from := contracts.Day(d.AddDate(0, 0, -liquidityLookbackDays).Format(dayLayout))
	got := make(map[string]compute.Series, len(liquidityIDs))
	for _, id := range liquidityIDs {
		f := from
		if id == highYieldID {
			f = highYieldFloor
		}
		rows, err := r.SeriesPoints(ctx, id, f, day)
		if err != nil {
			return liquidityapi.Input{}, fmt.Errorf("macro_page: read %s: %w", id, err)
		}
		got[id] = seriesOf(rows, d)
	}
	return liquidityapi.Input{
		Day:     day,
		WRESBAL: got["WRESBAL"], RRPONTSYD: got["RRPONTSYD"], WALCL: got["WALCL"], WDTGAL: got["WDTGAL"],
		TLAACBW027SBOG: got["TLAACBW027SBOG"], WLCFLPCL: got["WLCFLPCL"],
		SOFR: got["SOFR"], IORB: got["IORB"], SOFR99: got["SOFR99"],
		BAMLH0A0HYM2: got[highYieldID], BAMLC0A0CM: got["BAMLC0A0CM"], BAA10Y: got["BAA10Y"],
		NFCI: got["NFCI"], STLFSI4: got["STLFSI4"],
		Net:    c.Net,
		Params: c.LiquidityParams,
	}, nil
}

// seriesOf keeps, per observation date, the row of the greatest RealtimeStart not after d, drops rows with no
// value or a status other than present, and returns the days ascending; nil when none is left.
func seriesOf(rows []contracts.SeriesPoint, d time.Time) compute.Series {
	kept := map[string]contracts.SeriesPoint{}
	for _, p := range rows {
		if p.RealtimeStart.After(d) || (p.Status != "" && p.Status != "present") {
			continue
		}
		k := p.ObservationDate.Format(dayLayout)
		if cur, ok := kept[k]; !ok || p.RealtimeStart.After(cur.RealtimeStart.Time) {
			kept[k] = p
		}
	}
	days := make([]string, 0, len(kept))
	for k, p := range kept {
		if p.Value != nil {
			days = append(days, k)
		}
	}
	if len(days) == 0 {
		return nil
	}
	slices.Sort(days)
	out := make(compute.Series, 0, len(days))
	for _, k := range days {
		out = append(out, compute.Point{Day: contracts.Day(k), Value: *kept[k].Value})
	}
	return out
}

func liquidityFrom(res liquidityapi.Result) *liquidityBlock {
	return &liquidityBlock{
		BackgroundText: liquidityBackground,
		Lines: []liquidityLine{
			{"reserves", "Reserves", reservesLine(res), "FRED WRESBAL, RRPONTSYD"},
			{"money_market", "Money market", moneyLine(res), "FRED SOFR, IORB, SOFR99"},
			{"credit", "Credit", creditLine(res), "FRED BAMLH0A0HYM2, BAMLC0A0CM, BAA10Y"},
			{"conditions", "Conditions", conditionsLine(res), "FRED NFCI"},
		},
		Taps: []liquidityTap{
			tap("stlfsi", "St. Louis Fed Financial Stress Index", noteStlfsi,
				tapRow("STLFSI4", res.STLFSI4, "St. Louis Fed Financial Stress Index 4", func(v float64) string { return fixed(v, 2) })),
			tap("reserves_over_assets", "Reserves over bank assets", "",
				tapRow("Reserves over all commercial banks' assets", res.ReservesOverAssets, "Reserves over bank assets", pct)),
			tap("discount_window", "Discount window", "",
				tapRow("Primary credit", res.DiscountWindow, "Primary credit", money)),
			tap("net_liquidity", "Net liquidity", noteNetLiquidity, netRow(res.NetLiquidity)),
		},
	}
}

func tap(key, label, note string, rows ...liquidityRow) liquidityTap {
	t := liquidityTap{Key: key, Label: label, Rows: rows}
	if note != "" {
		t.Note = &note
	}
	return t
}

// tapRow is one tap row of a plain figure: its text and its own day, or the gap of its Reason.
func tapRow(label string, n liquidityapi.Num, what string, f func(float64) string) liquidityRow {
	if n.Reason != "" {
		return liquidityRow{Label: label, Text: gapOf(what, n.Reason)}
	}
	v, ok := parse(n.Value)
	if !ok {
		return liquidityRow{Label: label, Text: gapOf(what, liquidityapi.MissingInput)}
	}
	return liquidityRow{Label: label, Text: f(v), DataDay: dayPtr(n.Day)}
}

// netRow names the Wednesday and the RRP day it was taken with.
func netRow(n liquidityapi.Num) liquidityRow {
	const label = "Net liquidity"
	if n.Reason != "" {
		return liquidityRow{Label: label, Text: gapOf(label, n.Reason)}
	}
	v, ok := parse(n.Value)
	if !ok {
		return liquidityRow{Label: label, Text: gapOf(label, liquidityapi.MissingInput)}
	}
	text := money(v) + " on Wednesday " + dayText(n.Day)
	if n.Aux != "" {
		text += ", RRP of " + dayText(n.Aux)
	}
	return liquidityRow{Label: label, Text: text, DataDay: dayPtr(n.Day)}
}

// firstReason is the Reason of the first figure that has one, with the name of that figure.
type named struct {
	what string
	r    compute.Reason
}

func firstGap(ns ...named) *gap {
	for _, n := range ns {
		if n.r != "" {
			g := gapOf(n.what, n.r)
			return &g
		}
	}
	return nil
}

// lineOf is the gap of the first figure with a Reason, else the text build returns; a value that is not a
// number is a missing input.
func lineOf(what string, ns []named, vals []string, build func(v []float64) string) any {
	if g := firstGap(ns...); g != nil {
		return *g
	}
	fs := make([]float64, len(vals))
	for i, s := range vals {
		v, ok := parse(s)
		if !ok {
			return gapOf(what, liquidityapi.MissingInput)
		}
		fs[i] = v
	}
	return build(fs)
}

func reservesLine(r liquidityapi.Result) any {
	ns := []named{
		{"Reserves", r.Reserves.Reason}, {"Reserves 4-week change", r.Reserves4w.Reason},
		{"Reserves 13-week change", r.Reserves13w.Reason}, {"ON RRP", r.RRP.Reason},
	}
	vals := []string{r.Reserves.Value, r.Reserves4w.Value, r.Reserves13w.Value, r.RRP.Value}
	return lineOf("Reserves", ns, vals, func(v []float64) string {
		return "Bank reserves were " + money(v[0]) + " in the week to " + dayText(r.Reserves.Day) + ", " +
			change(v[1], weeks(r.Reserves4w.Weeks, 4)) + " and " + change(v[2], weeks(r.Reserves13w.Weeks, 13)) +
			"; ON RRP is " + money(v[3]) + " on " + dayText(r.RRP.Day) + "."
	})
}

func moneyLine(r liquidityapi.Result) any {
	ns := []named{
		{"SOFR", r.SOFR.Reason}, {"SOFR minus IORB", r.SofrIorbBP.Reason},
		{"IORB", r.IORB.Reason}, {"SOFR 99th percentile", r.SOFR99.Reason},
	}
	vals := []string{r.SOFR.Value, r.SofrIorbBP.Value, r.IORB.Value, r.SOFR99.Value}
	return lineOf("Money market", ns, vals, func(v []float64) string {
		return "SOFR was " + fixed(v[0], 2) + "% on " + dayText(r.SOFR.Day) + ", " + basis(v[1]) +
			" IORB " + fixed(v[2], 2) + "% on " + dayText(r.IORB.Day) +
			"; its 99th percentile was " + fixed(v[3], 2) + "% on " + dayText(r.SOFR99.Day) + "."
	})
}

func creditLine(r liquidityapi.Result) any {
	ns := []named{
		{"High-yield spread", r.HighYield.Reason}, {"High-yield percentile", r.HighYieldRank.Reason},
		{"Investment grade spread", r.InvestmentGrade.Reason}, {"Baa minus 10-year", r.Baa10Y.Reason},
	}
	vals := []string{r.HighYield.Value, r.HighYieldRank.Value, r.InvestmentGrade.Value, r.Baa10Y.Value}
	return lineOf("Credit", ns, vals, func(v []float64) string {
		return "High-yield spread is " + fixed(v[0], 2) + "% on " + dayText(r.HighYield.Day) +
			", at the " + ordinal(int(math.Round(v[1]))) + " percentile of our stored history since " +
			string(r.HighYieldRank.First) + " (" + strconv.Itoa(r.HighYieldRank.Days) + " days)" +
			"; investment grade " + fixed(v[2], 2) + "% on " + dayText(r.InvestmentGrade.Day) +
			"; Baa minus 10-year " + fixed(v[3], 2) + "% on " + dayText(r.Baa10Y.Day) + "."
	})
}

func conditionsLine(r liquidityapi.Result) any {
	return lineOf("Chicago Fed NFCI", []named{{"Chicago Fed NFCI", r.NFCI.Reason}}, []string{r.NFCI.Value},
		func(v []float64) string {
			return "Chicago Fed NFCI is " + fixed(v[0], 2) + " for the week to " + dayText(r.NFCI.Day) +
				"; " + zeroLineNFCI + "."
		})
}

func weeks(n, def int) int {
	if n > 0 {
		return n
	}
	return def
}

// change is "up $48bn over 4 weeks", "down $112bn over 13 weeks" or "unchanged over 4 weeks".
func change(v float64, weeks int) string {
	span := " over " + strconv.Itoa(weeks) + " weeks"
	bn := math.Round(v / 1e9)
	switch {
	case bn > 0:
		return "up $" + strconv.FormatFloat(bn, 'f', 0, 64) + "bn" + span
	case bn < 0:
		return "down $" + strconv.FormatFloat(-bn, 'f', 0, 64) + "bn" + span
	}
	return "unchanged" + span
}

// basis is "2.0 bp above", "1.5 bp below" or "level with".
func basis(v float64) string {
	s := fixed(math.Abs(v), 1)
	switch {
	case s == "0.0":
		return "level with"
	case v > 0:
		return s + " bp above"
	}
	return s + " bp below"
}

// money is whole USD as $x.xxtn from one trillion, else $x.xbn under ten billion and $xbn above.
func money(v float64) string {
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	switch {
	case v >= 1e12:
		return sign + "$" + fixed(v/1e12, 2) + "tn"
	case v >= 1e10:
		return sign + "$" + fixed(v/1e9, 0) + "bn"
	}
	return sign + "$" + fixed(v/1e9, 1) + "bn"
}

func pct(v float64) string { return fixed(v, 2) + "%" }

func fixed(v float64, places int) string { return strconv.FormatFloat(v, 'f', places, 64) }

func parse(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}

// dayText is "Nov 11" for a day text; a text that is not a day is printed as it is.
func dayText(d contracts.Day) string {
	t, err := time.Parse(dayLayout, string(d))
	if err != nil {
		return string(d)
	}
	return t.Format("Jan 2")
}

func dayPtr(d contracts.Day) *string {
	if d == "" {
		return nil
	}
	s := string(d)
	return &s
}
