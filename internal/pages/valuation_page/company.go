package valuation_page

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/store/valuation"
)

type adjustmentJSON struct {
	Percent string `json:"percent"`
	Reason  string `json:"reason"`
}

type scenarioJSON struct {
	Label       string `json:"label"`
	Probability string `json:"probability"`
	EPS         string `json:"eps"`
	Multiple    string `json:"multiple"`
}

type reasonsJSON struct {
	P          string `json:"p"`
	Moat       string `json:"moat"`
	Understand string `json:"understand"`
	Management string `json:"management"`
	Financial  string `json:"financial"`
}

type detailJSON struct {
	Source          string           `json:"source"`
	FiscalPeriodEnd string           `json:"fiscal_period_end"`
	ReportDate      string           `json:"report_date"`
	EarningsBase    string           `json:"earnings_base"`
	BaseBasis       string           `json:"base_basis"`
	ForwardFactor   string           `json:"forward_factor"`
	LockedMultiple  string           `json:"locked_multiple"`
	MethodResults   []string         `json:"method_results"`
	Adjustments     []adjustmentJSON `json:"adjustments"`
	Scenarios       []scenarioJSON   `json:"scenarios"`
	Reaction        string           `json:"reaction"`
	Moat            string           `json:"moat"`
	Understand      string           `json:"understand"`
	Management      string           `json:"management"`
	Financial       string           `json:"financial"`
	Reasons         reasonsJSON      `json:"reasons"`
	ReportText      string           `json:"report_text"`
	EnteredAt       string           `json:"entered_at"`
	EffectiveAt     *string          `json:"effective_at"`
}

type currentJSON struct {
	ReviewID    string     `json:"review_id"`
	Type        string     `json:"type"`
	VersionDate string     `json:"version_date"`
	Status      string     `json:"status"`
	V           string     `json:"v"`
	P           string     `json:"p"`
	Trigger     string     `json:"trigger"`
	ChangeMemo  string     `json:"change_memo"`
	Detail      detailJSON `json:"detail"`
}

type historyJSON struct {
	ReviewID      string     `json:"review_id"`
	Date          string     `json:"date"`
	Type          string     `json:"type"`
	V             string     `json:"v"`
	VFrom         string     `json:"v_from"`
	VTo           string     `json:"v_to"`
	ChangePercent string     `json:"change_percent"`
	Trigger       string     `json:"trigger"`
	Memo          string     `json:"memo"`
	Status        string     `json:"status"`
	VoidReason    string     `json:"void_reason"`
	Detail        detailJSON `json:"detail"`
}

// BuildCompany returns the JSON of page.valuation_page.v1, view company: the profile, the current review, its
// lines, band and close from Resolve, and the history of every review row, newest first. A company with only
// drafts shows them in history and has no lines. An unknown key gives an empty page with a gap.
func BuildCompany(ctx context.Context, in Input) ([]byte, error) {
	p, err := newPage(in, ViewCompany)
	if err != nil {
		return nil, err
	}
	e, ok := findEntry(in)
	if !ok {
		if in.StateOK {
			p.addGap("store_unavailable", noCompanyText)
		}
		return p.marshal()
	}
	p.Company = companyDoc(e.Profile)
	p.History = history(in)
	if e.Current != nil {
		p.Current = currentDoc(*e.Current)
		fillLines(ctx, p, in, e)
	}
	return p.marshal()
}

// findEntry is the entry of in.Company: its profile row, else a profile defaulted from its row of
// settings.underlying with no review.
func findEntry(in Input) (valuation.Entry, bool) {
	for _, e := range in.State.Entries {
		if e.Profile.CompanyKey == in.Company {
			return e, true
		}
	}
	for _, r := range in.Underlying {
		if r.Key == in.Company {
			return valuation.Entry{Profile: profileFromRow(r)}, true
		}
	}
	return valuation.Entry{}, false
}

// fillLines writes lines, band and close of one entry through Resolve.
func fillLines(ctx context.Context, p *pageJSON, in Input, e valuation.Entry) {
	res, err := Resolve(ctx, e, in.State, in.Closes, in.Calc, in.Day)
	switch {
	case errors.Is(err, ErrNoTables):
		p.addGap("tables_missing", tablesMissingText)
		return
	case err != nil || res == nil:
		return
	}
	if res.CloseErr != nil {
		p.addGap("close_unavailable", closeUnavailableText)
	}
	p.Lines = linesDoc(in.Calc, res.Lines)
	p.Band = bandDoc(in.Calc, res.Band)
	if res.Close != "" {
		p.Close = &closeJSON{Value: round(in.Calc, res.Close, placesPrice), Day: day(res.CloseDay)}
	}
}

func currentDoc(r valuation.Review) *currentJSON {
	return &currentJSON{
		ReviewID: r.ReviewID, Type: reviewType(r), VersionDate: reviewDate(r), Status: r.Status, V: r.V, P: r.P,
		Trigger: r.Trigger, ChangeMemo: r.ChangeMemo, Detail: detailDoc(r),
	}
}

// reviewType is the type of a review; a draft with none is shown as "first", the schema has no empty type.
func reviewType(r valuation.Review) string {
	if r.Type == "" {
		return "first"
	}
	return r.Type
}

// reviewDate is the version date, or the day of entry for a draft that has none (the schema needs a date).
func reviewDate(r valuation.Review) string {
	if r.VersionDate != "" {
		return day(r.VersionDate)
	}
	return r.EnteredAt.UTC().Format(time.DateOnly)
}

func detailDoc(r valuation.Review) detailJSON {
	d := detailJSON{
		Source: r.Source, FiscalPeriodEnd: day(r.FiscalPeriodEnd), ReportDate: day(r.ReportDate),
		EarningsBase: r.EarningsBase, BaseBasis: r.BaseBasis, ForwardFactor: r.ForwardFactor,
		LockedMultiple: r.LockedMultiple, MethodResults: append([]string{}, r.MethodResults...),
		Adjustments: []adjustmentJSON{}, Scenarios: []scenarioJSON{}, Reaction: r.Reaction,
		Moat: r.Moat, Understand: r.Understand, Management: r.Management, Financial: r.Financial,
		Reasons: reasonsJSON{
			P: r.Reasons["p"], Moat: r.Reasons["moat"], Understand: r.Reasons["understand"],
			Management: r.Reasons["management"], Financial: r.Reasons["financial"],
		},
		ReportText: reportTextOf(r.ReportText), EnteredAt: r.EnteredAt.UTC().Format(time.RFC3339),
	}
	if d.Source == "" {
		d.Source = "owner"
	}
	for _, a := range r.Adjustments {
		d.Adjustments = append(d.Adjustments, adjustmentJSON{Percent: a.Percent, Reason: a.Reason})
	}
	for _, s := range r.Scenarios {
		d.Scenarios = append(d.Scenarios, scenarioJSON{Label: s.Label, Probability: s.Probability, EPS: s.EPS, Multiple: s.Multiple})
	}
	if r.EffectiveAt != nil {
		t := r.EffectiveAt.UTC().Format(time.RFC3339)
		d.EffectiveAt = &t
	}
	return d
}

// history is one row per review row, newest first (the order of in.Reviews). An effective review carries the V of
// the effective review before it as v_from, its own V as v_to and the calculator's change percentage; a void row
// repeats the figures of the effective row it voids; a draft has none.
func history(in Input) []historyJSON {
	out := make([]historyJSON, 0, len(in.Reviews))
	type change struct{ from, to, percent string }
	changes := map[string]change{} // by review id
	voided := map[string]bool{}
	for _, r := range in.Reviews {
		if r.Status == "void" {
			voided[r.ReviewID] = true
		}
	}
	prev := ""
	for i := len(in.Reviews) - 1; i >= 0; i-- { // oldest first
		r := in.Reviews[i]
		if r.Status != "effective" {
			continue
		}
		c := change{to: r.V}
		if prev != "" {
			c.from = prev
			if pct, err := in.Calc.ChangePercent(prev, r.V); err == nil {
				c.percent = round(in.Calc, pct, placesPrice)
			}
		}
		changes[r.ReviewID] = c
		if !voided[r.ReviewID] {
			prev = r.V // a voided review is not the base of the next change
		}
	}
	for _, r := range in.Reviews {
		h := historyJSON{
			ReviewID: r.ReviewID, Date: reviewDate(r), Type: reviewType(r), V: r.V, Trigger: r.Trigger,
			Memo: r.ChangeMemo, Status: r.Status, VoidReason: r.VoidReason, Detail: detailDoc(r),
		}
		if c, ok := changes[r.ReviewID]; ok && r.Status != "draft" {
			h.VFrom, h.VTo, h.ChangePercent = c.from, c.to, c.percent
		}
		out = append(out, h)
	}
	return out
}

// reportTextOf hides a stored report_text that is only a local file path
// (one line, no space, ends in .md or .txt, holds a "/"); the row is untouched.
func reportTextOf(s string) string {
	if strings.ContainsAny(s, " \n\r") || !strings.Contains(s, "/") {
		return s
	}
	if strings.HasSuffix(s, ".md") || strings.HasSuffix(s, ".txt") {
		return ""
	}
	return s
}
