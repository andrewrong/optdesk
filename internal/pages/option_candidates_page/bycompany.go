package option_candidates_page

import (
	"slices"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// byCompanyView is the company-block layout of the tier view (candidates-by-company.md, Interface): one block
// per listed company in the order of Result.Blocks, with the legend, columns and heading note that go with it.
type byCompanyView struct {
	HeadingNote string       `json:"heading_note"`
	Legend      []legendItem `json:"legend"`
	Columns     []tierColumn `json:"columns"`
	Blocks      []blockView  `json:"blocks"`
}

// verdictView is the plain verdict of a company: its id, the word and glyph of the pill, and its tone.
type verdictView struct {
	ID    string `json:"id"`
	Word  string `json:"word"`
	Glyph string `json:"glyph"`
	Tone  string `json:"tone"`
}

// foldReasonView is one fold reason with its row count as text.
type foldReasonView struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Count string `json:"count"`
}

// foldedView is the rest of a company's rows: a title, one summary line and one count per reason.
type foldedView struct {
	Title   string           `json:"title"`
	Summary string           `json:"summary"`
	Reasons []foldReasonView `json:"reasons"`
	RowIDs  []string         `json:"row_ids"`
}

// blockView is one company: the verdict and its sentence, the best yield, the ids of the shown rows and the fold.
// The ids are TierRow ids of rows that stay in tiers[].rows.
type blockView struct {
	Company     string      `json:"company"`
	Verdict     verdictView `json:"verdict"`
	VerdictText string      `json:"verdict_text"`
	BestYield   *string     `json:"best_yield"`
	RowIDs      []string    `json:"row_ids"`
	Folded      *foldedView `json:"folded"`
}

const (
	toneSuccess = "success"
	toneWarn    = "warn"
	toneMuted   = "muted"

	legendFoldedTier = "folded"
	legendYieldTier  = "yield"
)

// verdictSpecs are the word, glyph and tone of each company state.
var verdictSpecs = map[string]verdictView{
	optiontierapi.StateSellable:   {optiontierapi.StateSellable, zhVerdictSellable, glyphFits, toneSuccess},
	optiontierapi.StateOutBand:    {optiontierapi.StateOutBand, zhVerdictOutBand, glyphCheck, toneWarn},
	optiontierapi.StateAboveBuy:   {optiontierapi.StateAboveBuy, zhVerdictAboveBuy, glyphCheck, toneWarn},
	optiontierapi.StateNoBuyPrice: {optiontierapi.StateNoBuyPrice, zhVerdictNoBuyPrice, glyphGrey, toneMuted},
	optiontierapi.StateNoQuote:    {optiontierapi.StateNoQuote, zhVerdictNoQuote, glyphGrey, toneMuted},
	optiontierapi.StateBlocked:    {optiontierapi.StateBlocked, zhVerdictBlocked, glyphGrey, toneMuted},
}

// byCompanyOf turns res.Blocks into the by_company view of v and gives each shown row its own tier badge. A
// block is written only for a company of v.Companies (the others are in the idle line). It adds, subtracts,
// compares and sorts nothing: it keeps the order of Blocks and takes every count and figure from the block,
// the echoed Params and the strip. nil when res has no Blocks or none is listed.
func byCompanyOf(v *tiersView, res optiontierapi.Result, f tierFacts, n tierCounts, day, cur string) *byCompanyView {
	if len(res.Blocks) == 0 {
		return nil
	}
	strip := make(map[string]companyView, len(v.Companies))
	for _, cv := range v.Companies {
		strip[cv.Key] = cv
	}
	have := map[string]bool{}
	for _, t := range v.Tiers {
		for _, r := range t.Rows {
			have[r.ID] = true
		}
	}
	out := &byCompanyView{
		HeadingNote: zhHeadingNote, Legend: byCompanyLegend(res, f, n), Columns: byCompanyColumns(day),
		Blocks: make([]blockView, 0, len(res.Blocks)),
	}
	shown := map[string]bool{}
	for _, cb := range res.Blocks {
		cv, ok := strip[cb.Company]
		if !ok {
			continue
		}
		cb.Shown, cb.Folded = onlyIDs(cb.Shown, have), onlyIDs(cb.Folded, have)
		out.Blocks = append(out.Blocks, blockViewOf(cb, cv, res.Params))
		for _, k := range cb.Shown {
			shown[k] = true
		}
	}
	if len(out.Blocks) == 0 {
		return nil
	}
	for i := range v.Tiers {
		if v.Tiers[i].ID != tierFitsID && v.Tiers[i].ID != tierCheckID {
			continue
		}
		for j := range v.Tiers[i].Rows {
			if shown[v.Tiers[i].Rows[j].ID] {
				v.Tiers[i].Rows[j].Tier = &tierBadge{ID: v.Tiers[i].ID, Word: v.Tiers[i].Word, Glyph: glyphOf(v.Tiers[i].ID)}
			}
		}
	}
	return out
}

func glyphOf(tier string) string {
	if tier == tierFitsID {
		return glyphFits
	}
	return glyphCheck
}

// onlyIDs keeps the ids that are rows of the tier view, in order, so a block never names a row that is not there.
func onlyIDs(ids []string, have map[string]bool) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if have[id] {
			out = append(out, id)
		}
	}
	return out
}

// blockViewOf writes one block from the compute block, its strip and the echoed params.
func blockViewOf(cb optiontierapi.CompanyBlock, cv companyView, p optiontierapi.Params) blockView {
	spec, ok := verdictSpecs[cb.State]
	if !ok {
		spec = verdictSpecs[optiontierapi.StateBlocked]
	}
	b := blockView{
		Company: cb.Company, Verdict: spec, VerdictText: verdictTextOf(spec.ID, cb, cv, p),
		RowIDs: append([]string{}, cb.Shown...),
	}
	if cb.Best != "" {
		best := zhBestYieldHead + fixedOr(cb.Best, 1) + zhBestYieldTail
		b.BestYield = &best
	}
	if len(cb.Folded) > 0 {
		b.Folded = foldedViewOf(cb, p)
	}
	return b
}

// verdictTextOf is the sentence under the verdict, filled with the counts of the block and the figures of the strip.
func verdictTextOf(state string, cb optiontierapi.CompanyBlock, cv companyView, p optiontierapi.Params) string {
	switch state {
	case optiontierapi.StateSellable:
		return zhSellHead + strconv.Itoa(p.PerCompany) + zhSellFit + strconv.Itoa(cb.NFit) + zhSellCheck +
			strconv.Itoa(cb.NCheck) + zhSellTail
	case optiontierapi.StateOutBand:
		return zhOutBandHead + strconv.Itoa(cb.NOpen) + zhOutBandOTM + decimalText(p.BandOTMMinPct, 0) + zhOutBandTo +
			decimalText(p.BandOTMMaxPct, 0) + zhOutBandDelta + decimalText(p.BandDeltaMin, 2) + zhOutBandDTo +
			decimalText(p.DeltaMax, 2) + zhOutBandTail
	case optiontierapi.StateAboveBuy:
		return zhAboveBuyHead + cv.Spot + zhAboveBuyMid + deref(cv.BuyPrice) + zhAboveBuyTail
	case optiontierapi.StateNoBuyPrice:
		return zhNoBuyPriceText
	case optiontierapi.StateNoQuote:
		return zhNoQuoteText
	}
	return zhBlockedText
}

// foldedViewOf is the fold of a block: the title with the number of folded rows, the summary in the order of Why
// and one reason per entry of Why.
func foldedViewOf(cb optiontierapi.CompanyBlock, p optiontierapi.Params) *foldedView {
	f := &foldedView{
		Title: zhFoldedTitle + strconv.Itoa(len(cb.Folded)), Reasons: make([]foldReasonView, 0, len(cb.Why)),
		RowIDs: append([]string{}, cb.Folded...),
	}
	parts := make([]string, 0, len(cb.Why))
	for _, w := range cb.Why {
		r := foldReasonView{Code: w.Code, Label: foldLabelOf(w.Code, p), Count: strconv.Itoa(w.N)}
		f.Reasons = append(f.Reasons, r)
		parts = append(parts, r.Label+" "+r.Count)
	}
	f.Summary = strings.Join(parts, zhFoldedSep)
	return f
}

// foldLabelOf is the Chinese label of a fold reason: the kept label of a grey code, the band label of a band code.
func foldLabelOf(code string, p optiontierapi.Params) string {
	if g, ok := greySpecs[code]; ok {
		return g.keptLabel
	}
	switch code {
	case optiontierapi.FoldNoOpenInterest:
		return zhBandNoOI
	case optiontierapi.FoldOTMLow:
		return zhBandOTMLow + decimalText(p.BandOTMMinPct, 0) + zhBandPct
	case optiontierapi.FoldOTMHigh:
		return zhBandOTMHigh + decimalText(p.BandOTMMaxPct, 0) + zhBandPct
	case optiontierapi.FoldNoDelta:
		return zhBandNoDelta
	case optiontierapi.FoldDeltaOut:
		return zhBandDeltaOut + decimalText(p.BandDeltaMin, 2) + zhBandDeltaTo + decimalText(p.DeltaMax, 2)
	case optiontierapi.FoldOverCap:
		return zhBandOverCap + strconv.Itoa(p.PerCompany) + zhBandOverCapGe
	}
	return code
}

// decimalText is a stored decimal text with the digits it has (at least min), so "0.3" stays "0.3" and "5"
// stays "5"; unreadable text is returned as it is.
func decimalText(text string, min int) string {
	_, frac, _ := strings.Cut(text, ".")
	places := max(len(frac), min)
	if f, ok := fixedText(text, places); ok {
		return f
	}
	return text
}

// fixedOr is fixedText of the text, or the text itself when it is unreadable.
func fixedOr(text string, places int) string {
	if f, ok := fixedText(text, places); ok {
		return f
	}
	return text
}

// byCompanyLegend is the legend of the block layout: the fits and check items and the cash item of the old
// legend, with the folded and yield-basis items between them.
func byCompanyLegend(res optiontierapi.Result, f tierFacts, n tierCounts) []legendItem {
	old := legendOf(res, f, n)
	perCo := strconv.Itoa(res.Params.PerCompany)
	idx := slices.IndexFunc(old, func(l legendItem) bool { return l.Tier == legendCashTier })
	return []legendItem{
		old[0], old[1],
		{Tier: legendFoldedTier, Title: zhLegendFolded, Text: zhFoldedLegendHead + perCo + zhFoldedLegendTail},
		{Tier: legendYieldTier, Title: zhLegendYield, Text: zhYieldLegend},
		old[idx],
	}
}

// byCompanyColumns are the seven columns with the premium column relabelled and hinted for the yield.
func byCompanyColumns(day string) []tierColumn {
	cols := columnsOf(day)
	for i := range cols {
		if cols[i].ID == colPremiumID {
			cols[i].Label, cols[i].Hint = zhColPremiumYield, ptr(zhHintPremiumYield)
		}
	}
	return cols
}
