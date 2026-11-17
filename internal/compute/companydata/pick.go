package companydata

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// The year band is the one of internal/compute/pepb/ttm.go:102-105, repeated on purpose
// (Decision 3 of docs/design/data-collection.md): this package does not import pepb.
const (
	yearMin = 364
	yearMax = 371
)

// row is one stored fact with its dates parsed.
type row struct {
	concept, unit, doc, fp string
	fy                     *int
	start, end, filed      time.Time
	hasStart               bool
	val                    decimal.Decimal
}

// pkey is the period of a fact; start is the zero time for an instant.
type pkey struct{ start, end time.Time }

// operand is the pick of one period: the latest filed row, and what the other rows say about it.
type operand struct {
	val      decimal.Decimal
	filed    time.Time // of the picked row
	restated bool      // rows of the period hold different values
	changed  bool      // the picked value differs from the first filed one
}

// filing is the original filing of a period end: the earliest filed of the facts with that end.
type filing struct {
	filed time.Time
	fy    *int
	fp    string
}

// daysBetween counts the days from a to b with both ends in, so a 52 week year that starts on
// 2024-09-29 and ends on 2025-09-27 is 364 days (the way the filings write a fiscal year).
func daysBetween(a, b time.Time) int { return int(b.Sub(a).Hours()/24+0.5) + 1 }

// later reports whether a was filed after b; ties go to the larger doc id.
func later(a, b row) bool {
	if !a.filed.Equal(b.filed) {
		return a.filed.After(b.filed)
	}
	return a.doc > b.doc
}

// parse reads every fact with parseable dates and value; the input is not changed.
func parse(facts []contracts.FundamentalFact) []row {
	out := make([]row, 0, len(facts))
	for _, f := range facts {
		v, err := decimal.NewFromString(f.Value)
		if err != nil {
			continue
		}
		r := row{concept: f.Concept, unit: f.Unit, doc: f.DocID, fp: f.Fp, fy: f.Fy,
			end: f.PeriodEnd.Time, filed: f.Filed.Time, val: v}
		if f.Taxonomy != "us-gaap" {
			r.concept = "" // kept for the filing date only
		}
		if f.PeriodStart != nil {
			r.start, r.hasStart = f.PeriodStart.Time, true
		}
		out = append(out, r)
	}
	return out
}

// pick chooses the row filed last of one period (ties by the larger doc id) and compares the rest
// with it. rows is not changed.
func pick(rows []row) operand {
	best, first := rows[0], rows[0]
	for _, r := range rows[1:] {
		if later(r, best) {
			best = r
		}
		if later(first, r) {
			first = r
		}
	}
	op := operand{val: best.val, filed: best.filed, changed: !best.val.Equal(first.val)}
	for _, r := range rows {
		if !r.val.Equal(best.val) {
			op.restated = true
		}
	}
	return op
}

// consistent keeps a subtraction on one basis: an operand whose picked value changed needs every
// other operand picked from a filing filed on or after the restating one (Decision 4).
func consistent(ops ...operand) bool {
	for i, a := range ops {
		if !a.changed {
			continue
		}
		for j, b := range ops {
			if i != j && b.filed.Before(a.filed) {
				return false
			}
		}
	}
	return true
}

// originals finds the original filing of every period end.
func originals(rows []row) map[time.Time]filing {
	first := map[time.Time]time.Time{}
	for _, r := range rows {
		if f, ok := first[r.end]; !ok || r.filed.Before(f) {
			first[r.end] = r.filed
		}
	}
	out := map[time.Time]filing{}
	for _, r := range rows {
		if !r.filed.Equal(first[r.end]) {
			continue
		}
		f := out[r.end]
		f.filed = r.filed
		if f.fy == nil {
			f.fy = r.fy
		}
		if f.fp == "" {
			f.fp = r.fp
		}
		out[r.end] = f
	}
	return out
}

// index groups the facts of the wanted concepts and units by concept and period.
func index(rows []row) map[string]map[pkey][]row {
	idx := map[string]map[pkey][]row{}
	for _, r := range rows {
		if r.concept == "" || wantedUnit[r.concept] != r.unit {
			continue
		}
		g := idx[r.concept]
		if g == nil {
			g = map[pkey][]row{}
			idx[r.concept] = g
		}
		k := pkey{r.start, r.end}
		g[k] = append(g[k], r)
	}
	return idx
}

func sortedDays(m map[time.Time]bool) []time.Time {
	out := make([]time.Time, 0, len(m))
	for d := range m {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}
