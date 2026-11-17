package valuelines

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

var dHalf = decimal.RequireFromString("0.5")

// Check lists the failed checks of a review that is to become effective: type, version date,
// V and P present; V above 0; P from 1 to 5 in steps of 0.5; and, for the margin row of P,
// the margin above the fair half width. Optional fields are not looked at.
func (Calc) Check(in valuelinesapi.CheckInput) []valuelinesapi.Problem {
	var out []valuelinesapi.Problem
	add := func(field, text string) { out = append(out, valuelinesapi.Problem{Field: field, Text: text}) }
	for _, f := range []struct{ field, label, val string }{
		{"type", "review type", in.Type}, {"version_date", "version date", in.VersionDate},
		{"v", "V", in.V}, {"p", "P", in.P},
	} {
		if strings.TrimSpace(f.val) == "" {
			add(f.field, f.label+" is required")
		}
	}
	if strings.TrimSpace(in.V) != "" {
		if v, err := decimal.NewFromString(strings.TrimSpace(in.V)); err != nil || !v.IsPositive() {
			add("v", "V must be a number above 0")
		}
	}
	if strings.TrimSpace(in.P) != "" {
		p, err := decimal.NewFromString(strings.TrimSpace(in.P))
		switch {
		case err != nil || p.LessThan(dOne) || p.GreaterThan(dFive) || !p.Div(dHalf).IsInteger():
			add("p", "P must be from 1 to 5 in steps of 0.5")
		case len(in.Margin) > 0:
			row, ok, err := pickMargin(in.Margin, p)
			switch {
			case err != nil:
				add("margin", "margin table has a value that is not a number")
			case !ok:
				add("margin", fmt.Sprintf("margin table has no row for P %s", p))
			case !row.m.GreaterThan(row.w):
				add("margin", fmt.Sprintf("margin row from P %s: buy zone upper line is not below the fair zone lower line (margin %s is not above fair half width %s)", row.pFrom, row.m, row.w))
			}
		}
	}
	return out
}
