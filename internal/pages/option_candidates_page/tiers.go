package option_candidates_page

import (
	"context"
	"fmt"
	"slices"
	"strconv"
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
}

// bucket is the rows (keys, in display order) of one tier of the answer.
type bucket struct {
	id, grey string
	keys     []string
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
	env := rowEnv{
		day: string(gr.day), common: res.Common, cols: evidenceCols(withoutBand(ev)), cur: gr.cur,
		cashDay: map[string]string{},
	}
	if gr.cur == "" { // a yen row has no US figure: its cells are the gap "not computed"
		env.figs = putFigures(c.Figures, gr.rate, gr.day, judged)
	}
	for _, a := range f.in.Accounts {
		env.cashDay[a.Alias] = string(a.CaptureDay)
	}
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
		t.Title = strconv.Itoa(len(t.Rows)) + t.Title
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
		t.Word, t.Icon, t.Title, t.Explain, t.Collapsed = tierGreyWord, iconMinus, g.titleTail, g.gloss, true
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
			Counts: strconv.Itoa(cc.Fits) + countFit + strconv.Itoa(cc.Check) + countCheck + strconv.Itoa(cc.Grey) + countCollapsed,
		}
		if m, ok := moneyText(deref(r.Spot), cur); deref(r.Spot) != "" && ok {
			v.Spot = m
		}
		// The price and its source come from the first row of the company that has a price; the source is
		// read from the stored marks and never recomputed.
		priced := r
		if j := slices.IndexFunc(rows, func(x store.Candidate) bool {
			return strconv.Itoa(x.UnderlyingID) == cc.Company && deref(x.BuyPrice) != ""
		}); j >= 0 {
			priced = rows[j]
		}
		v.BuySource = buySourceBadge(buySourceCode(priced))
		if m, ok := moneyText(deref(priced.BuyPrice), cur); deref(priced.BuyPrice) != "" && ok {
			v.BuyPrice = &m
		} else {
			note := notJudged
			v.BuyPriceNote = &note
		}
		if r.NextEarnings != nil && *r.NextEarnings >= day {
			v.NextEarnings = string(*r.NextEarnings)
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

// columnsOf are the seven columns of the first view with their hints, each ending with the data day.
func columnsOf(day string) []tierColumn {
	return []tierColumn{
		{ID: colArrowID, Label: colArrowLabel},
		{ID: colTierID, Label: colTierLabel, Hint: dayHinted(hintTier, day)},
		{ID: colContractID, Label: colContractLabel, Hint: dayHinted(hintContract, day)},
		{ID: colMarginID, Label: colMarginLabel, Hint: dayHinted(hintMargin, day)},
		{ID: colBelowSpotID, Label: colBelowLabel, Hint: dayHinted(hintBelow, day)},
		{ID: colPremiumID, Label: colPremiumLabel, Hint: dayHinted(hintPremium, day)},
		{ID: colBadgesID, Label: colBadgesLabel, Hint: dayHinted(hintBadges, day)},
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
	spot := spotHintZh
	v.SpotHint = &spot
}
