package option_boards

import (
	"fmt"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// maxCards is the most cards the page shows (Decision 4).
const maxCards = 6

// The stored candidate statuses.
const (
	statusOK      = "ok"
	statusUnknown = "unknown"
)

// verdictOf is the verdict of a stored status only: ok is worth, unknown is pending, anything else (blocked)
// is skip. The marginal key needs a judgement or a score and is never written.
func verdictOf(status string) verdict {
	switch status {
	case statusOK:
		return verdict{Key: "worth", Label: verdictWorth, Tone: "good"}
	case statusUnknown:
		return verdict{Key: "pending", Label: verdictPending, Tone: "warn"}
	}
	return verdict{Key: "skip", Label: verdictSkip, Tone: "muted"}
}

// cardRows are the ok rows whose RankInUnderlying is 1, in the stored order, at most maxCards.
func cardRows(rows []store.Candidate) []store.Candidate {
	out := []store.Candidate{}
	for _, r := range rows {
		if len(out) == maxCards {
			break
		}
		if r.Status == statusOK && r.RankInUnderlying != nil && *r.RankInUnderlying == 1 {
			out = append(out, r)
		}
	}
	return out
}

// earningsTag is "E MM-DD" for a listed date, "E none" for absent and "E n/a" otherwise.
func earningsTag(r store.Candidate) string {
	switch {
	case r.EarningsState == "listed" && r.NextEarnings != nil:
		if md, err := monthDay(*r.NextEarnings); err == nil {
			return "E " + md
		}
	case r.EarningsState == "absent":
		return earnAbsent
	}
	return earnMissing
}

// optionDeltaText is "−0.15" for a stored delta, "n/a" when it is null or not a decimal.
func optionDeltaText(delta *string) string {
	if delta == nil || !decimalPattern.MatchString(*delta) {
		return gapText
	}
	return signedText(*delta)
}

// ivLine is "IV 38% · HV30 n/a · IVP n/a": HV30 and IVP have no producer and are always n/a.
func ivLine(iv *string) string {
	pct := gapText
	if iv != nil {
		if s, err := shiftPercent(*iv); err == nil {
			pct = s
		}
	}
	return "IV " + pct + " · HV30 n/a · IVP n/a"
}

// annualisedOf is the PercentAmount of the stored yield, or a gap when it is null or not a decimal; the text
// is the figure or "n/a".
func annualisedOf(yield *string) (cell any, text string) {
	if yield != nil {
		if s, err := shiftPercent(*yield); err == nil {
			if n, ok := numberLiteral(strings.TrimSuffix(s, "%")); ok {
				return percentAmount{Value: n, Text: s, Unit: "percent"}, s
			}
		}
	}
	return mkGap("no_data", gapText), gapText
}

func buildOptionCards(rows []store.Candidate, eod map[int]contracts.OptionEod, names displayNames) []opportunity {
	out := make([]opportunity, 0, len(rows))
	for _, r := range rows {
		out = append(out, buildCard(r, eod, names))
	}
	return out
}

func buildCard(r store.Candidate, eod map[int]contracts.OptionEod, names displayNames) opportunity {
	md, _ := monthDay(r.Expiry)
	wd, _ := weekdayText(r.Expiry)
	ann, annText := annualisedOf(r.AnnualizedYield)
	e := eodTextFor(eod, r.InstrumentID)

	tags := []string{earningsTag(r)}
	reason := gatesPassed
	if marks, err := parseMarks(r.Marks); err == nil {
		badges := markBadges(marks)
		tags = append(tags, badges...)
		if len(badges) > 0 {
			reason = passedWithMarks + strings.Join(badges, ", ")
		}
	}
	return opportunity{
		ID:             fmt.Sprintf("c%d", r.InstrumentID),
		Symbol:         names.of(r),
		Verdict:        verdictOf(r.Status),
		Reason:         reason,
		Annualised:     ann,
		AnnualisedText: annText,
		ContractLine:   fmt.Sprintf("sell put %s · %s %s · %d days · Δ %s", r.Strike, md, wd, r.DTE, optionDeltaText(r.Delta)),
		LiquidityLine:  fmt.Sprintf("Volume %s · Open interest %s", e.Volume, e.OpenInterest),
		Tags:           tags,
		SizeLine:       fmt.Sprintf("1 contract · Strike %s · Multiplier %d", r.Strike, r.Multiplier),
		IvLine:         ivLine(r.Iv),
		PremiumLine:    fmt.Sprintf("Premium %s (close bid est.) · Breakeven %s · in-the-money probability at expiry n/a · above cash n/a", r.BidEod, r.BreakEven),
	}
}

// worthCount is the number of cards that are not in the money with no buy price.
func worthCount(rows []store.Candidate) int {
	n := 0
	for _, r := range rows {
		marks, err := parseMarks(r.Marks)
		if err == nil && slices.Contains(marks, "itm") && slices.Contains(marks, "no_buy_price") {
			continue
		}
		n++
	}
	return n
}
