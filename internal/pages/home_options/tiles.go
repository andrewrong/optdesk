package home_options

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

// The texts of the three figure tiles. The figures come from blockapi.OptionTiles; this file only places
// them: the count, the dollar sign, the thousands separators and the captions.
const (
	reasonNoData   = "no_data"
	reasonCurrency = "unsupported_currency"

	textLots          = "lots opened this month"
	textRemaining     = "remaining part of each lot"
	textNoOpen        = "no open structures in the ledger"
	textNetPaid       = "Net paid, not shown"
	textNoFigure      = "No figure from the ledger"
	textAtLeast       = "At least: %d structures have no loss number"
	textOtherCurrency = "%d non-USD structures not counted"
	textUnreadable    = "%d structures with unreadable rows not counted"
	textOnlyOtherCcy  = "Only non-USD structures are open (%d), not converted"
	textUncovered     = "%d calls without assigned shares in the ledger; other structures at least $%s"
	textReview        = "%d ledger rows need review, figures may be incomplete"
	captionBase       = "%s, all accounts combined"
	captionSep        = "; "
	unitStructures    = "structures"
)

// countFigure is the CountFigure of the schema.
type countFigure struct {
	Kind  string `json:"kind"`
	Value int    `json:"value"`
	Text  string `json:"text"`
	Unit  string `json:"unit"`
}

// usdFigure is the unsigned UsdFigure of the schema.
type usdFigure struct {
	Kind  string      `json:"kind"`
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

// figureState is what Build computed for the figure tiles: on is true when both Structures and Tiles are set.
type figureState struct {
	on     bool
	out    blockapi.OptionTilesOut
	review int
}

// placeFigures sets the three tiles from the figures; asOf is the text "Ledger as of MM-DD".
func placeFigures(t *tiles, asOf string, o blockapi.OptionTilesOut) {
	base := fmt.Sprintf(captionBase, asOf)
	extra := ""
	if o.OtherCurrency > 0 {
		extra += captionSep + fmt.Sprintf(textOtherCurrency, o.OtherCurrency)
	}
	if o.Unreadable > 0 {
		extra += captionSep + fmt.Sprintf(textUnreadable, o.Unreadable)
	}
	t.Sold = tile{
		Label:   t.Sold.Label,
		Figure:  countFigure{Kind: "value", Value: o.SoldCount, Text: fmt.Sprint(o.SoldCount), Unit: unitStructures},
		Caption: base + captionSep + textLots,
	}
	prem := base + captionSep + textRemaining
	if o.OpenCount == 0 {
		prem = base + captionSep + textNoOpen
	}
	t.OpenPremium = tile{
		Label:   t.OpenPremium.Label,
		Figure:  openFigure(o),
		Caption: prem + extra,
	}
	loss := base
	if o.OpenCount == 0 {
		loss += captionSep + textNoOpen
	}
	if o.MaxLossGap == "" && o.Unnumbered > 0 {
		loss += captionSep + fmt.Sprintf(textAtLeast, o.Unnumbered)
	}
	t.MaxLoss = tile{Label: t.MaxLoss.Label, Figure: lossFigure(o), Caption: loss + extra}
}

func openFigure(o blockapi.OptionTilesOut) any {
	switch o.OpenPremiumGap {
	case "":
		return usd2(o.OpenPremium)
	case reasonCurrency:
		return gap{Kind: "gap", Reason: reasonCurrency, Text: fmt.Sprintf(textOnlyOtherCcy, o.OtherCurrency)}
	}
	return noData(textNetPaid)
}

func lossFigure(o blockapi.OptionTilesOut) any {
	switch {
	case o.MaxLossGap == reasonCurrency:
		return gap{Kind: "gap", Reason: reasonCurrency, Text: fmt.Sprintf(textOnlyOtherCcy, o.OtherCurrency)}
	case o.MaxLossGap != "" && o.Uncovered > 0:
		rest := o.MaxLossUsd.Dollars
		if rest == "" {
			rest = "0"
		}
		return noData(fmt.Sprintf(textUncovered, o.Uncovered, group(rest)))
	case o.MaxLossGap != "":
		return noData(textNoFigure)
	}
	return usd2(o.MaxLossUsd)
}

// usd2 places one UsdText; an empty one is zero.
func usd2(u blockapi.UsdText) usdFigure {
	if u.Value == "" || u.Dollars == "" {
		u = blockapi.UsdText{Value: "0.00", Dollars: "0"}
	}
	return usdFigure{Kind: "value", Value: json.Number(u.Value), Text: "$" + group(u.Dollars), Unit: "usd"}
}

// group puts a comma between the thousands of a digit text.
func group(digits string) string {
	var parts []string
	for len(digits) > 3 {
		parts = append([]string{digits[len(digits)-3:]}, parts...)
		digits = digits[:len(digits)-3]
	}
	return strings.Join(append([]string{digits}, parts...), ",")
}

// placeFigureState applies the figures to the card: the tiles, the card gap that leaves and the review line.
// With figures off the card is untouched.
func placeFigureState(k *card, fs figureState, baseText string, d time.Time, noRows bool) {
	if !fs.on {
		return
	}
	asOf := "Ledger as of " + d.Format("01-02")
	if baseText != textNoDay {
		asOf = baseText
	}
	k.Gaps = slices.DeleteFunc(k.Gaps, func(g gap) bool { return g.Text == textMaxLoss })
	if !noRows {
		placeFigures(&k.Tiles, asOf, fs.out)
	}
	if fs.review > 0 {
		k.Gaps = append(k.Gaps, noData(fmt.Sprintf(textReview, fs.review)))
	}
}
