package option_candidates_page

import (
	"slices"

	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/store"
)

// buySourceCode is the source code of a stored row's buy price (buy-price-source.md Decision 7): valuation for
// the mark buy_from_valuation, hand for buy_from_hand or a stored price with neither mark, none for no price.
// It is the one reader of the two marks; the page never recomputes which price a run used.
func buySourceCode(c store.Candidate) string {
	if deref(c.BuyPrice) == "" {
		return buypriceapi.SourceNone
	}
	marks, err := parseMarks(c.Marks)
	if err == nil && slices.Contains(marks, buypriceapi.MarkValuation) {
		return buypriceapi.SourceValuation
	}
	return buypriceapi.SourceHand
}

// buySourceBadge is the badge of a source code, nil for a code with no text.
func buySourceBadge(code string) *badgeView {
	txt, ok := buySourceTexts[code]
	if !ok {
		return nil
	}
	return &badgeView{Code: code, Label: txt.label, Gloss: txt.gloss, Tone: badgeToneInfo}
}
