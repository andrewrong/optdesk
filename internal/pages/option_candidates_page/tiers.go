package option_candidates_page

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type companyView struct {
	Key          string         `json:"key"`
	Ticker       string         `json:"ticker"`
	Spot         string         `json:"spot"`
	SpotDay      string         `json:"spot_day"`
	BuyPrice     *string        `json:"buy_price"`
	BuyPriceNote *string        `json:"buy_price_note"`
	NextEarnings string         `json:"next_earnings"`
	Counts       string         `json:"counts"`
	Valuation    *valueLineView `json:"valuation"`
	BuySource    *badgeView     `json:"buy_source"`
	// The fields of the Chinese card (candidates-zh.md Decision 3); the old fields above keep their values.
	Tier         tierBadge  `json:"tier"`
	CountParts   countParts `json:"count_parts"`
	SpotText     *string    `json:"spot_text"`
	SpotNote     *string    `json:"spot_note"`
	PriceNote    *string    `json:"price_note"`
	EarningsText string     `json:"earnings_text"`
	SourceBadge  *badgeView `json:"source_badge"`
	source       string     // the buySourceCode of the company, kept as a value before any badge is made
}

// tierBadge is the verdict of a company: its id, the word and the glyph of the pill.
type tierBadge struct {
	ID    string `json:"id"`
	Word  string `json:"word"`
	Glyph string `json:"glyph"`
}

// countParts are the three counts of a company as whole-number texts.
type countParts struct {
	Fits  string `json:"fits"`
	Check string `json:"check"`
	Grey  string `json:"grey"`
}

// stripLabels are the words of the company card.
type stripLabels struct {
	Spot         string `json:"spot"`
	BuyPrice     string `json:"buy_price"`
	NextEarnings string `json:"next_earnings"`
	Contracts    string `json:"contracts"`
	Fits         string `json:"fits"`
	Check        string `json:"check"`
	Grey         string `json:"grey"`
	GreyHint     string `json:"grey_hint"`
}

type markView struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Gloss string `json:"gloss"`
}

type tierColumn struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Hint  *string `json:"hint"`
}

type tierView struct {
	ID        string    `json:"id"`
	Word      string    `json:"word"`
	Icon      string    `json:"icon"`
	Title     string    `json:"title"`
	Explain   string    `json:"explain"`
	Collapsed bool      `json:"collapsed"`
	Rows      []tierRow `json:"rows"`
}

type tiersView struct {
	ChainDay    string        `json:"chain_day"`
	Companies   []companyView `json:"companies"`
	Common      []markView    `json:"common"`
	Kept        *string       `json:"kept"`
	KeptNote    *string       `json:"kept_note"`
	Legend      []legendItem  `json:"legend"`
	Columns     []tierColumn  `json:"columns"`
	Tiers       []tierView    `json:"tiers"`
	StoredTitle string        `json:"stored_title"`
	IdleNote    *string       `json:"idle_note"`
	BuyHint     *string       `json:"buy_hint"`
	SpotHint    *string       `json:"spot_hint"`
	StripLabels stripLabels   `json:"strip_labels"`
	CardNote    *string       `json:"card_note"`
	// Heads are the headings of the company cards and the tier tables, omitted on a page with no tier view.
	Heads *tierHeads `json:"heads,omitempty"`
}

// tierHeads are the three headings of the tier view (candidates-zh-headings.md Decision 2).
type tierHeads struct {
	Companies     string `json:"companies"`
	CompaniesNote string `json:"companies_note"`
	Tiers         string `json:"tiers"`
}

// bucket is the rows (keys, in display order) of one tier of the answer.
type bucket struct {
	id, grey string
	keys     []string
}

// newRowEnv is what a tier row needs from its group: the evidence columns, the cash days of the accounts and,
// for a US group only, the figures (a yen row has no US figure: its cells are the gap "not computed").
func newRowEnv(c Config, gr groupRead, res optiontierapi.Result, f tierFacts, ev evidenceSet, judged []store.Candidate) rowEnv {
	env := rowEnv{
		day: string(gr.day), common: res.Common, cols: evidenceCols(withoutBand(ev)), cur: gr.cur,
		cashDay: map[string]string{},
	}
	if gr.cur == "" {
		env.figs = putFigures(c.Figures, gr.rate, gr.day, judged)
	}
	for _, a := range f.in.Accounts {
		env.cashDay[a.Alias] = string(a.CaptureDay)
	}
	return env
}

// buildTiers sorts the stored short-put rows into the tiers of Config.Tiers. It calls Judge once and
// the evidence builder and Config.Figures once for the rows of the Fits and Check tiers, whatever their stored
// status, and writes the texts; day is the day Build was called for, which a past earnings date is compared
// with; a nil Tiers or no row gives nil. A reader or Judge error is returned.
func buildTiers(ctx context.Context, c Config, gr groupRead, day contracts.Day, at time.Time) (*tiersView, error) {
	if c.Tiers == nil || len(gr.rows) == 0 {
		return nil, nil
	}
	f, err := tierInputOf(ctx, c, gr.day, gr.rows, at)
	if err != nil {
		return nil, err
	}
	for i := range f.in.Rows {
		f.in.Rows[i].Currency = gr.cur
	}
	res, err := c.Tiers.Judge(f.in)
	if err != nil {
		return nil, fmt.Errorf("option_candidates_page: tiers: %w", err)
	}
	verdicts := make(map[string]optiontierapi.Verdict, len(res.Verdicts))
	for _, v := range res.Verdicts {
		verdicts[v.Key] = v
	}
	rowOf := make(map[string]store.Candidate, len(gr.rows))
	for i, r := range gr.rows {
		rowOf[tierKey(r, i)] = r
	}
	buckets := bucketsOf(res.Order, verdicts, rowOf)

	var judged []store.Candidate
	posOf := map[string]int{}
	for _, b := range buckets[:2] {
		for _, k := range b.keys {
			posOf[k] = len(judged)
			judged = append(judged, rowOf[k])
		}
	}
	ev, err := evidenceOf(ctx, inputsOf(c, gr.cur), gr.tickers, gr.day, gr.rate, judged)
	if err != nil {
		return nil, err
	}
	env := newRowEnv(c, gr, res, f, ev, judged)
	v := &tiersView{
		ChainDay: string(gr.day), Common: commonOf(res.Common),
		Kept: keptLine(res.Kept), StoredTitle: storedTitle, Columns: columnsOf(string(gr.day)), Tiers: make([]tierView, 0, len(buckets)),
	}
	var n tierCounts
	for i, b := range buckets {
		t := tierOf(b, res, f)
		for _, k := range b.keys {
			pos, ok := posOf[k]
			if !ok || i > 1 {
				pos = -1
			}
			t.Rows = append(t.Rows, env.tierRowOf(verdicts[k], rowOf[k], pos))
		}
		t.Title = titleOf(i, t.Title, len(t.Rows))
		switch i {
		case 0:
			n.fits = len(t.Rows)
		case 1:
			n.check = len(t.Rows)
		default:
			n.grey += len(t.Rows)
		}
		v.Tiers = append(v.Tiers, t)
	}
	fillStrips(ctx, c, v, companiesOf(res, gr.rows, gr.day, day, gr.cur), gr.cur)
	v.Legend = legendOf(res, f, n)
	v.KeptNote = keptNoteOf(v.Kept)
	return v, nil
}

// titleOf is the title of tier i of the answer, whose tail is tail: the word and the count for Fits and Check,
// the count and the reason for a grey tier.
func titleOf(i int, tail string, n int) string {
	if i < 2 {
		return tail + strconv.Itoa(n)
	}
	return strconv.Itoa(n) + tail
}

// companyTierOf is the verdict of a company from its three counts: Fits when it has a fitting row, else Check
// when it has one to check, else not judged.
func companyTierOf(fits, check int) tierBadge {
	switch {
	case fits > 0:
		return tierBadge{ID: tierFitsID, Word: tierFitsWord, Glyph: glyphFits}
	case check > 0:
		return tierBadge{ID: tierCheckID, Word: tierCheckWord, Glyph: glyphCheck}
	}
	return tierBadge{ID: legendGreyTier, Word: tierGreyView, Glyph: glyphGrey}
}

// spotNoteOf is the stored day without its year, cut as text.
func spotNoteOf(day string) string {
	_, md, ok := strings.Cut(day, "-")
	if !ok {
		md = day
	}
	return zhSpotArchive + md
}

// stripLabelsOf are the eight words of the card.
func stripLabelsOf() stripLabels {
	return stripLabels{
		Spot: zhStripSpot, BuyPrice: zhStripBuyPrice, NextEarnings: zhStripEarnings, Contracts: zhStripContracts,
		Fits: zhStripFits, Check: zhStripCheck, Grey: zhStripGrey, GreyHint: zhStripGreyHint,
	}
}

// keptNoteOf is the note that one row can have several reasons; nil when there is no kept line.
func keptNoteOf(kept *string) *string {
	if kept == nil {
		return nil
	}
	note := keptNote
	return &note
}

// tierIDOf is the id of the tier of a verdict; a grey reason the page has no text for is bad_input.
func tierIDOf(v optiontierapi.Verdict) (id, grey string) {
	switch v.Tier {
	case tierFitsID:
		return tierFitsID, ""
	case tierCheckID:
		return tierCheckID, ""
	}
	if _, ok := greySpecs[v.Grey]; !ok {
		return greyIDHead + fallbackGrey, fallbackGrey
	}
	return greyIDHead + v.Grey, v.Grey
}

// bucketsOf groups the keys of order by tier: Fits and Check first, always present, then each grey reason
// in the order it first appears. A key with no verdict or no stored row is not shown.
func bucketsOf(order []string, verdicts map[string]optiontierapi.Verdict, rowOf map[string]store.Candidate) []bucket {
	out := []bucket{{id: tierFitsID}, {id: tierCheckID}}
	for _, k := range order {
		v, ok := verdicts[k]
		if _, has := rowOf[k]; !ok || !has {
			continue
		}
		id, grey := tierIDOf(v)
		i := slices.IndexFunc(out, func(b bucket) bool { return b.id == id })
		if i < 0 {
			out = append(out, bucket{id: id, grey: grey})
			i = len(out) - 1
		}
		out[i].keys = append(out[i].keys, k)
	}
	return out
}

// tierOf is the heading of a tier; its Title holds the text after the count, which the caller prefixes.
func tierOf(b bucket, res optiontierapi.Result, f tierFacts) tierView {
	t := tierView{ID: b.id, Rows: []tierRow{}}
	switch b.id {
	case tierFitsID:
		t.Word, t.Icon, t.Title, t.Explain = tierFitsWord, iconCheck, titleFitsTail, explainFits(res, f)
	case tierCheckID:
		t.Word, t.Icon, t.Title, t.Explain = tierCheckWord, iconAlert, titleCheckTail, explainCheck
	default:
		g := greySpecs[b.grey]
		t.Word, t.Icon, t.Title, t.Explain, t.Collapsed = tierGreyView, iconMinus, g.titleTail, g.gloss, true
	}
	return t
}

// companiesOf is the strip of the answer's companies: ticker, stored spot and its day, buy price or "not judged", its source
// and the next earnings, from the first row of the company. SpotDay is the day of the stored rows; an
// earnings date before day, the data day, is shown as unknown.
func companiesOf(res optiontierapi.Result, rows []store.Candidate, rowsDay, day contracts.Day, cur string) []companyView {
	out := make([]companyView, 0, len(res.Companies))
	for _, cc := range res.Companies {
		i := slices.IndexFunc(rows, func(r store.Candidate) bool { return strconv.Itoa(r.UnderlyingID) == cc.Company })
		if i < 0 {
			continue
		}
		r := rows[i]
		v := companyView{
			Key: cc.Company, Ticker: r.Issuer, Spot: noSpotText, SpotDay: string(rowsDay), NextEarnings: earningsUnknown,
			Tier: companyTierOf(cc.Fits, cc.Check), EarningsText: zhEarningsNone,
			CountParts: countParts{Fits: strconv.Itoa(cc.Fits), Check: strconv.Itoa(cc.Check), Grey: strconv.Itoa(cc.Grey)},
			Counts:     strconv.Itoa(cc.Fits) + countFit + strconv.Itoa(cc.Check) + countCheck + strconv.Itoa(cc.Grey) + countCollapsed,
		}
		if m, ok := moneyText(deref(r.Spot), cur); deref(r.Spot) != "" && ok {
			v.Spot = m
			note := spotNoteOf(string(rowsDay))
			v.SpotText, v.SpotNote = &m, &note
		}
		// The price and its source come from the first row of the company that has a price; the source is
		// read from the stored marks and never recomputed.
		priced := r
		if j := slices.IndexFunc(rows, func(x store.Candidate) bool {
			return strconv.Itoa(x.UnderlyingID) == cc.Company && deref(x.BuyPrice) != ""
		}); j >= 0 {
			priced = rows[j]
		}
		v.source = buySourceCode(priced)
		v.BuySource = buySourceBadge(v.source)
		if m, ok := moneyText(deref(priced.BuyPrice), cur); deref(priced.BuyPrice) != "" && ok {
			v.BuyPrice = &m
			v.SourceBadge = stripSourceBadge(v.source)
		} else {
			note, none := notJudged, zhNoBuyPrice
			v.BuyPriceNote, v.PriceNote = &note, &none
		}
		if r.NextEarnings != nil && *r.NextEarnings >= day {
			v.NextEarnings = string(*r.NextEarnings)
			v.EarningsText = v.NextEarnings
		}
		out = append(out, v)
	}
	return out
}

// commonOf is the marks every row carries, each with the run line of Decision 11 as its gloss.
func commonOf(codes []string) []markView {
	out := []markView{}
	for _, code := range codes {
		b := badgeOf(code, badgeToneInfo, store.Candidate{})
		out = append(out, markView{Code: code, Label: b.Label, Gloss: commonHead + b.Label + commonMid + b.Gloss + commonTail})
	}
	return out
}

// keptLine is the line of Decision 7a, nil when no condition kept a row out of Fits. It stops with an
// ellipsis before it would pass the 200 characters a text may have.
func keptLine(kept []optiontierapi.Kept) *string {
	if len(kept) == 0 {
		return nil
	}
	line := keptHead
	for i, k := range kept {
		label := keptLabelOf(k.Code)
		entry := label + " " + strconv.Itoa(k.N)
		if k.Every {
			entry += keptEvery
		}
		if i > 0 {
			entry = keptSeparator + entry
		}
		if utf8.RuneCountInString(line+entry+keptEllipsis) > keptMaxChars {
			line += keptEllipsis
			break
		}
		line += entry
	}
	return &line
}

// keptLabelOf is the Chinese word of a grey reason or of a look-at badge.
func keptLabelOf(code string) string {
	if g, ok := greySpecs[code]; ok {
		return g.keptLabel
	}
	if l, ok := keptLabels[code]; ok {
		return l
	}
	return code
}

// columnsOf are the seven columns of the first view with their short hints; only the expanded-row hints end with the data day.
func columnsOf(day string) []tierColumn {
	return []tierColumn{
		{ID: colArrowID, Label: colArrowLabel},
		{ID: colTierID, Label: colTierLabel},
		{ID: colContractID, Label: colContractLabel, Hint: ptr(hintContract)},
		{ID: colMarginID, Label: colMarginLabel, Hint: ptr(hintMargin)},
		{ID: colBelowSpotID, Label: colBelowLabel, Hint: ptr(hintBelow)},
		{ID: colPremiumID, Label: colPremiumLabel, Hint: ptr(hintPremium)},
		{ID: colBadgesID, Label: colBadgesLabel, Hint: ptr(hintBadges)},
	}
}

// splitIdle keeps the companies that have a buy price or a valuation figure and names the others in one
// line, cut with an ellipsis before 200 characters; the hint says where a buy price is set when any company
// has none. Each is nil when it does not apply.
func splitIdle(all []companyView) (listed []companyView, idle, hint *string) {
	listed = make([]companyView, 0, len(all))
	var idleTickers []string
	for _, cv := range all {
		if cv.BuyPrice == nil {
			h := buyHintZh
			hint = &h
		}
		if cv.BuyPrice != nil || hasFigure(cv) {
			listed = append(listed, cv)
		} else {
			idleTickers = append(idleTickers, cv.Ticker)
		}
	}
	if len(idleTickers) == 0 {
		return listed, nil, hint
	}
	head := strconv.Itoa(len(idleTickers)) + idleNoteTail
	line := head
	for i, tk := range idleTickers {
		entry := tk
		if i > 0 {
			entry = idleNoteSep + tk
		}
		if utf8.RuneCountInString(line+entry+idleNoteEllipsis) > keptMaxChars {
			line += idleNoteEllipsis
			break
		}
		line += entry
	}
	return listed, &line, hint
}

// sayNone starts the explanation of an empty Fits or Check tier with "None:" and the reason that applies;
// a tier with rows keeps its own explanation. ts holds Fits, Check, then the grey tiers.
func sayNone(ts []tierView) {
	if len(ts) < 2 {
		return
	}
	if len(ts[0].Rows) == 0 {
		reason := noneFits
		switch {
		case len(ts[1].Rows) > 0:
			reason = noneFitsHasCheck
		case len(ts) > 2:
			reason = noneFitsAllGrey
		}
		ts[0].Explain = noneHead + reason
	}
	if len(ts[1].Rows) == 0 {
		ts[1].Explain = noneHead + noneCheck
	}
}

// fillStrips writes the company strips, the idle line, the hints and the None explanations of v from all.
func fillStrips(ctx context.Context, c Config, v *tiersView, all []companyView, cur string) {
	fillValuation(ctx, c, all, cur)
	v.Companies, v.IdleNote, v.BuyHint = splitIdle(all)
	sayNone(v.Tiers)
	spot, card := spotHintZh, zhCardNote
	v.SpotHint, v.CardNote, v.StripLabels = &spot, &card, stripLabelsOf()
	v.Heads = &tierHeads{Companies: zhHeadCompanies, CompaniesNote: zhHeadCompNote, Tiers: zhHeadTierTables}
}
