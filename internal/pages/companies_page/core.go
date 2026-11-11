package companies_page

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// The display forms of the page are cut from the decimal texts of valuationapi; no float is
// used and no figure is computed beyond the share of a bar. Design:
// docs/design/valuation.md#Decisions (14).

// barMax is the scale table: the top of each bar in percent, the bottom being 0.
var (
	hundred = decimal.NewFromInt(100)

	treasuryBarMax  = decimal.NewFromInt(10)
	returnBarMax    = decimal.NewFromInt(20)
	diagnosisBarMax = decimal.NewFromInt(10)
)

const (
	treasurySeries   = "DGS10"
	treasuryDays     = 7
	leftOutPrefix    = "left out: "
	noIssuerText     = "no issuer id"
	jpRateText       = "No Japan 10-year yield source"
	coreBasisNote    = "Return at the bare strike of each shown put, four named cases."
	coreLowText      = "0%"
	coreHighText     = "10%"
	stateLeftOut     = "left_out"
	reasonNoReader   = "no_reader"
	reasonMissingIn  = "missing_input"
	reasonNoData     = "no_data"
	reasonStaleGap   = "stale"
	noAnalystDisplay = "no analyst input"
)

// num is a JSON number written from decimal text, so that 31.0 stays 31.0.
type num string

// MarshalJSON writes the text as a number.
func (n num) MarshalJSON() ([]byte, error) { return []byte(n), nil }

type treasury struct {
	Text          string `json:"text"`
	MarkerPercent num    `json:"marker_percent"`
}

type returnPoint struct {
	Assumption    string `json:"assumption"`
	Text          string `json:"text"`
	MarkerPercent num    `json:"marker_percent"`
}

type coreRow struct {
	Symbol    string  `json:"symbol"`
	PriceText string  `json:"price_text"`
	Cyclical  bool    `json:"cyclical"`
	Flag      *string `json:"flag"`
	Returns   any     `json:"returns"`
}

type coreBlock struct {
	BasisNote    string    `json:"basis_note"`
	LowText      string    `json:"low_text"`
	HighText     string    `json:"high_text"`
	Treasury     any       `json:"treasury"`
	JpRate       gap       `json:"jp_rate"`
	Rows         []coreRow `json:"rows"`
	Legend       []string  `json:"legend"`
	SnapshotText string    `json:"snapshot_text"`
}

// assumptions are the labels of the four line keys, in the order the lines are given.
var assumptions = map[string]string{
	valuationapi.LineSpendingStays:        "Spending stays",
	valuationapi.LineSpendingEarns:        "Spending earns",
	valuationapi.LineAnalystPath:          "Analyst path",
	valuationapi.LineSpendingEarnsAnalyst: "Spending earns, analyst path",
}

// subjectFigs holds what the compute functions gave for one subject; the three blocks read it.
type subjectFigs struct {
	sub     Subject
	fr      valuationapi.FilingsResult
	refused string // why no filings were read, empty when they were
	diag    valuationapi.DiagnosisResult
}

// hasYears reports whether filings exist to run the figures on.
func (s subjectFigs) hasYears() bool { return s.refused == "" && len(s.fr.Filings.Years) > 0 }

// gather reads the facts of every subject once and runs BuildFilings and, when set, Diagnosis.
func (c Config) gather(ctx context.Context, today contracts.Day) ([]subjectFigs, error) {
	out := make([]subjectFigs, 0, len(c.Subjects))
	for _, s := range c.Subjects {
		f := subjectFigs{sub: s}
		source, ok := factSource(s.Issuer)
		if !ok {
			f.refused = noIssuerText
			out = append(out, f)
			continue
		}
		facts, err := c.Facts.FactsFiledBy(ctx, source, s.Issuer, today)
		if err != nil {
			return nil, fmt.Errorf("companies_page: facts of %s: %w", s.Symbol, err)
		}
		f.fr = c.BuildFilings(facts, today, c.StandardOf(facts))
		if c.Diagnosis != nil && len(f.fr.Filings.Years) > 0 {
			f.diag = c.Diagnosis(f.fr.Filings, today)
		}
		out = append(out, f)
	}
	return out, nil
}

func blockGap(reason, text string) gap { return gap{Reason: reason, Text: text} }

// reasonText joins the reasons of a refusal, or says the figure is missing.
func reasonText(reasons ...string) string {
	var kept []string
	for _, r := range reasons {
		if r != "" {
			kept = append(kept, strings.ReplaceAll(r, "_", " "))
		}
	}
	if len(kept) == 0 {
		return "missing input"
	}
	return strings.Join(kept, ", ")
}

func parseDec(s string) (decimal.Decimal, bool) {
	d, err := decimal.NewFromString(s)
	return d, err == nil
}

// pct is the fraction as percent text with places decimals, half-even.
func pct(frac decimal.Decimal, places int32) string {
	return frac.Mul(hundred).RoundBank(places).StringFixed(places)
}

// pctTrim is the fraction as percent text without trailing zeros (0.10 is 10).
func pctTrim(frac decimal.Decimal) string { return frac.Mul(hundred).RoundBank(1).String() }

// marker is the share of a bar that runs from 0 to barMax percent, for a value in percent,
// cut to 0 to 100 and one decimal.
func marker(percent, barMax decimal.Decimal) num {
	v := percent.Mul(hundred).Div(barMax)
	if v.Sign() < 0 {
		v = decimal.Zero
	}
	if v.GreaterThan(hundred) {
		v = hundred
	}
	return num(v.RoundBank(1).StringFixed(1))
}

// coreBlockOf builds the core block, or the gap that says why it cannot be built.
func (c Config) coreBlockOf(ctx context.Context, figs []subjectFigs, today contracts.Day) (any, error) {
	if c.Offers == nil || c.Spot == nil {
		return blockGap(reasonNoReader, "core: offers or quote reader not set"), nil
	}
	if c.ReturnAtPrice == nil {
		return blockGap(reasonNoReader, "core: return function not set"), nil
	}
	tr, err := c.treasuryOf(ctx, figs, today)
	if err != nil {
		return nil, err
	}
	blk := coreBlock{
		BasisNote: coreBasisNote, LowText: coreLowText, HighText: coreHighText,
		Treasury: tr, JpRate: blockGap(reasonNoData, jpRateText),
		Rows: []coreRow{}, Legend: []string{}, SnapshotText: "Quotes of " + string(today)[5:],
	}
	for _, f := range figs {
		rows, legend, err := c.coreRowsOf(ctx, f, today)
		if err != nil {
			return nil, err
		}
		blk.Rows = append(blk.Rows, rows...)
		blk.Legend = append(blk.Legend, legend...)
	}
	return blk, nil
}

// coreRowsOf is the rows of one subject, one per shown offer (one row for all offers when
// no figures can be built), and its growth legend line.
func (c Config) coreRowsOf(ctx context.Context, f subjectFigs, today contracts.Day) ([]coreRow, []string, error) {
	costs, err := c.Offers(ctx, f.sub.Symbol, today)
	if err != nil {
		return nil, nil, fmt.Errorf("companies_page: offers of %s: %w", f.sub.Symbol, err)
	}
	if len(costs) == 0 {
		return nil, nil, nil
	}
	cyclical := f.diag.Group == "cyclical"
	if !f.hasYears() {
		// No figures: one row says the gap once, with every shown strike in its price text.
		row := coreRow{Symbol: f.sub.Symbol, PriceText: strings.Join(costs, ", "), Cyclical: cyclical}
		if f.refused != "" {
			row.Returns = blockGap(reasonMissingIn, f.refused)
		} else {
			row.Returns = blockGap(reasonMissingIn, reasonText(f.fr.Reasons...))
		}
		return []coreRow{row}, nil, nil
	}
	// The analyst input stays absent (Decision 10).
	rr := c.ReturnAtPrice(f.fr.Filings, costs, nil, today)
	rows := make([]coreRow, 0, len(costs))
	for _, cost := range costs {
		row := coreRow{Symbol: f.sub.Symbol, PriceText: cost, Cyclical: cyclical}
		row.Returns, row.Flag = returnsOf(rr, cost)
		rows = append(rows, row)
	}
	var legend []string
	if c.ImpliedGrowth != nil {
		if spot, ok, err := c.Spot(ctx, f.sub.Symbol, today); err != nil {
			return nil, nil, fmt.Errorf("companies_page: spot of %s: %w", f.sub.Symbol, err)
		} else if ok && spot != "" {
			if line := impliedLine(f.sub.Symbol, c.ImpliedGrowth(f.fr.Filings, spot)); line != "" {
				legend = append(legend, line)
			}
		}
	}
	return rows, legend, nil
}

// returnsOf is the four lines of one cost as points, or a gap when the result is a refusal.
func returnsOf(rr valuationapi.ReturnResult, cost string) (any, *string) {
	var flag *string
	if len(rr.Flags) > 0 {
		t := reasonText(rr.Flags...)
		flag = &t
	}
	var pts []returnPoint
	for _, l := range rr.Lines {
		if l.Cost == cost {
			pts = append(pts, pointOf(l))
		}
	}
	if len(pts) == 0 {
		return blockGap(reasonMissingIn, reasonText(rr.Reasons...)), nil
	}
	return pts, flag
}

func pointOf(l valuationapi.ReturnLine) returnPoint {
	label := assumptions[l.Key]
	if label == "" {
		label = l.Key
	}
	p := returnPoint{Assumption: label, MarkerPercent: "0.0"}
	switch l.State {
	case valuationapi.StateOK:
		if v, ok := parseDec(l.Value); ok {
			p.Text = pct(v, 1) + "% a year"
			p.MarkerPercent = marker(v.Mul(hundred), returnBarMax)
			return p
		}
		p.Text = "missing input"
	case stateLeftOut:
		reason := l.Reason
		if reason == "" {
			reason = noAnalystDisplay
		}
		p.Text = leftOutPrefix + reason
	default:
		p.Text = "missing input: " + reasonText(l.Reason)
	}
	return p
}

// impliedLine is the legend line of the growth the price needs, empty when not formed.
func impliedLine(symbol string, im valuationapi.ImpliedResult) string {
	if im.State != valuationapi.StateOK || len(im.Rates) == 0 {
		return ""
	}
	var parts []string
	for _, r := range im.Rates {
		rate, ok1 := parseDec(r.Rate)
		g, ok2 := parseDec(r.Growth)
		if ok1 && ok2 {
			parts = append(parts, pctTrim(rate)+"%: "+pct(g, 1)+"%")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "Growth the price needs at " + strings.Join(parts, "; at ") + " (" + symbol + ")."
}

// treasuryOf is the newest DGS10 point within 7 days of today, for a page with a US name;
// otherwise a gap that says why not.
func (c Config) treasuryOf(ctx context.Context, figs []subjectFigs, today contracts.Day) (any, error) {
	us := false
	for _, f := range figs {
		if f.sub.Market != "JP" {
			us = true
		}
	}
	if !us {
		return blockGap(reasonNoData, "No US name on the page"), nil
	}
	if c.Series == nil {
		return blockGap(reasonNoReader, "US 10-year yield reader not set"), nil
	}
	t, err := time.Parse(dayLayout, string(today))
	if err != nil {
		return nil, fmt.Errorf("companies_page: bad day %q: %w", today, err)
	}
	from := t.AddDate(0, 0, -treasuryDays)
	pts, err := c.Series.SeriesPoints(ctx, treasurySeries, contracts.Day(from.Format(dayLayout)), today)
	if err != nil {
		return nil, fmt.Errorf("companies_page: %s: %w", treasurySeries, err)
	}
	var best *contracts.SeriesPoint
	for i := range pts {
		p := pts[i]
		if p.Status != "present" || p.Value == nil || p.ObservationDate.Before(from) || p.ObservationDate.After(t) {
			continue
		}
		if best == nil || p.ObservationDate.After(best.ObservationDate.Time) {
			best = &pts[i]
		}
	}
	if best == nil {
		return blockGap(reasonStaleGap, "No US 10-year yield in the last 7 days"), nil
	}
	v, ok := parseDec(strconv.FormatFloat(*best.Value, 'f', -1, 64))
	if !ok {
		return blockGap(reasonNoData, "US 10-year yield is not a number"), nil
	}
	return treasury{Text: "US 10-year " + v.String() + "%", MarkerPercent: marker(v, treasuryBarMax)}, nil
}

// fyLabel is FY<year> from the fiscal year end.
func fyLabel(end string) string {
	if len(end) < 4 {
		return end
	}
	return "FY" + end[:4]
}

// grouped is a whole number with a comma every three digits.
func grouped(d decimal.Decimal) string {
	s := d.RoundBank(0).StringFixed(0)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// sortedEnds returns the fiscal year ends found under a key prefix, newest first.
func sortedEnds(figs map[string]valuationapi.DiagnosisFigure, name string) []string {
	var ends []string
	for k := range figs {
		if strings.HasPrefix(k, name+"@") {
			ends = append(ends, strings.TrimPrefix(k, name+"@"))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ends)))
	return ends
}
