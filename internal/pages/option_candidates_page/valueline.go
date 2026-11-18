package option_candidates_page

import (
	"context"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// valueLineView is the valuation block of a company strip: B_hi beside the buy price the gate uses.
type valueLineView struct {
	Label     string  `json:"label"`
	Text      string  `json:"text"`
	Hint      *string `json:"hint"`
	Reason    *string `json:"reason"`
	GateLabel string  `json:"gate_label"`
	GateText  string  `json:"gate_text"`
	GateHint  *string `json:"gate_hint"`
}

const (
	valueLabel     = "Valuation buy line B_hi"
	gateLabel      = "Buy price used by the gate (hand-typed)"
	valueReviewTag = " (review "
	valueUSD       = "USD"
)

// The reasons a strip shows no figure.
const (
	reasonNoProfile  = "no_profile"
	reasonAmbiguous  = "ambiguous"
	reasonRemoved    = "removed"
	reasonNoReview   = "no_review"
	reasonNoTables   = "no_tables"
	reasonCurrency   = "currency"
	reasonBadInput   = "bad_input"
	reasonReadFailed = "read_failed"
)

// valueReasonText is the Chinese sentence of a reason.
var valueReasonText = map[string]string{
	reasonNoProfile:  valueReasonNoProfileZh,
	reasonAmbiguous:  valueReasonAmbiguousZh,
	reasonRemoved:    valueReasonRemovedZh,
	reasonNoReview:   valueReasonNoReviewZh,
	reasonNoTables:   valueReasonNoTablesZh,
	reasonCurrency:   valueReasonCurrencyZh,
	reasonBadInput:   valueReasonBadInputZh,
	reasonReadFailed: valueReasonReadFailedZh,
}

// fillValuation writes the valuation field of each company from Config.Valuation and Config.ValueLines. A nil
// reader or calculator leaves every field nil; a Current error gives read_failed on every company and is not
// returned. The figure is Lines.BuyHigh as the calculator gives it; nothing is added, compared or rounded.
func fillValuation(ctx context.Context, c Config, cs []companyView) {
	if c.Valuation == nil || c.ValueLines == nil || len(cs) == 0 {
		return
	}
	st, err := c.Valuation.Current(ctx)
	for i := range cs {
		if err != nil {
			cs[i].Valuation = valueLineOf(&cs[i], "", reasonReadFailed)
			continue
		}
		text, reason := valueFigure(c.ValueLines, st, cs, i)
		cs[i].Valuation = valueLineOf(&cs[i], text, reason)
	}
}

// valueLineOf is the block of one strip: the figure text, or the sentence of the reason.
func valueLineOf(cv *companyView, text, reason string) *valueLineView {
	hint, gateHint := valueHintZh, gateHintZh
	v := &valueLineView{Label: valueLabel, Text: text, Hint: &hint, GateLabel: gateLabel, GateText: notSet, GateHint: &gateHint}
	if cv.BuyPrice != nil {
		v.GateText = *cv.BuyPrice
	}
	if reason != "" {
		r := reason
		v.Reason, v.Text = &r, valueReasonText[reason]
	}
	return v
}

// valueFigure is the figure text of company i, or the one reason it has none, tested in the order of Decision 4.
func valueFigure(calc valuelinesapi.Calculator, st valuation.State, cs []companyView, i int) (text, reason string) {
	ticker := cs[i].Ticker
	e := entryOf(st, ticker)
	if e == nil {
		return "", reasonNoProfile
	}
	for j := range cs {
		if j != i && strings.EqualFold(cs[j].Ticker, ticker) {
			return "", reasonAmbiguous
		}
	}
	switch {
	case e.Profile.Pool == "removed":
		return "", reasonRemoved
	case e.Current == nil:
		return "", reasonNoReview
	case st.RVersion == nil || st.Margin == nil:
		return "", reasonNoTables
	case e.Profile.Currency != valueUSD:
		return "", reasonCurrency
	}
	margin := make([]valuelinesapi.MarginRow, 0, len(st.Margin.Rows))
	for _, r := range st.Margin.Rows {
		margin = append(margin, valuelinesapi.MarginRow{PFrom: r.PFrom, M: r.M, W: r.W, H: r.H, S: r.S})
	}
	l, err := calc.Lines(valuelinesapi.Input{
		V: e.Current.V, P: e.Current.P, Market: e.Profile.RMarket,
		R:      valuelinesapi.RValues{US: st.RVersion.RUS, JP: st.RVersion.RJP, HKCN: st.RVersion.RHKCN},
		Margin: margin,
	})
	if err != nil {
		return "", reasonBadInput
	}
	m, ok := moneyText(l.BuyHigh)
	if !ok {
		return "", reasonBadInput
	}
	return m + valueReviewTag + string(e.Current.VersionDate) + ")", ""
}

// entryOf is the first entry whose symbol equals ticker ignoring case; nil when none or the ticker is empty.
func entryOf(st valuation.State, ticker string) *valuation.Entry {
	if ticker == "" {
		return nil
	}
	for i := range st.Entries {
		if strings.EqualFold(st.Entries[i].Profile.Symbol, ticker) {
			return &st.Entries[i]
		}
	}
	return nil
}

// hasFigure is true when the strip shows a valuation figure, not a reason.
func hasFigure(cv companyView) bool { return cv.Valuation != nil && cv.Valuation.Reason == nil }
