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
}

// bucket is the rows (keys, in display order) of one tier of the answer.
type bucket struct {
	id, grey string
	keys     []string
}

// buildTiers sorts the stored short-put rows into the tiers of Config.Tiers. It calls Judge once and
// the evidence builder and Config.Figures once for the rows of the Fits and Check tiers, whatever their stored
// status, and writes the texts; a nil Tiers or no row gives nil. A reader or Judge error is returned.
func buildTiers(ctx context.Context, c Config, gr groupRead, at time.Time) (*tiersView, error) {
	if c.Tiers == nil || len(gr.rows) == 0 {
		return nil, nil
	}
	f, err := tierInputOf(ctx, c, gr.day, gr.rows, at)
	if err != nil {
		return nil, err
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
	ev, err := evidenceOf(ctx, inputsOf(c), gr.tickers, gr.day, gr.rate, judged)
	if err != nil {
		return nil, err
	}
	env := rowEnv{
		day: string(gr.day), common: res.Common, cols: evidenceCols(withoutBand(ev)),
		figs: putFigures(c.Figures, gr.rate, gr.day, judged), cashDay: map[string]string{},
	}
	for _, a := range f.in.Accounts {
		env.cashDay[a.Alias] = string(a.CaptureDay)
	}
	v := &tiersView{
		ChainDay: string(gr.day), Companies: companiesOf(res, gr.rows, gr.day), Common: commonOf(res.Common),
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
	fillValuation(ctx, c, v.Companies)
	v.Legend = legendOf(res, f, n)
	if v.Kept != nil {
		note := keptNote
		v.KeptNote = &note
	}
	return v, nil
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

// companiesOf is the strip of the answer's companies: ticker, stored spot and its day, buy price or "not set"
// and the next earnings, from the first row of the company.
func companiesOf(res optiontierapi.Result, rows []store.Candidate, day contracts.Day) []companyView {
	out := make([]companyView, 0, len(res.Companies))
	for _, cc := range res.Companies {
		i := slices.IndexFunc(rows, func(r store.Candidate) bool { return strconv.Itoa(r.UnderlyingID) == cc.Company })
		if i < 0 {
			continue
		}
		r := rows[i]
		v := companyView{
			Key: cc.Company, Ticker: r.Issuer, Spot: noSpotText, SpotDay: string(day), NextEarnings: earningsUnknown,
			Counts: strconv.Itoa(cc.Fits) + countFit + strconv.Itoa(cc.Check) + countCheck + strconv.Itoa(cc.Grey) + countCollapsed,
		}
		if m, ok := moneyText(deref(r.Spot)); deref(r.Spot) != "" && ok {
			v.Spot = m
		}
		if m, ok := moneyText(deref(r.BuyPrice)); deref(r.BuyPrice) != "" && ok {
			v.BuyPrice = &m
		} else {
			note := notSet
			v.BuyPriceNote = &note
		}
		if r.NextEarnings != nil {
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
