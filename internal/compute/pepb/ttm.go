package pepb

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	conceptIncome  = "NetIncomeLoss"
	conceptEquity  = "StockholdersEquity"
	conceptShares  = "EntityCommonStockSharesOutstanding"
	conceptDiluted = "WeightedAverageNumberOfDilutedSharesOutstanding"
	unitUSD        = "USD"
)

// Thresholds of Decisions 3, 4 and 11.
var (
	jumpLimit     = decimal.RequireFromString("0.2")
	mismatchLimit = decimal.RequireFromString("0.25") // [T], a trial value of Decision 11
)

// pf is one stored fact with its dates and value parsed.
type pf struct {
	taxonomy, concept, unit, form, doc string
	filed, end, start                  time.Time
	hasStart                           bool
	val                                decimal.Decimal
}

// parseFacts keeps the facts the ratios read that were filed by today, oldest filed first; an unparsable value is dropped.
func parseFacts(in []contracts.FundamentalFact, today time.Time) []pf {
	var out []pf
	for _, f := range in {
		switch f.Concept {
		case conceptIncome, conceptEquity, conceptShares, conceptDiluted:
		default:
			continue
		}
		if f.Filed.After(today) {
			continue
		}
		v, err := decimal.NewFromString(f.Value)
		if err != nil {
			continue
		}
		p := pf{taxonomy: f.Taxonomy, concept: f.Concept, unit: f.Unit, form: f.Form, doc: f.DocID,
			filed: f.Filed.Time, end: f.PeriodEnd.Time, val: v}
		if f.PeriodStart != nil {
			p.start, p.hasStart = f.PeriodStart.Time, true
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].filed.Before(out[j].filed) })
	return out
}

func daysBetween(a, b time.Time) int { return int(b.Sub(a).Hours()/24 + 0.5) }

func isAnnualForm(f string) bool  { return f == "10-K" || f == "10-K/A" }
func isQuarterForm(f string) bool { return f == "10-Q" || f == "10-Q/A" }

type pkey struct{ start, end time.Time }

// ttmRes is the trailing twelve months of net income as of a cut.
type ttmRes struct {
	reason     string // empty when ok
	val        decimal.Decimal
	end, filed time.Time // of the newest piece
}

// latestByPeriod keeps, per period, the row filed last (the slice is oldest filed first).
func latestByPeriod(fs []pf, keep func(pf) bool) []pf {
	m := map[pkey]pf{}
	for _, f := range fs {
		if keep(f) {
			m[pkey{f.start, f.end}] = f
		}
	}
	out := make([]pf, 0, len(m))
	for _, f := range m {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].end.Before(out[j].end) })
	return out
}

// ttm applies Decision 3 to the facts filed on or before the cut (fs is already cut).
func ttm(fs []pf) ttmRes {
	isIncome := func(f pf) bool { return f.concept == conceptIncome && f.taxonomy == "us-gaap" && f.hasStart }
	annual := latestByPeriod(fs, func(f pf) bool { return isIncome(f) && isAnnualForm(f.form) })
	quarter := latestByPeriod(fs, func(f pf) bool { return isIncome(f) && isQuarterForm(f.form) })
	if len(annual) == 0 && len(quarter) == 0 {
		return ttmRes{reason: "no_income"}
	}
	var a *pf
	for i := range annual {
		d := daysBetween(annual[i].start, annual[i].end)
		if d >= 364 && d <= 371 {
			a = &annual[i] // ascending by end: the last one is the latest
		}
	}
	if a == nil {
		return ttmRes{reason: "no_annual"}
	}
	y, later := ytdAfter(a, quarter)
	if y == nil {
		if later {
			return ttmRes{reason: "ttm_piece_missing"}
		}
		return finishTTM(a.val, []*pf{a})
	}
	p := priorYTD(a, y, quarter)
	if p == nil {
		return ttmRes{reason: "ttm_piece_missing"}
	}
	return finishTTM(a.val.Add(y.val).Sub(p.val), []*pf{a, y, p})
}

func finishTTM(v decimal.Decimal, pieces []*pf) ttmRes {
	newest := pieces[0]
	for _, p := range pieces {
		if p.unit != unitUSD {
			return ttmRes{reason: "non_usd"}
		}
		if p.end.After(newest.end) {
			newest = p
		}
	}
	if !v.IsPositive() {
		return ttmRes{reason: "loss", val: v, end: newest.end, filed: newest.filed}
	}
	return ttmRes{val: v, end: newest.end, filed: newest.filed}
}

// eqRes is the stockholders' equity as of a cut.
type eqRes struct {
	reason     string
	val        decimal.Decimal
	end, filed time.Time
}

// equity takes the row with the latest period end, ties the latest filed (Decision 1).
func equity(fs []pf) eqRes {
	var best *pf
	for i := range fs {
		f := &fs[i]
		if f.concept != conceptEquity || f.taxonomy != "us-gaap" {
			continue
		}
		if best == nil || !f.end.Before(best.end) {
			best = f
		}
	}
	switch {
	case best == nil:
		return eqRes{reason: "no_equity"}
	case best.unit != unitUSD:
		return eqRes{reason: "non_usd"}
	case !best.val.IsPositive():
		return eqRes{reason: "negative_equity", val: best.val, end: best.end, filed: best.filed}
	}
	return eqRes{val: best.val, end: best.end, filed: best.filed}
}

// shRes is the cover share count as of a cut.
type shRes struct {
	ok       bool
	count    decimal.Decimal
	filed    time.Time
	mismatch bool
}

// coverCounts is the cover count of each filed day, the one with the latest period end,
// oldest filed first.
func coverCounts(fs []pf) []pf {
	byDay := map[time.Time]pf{}
	for _, f := range fs {
		if f.concept != conceptShares || f.taxonomy != "dei" || !f.val.IsPositive() {
			continue
		}
		if c, ok := byDay[f.filed]; !ok || !f.end.Before(c.end) {
			byDay[f.filed] = f
		}
	}
	out := make([]pf, 0, len(byDay))
	for _, f := range byDay {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].filed.Before(out[j].filed) })
	return out
}

// shares is the latest cover count filed, with the Decision 11 check against its own filing.
func shares(fs []pf) shRes {
	cc := coverCounts(fs)
	if len(cc) == 0 {
		return shRes{}
	}
	c := cc[len(cc)-1]
	r := shRes{ok: true, count: c.val, filed: c.filed}
	var d *pf
	for i := range fs {
		f := &fs[i]
		if f.doc != c.doc || f.concept != conceptDiluted || !f.val.IsPositive() {
			continue
		}
		if d == nil || f.end.After(d.end) || (f.end.Equal(d.end) && dur(f) < dur(d)) {
			d = f
		}
	}
	if d != nil && c.val.Sub(d.val).Abs().Div(d.val).GreaterThan(mismatchLimit) {
		r.mismatch = true
	}
	return r
}

func dur(f *pf) int {
	if !f.hasStart {
		return 0
	}
	return daysBetween(f.start, f.end)
}

// jumpCut is the filed day of the latest cover count that differs from the one before it by
// more than 20 percent (Decision 4); days before it have no figure. The newest count has no
// later one, so it is only ever the later of a pair. ok is false when there is no jump.
func jumpCut(fs []pf) (time.Time, bool) {
	cc := coverCounts(fs)
	var cut time.Time
	found := false
	for i := 1; i < len(cc); i++ {
		if cc[i].val.Sub(cc[i-1].val).Abs().Div(cc[i-1].val).GreaterThan(jumpLimit) {
			cut, found = cc[i].filed, true
		}
	}
	return cut, found
}

// reasonWords is the card text of a reason; kind is "PE" or "PB".
func reasonWords(kind, reason string) string {
	switch reason {
	case "loss":
		return "loss, no PE"
	case "negative_equity":
		return "negative equity, no PB"
	case "no_annual":
		return kind + ": no 10-K stored"
	case "ttm_piece_missing":
		return kind + ": a piece of the trailing twelve months is not stored"
	case "no_income":
		return kind + ": no net income stored"
	case "no_equity":
		return kind + ": no stockholders' equity stored"
	case "no_shares":
		return kind + ": no share count stored"
	case "no_price":
		return kind + ": no price bar stored"
	case "price_stale":
		return kind + ": newest price bar is stale"
	case "non_usd":
		return kind + ": facts not in USD"
	case "share_count_jump":
		return kind + ": share count jumped, no figure before it"
	case "share_count_mismatch":
		return kind + ": share count differs from the diluted count"
	}
	return kind + ": " + reason
}

func stateOf(reason string) string {
	if reason == "loss" || reason == "negative_equity" {
		return pepbapi.StateLoss
	}
	return pepbapi.StateMissing
}

// ytdAfter is the latest year-to-date row that starts 1 to 7 days after A ends; later says
// whether any 10-Q row ends after A, so a missing Y can be told from no 10-Q yet.
func ytdAfter(a *pf, quarter []pf) (y *pf, later bool) {
	for i := range quarter {
		q := &quarter[i]
		if q.end.After(a.end) {
			later = true
		}
		if g := daysBetween(a.end, q.start); g >= 1 && g <= 7 {
			y = q
		}
	}
	return y, later
}

// priorYTD is the row that starts at A's start and ends 364 to 371 days before Y's end.
func priorYTD(a, y *pf, quarter []pf) (p *pf) {
	for i := range quarter {
		q := &quarter[i]
		if !q.start.Equal(a.start) {
			continue
		}
		if g := daysBetween(q.end, y.end); g >= 364 && g <= 371 {
			p = q
		}
	}
	return p
}
