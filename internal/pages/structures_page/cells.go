package structures_page

import (
	"encoding/json"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

// gap is the schema's Gap: a named reason in place of a figure.
type gap struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

// usd is the schema's Usd cell; Value is the stored decimal text as a JSON number.
type usd struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

const (
	reasonNoReader = "no_reader"
	reasonNoData   = "no_data"
	reasonNoRule   = "no_rule"
	reasonMissing  = "missing_input"
	reasonQuote    = "missing_quote"
)

var gapTexts = map[string]string{
	"missing_quote":        "No quote",
	"stale_input":          "Stale quote",
	"unsupported_currency": "Non-USD contract, not converted",
	"no_rule":              "No such figure for this structure",
	"no_data":              "No data",
	"missing_input":        "Missing input",
	"no_reader":            "Reader not connected",
}

func mkGap(reason string) gap {
	t, ok := gapTexts[reason]
	if !ok {
		reason, t = reasonNoData, gapTexts[reasonNoData]
	}
	return gap{Reason: reason, Text: t}
}

// cells turns the figures of one structure into cells. noQuotes is true when no quote reader is wired:
// a missing quote is then the gap no_reader.
type cells struct{ noQuotes bool }

// reasonOf is the gap reason of a figure, or "" when it has a value.
func (c cells) reasonOf(f compute.FigureValue) string {
	if f.Gap == "" {
		if f.Value == "" {
			return reasonNoData
		}
		return ""
	}
	if f.Gap == reasonQuote && c.noQuotes {
		return reasonNoReader
	}
	return f.Gap
}

// text copies the money text of a figure, or a gap.
func (c cells) text(f compute.FigureValue) any {
	if r := c.reasonOf(f); r != "" {
		return mkGap(r)
	}
	return f.Value
}

// money is the Usd cell of a figure; a text that is not a number is a gap.
func (c cells) money(f compute.FigureValue) any {
	if r := c.reasonOf(f); r != "" {
		return mkGap(r)
	}
	n, ok := numberLiteral(f.Value)
	if !ok {
		return mkGap(reasonMissing)
	}
	return usd{Value: json.Number(n), Text: f.Value, Unit: "usd"}
}

// percent is the figure's ratio text moved two places; bad text is a gap, never a panic.
func (c cells) percent(f compute.FigureValue) any {
	if r := c.reasonOf(f); r != "" {
		return mkGap(r)
	}
	p, err := percentText(f.Value)
	if err != nil {
		return mkGap(reasonMissing)
	}
	return p
}

// moneyness names where the first sold leg stands, from the sign of the stored distance text.
func (c cells) moneyness(s compute.Structure) any {
	f := s.Figures.ExpiryDistance
	if r := c.reasonOf(f); r != "" {
		return mkGap(r)
	}
	if len(s.Legs) == 0 {
		return mkGap(reasonNoData)
	}
	if _, err := percentText(f.Value); err != nil {
		return mkGap(reasonMissing)
	}
	if strings.Trim(f.Value, "-0.") == "" {
		return "at the money"
	}
	below := strings.HasPrefix(f.Value, "-") // the strike lies below the underlying
	if (s.Legs[0].Right == "P") == below {
		return "out of the money"
	}
	return "in the money"
}

// maxLossCell is max_loss; an unbounded naked call is shown as such.
func (c cells) maxLossCell(s compute.Structure) any {
	m := c.money(s.Figures.MaxLoss)
	if g, ok := m.(gap); ok && s.Kind == "naked_call" && g.Reason == reasonNoRule {
		return gap{Reason: reasonNoRule, Text: "No limit"}
	}
	return m
}

// compare is the compare row of a structure with exactly one sold leg and nothing else, else nil.
func (c cells) compare(s compute.Structure) *compare {
	if len(s.Legs) != 1 || s.ExitDTEParam == "" || s.CaptureParam == "" {
		return nil
	}
	captured, err := percentText(s.CaptureParam)
	if err != nil {
		captured = s.CaptureParam
	}
	return &compare{
		CloseCost: c.text(s.Figures.CloseCost), RollNet: c.text(s.Figures.RollNet),
		CashAfter: c.text(s.Figures.CashAfter), RecentCommission: c.text(s.Figures.RecentCommission),
		CapturedParam: "Earned ratio " + captured, ExitDTEParam: "Days before expiry " + s.ExitDTEParam,
	}
}

// compare is the schema's Compare; Sentence stays null, compute gives no sentence.
type compare struct {
	CloseCost        any     `json:"close_cost"`
	RollNet          any     `json:"roll_net"`
	CashAfter        any     `json:"cash_after"`
	RecentCommission any     `json:"recent_commission"`
	Sentence         *string `json:"sentence"`
	CapturedParam    string  `json:"captured_param"`
	ExitDTEParam     string  `json:"exit_dte_param"`
}
