package bookrisk

import (
	"encoding/json"
	"math/big"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
	"github.com/andrewrong/optdesk/internal/strategy"
)

// basisNote is the sentence every page prints beside a figure (design Decision 2); it is a fact of the row.
const basisNote = "this is the broker-dealer portfolio-margin ladder used as a sourced scenario, not your account's margin."

// header is what every row of one run shares.
type header struct {
	day       contracts.Day
	runID     string
	at        time.Time
	paramHash string
}

func (h header) row(strat, rule, key string, underlying int, status, legs, checks string, facts map[string]string) strategyresult.Row {
	text, _ := json.Marshal(facts) // a map of strings cannot fail; the keys are sorted
	return strategyresult.Row{SessionDay: h.day, RunID: h.runID, Strategy: strat, RuleVersion: rule, ParamHash: h.paramHash,
		Key: key, UnderlyingID: underlying, Status: status, Legs: legs, Checks: checks, Facts: string(text), ComputedAt: h.at}
}

// mergeExcluded adds the holdings the loader excluded to the outcome and drops the total of their currencies: a
// currency with an exclusion is not complete. A currency left empty is the book's.
func mergeExcluded(out *strategy.RiskOutcome, extra []strategy.RiskExcluded, bookCurrency string) {
	for i := range out.Excluded {
		if out.Excluded[i].Currency == "" {
			out.Excluded[i].Currency = bookCurrency
		}
	}
	out.Excluded = append(out.Excluded, extra...)
	if len(out.Excluded) == 0 {
		return
	}
	drop := map[string]bool{}
	for _, e := range out.Excluded {
		drop[e.Currency] = true
	}
	kept := out.Totals[:0:0]
	for _, t := range out.Totals {
		if !drop[t.Currency] {
			kept = append(kept, t)
		}
	}
	out.Totals = kept
}

// currencies are the currencies an alias has a row for, sorted: those of its holdings and of what the stress
// returned. An alias with nothing in it has none.
func (r aliasRun) currencies() []string {
	set := map[string]bool{}
	for _, h := range r.book.Holdings {
		set[h.Currency] = true
	}
	for _, c := range r.out.Classes {
		set[c.Currency] = true
	}
	for _, t := range r.out.Totals {
		set[t.Currency] = true
	}
	for _, e := range r.out.Excluded {
		set[e.Currency] = true
	}
	delete(set, "")
	if len(set) == 0 && len(r.out.Excluded) > 0 {
		set[r.book.Currency] = true
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	slices.Sort(out)
	return out
}

func (r aliasRun) total(cur string) (string, bool) {
	for _, e := range r.out.Excluded {
		if e.Currency == cur {
			return "", false
		}
	}
	for _, t := range r.out.Totals {
		if t.Currency == cur && t.WorstLoss != "" {
			return t.WorstLoss, true
		}
	}
	return "", false
}

type pointJSON struct {
	Move           string `json:"move"`
	PnL            string `json:"pnl"`
	AssignmentCash string `json:"assignment_cash"`
}

func pointOf(p strategy.RiskPoint) pointJSON { return pointJSON{p.Move, p.PnL, p.AssignmentCash} }

type excludedJSON struct {
	Underlying string `json:"underlying"`
	Reason     string `json:"reason"`
	Count      int    `json:"count"`
}

// bookRows is one row per alias and currency, then one per currency for the household. Aliases keep their order,
// currencies are sorted.
func bookRows(h header, runs []aliasRun) []strategyresult.Row {
	var rows []strategyresult.Row
	type sum struct {
		total      *big.Rat
		incomplete []string
	}
	house := map[string]*sum{}
	sumOf := func(m map[string]*sum, cur string) *sum {
		if m[cur] == nil {
			m[cur] = &sum{total: new(big.Rat)}
		}
		return m[cur]
	}
	for _, r := range runs {
		if r.failed {
			rows = append(rows, h.row(strategyBook, ruleBook, r.alias+":"+defaultCurrency, 0, "unknown", "[]", "[]",
				map[string]string{"reason": "read_failed"}))
			s := sumOf(house, defaultCurrency)
			s.incomplete = append(s.incomplete, r.alias)
			continue
		}
		for _, cur := range r.currencies() {
			facts, total, ok := aliasFacts(r, cur)
			status := "unknown"
			if ok {
				status = "ok"
			}
			rows = append(rows, h.row(strategyBook, ruleBook, r.alias+":"+cur, 0, status, "[]", "[]", facts))
			s := sumOf(house, cur)
			if v, good := new(big.Rat).SetString(total); ok && good {
				s.total.Add(s.total, v)
			} else {
				s.incomplete = append(s.incomplete, r.alias)
			}
		}
	}
	curs := make([]string, 0, len(house))
	for c := range house {
		curs = append(curs, c)
	}
	slices.Sort(curs)
	for _, cur := range curs {
		s := house[cur]
		facts := map[string]string{"basis_note": basisNote}
		status := "unknown"
		if len(s.incomplete) == 0 {
			status = "ok"
			facts["worst_loss_"+cur] = s.total.FloatString(2)
		} else {
			facts["incomplete_aliases"] = joinSorted(s.incomplete)
		}
		rows = append(rows, h.row(strategyBook, ruleBook, "household:"+cur, 0, status, "[]", "[]", facts))
	}
	return rows
}

func joinSorted(in []string) string {
	s := append([]string(nil), in...)
	slices.Sort(s)
	b, _ := json.Marshal(s)
	return string(b)
}

// aliasFacts is the facts of one alias row in one currency (docs/contracts/strategy.md, Book risk); ok is true and
// total the text of the total when the currency is complete.
func aliasFacts(r aliasRun, cur string) (facts map[string]string, total string, ok bool) {
	facts = map[string]string{"basis_note": basisNote, "margin_change_not_modelled": "true"}
	total, ok = r.total(cur)
	if ok {
		facts["worst_loss_"+cur] = total
	}
	assign := new(big.Rat)
	haveAssign := false
	for _, c := range r.out.Classes {
		if c.Currency != cur {
			continue
		}
		pts := make([]pointJSON, 0, len(c.Ladder))
		for _, p := range c.Ladder {
			pts = append(pts, pointOf(p))
		}
		b, _ := json.Marshal(pts)
		facts["ladder_"+string(c.Class)] = string(b)
		w, _ := json.Marshal(pointOf(c.Worst))
		facts["worst_"+string(c.Class)] = string(w)
		if c.OldestSpotDay != "" {
			facts["oldest_spot_day_"+string(c.Class)] = string(c.OldestSpotDay)
		}
		if v, good := new(big.Rat).SetString(c.Worst.AssignmentCash); good {
			assign.Add(assign, v)
			haveAssign = true
		}
		if cur != defaultCurrency {
			facts["range_basis"] = "finra_equity_applied"
		}
	}
	if haveAssign {
		facts["assignment_cash"] = assign.FloatString(2)
	}
	if ex := excludedFact(r.out.Excluded, cur); ex != "" {
		facts["excluded"] = ex
	}
	for c, n := range r.out.NotStressed {
		if c != cur && n > 0 {
			facts["not_stressed_"+c] = strconv.Itoa(n)
		}
	}
	if r.book.Funds == nil {
		facts["reason"] = "no_funds" // no account.value of the day or the day before: no loss-against-liquidity pair
	}
	if f := r.book.Funds; ok && f != nil && f.Currency == cur && f.ExcessLiquidity != "" {
		b, _ := json.Marshal(map[string]string{"worst_loss": total, "excess_liquidity": f.ExcessLiquidity})
		facts["equity_loss_vs_excess_liquidity"] = string(b)
	}
	return facts, total, ok
}

func excludedFact(ex []strategy.RiskExcluded, cur string) string {
	counts := map[[2]string]int{}
	for _, e := range ex {
		if e.Currency == cur {
			counts[[2]string{e.Underlying, e.Reason}]++
		}
	}
	if len(counts) == 0 {
		return ""
	}
	list := make([]excludedJSON, 0, len(counts))
	for k, n := range counts {
		list = append(list, excludedJSON{k[0], k[1], n})
	}
	sort.Slice(list, func(a, b int) bool {
		if list[a].Underlying != list[b].Underlying {
			return list[a].Underlying < list[b].Underlying
		}
		return list[a].Reason < list[b].Reason
	})
	b, _ := json.Marshal(list)
	return string(b)
}

type legJSON struct {
	InstrumentID string `json:"instrument_id"`
	Side         string `json:"side"`
	Right        string `json:"right"`
	Strike       string `json:"strike"`
	Expiry       string `json:"expiry"`
	Multiplier   string `json:"multiplier"`
	Quantity     int    `json:"quantity"`
}

type gateJSON struct {
	Gate   string `json:"gate"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// candidateOf reads the ok row of a put candidate: its legs as holdings and the facts the stress needs. The
// underlying is the row's issuer, its currency USD (the put rule reads US stocks only).
func (j *job) candidateOf(r strategyresult.Row) (strategy.RiskCandidate, bool) {
	var legs []legJSON
	facts := map[string]string{}
	if json.Unmarshal([]byte(r.Legs), &legs) != nil || json.Unmarshal([]byte(r.Facts), &facts) != nil || len(legs) == 0 {
		return strategy.RiskCandidate{}, false
	}
	under := facts["issuer"]
	class := j.classOf(under)
	c := strategy.RiskCandidate{Key: r.Key, Class: class}
	for _, l := range legs {
		qty := l.Quantity
		if l.Side == "short" {
			qty = -qty
		}
		c.Legs = append(c.Legs, strategy.RiskHolding{
			Underlying: under, Currency: defaultCurrency, Class: class,
			Option: &strategy.RiskLeg{Right: l.Right, Strike: l.Strike, Expiry: l.Expiry, QuoteIV: facts["iv"],
				CashSettled: class != classEquity},
			Quantity: strconv.Itoa(qty), Multiplier: l.Multiplier, Spot: facts["spot"], SpotDay: contracts.Day(r.SessionDay),
		})
	}
	return c, true
}

// fitRows is one fit row per ok candidate and alias with funds, in the order of the candidates then the aliases.
func (j *job) fitRows(h header, runs []aliasRun, cands []strategyresult.Row) []strategyresult.Row {
	var rows []strategyresult.Row
	for _, cr := range cands {
		if cr.Status != "ok" {
			continue
		}
		cand, ok := j.candidateOf(cr)
		if !ok {
			continue
		}
		for _, r := range runs {
			if r.failed || r.book.Funds == nil {
				continue
			}
			fit := j.st.Fitter.Fit(r.book, r.out, cand)
			status := "unknown"
			switch fit.Check.Status {
			case "pass":
				status = "ok"
			case "block":
				status = "blocked"
			}
			checks, _ := json.Marshal([]gateJSON{{fit.Check.Name, fit.Check.Status, fit.Check.Reason}})
			facts := fit.Facts
			if facts == nil {
				facts = map[string]string{}
			}
			rows = append(rows, h.row(strategyFit, ruleFit, cr.Key+":"+r.alias, cr.UnderlyingID, status, cr.Legs, string(checks), facts))
		}
	}
	return rows
}
