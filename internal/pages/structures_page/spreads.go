package structures_page

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/spreadlegsapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Design: docs/design/spread-legs.md#3-legs-3-page.

const (
	spreadsScopeUnknown = "unknown" // the scope of the gap page of an alias that is not in Aliases
	maxBreakEvens       = 8
	kindCoveredCallID   = "covered_call"
	kindOtherID         = "other"
)

type kindText struct{ name, gloss string }

var spreadKinds = map[string]kindText{
	"put_spread": {kindPutSpread, glossPutSpread}, "call_spread": {kindCallSpread, glossCallSpread},
	"put_debit_spread": {kindPutDebitSpread, glossPutDebitSpread}, "call_debit_spread": {kindCallDebitSpread, glossCallDebitSpread},
	"calendar_spread": {kindCalendarSpread, glossCalendarSpread}, "diagonal_spread": {kindDiagonalSpread, glossDiagonalSpread},
	"long_straddle": {kindLongStraddle, glossLongStraddle}, "short_straddle": {kindShortStraddle, glossShortStraddle},
	"long_strangle": {kindLongStrangle, glossLongStrangle}, "short_strangle": {kindShortStrangle, glossShortStrangle},
	"iron_condor": {kindIronCondor, glossIronCondor}, "reverse_iron_condor": {kindReverseIronCondor, glossReverseIronCondor},
	"collar": {kindCollar, glossCollar}, kindCoveredCallID: {kindCoveredCall, glossCoveredCall},
	kindOtherID: {kindOther, glossOther},
}

var spreadSides = map[string]string{
	"-P": sideShortPut, "+P": sideLongPut, "-C": sideShortCall, "+C": sideLongCall,
}

var spreadGapTexts = map[string]string{
	"missing_input": gapTextMissingInput, "unsupported_currency": gapTextUnsupported, "no_rule": gapTextNoRule,
}

type spreadCell struct {
	Kind   string      `json:"kind"`
	Text   string      `json:"text"`
	Value  json.Number `json:"value,omitempty"`
	Unit   string      `json:"unit,omitempty"`
	Reason string      `json:"reason,omitempty"`
}

type spreadBreak struct {
	Kind   string        `json:"kind"`
	Text   string        `json:"text"`
	Values []json.Number `json:"values"`
	Reason string        `json:"reason,omitempty"`
}

type spreadLeg struct {
	SideText   string `json:"side_text"`
	Strike     string `json:"strike"`
	ExpiryText string `json:"expiry_text"`
	Quantity   string `json:"quantity"`
	OpenPrice  string `json:"open_price"`
}

type spreadBlock struct {
	Symbol     string      `json:"symbol"`
	Href       string      `json:"href"`
	KindText   string      `json:"kind_text"`
	KindGloss  string      `json:"kind_gloss"`
	ExpiryText string      `json:"expiry_text"`
	Legs       []spreadLeg `json:"legs"`
	NetCredit  spreadCell  `json:"net_credit"`
	MaxLoss    spreadCell  `json:"max_loss"`
	MaxGain    spreadCell  `json:"max_gain"`
	BreakEven  spreadBreak `json:"break_even"`
	Reason     *string     `json:"reason"`
}

type spreadGroup struct {
	Title      string        `json:"title"`
	Structures []spreadBlock `json:"structures"`
}

type spreadsModel struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	BuildID       string        `json:"build_id"`
	Scope         string        `json:"scope"`
	GeneratedAt   string        `json:"generated_at"`
	DataTimes     []sourceTime  `json:"data_times"`
	RunStatus     string        `json:"run_status"`
	Stale         spreadStale   `json:"stale"`
	Title         string        `json:"title"`
	Sentence      string        `json:"sentence"`
	Note          *string       `json:"note"`
	Groups        []spreadGroup `json:"groups"`
}

type spreadStale struct {
	DataTime string `json:"data_time"`
	Text     string `json:"text"`
	IsStale  bool   `json:"is_stale"`
}

// buildSpreads writes the spread_legs_page of alias for day. It computes nothing: it loads through Config.load,
// asks the figure for the net credit, break-evens, day count and order, and copies text.
func buildSpreads(ctx context.Context, c Config, alias string, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, fmt.Errorf("structures_page: nil zone")
	}
	if alias == "" && len(c.Aliases) > 0 {
		alias = c.Aliases[0] // the main alias is the first
	}
	dayAt, err := midnight(day, c.Zone)
	if err != nil {
		return nil, fmt.Errorf("structures_page: bad day %q: %w", day, err)
	}
	vm := &spreadsModel{
		Schema: "spread_legs_page", SchemaVersion: 1, RunID: c.RunID, BuildID: c.BuildID, Scope: alias,
		GeneratedAt: at.In(c.Zone).Format(time.RFC3339), DataTimes: []sourceTime{}, RunStatus: "ok",
		Stale: spreadStale{DataTime: dayAt, Text: spreadsStaleHead + string(day)},
		Title: spreadsTitle,
		Groups: []spreadGroup{{Title: spreadsGroupSpreads, Structures: []spreadBlock{}},
			{Title: spreadsGroupOther, Structures: []spreadBlock{}}},
	}
	switch {
	case !slices.Contains(c.Aliases, alias):
		vm.Scope = spreadsScopeUnknown
		vm.gapPage("failed", spreadsUnknownAcct)
		return json.Marshal(vm)
	case c.Spreads == nil || c.Ledger == nil || c.Sessions == nil:
		vm.gapPage("failed", spreadsNoReader)
		return json.Marshal(vm)
	}
	in, _, err := c.load(ctx, alias, day)
	if err != nil {
		return nil, err
	}
	res := c.Structures().Evaluate(in, compute.StructureParams{MarkMaxAgeSessions: markMaxAgeSessions,
		CaptureTarget: captureTarget, ExitDTE: exitDTE, CommissionDays: commissionDays})
	spreads, singles := splitSpreads(res.Structures)
	figs := c.Spreads().Spreads(spreadlegsapi.SpreadIn{
		Structures: append(slices.Clone(spreads), res.Other...), AsOf: day})
	vm.DataTimes = append(vm.DataTimes, sourceTime{Source: "ledger_replay", DataTime: dayAt})
	gaps := vm.fill(figs, spreads, res.Other)
	switch {
	case len(spreads)+singles == 0 && len(res.Other) == 0:
		vm.Sentence = clip(spreadsEmptyHead + alias + spreadsEmptyTail)
	default:
		vm.Sentence = spreadsSentenceHead + strconv.Itoa(len(vm.Groups[0].Structures)) + spreadsSentenceMid +
			strconv.Itoa(singles) + spreadsSentenceTail
	}
	note := clip(spreadsNote + spreadsLegHeads)
	vm.Note = &note
	if gaps > 0 || len(res.Other) > 0 {
		vm.RunStatus = "partial"
	}
	return json.Marshal(vm)
}

// gapPage makes the page of a page that cannot be shown: its sentence is the reason and the groups are empty.
func (vm *spreadsModel) gapPage(status, sentence string) {
	vm.RunStatus, vm.Sentence = status, sentence
	note := sentence
	vm.Note = &note
}

// splitSpreads keeps every structure of two or more legs and every covered call, in the order given, and counts
// the single-leg structures it leaves out.
func splitSpreads(all []compute.Structure) (kept []compute.Structure, singles int) {
	for _, s := range all {
		if len(s.Legs) >= 2 || s.Kind == kindCoveredCallID {
			kept = append(kept, s)
		} else {
			singles++
		}
	}
	return kept, singles
}

// fill puts one block per figure row in the group of its structure, in the order of the rows, and counts the
// gap cells that are not the by-design no_rule of the figure.
func (vm *spreadsModel) fill(figs spreadlegsapi.SpreadResult, spreads, other []compute.Structure) int {
	byID := map[string]compute.Structure{}
	inOther := map[string]bool{}
	for _, s := range append(slices.Clone(spreads), other...) {
		byID[s.ID] = s
	}
	for _, s := range other {
		inOther[s.ID] = true
	}
	gaps := 0
	for _, f := range figs.Rows {
		s, ok := byID[f.ID]
		if !ok {
			continue
		}
		if inOther[f.ID] {
			vm.Groups[1].Structures = append(vm.Groups[1].Structures, otherBlock(s, f))
			continue
		}
		b := spreadBlockOf(s, f)
		for _, r := range []string{b.NetCredit.Reason, b.MaxLoss.Reason, b.MaxGain.Reason, b.BreakEven.Reason} {
			if r != "" && r != reasonNoRule {
				gaps++
			}
		}
		vm.Groups[0].Structures = append(vm.Groups[0].Structures, b)
	}
	return gaps
}

func spreadKindOf(kind string) kindText {
	if k, ok := spreadKinds[kind]; ok {
		return k
	}
	return kindText{kindUnknown, glossUnknown}
}

func spreadHead(s compute.Structure, f spreadlegsapi.SpreadFigures) spreadBlock {
	k := spreadKindOf(s.Kind)
	symbol := s.Underlying
	if symbol == "" {
		symbol = "-"
	}
	expiry := string(f.Expiry)
	if expiry == "" {
		expiry = "-"
	}
	legs := make([]spreadLeg, 0, len(s.Legs))
	for _, l := range s.Legs[:min(len(s.Legs), maxLegs)] {
		legs = append(legs, spreadLegOf(l))
	}
	return spreadBlock{
		Symbol: symbol, Href: hrefOf(s.ID), KindText: k.name, KindGloss: k.gloss, Legs: legs,
		ExpiryText: spreadsExpiryHead + expiry + spreadsDaysOpen + strconv.Itoa(f.DaysToExpiry) + spreadsDaysClose,
	}
}

// spreadBlockOf is the block of a structure the matcher named: the net credit and break-evens of the figure,
// max loss and max gain of the structure.
func spreadBlockOf(s compute.Structure, f spreadlegsapi.SpreadFigures) spreadBlock {
	b := spreadHead(s, f)
	b.NetCredit = moneyCell(f.NetCredit, spreadsCreditUnit)
	b.MaxLoss = moneyCell(s.Figures.MaxLoss, "")
	b.MaxGain = moneyCell(s.Figures.MaxGain, "")
	b.BreakEven = breakCell(f)
	if s.Kind == kindCoveredCallID { // the stock leg is not on this page: both figures are gaps
		if s.Figures.MaxLoss.Gap != "unsupported_currency" {
			b.MaxLoss = gapCell(reasonNoRule, gapTextCallMaxLoss)
		}
		if f.BreakEvenGap == "" || f.BreakEvenGap == reasonNoRule {
			b.BreakEven = spreadBreak{Kind: "gap", Text: gapTextCallBreak, Values: []json.Number{}, Reason: reasonNoRule}
		}
	}
	return b
}

// otherBlock is the block of a unit the matcher could not name: its legs, the reason line and no figure.
func otherBlock(s compute.Structure, f spreadlegsapi.SpreadFigures) spreadBlock {
	b := spreadHead(s, f)
	g := gapCell(reasonNoRule, gapTextNoRule)
	b.NetCredit, b.MaxLoss, b.MaxGain = g, g, g
	b.BreakEven = spreadBreak{Kind: "gap", Text: gapTextNoRule, Values: []json.Number{}, Reason: reasonNoRule}
	line := spreadsUnnamedLine
	if r := otherReason(s.Legs); r != "" {
		line += spreadsReasonOpen + r + spreadsReasonClose
	}
	b.Reason = &line
	return b
}

func gapCell(reason, text string) spreadCell {
	return spreadCell{Kind: "gap", Text: text, Reason: reason}
}

// reasonCell is the gap cell of a figure reason, with its words.
func reasonCell(reason string) spreadCell {
	t, ok := spreadGapTexts[reason]
	if !ok {
		reason, t = reasonNoData, gapTextNoData
	}
	return gapCell(reason, t)
}

// moneyCell is the cell of a figure with a signed text ("+" for a gain, "-" for a loss); suffix follows the text.
func moneyCell(f compute.FigureValue, suffix string) spreadCell {
	if f.Gap != "" || f.Value == "" {
		return reasonCell(f.Gap)
	}
	lit, ok := numberLiteral(f.Value)
	d, err := decimal.NewFromString(f.Value)
	if !ok || err != nil {
		return reasonCell(reasonMissing)
	}
	return spreadCell{Kind: "value", Text: signedMoney(d) + suffix, Value: lit, Unit: "usd"}
}

func breakCell(f spreadlegsapi.SpreadFigures) spreadBreak {
	gapOf := func(reason string) spreadBreak {
		c := reasonCell(reason)
		return spreadBreak{Kind: "gap", Text: c.Text, Values: []json.Number{}, Reason: c.Reason}
	}
	if f.BreakEvenGap != "" || len(f.BreakEvens) == 0 {
		return gapOf(f.BreakEvenGap)
	}
	vals := make([]json.Number, 0, len(f.BreakEvens))
	texts := make([]string, 0, len(f.BreakEvens))
	for _, v := range f.BreakEvens[:min(len(f.BreakEvens), maxBreakEvens)] {
		lit, ok := numberLiteral(v)
		d, err := decimal.NewFromString(v)
		if !ok || err != nil {
			return gapOf(reasonMissing)
		}
		vals, texts = append(vals, lit), append(texts, twoPlaces(d))
	}
	return spreadBreak{Kind: "values", Text: strings.Join(texts, spreadsBreakJoin), Values: vals}
}

func spreadLegOf(l compute.StructureLeg) spreadLeg {
	out := spreadLeg{SideText: sideOther, Strike: dashed(l.Strike), ExpiryText: dashed(l.Expiry),
		Quantity: dashed(l.Quantity), OpenPrice: spreadsNoOpenPrice}
	if d, err := decimal.NewFromString(l.Strike); err == nil {
		out.Strike = twoPlaces(d)
	}
	if d, err := decimal.NewFromString(l.OpenPrice); err == nil && l.OpenPrice != "" {
		out.OpenPrice = twoPlaces(d)
	}
	sign := ""
	if d, err := decimal.NewFromString(l.Quantity); err == nil {
		out.Quantity = d.String()
		switch {
		case d.IsNegative():
			sign = "-"
		case d.IsPositive():
			sign = "+"
			out.Quantity = "+" + out.Quantity
		}
	}
	if t, ok := spreadSides[sign+l.Right]; ok {
		out.SideText = t
	}
	return out
}

func dashed(s string) string {
	if s == "" {
		return "-"
	}
	return clip(s)
}

// twoPlaces writes d with at least two decimal places and no digit dropped.
func twoPlaces(d decimal.Decimal) string {
	s := d.String()
	whole, frac, found := strings.Cut(s, ".")
	if !found {
		return whole + ".00"
	}
	return whole + "." + frac + strings.Repeat("0", max(0, 2-len(frac)))
}

// signedMoney is d to two places half-even with thousands separators and a sign: "+" for a gain, "-" for a
// loss, none for zero.
func signedMoney(d decimal.Decimal) string {
	r := d.RoundBank(2)
	whole, frac, _ := strings.Cut(r.Abs().StringFixed(2), ".")
	var b strings.Builder
	for i, ch := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	sign := ""
	switch {
	case r.IsNegative():
		sign = "-"
	case r.IsPositive():
		sign = "+"
	}
	return sign + b.String() + "." + frac
}
