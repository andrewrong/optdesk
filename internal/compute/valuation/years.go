package valuation

import (
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const dayLayout = "2006-01-02"

// fiscalYear is one eligible year: its first and last day.
type fiscalYear struct {
	start, end time.Time
}

func dayText(t time.Time) contracts.Day { return contracts.Day(t.UTC().Format(dayLayout)) }

func parseDay(d contracts.Day) (time.Time, bool) {
	t, err := time.Parse(dayLayout, string(d))
	return t, err == nil
}

// factIndex finds facts by what identifies a figure: taxonomy, concept, unit, period.
type factIndex map[string][]contracts.FundamentalFact

func indexKey(taxonomy, concept, unit string, start *time.Time, end time.Time) string {
	s := ""
	if start != nil {
		s = start.UTC().Format(dayLayout)
	}
	return strings.Join([]string{taxonomy, concept, unit, s, end.UTC().Format(dayLayout)}, "|")
}

func newFactIndex(facts []contracts.FundamentalFact) factIndex {
	ix := factIndex{}
	for _, f := range facts {
		var start *time.Time
		if f.PeriodStart != nil {
			t := f.PeriodStart.Time
			start = &t
		}
		k := indexKey(f.Taxonomy, f.Concept, f.Unit, start, f.PeriodEnd.Time)
		ix[k] = append(ix[k], f)
	}
	return ix
}

// asOfValue is the as-of rule: of the facts for one figure, those filed on the latest
// day win, whatever their form (a later 8-K recast replaces the value). found is false
// when there is none; conflict is true when facts of that day disagree.
func (ix factIndex) asOfValue(taxonomy, concept, unit string, start *time.Time, end time.Time) (value decimal.Decimal, found, conflict bool) {
	var latest time.Time
	var vals []decimal.Decimal
	for _, f := range ix[indexKey(taxonomy, concept, unit, start, end)] {
		v, err := decimal.NewFromString(f.Value)
		if err != nil {
			continue
		}
		switch filed := f.Filed.Time; {
		case len(vals) == 0 || filed.After(latest):
			latest, vals = filed, []decimal.Decimal{v}
		case filed.Equal(latest):
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return decimal.Zero, false, false
	}
	for _, v := range vals[1:] {
		if !v.Equal(vals[0]) {
			return decimal.Zero, false, true
		}
	}
	return vals[0], true, false
}

func isAnnualForm(form string, p Params) bool {
	for _, f := range p.AnnualForms {
		if f == form {
			return true
		}
	}
	return false
}

// eligibleYears returns the latest p.Years fiscal years, newest first. A year is an
// annual-form fact of the year role whose period is 350 to 380 days long; a fact of any
// other form (a quarterly report, an 8-K) never makes a year, and a later recast does not
// remove one. The start of a year comes from the latest filed such fact.
func eligibleYears(facts []contracts.FundamentalFact, p Params) []fiscalYear {
	role := p.role(p.YearRole)
	concepts := map[string]bool{}
	for _, c := range role.Concepts {
		concepts[c] = true
	}
	type pick struct {
		start time.Time
		filed time.Time
		doc   string
	}
	byEnd := map[time.Time]pick{}
	for _, f := range facts {
		if f.Taxonomy != p.Taxonomy || f.Unit != role.Unit || !concepts[f.Concept] ||
			!isAnnualForm(f.Form, p) || f.PeriodStart == nil {
			continue
		}
		start, end := f.PeriodStart.UTC(), f.PeriodEnd.UTC()
		days := int(end.Sub(start).Hours() / 24)
		if days < p.AnnualDaysMin || days > p.AnnualDaysMax {
			continue
		}
		c := pick{start: start, filed: f.Filed.Time, doc: f.DocID}
		old, seen := byEnd[end]
		if !seen || c.filed.After(old.filed) ||
			(c.filed.Equal(old.filed) && (c.doc > old.doc || (c.doc == old.doc && c.start.After(old.start)))) {
			byEnd[end] = c
		}
	}
	years := make([]fiscalYear, 0, len(byEnd))
	for end, c := range byEnd {
		years = append(years, fiscalYear{start: c.start, end: end})
	}
	sort.Slice(years, func(i, j int) bool { return years[i].end.After(years[j].end) })
	if len(years) > p.Years {
		years = years[:p.Years]
	}
	return years
}

// consecutiveRun returns the newest years (newest first) that each start the day after the
// previous one ends, and whether the latest minYears of them are such a run.
func consecutiveRun(years []fiscalYear, minYears int) (run []fiscalYear, ok bool) {
	if len(years) == 0 {
		return nil, false
	}
	run = years[:1]
	for _, y := range years[1:] {
		prev := run[len(run)-1]
		if !prev.start.Equal(y.end.AddDate(0, 0, 1)) {
			break
		}
		run = append(run, y)
	}
	return run, len(run) >= minYears
}
