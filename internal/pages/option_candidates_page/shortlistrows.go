package option_candidates_page

import (
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
	"github.com/andrewrong/optdesk/internal/store"
)

type shortRow struct {
	Key        string      `json:"key"`
	Strike     pairView    `json:"strike"`
	Expiry     pairView    `json:"expiry"`
	OTM        pairView    `json:"otm"`
	Delta      pairView    `json:"delta"`
	Premium    pairView    `json:"premium"`
	Annualized pairView    `json:"annualized"`
	Buy        pairView    `json:"buy"`
	BuySource  *badgeView  `json:"buy_source"`
	BuyLink    *string     `json:"buy_link"`
	Badges     []badgeView `json:"badges"`
}

type shortGroup struct {
	Ticker  string     `json:"ticker"`
	Spot    string     `json:"spot"`
	Summary string     `json:"summary"`
	Rows    []shortRow `json:"rows"`
}

// buySourceOf is the code of the source of the row's buy price, "" when no source gives one. Today the only
// source is the price the owner set by hand; a later design adds codes here and in buySourceTexts.
func buySourceOf(c store.Candidate) string {
	if deref(c.BuyPrice) != "" {
		return buySourceHand
	}
	return ""
}

// shortRowOf writes one placed row from its stored row and, when the tier view has it, its tier row.
func shortRowOf(p optionshortlistapi.Placed, c store.Candidate, t *tierRow) shortRow {
	r := shortRow{
		Key: p.Key, Strike: moneyPair(c.Strike, unreadable),
		Expiry:     pairView{Main: string(c.Expiry), Sub: subOf(strconv.Itoa(c.DTE) + dteSuffix)},
		OTM:        otmPair(p.OTM),
		Delta:      deltaPair(c, p.AbsDelta),
		Annualized: annualizedPair(c),
		Premium:    premiumOf(c, t),
		Badges:     []badgeView{},
	}
	r.Buy, r.BuySource, r.BuyLink = buyOf(c, t)
	if p.Liquidity == slWideSpreadID {
		r.Badges = append(r.Badges, badgeView{Code: slWideSpreadID, Label: slWideSpread, Gloss: zhWideSpread, Tone: badgeToneWarn})
	}
	return r
}

func moneyPair(text, none string) pairView {
	if text == "" {
		return gapPair("no_data", none)
	}
	m, ok := moneyText(text)
	if !ok {
		return gapPair("unreadable", unreadable)
	}
	return pairView{Main: m}
}

func otmPair(text string) pairView {
	if text == "" {
		return gapPair("no_data", noSpotText)
	}
	f, ok := fixedText(text, 1)
	if !ok {
		return gapPair("unreadable", unreadable)
	}
	return pairView{Main: minusText(f) + slPct}
}

func deltaPair(c store.Candidate, abs string) pairView {
	if deref(c.Delta) == "" || abs == "" {
		return gapPair("missing_input", slNoDelta)
	}
	f, ok := fixedText(abs, 2)
	if !ok {
		return gapPair("unreadable", unreadable)
	}
	return pairView{Main: f}
}

func annualizedPair(c store.Candidate) pairView {
	text := deref(c.AnnualizedYield)
	if text == "" {
		return gapPair("no_data", slNoYield)
	}
	p, ok := percentText(text)
	if !ok {
		return gapPair("unreadable", unreadable)
	}
	return pairView{Main: p}
}

// premiumOf is the bid with the money per contract of the tier verdict under it.
func premiumOf(c store.Candidate, t *tierRow) pairView {
	p := moneyPair(c.BidEod, gapNoBid)
	if p.Gap == nil && t != nil && t.Premium.Gap == nil {
		p.Sub = subOf(t.Premium.Main)
	}
	return p
}

// buyOf is the buy price with its break-even and margin under it, the source badge, and the link to the
// price lines, which exists only when no source gives a price.
func buyOf(c store.Candidate, t *tierRow) (pairView, *badgeView, *string) {
	code := buySourceOf(c)
	if code == "" {
		link := slPriceLines
		return gapPair("no_data", slNoBuyPrice), nil, &link
	}
	p := moneyPair(deref(c.BuyPrice), slNoBuyPrice)
	if p.Gap == nil && t != nil && t.Margin.Gap == nil && t.Margin.Sub != nil {
		p.Sub = subOf(*t.Margin.Sub + slMarginJoin + t.Margin.Main)
	}
	txt := buySourceTexts[code]
	return p, &badgeView{Code: code, Label: txt.label, Gloss: txt.gloss, Tone: badgeToneInfo}, nil
}
