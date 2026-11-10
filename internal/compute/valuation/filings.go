package valuation

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Refusal reasons of Decision 5.
const (
	reasonNoStandard       = "no_standard"
	reasonNoSIC            = "no_sic"
	reasonFinancialSIC     = "financial_sic"
	reasonCyclicalSIC      = "cyclical_sic"
	reasonFewerYears       = "fewer_than_5_years"
	reasonNonConsecutive   = "non_consecutive_years"
	reasonShareCountJump   = "share_count_jump"
	reasonStaleLatestYear  = "stale_latest_year"
	reasonUnsupportedStd   = "unsupported_standard"
	reasonBadAsOf          = "bad_as_of"
	reasonInputConflictFmt = "input_conflict:%s:%s"
)

const (
	sicOK      = "ok"
	sicMissing = "missing"
)

// StandardOf reads the accounting standard of a company from its own facts: US_GAAP when
// a fact has source "sec"; else the unit of the latest fact with taxonomy "dei" and concept
// "AccountingStandard" (J_GAAP or IFRS). None gives the empty Standard. It implements
// valuationapi.StandardOfFunc.
func StandardOf(facts []contracts.FundamentalFact) valuationapi.Standard {
	var best *contracts.FundamentalFact
	for i := range facts {
		f := &facts[i]
		if f.Source == "sec" {
			return valuationapi.USGAAP
		}
		if f.Taxonomy != "dei" || f.Concept != "AccountingStandard" {
			continue
		}
		if u := valuationapi.Standard(f.Unit); u != valuationapi.JGAAP && u != valuationapi.IFRS {
			continue
		}
		if best == nil || f.Filed.After(best.Filed.Time) {
			best = f
		}
	}
	if best == nil {
		return ""
	}
	return valuationapi.Standard(best.Unit)
}

// BuildFilings turns the facts of one company into its fiscal years as of asOf (see
// eligibleYears for the year rule and factIndex.asOfValue for the value rule), or a
// refusal with reasons. It implements valuationapi.BuildFilingsFunc. Facts filed after
// asOf are ignored.
func BuildFilings(facts []contracts.FundamentalFact, asOf contracts.Day, std valuationapi.Standard) valuationapi.FilingsResult {
	res := valuationapi.FilingsResult{SICState: sicMissing, State: valuationapi.StateMissingInput}
	if std == "" {
		res.Reasons = []string{reasonNoStandard}
		return res
	}
	p, ok := ParamsFor(std)
	if !ok {
		res.Reasons = []string{reasonUnsupportedStd}
		return res
	}
	cutoff, ok := parseDay(asOf)
	if !ok {
		res.Reasons = []string{reasonBadAsOf}
		return res
	}
	known := make([]contracts.FundamentalFact, 0, len(facts))
	for _, f := range facts {
		if !f.Filed.After(cutoff) {
			known = append(known, f)
		}
	}
	ix := newFactIndex(known)

	var reasons []string
	unvalued := false
	res.SIC, res.SICState, reasons, unvalued = readSIC(known, p)

	years := eligibleYears(known, p)
	filings := valuationapi.Filings{Standard: std, Currency: p.Currency, SIC: res.SIC}
	switch {
	case len(years) < p.MinYears:
		reasons = append(reasons, reasonFewerYears)
	default:
		run, consecutive := consecutiveRun(years, p.MinYears)
		if !consecutive {
			reasons = append(reasons, reasonNonConsecutive)
		}
		var conflicts []string
		filings.Years, conflicts = yearFigures(ix, p, run)
		reasons = append(reasons, conflicts...)
		if consecutive {
			reasons = append(reasons, checkYears(filings.Years, run, cutoff, p)...)
		}
		fillShares(&filings, known, p)
	}
	res.Filings = filings
	res.Reasons = reasons
	switch {
	case len(reasons) == 0:
		res.State = valuationapi.StateOK
	case unvalued:
		res.State = valuationapi.StateUnvalued
	}
	return res
}

// readSIC reads the SIC fact. unvalued is true when the code is in a financial or
// cyclical range.
func readSIC(facts []contracts.FundamentalFact, p Params) (sic, state string, reasons []string, unvalued bool) {
	var best *contracts.FundamentalFact
	for i := range facts {
		f := &facts[i]
		if f.Taxonomy != p.SIC.Taxonomy || f.Concept != p.SIC.Concept || f.Unit != p.SIC.Unit {
			continue
		}
		if best == nil || f.PeriodEnd.After(best.PeriodEnd.Time) ||
			(f.PeriodEnd.Equal(best.PeriodEnd.Time) && f.Filed.After(best.Filed.Time)) {
			best = f
		}
	}
	if best != nil {
		if v, err := decimal.NewFromString(best.Value); err == nil && v.IsInteger() && v.Sign() >= 0 {
			code := int(v.IntPart())
			switch {
			case inRanges(code, p.FinancialSIC):
				reasons, unvalued = append(reasons, reasonFinancialSIC), true
			case inRanges(code, p.CyclicalSIC):
				reasons, unvalued = append(reasons, reasonCyclicalSIC), true
			}
			return fmt.Sprintf("%04d", code), sicOK, reasons, unvalued
		}
	}
	return "", sicMissing, []string{reasonNoSIC}, false
}

func inRanges(code int, ranges [][2]int) bool {
	for _, r := range ranges {
		if r[0] <= code && code <= r[1] {
			return true
		}
	}
	return false
}

// yearFigures reads every role of every year of the run, newest first. A role with no
// fact is absent from the year's Values; a conflict (facts of the latest filed day that
// disagree) is reported and leaves the role absent.
func yearFigures(ix factIndex, p Params, run []fiscalYear) ([]valuationapi.FilingYear, []string) {
	var conflicts []string
	out := make([]valuationapi.FilingYear, 0, len(run))
	for _, y := range run {
		fy := valuationapi.FilingYear{FYEnd: dayText(y.end), Values: map[string]string{}}
		var filed time.Time
		for _, r := range p.Roles {
			start := &y.start
			if r.Instant {
				start = nil
			}
			for _, concept := range r.Concepts {
				v, found, conflict := ix.asOfValue(p.Taxonomy, concept, r.Unit, start, y.end)
				if conflict {
					conflicts = append(conflicts, fmt.Sprintf(reasonInputConflictFmt, r.Name, dayText(y.end)))
					break
				}
				if found {
					fy.Values[r.Name] = v.String()
					if r.Name == p.YearRole {
						filed = latestFiled(ix, p, concept, r, start, y.end)
					}
					break
				}
			}
		}
		fy.Filed = dayText(filed)
		out = append(out, fy)
	}
	return out, conflicts
}

func latestFiled(ix factIndex, p Params, concept string, r Role, start *time.Time, end time.Time) time.Time {
	var latest time.Time
	for _, f := range ix[indexKey(p.Taxonomy, concept, r.Unit, start, end)] {
		if f.Filed.After(latest) {
			latest = f.Filed.Time
		}
	}
	return latest
}

// checkYears applies the stale-year and share-count rules to a consecutive run.
func checkYears(years []valuationapi.FilingYear, run []fiscalYear, cutoff time.Time, p Params) []string {
	var reasons []string
	if int(cutoff.Sub(run[0].end).Hours()/24) > p.MaxAgeDays {
		reasons = append(reasons, reasonStaleLatestYear)
	}
	// Oldest to newest: compare each year's diluted shares with the year before.
	var prev decimal.Decimal
	havePrev := false
	for i := len(years) - 1; i >= 0; i-- {
		text, ok := years[i].Values[roleDilutedShares]
		if !ok {
			havePrev = false
			continue
		}
		cur, err := decimal.NewFromString(text)
		if err != nil {
			havePrev = false
			continue
		}
		if havePrev {
			ratio, ok := div(cur, prev)
			if !ok || ratio.Sub(decimal.NewFromInt(1)).Abs().GreaterThan(p.ShareJump) {
				reasons = append(reasons, reasonShareCountJump)
				break
			}
		}
		prev, havePrev = cur, true
	}
	return reasons
}

// fillShares sets the latest diluted share count and the latest cover-page count.
func fillShares(f *valuationapi.Filings, facts []contracts.FundamentalFact, p Params) {
	if len(f.Years) > 0 {
		f.SharesOutstanding = f.Years[0].Values[roleDilutedShares]
	}
	var best *contracts.FundamentalFact
	for i := range facts {
		c := &facts[i]
		if c.Taxonomy != p.CoverShares.Taxonomy || c.Concept != p.CoverShares.Concept || c.Unit != p.CoverShares.Unit {
			continue
		}
		if best == nil || c.PeriodEnd.After(best.PeriodEnd.Time) ||
			(c.PeriodEnd.Equal(best.PeriodEnd.Time) && c.Filed.After(best.Filed.Time)) {
			best = c
		}
	}
	if best != nil {
		if v, err := decimal.NewFromString(best.Value); err == nil {
			f.CoverShares = v.String()
		}
	}
}
