package valuation_params_page

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// Build returns the JSON of page.valuation_params_page.v1. The proposal is ProposeR of the newest R table and the
// latest yield, shown only when it differs from the newest R version (compared as displayed); its step warnings
// are RStepWarnings of the two. Missing input is a gap and never a failed page: no store, no yield, no R table
// (no proposal) and no margin table each give one gap.
func Build(in Input) ([]byte, error) {
	if !idPattern.MatchString(in.RunID) {
		return nil, fmt.Errorf("valuation_params_page: bad run_id %q", in.RunID)
	}
	if in.Calc == nil {
		return nil, fmt.Errorf("valuation_params_page: no calculator")
	}
	p := &pageJSON{
		Schema: "valuation_params_page", SchemaVersion: 1, RunID: in.RunID,
		BuiltAt:  in.Now.UTC().Format(time.RFC3339),
		RHistory: []rVersionJSON{}, Gaps: []gapJSON{}, Error: cut(in.Error),
	}
	if in.Form != nil {
		p.Form = &formJSON{PostPath: postPath, Expires: in.Form.Expires, MAC: in.Form.MAC}
	}
	if !in.StoreOK {
		p.gap("store_unavailable", "The valuation store is not readable here.")
	} else {
		fill(p, in)
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("valuation_params_page: %w", err)
	}
	return out, nil
}

func fill(p *pageJSON, in Input) {
	c := in.Calc
	for i, v := range in.Versions {
		var prev *valuation.RVersion
		if i+1 < len(in.Versions) {
			prev = &in.Versions[i+1]
		}
		p.RHistory = append(p.RHistory, versionDoc(c, v, prev))
	}
	if len(p.RHistory) > 0 {
		cur := p.RHistory[0]
		p.CurrentR = &cur
	}
	if in.Yield == nil {
		p.gap("yield_unavailable", "The latest 10-year yield (DGS10) is not stored.")
	} else {
		p.LatestYield = &yieldJSON{Value: c.RoundText(in.Yield.Value, placesR), Day: string(in.Yield.Day)}
	}
	if in.RTable == nil {
		p.gap("no_r_table", "No R table is stored; enter one to get a proposal.")
	} else {
		t := &rTableJSON{EffectiveDate: string(in.RTable.EffectiveDate), Rows: []rTableRowJSON{}}
		for _, r := range in.RTable.Rows {
			t.Rows = append(t.Rows, rTableRowJSON{YieldUpTo: r.YieldUpTo, US: r.US, JP: r.JP, HKCN: r.HKCN})
		}
		p.RTable = t
	}
	if in.Margin == nil {
		p.gap("no_margin_table", "No margin table is stored; no price line can be shown.")
	} else {
		t := &marginTableJSON{EffectiveDate: string(in.Margin.EffectiveDate), Rows: []marginRowJSON{}}
		for _, r := range in.Margin.Rows {
			t.Rows = append(t.Rows, marginRowJSON{PFrom: r.PFrom, M: r.M, W: r.W, H: r.H, S: r.S})
		}
		p.MarginTable = t
	}
	if in.RTable != nil && in.Yield != nil {
		p.Proposal = proposal(p, in)
	}
}

// proposal is nil when the R of the yield's bucket equals the newest R version.
func proposal(p *pageJSON, in Input) *proposalJSON {
	c := in.Calc
	rows := make([]valuelinesapi.RRow, 0, len(in.RTable.Rows))
	for _, r := range in.RTable.Rows {
		rows = append(rows, valuelinesapi.RRow{YieldUpTo: r.YieldUpTo, US: r.US, JP: r.JP, HKCN: r.HKCN})
	}
	next, err := c.ProposeR(rows, in.Yield.Value)
	if err != nil {
		p.gap("no_r_table", "No bucket of the R table fits the latest yield.")
		return nil
	}
	shown := rDoc(c, next)
	warnings := []string{}
	if len(in.Versions) > 0 {
		cur := in.Versions[0]
		if rDoc(c, valuelinesapi.RValues{US: cur.RUS, JP: cur.RJP, HKCN: cur.RHKCN}) == shown {
			return nil
		}
		warnings = nonNil(c.RStepWarnings(valuelinesapi.RValues{US: cur.RUS, JP: cur.RJP, HKCN: cur.RHKCN}, next))
	}
	return &proposalJSON{R: shown, StepWarnings: warnings}
}

func versionDoc(c valuelinesapi.Calculator, v valuation.RVersion, prev *valuation.RVersion) rVersionJSON {
	cur := valuelinesapi.RValues{US: v.RUS, JP: v.RJP, HKCN: v.RHKCN}
	warnings := []string{}
	if prev != nil {
		warnings = nonNil(c.RStepWarnings(valuelinesapi.RValues{US: prev.RUS, JP: prev.RJP, HKCN: prev.RHKCN}, cur))
	}
	yield := ""
	if v.YieldUsed != "" {
		yield = c.RoundText(v.YieldUsed, placesR)
	}
	return rVersionJSON{
		EffectiveDate: string(v.EffectiveDate), YieldUsed: yield, R: rDoc(c, cur),
		AISwitch: v.AISwitch, AIExpiry: cut(v.AIExpiry), Note: cut(v.Note), StepWarnings: warnings,
	}
}

func rDoc(c valuelinesapi.Calculator, r valuelinesapi.RValues) rValuesJSON {
	return rValuesJSON{US: c.RoundText(r.US, placesR), JP: c.RoundText(r.JP, placesR), HKCN: c.RoundText(r.HKCN, placesR)}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (p *pageJSON) gap(reason, text string) {
	for _, g := range p.Gaps {
		if g.Reason == reason {
			return
		}
	}
	p.Gaps = append(p.Gaps, gapJSON{Kind: "gap", Reason: reason, Text: text})
}

func cut(s string) string {
	if r := []rune(s); len(r) > maxText {
		return string(r[:maxText])
	}
	return s
}
