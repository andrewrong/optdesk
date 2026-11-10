package option_boards

import (
	"fmt"
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

// earningsTag is "E MM-DD" for a listed date, "E 无" for absent and "E 缺" otherwise.
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

// deltaText is "−0.15" for a stored delta, "缺" when it is null or not a decimal.
func deltaText(delta *string) string {
	if delta == nil || !decimalPattern.MatchString(*delta) {
		return gapText
	}
	return signedText(*delta)
}

// ivLine is "IV 38% · HV30 缺 · IVP 缺": HV30 and IVP have no producer and are always 缺.
func ivLine(iv *string) string {
	pct := gapText
	if iv != nil {
		if s, err := shiftPercent(*iv); err == nil {
			pct = s
		}
	}
	return "IV " + pct + " · HV30 缺 · IVP 缺"
}

// annualisedOf is the PercentAmount of the stored yield, or a gap when it is null or not a decimal; the text
// is the figure or "缺".
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

func buildCards(rows []store.Candidate, eod map[int]contracts.OptionEod) []opportunity {
	out := make([]opportunity, 0, len(rows))
	for _, r := range rows {
		out = append(out, buildCard(r, eod))
	}
	return out
}

func buildCard(r store.Candidate, eod map[int]contracts.OptionEod) opportunity {
	md, _ := monthDay(r.Expiry)
	wd, _ := weekdayText(r.Expiry)
	ann, annText := annualisedOf(r.AnnualizedYield)
	e := eodTextFor(eod, r.InstrumentID)

	tags := []string{earningsTag(r)}
	reason := gatesPassed
	if marks, err := parseMarks(r.Marks); err == nil {
		badges := markBadges(marks)
		tags = append(tags, badges...)
		for _, b := range badges {
			reason += " · " + b
		}
	}
	return opportunity{
		ID:             fmt.Sprintf("c%d", r.InstrumentID),
		Symbol:         r.Issuer,
		Verdict:        verdictOf(r.Status),
		Reason:         reason,
		Annualised:     ann,
		AnnualisedText: annText,
		ContractLine:   fmt.Sprintf("sell put %s · %s %s · %d 天 · Δ %s", r.Strike, md, wd, r.DTE, deltaText(r.Delta)),
		LiquidityLine:  fmt.Sprintf("成交 %s · 持仓 %s", e.Volume, e.OpenInterest),
		Tags:           tags,
		SizeLine:       fmt.Sprintf("1 张 · 行权价 %s · 乘数 %d", r.Strike, r.Multiplier),
		IvLine:         ivLine(r.Iv),
		PremiumLine:    fmt.Sprintf("权利金 %s (收盘 bid 估) · 盈亏平衡 %s · 到期在价内概率 缺 · 比现金高 缺", r.BidEod, r.BreakEven),
	}
}
