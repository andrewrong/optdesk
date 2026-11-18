package option_candidates_page

import (
	"slices"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/optionfigapi"
	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/store"
)

type contractView struct {
	Name   string `json:"name"`
	Strike string `json:"strike"`
	Expiry string `json:"expiry"`
	DTE    string `json:"dte"`
}

// pairView is a main text with an optional sub text; a pair with a gap has the gap text in Main.
type pairView struct {
	Main string  `json:"main"`
	Sub  *string `json:"sub"`
	Gap  *gap    `json:"gap"`
}

type badgeView struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Gloss string `json:"gloss"`
	Tone  string `json:"tone"`
}

type fieldView struct {
	ID     string      `json:"id"`
	Label  string      `json:"label"`
	Hint   *string     `json:"hint"`
	Value  *string     `json:"value"`
	Gap    *gap        `json:"gap"`
	Badges []badgeView `json:"badges"`
}

type tierRow struct {
	ID        string       `json:"id"`
	Company   string       `json:"company"`
	Contract  contractView `json:"contract"`
	Margin    pairView     `json:"margin"`
	BelowSpot pairView     `json:"below_spot"`
	Premium   pairView     `json:"premium"`
	Badges    []badgeView  `json:"badges"`
	Notes     []badgeView  `json:"notes"`
	Fields    []fieldView  `json:"fields"`
	Line      *string      `json:"line"`
}

// rowEnv is what every row of one build shares: the data day, the marks said once above the table, the
// evidence columns bound to the rows of the Fits and Check tiers, their figures and the capture day of each alias.
type rowEnv struct {
	day     string
	common  []string
	cols    []colDef
	figs    []optionfigapi.PutFigures
	cashDay map[string]string
}

// tierRowOf writes one row of the answer. pos is the position of the row among the rows whose evidence and
// figures were read, -1 for a grey row, which gets a line and no fields.
func (e rowEnv) tierRowOf(v optiontierapi.Verdict, c store.Candidate, pos int) tierRow {
	r := tierRow{
		ID: v.Key, Company: strconv.Itoa(c.UnderlyingID), Contract: contractOf(c),
		Margin: marginPair(v, c), BelowSpot: belowSpotPair(v), Premium: premiumPair(v),
		Notes: e.badgesOf(v.Notes, badgeToneNone, c), Fields: []fieldView{},
	}
	r.Badges = append(e.badgesOf(v.Badges, badgeToneWarn, c), e.badgesOf(v.Infos, badgeToneInfo, c)...)
	if pos < 0 {
		line := greyLine(v, c)
		r.Line = &line
		return r
	}
	r.Fields = e.fieldsOf(v, c, pos)
	return r
}

func contractOf(c store.Candidate) contractView {
	strike, ok := moneyText(c.Strike)
	if !ok {
		strike = unreadable
	}
	return contractView{Name: c.Issuer, Strike: strike, Expiry: string(c.Expiry), DTE: strconv.Itoa(c.DTE) + dteSuffix}
}

func whyText(v optiontierapi.Verdict, id, fallback string) string {
	if t, ok := whyTexts[v.Why[id]]; ok {
		return t
	}
	return fallback
}

func gapPair(reason, text string) pairView {
	g := mkGap(reason, text)
	return pairView{Main: text, Gap: &g}
}

func subOf(s string) *string { return &s }

// figurePair is a percent figure as the main text of a pair, or the gap pair when it is empty or unreadable.
func figurePair(v optiontierapi.Verdict, id, text, fallback string) (pairView, bool) {
	if text == "" {
		return gapPair("no_data", whyText(v, id, fallback)), false
	}
	main, ok := pctText1(text)
	if !ok {
		return gapPair("unreadable", unreadable), false
	}
	return pairView{Main: main}, true
}

func marginPair(v optiontierapi.Verdict, c store.Candidate) pairView {
	p, ok := figurePair(v, "margin", v.MarginPct, noValue)
	if !ok {
		return p
	}
	be, ok1 := moneyText(v.BreakEven)
	buy, ok2 := moneyText(deref(c.BuyPrice))
	if ok1 && ok2 {
		p.Sub = subOf(beHead + be + beVs + buy)
	}
	return p
}

func belowSpotPair(v optiontierapi.Verdict) pairView {
	p, ok := figurePair(v, "below_spot", v.BelowSpotPct, noValue)
	if !ok {
		return p
	}
	if m, ok := fixedText(v.Moves, 1); v.Moves != "" && ok {
		p.Sub = subOf(minusText(m) + movesSuffix)
	} else {
		p.Sub = subOf(whyText(v, "moves", gapNoIV))
	}
	return p
}

func premiumPair(v optiontierapi.Verdict) pairView {
	if v.PerContract == "" {
		return gapPair("no_data", whyText(v, "premium", gapNoBid))
	}
	main, ok := moneyText(v.PerContract)
	if !ok {
		return gapPair("unreadable", unreadable)
	}
	p := pairView{Main: main}
	if y, ok := fixedText(v.YieldPct, 1); v.YieldPct != "" && ok {
		p.Sub = subOf(minusText(y) + yieldSuffix)
	} else {
		p.Sub = subOf(whyText(v, "yield", gapNoYield))
	}
	return p
}

// greyLine is the one line of a grey row: the contract, the break-even and, for an in-the-money row, the
// intrinsic and time value, then the yield.
func greyLine(v optiontierapi.Verdict, c store.Candidate) string {
	strike, _ := moneyText(c.Strike)
	parts := []string{c.Issuer + " " + strike + " " + string(c.Expiry)}
	if bid, ok := moneyText(c.BidEod); ok && c.BidEod != "" {
		parts = append(parts, lineBid+bid)
	}
	if be, ok := moneyText(v.BreakEven); v.BreakEven != "" && ok {
		parts = append(parts, beHead+be)
	} else {
		parts = append(parts, lineBEGap+whyText(v, "break_even", gapNoBid))
	}
	if v.Grey == "in_the_money" {
		in, ok1 := moneyText(v.Intrinsic)
		tv, ok2 := moneyText(v.TimeValue)
		if ok1 && ok2 {
			parts = append(parts, lineIntrinsic+in+lineTimeValue+tv)
		}
	}
	if y, ok := fixedText(v.YieldPct, 1); v.YieldPct != "" && ok {
		parts = append(parts, lineYield+minusText(y)+yieldSuffix)
	}
	return strings.Join(parts, lineSep)
}

// badgeOf is the label and gloss of a code; the badge of earnings before expiry names the stored date.
func badgeOf(code, tone string, c store.Candidate) badgeView {
	t, ok := badgeTexts[code]
	if !ok {
		return badgeView{Code: code, Label: code, Gloss: unknownGloss + code, Tone: tone}
	}
	b := badgeView{Code: code, Label: t.label, Gloss: t.gloss, Tone: tone}
	if code == "earnings_before_expiry" && c.NextEarnings != nil {
		b.Label += earningsLabelSp + string(*c.NextEarnings)
	}
	return b
}

// badgesOf are the badges of codes, never nil, without the marks the run line says once.
func (e rowEnv) badgesOf(codes []string, tone string, c store.Candidate) []badgeView {
	out := []badgeView{}
	for _, code := range codes {
		if !slices.Contains(e.common, code) {
			out = append(out, badgeOf(code, tone, c))
		}
	}
	return out
}

// dayHinted is a Chinese hint that ends with the data day of its figure.
func dayHinted(hint, day string) *string {
	s := hint + dayHint + day
	return &s
}

// cellField writes a cell of the table helpers as a field: a gap stays a gap, a value cell its text and badges.
func cellField(id, label string, hint *string, cell any) fieldView {
	f := fieldView{ID: id, Label: label, Hint: hint, Badges: []badgeView{}}
	switch x := cell.(type) {
	case valueCell:
		text := x.Text
		f.Value = &text
		gloss := ""
		if hint != nil {
			gloss = *hint
		}
		for _, b := range x.Badges {
			f.Badges = append(f.Badges, badgeView{Code: evBadgeCode, Label: b, Gloss: gloss, Tone: badgeToneNone})
		}
	case gap:
		f.Gap = &x
	}
	return f
}

// withoutBand is the cells of a row of the evidence columns with the label delta_outside_band left out: the
// tier rules have no lower delta bound (Decision 1d).
func withoutBand(set evidenceSet) evidenceSet {
	out := set
	out.rows = make([]rowEvidence, len(set.rows))
	for i, r := range set.rows {
		r.ev.Labels = slices.DeleteFunc(slices.Clone(r.ev.Labels), func(l string) bool { return l == deltaOutsideBand })
		out.rows[i] = r
	}
	return out
}

// fieldsOf is the expansion of a Fits or Check row (Decision 10), in order.
func (e rowEnv) fieldsOf(v optiontierapi.Verdict, c store.Candidate, pos int) []fieldView {
	fields := []fieldView{
		cellField(fieldIVID, fieldIVLabel, dayHinted(hintIV, e.day), storedCell(c.Iv, ivCell)),
		cellField(fieldDeltaID, fieldDeltaLabel, dayHinted(hintDelta, e.day), storedCell(c.Delta, deltaCell)),
		cellField(fieldITMID, fieldITMLabel, dayHinted(hintITM, e.day), e.figureAt(pos, itmFigure)),
		cellField(fieldVsCashID, fieldVsCashLabel, dayHinted(hintVsCash, e.day), e.figureAt(pos, excessFigure)),
		e.cashField(v),
		cellField(fieldBidID, fieldBidLabel, dayHinted(hintBid, e.day), moneyCell(c.BidEod)),
	}
	for _, d := range e.cols {
		fields = append(fields, cellField(d.ID, d.Label, d.Hint, d.cellAt(pos, c)))
	}
	return fields
}

// storedCell is a stored optional text as its cell; no text is the gap "not stored".
func storedCell(p *string, f func(*string) any) any {
	if p == nil {
		return mkGap("missing_input", gapNotStored)
	}
	return f(p)
}

func (e rowEnv) figureAt(pos int, f func(optionfigapi.PutFigures) any) any {
	return figureCell(e.figs, f)(pos, store.Candidate{})
}

// cashField names the alias that fits first, then has one line per alias.
func (e rowEnv) cashField(v optiontierapi.Verdict) fieldView {
	hint := dayHinted(hintCash, e.day)
	if len(v.Cash) == 0 {
		return cellField(fieldCashID, fieldCashLabel, hint, mkGap("no_data", gapNoAccount))
	}
	head := fitsNone
	if v.FitAlias != "" {
		head = fitsPrefix + v.FitAlias
	}
	f := fieldView{ID: fieldCashID, Label: fieldCashLabel, Hint: hint, Value: &head, Badges: []badgeView{}}
	for _, cc := range v.Cash {
		f.Badges = append(f.Badges, badgeView{Code: cashBadgeHead + cc.Alias, Label: e.cashLine(cc), Gloss: cashBadgeGloss, Tone: badgeToneNone})
	}
	return f
}

// cashLine is "A: enough, $120,000.00 free after $0.00 committed (2026-11-17)", the same with "short", or
// "A: no figure: <reason>".
func (e rowEnv) cashLine(cc optiontierapi.CashCell) string {
	head := cc.Alias + aliasSep
	if cc.Verdict != "fits" && cc.Verdict != "short" {
		why, ok := cashWhyTexts[cc.Why]
		if !ok {
			why = cc.Why
		}
		return head + cashNoFig + why
	}
	word := cashEnough
	if cc.Verdict == "short" {
		word = cashShort
	}
	free, ok1 := moneyText(cc.Free)
	committed, ok2 := moneyText(cc.Committed)
	if !ok1 || !ok2 {
		return head + cashNoFig + unreadable
	}
	return head + word + free + cashFree + committed + cashCommitted + e.cashDay[cc.Alias] + ")"
}
