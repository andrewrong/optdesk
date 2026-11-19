package option_candidates_page

import (
	"context"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
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
	gateLabel      = "Buy price used by the gate"
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

// fillValuation writes the valuation field of each company from Config.Valuation and Config.Buy. A nil reader
// or resolver leaves every field nil; a Current error gives read_failed on every company and is not returned.
// The figure is the resolver's price rounded to the cent by moneyText; nothing else is added or compared.
func fillValuation(ctx context.Context, c Config, cs []companyView, cur string) {
	if c.Valuation == nil || c.Buy == nil || len(cs) == 0 {
		return
	}
	st, err := c.Valuation.Current(ctx)
	zones, tables := zonesOf(st)
	for i := range cs {
		if err != nil {
			cs[i].Valuation = valueLineOf(&cs[i], "", reasonReadFailed)
			continue
		}
		text, reason := valueFigure(c.Buy, zones, tables, cs, i, cur)
		cs[i].Valuation = valueLineOf(&cs[i], text, reason)
	}
}

// valueLineOf is the block of one strip: the figure text, or the sentence of the reason.
func valueLineOf(cv *companyView, text, reason string) *valueLineView {
	hint, gateHint := valueHintZh, gateHintZh
	v := &valueLineView{Label: valueLabel, Text: text, Hint: &hint, GateLabel: gateLabel, GateText: notJudged, GateHint: &gateHint}
	if cv.BuyPrice != nil {
		v.GateText = *cv.BuyPrice
	}
	if reason != "" {
		r := reason
		v.Reason, v.Text = &r, valueReasonText[reason]
	}
	return v
}

// valueFigure is the figure text of company i, or the one reason it has none. The zone rule is the resolver's;
// the page adds one test, the strip-level ambiguous (two strips share the ticker), unless the resolver found no
// profile, so the order of the reasons on the strip is as it was.
func valueFigure(buy buypriceapi.Resolver, zones []buypriceapi.Zone, tables buypriceapi.Tables, cs []companyView, i int, cur string) (text, reason string) {
	ticker := cs[i].Ticker
	res := buy.Resolve(buypriceapi.Input{Symbol: ticker, Currency: stripCurrency(cur), Zones: zones, Tables: tables})
	if res.ZoneReason == reasonNoProfile {
		return "", reasonNoProfile
	}
	for j := range cs {
		if j != i && ticker != "" && strings.EqualFold(cs[j].Ticker, ticker) {
			return "", reasonAmbiguous
		}
	}
	if res.ZoneReason != "" {
		return "", res.ZoneReason
	}
	m, ok := moneyText(res.Price, cur)
	if !ok {
		return "", reasonBadInput
	}
	return m + valueReviewTag + res.ReviewDay + ")", ""
}

// stripCurrency is the currency the resolver is asked for: the group currency, USD for the US groups.
func stripCurrency(cur string) string {
	if cur == "" {
		return valueUSD
	}
	return cur
}

// hasFigure is true when the strip shows a valuation figure, not a reason.
func hasFigure(cv companyView) bool { return cv.Valuation != nil && cv.Valuation.Reason == nil }
