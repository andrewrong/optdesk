package optionopen

import (
	"math"
	"sort"
	"strings"
	"time"
)

// Class is the class of a planned contract; it is the key of the order and of
// the ceilings (Decision 3 of docs/design/option-open-pull.md).
type Class string

// The four classes of a plan row.
const (
	ClassAroundSpot Class = "around_spot"
	ClassPuts       Class = "puts"
	ClassCalls      Class = "calls"
	ClassHeld       Class = "held"
)

// SigmaSource records which of the three sources gave the sigma of a name.
type SigmaSource string

// The three sigma_source values.
const (
	SigmaPriorClose SigmaSource = "prior_close"
	SigmaLastKnown  SigmaSource = "last_known"
	SigmaDefault    SigmaSource = "default"
)

// DefaultSigma is the sigma used when a name has no implied vol at all.
const DefaultSigma = 0.30

// Right is the right of an option contract.
type Right string

// The two rights.
const (
	Put  Right = "P"
	Call Right = "C"
)

// CallsScope says which names get calls.
type CallsScope string

// The two scopes of PlanParams.Calls.
const (
	CallsHeld CallsScope = "held"
	CallsAll  CallsScope = "all"
)

// Contract is one resolved option contract (a stored con_id).
type Contract struct {
	InstrumentID int64
	Expiry       time.Time
	Strike       float64
	Right        Right
}

// NameInput is one stock of the plan. Spot is the last stored close.
// PriorCloseSigma and LastKnownSigma are annualised implied vols as fractions
// (0.35); a value of zero or less means none.
type NameInput struct {
	UnderlyingID    int64
	Symbol          string
	Spot            float64
	PriorCloseSigma float64
	LastKnownSigma  float64
	HasPosition     bool // the name has a held position: it gets calls
	DeepPuts        bool // owner's spread list; false for every name today
	Candidates      []Contract
	Held            []Contract // every held option contract, any expiry
}

// PlanParams is the named parameter set of job_plan.toml table [params].
type PlanParams struct {
	PutK, CallK     float64
	StoreK          float64
	StoreDrop       float64
	Calls           CallsScope
	MinDTE, MaxDTE  int
	MinPuts         int // least puts per expiry when the band is narrow
	MaxPerExpiry    int // cap per expiry on a 1 dollar grid
	FarOTM          float64
	FiveDollarNames []string // beyond FarOTM only 5 dollar strikes
	DeepPutsDrop    float64
	CeilAroundSpot  int
	CeilPuts        int
	CeilCalls       int
	CeilPerStock    int
}

// DefaultPlanParams are the US values of the owner decision 2026-11-10.
func DefaultPlanParams() PlanParams {
	return PlanParams{
		PutK: 1.4, CallK: 1.4, StoreK: 1.75, StoreDrop: 0.25,
		Calls:  CallsHeld,
		MinDTE: 20, MaxDTE: 45,
		MinPuts: 6, MaxPerExpiry: 25, FarOTM: 0.03,
		FiveDollarNames: []string{"SPY", "QQQ", "IWM"},
		DeepPutsDrop:    0.15,
		CeilAroundSpot:  350, CeilPuts: 1400, CeilCalls: 1200, CeilPerStock: 60,
	}
}

// JPPlanParams are the Japan values: the same rule over 14 to 90 days.
func JPPlanParams() PlanParams {
	p := DefaultPlanParams()
	p.MinDTE, p.MaxDTE = 14, 90
	return p
}

// Item is one planned contract. Rank is the order of the plan from 1; a
// deferred item keeps its class and ranks after every other item.
type Item struct {
	UnderlyingID int64
	Symbol       string
	Contract     Contract
	Class        Class
	Deferred     bool
	Rank         int
}

// NameSigma is the spot and sigma a name was planned with. The underlying
// itself is never an item and is never deferred: its price is always read.
type NameSigma struct {
	UnderlyingID int64
	Symbol       string
	Spot         float64
	Sigma        float64
	SigmaSource  SigmaSource
}

// Plan is the result of a PlanFunc.
type Plan struct {
	Names []NameSigma
	Items []Item
}

// PlanFunc plans the contracts of all names for one trading day. It is pure:
// no clock, no I/O; day is the calendar date of the run. The own
// implementation is BuildPlan; a library may replace it behind this type.
type PlanFunc func(day time.Time, p PlanParams, names []NameInput) Plan

var _ PlanFunc = BuildPlan

// ResolveSigma picks the sigma: prior close, else last known, else the default.
func ResolveSigma(priorClose, lastKnown float64) (float64, SigmaSource) {
	switch {
	case priorClose > 0:
		return priorClose, SigmaPriorClose
	case lastKnown > 0:
		return lastKnown, SigmaLastKnown
	}
	return DefaultSigma, SigmaDefault
}

// Band is the strike band spot*exp(-+k*sigma*sqrt(T)), T = days/365. It is a
// band, not an option price.
func Band(spot, sigma float64, days int, k float64) (lo, hi float64) {
	w := k * sigma * math.Sqrt(float64(days)/365)
	return spot * math.Exp(-w), spot * math.Exp(w)
}

// StoredBand is the wider band the evening resolve stores: StoreK or spot
// -StoreDrop for puts (and +StoreDrop for held calls), whichever is nearer.
func StoredBand(p PlanParams, spot, sigma float64, days int) (lo, hi float64) {
	lo, hi = Band(spot, sigma, days, p.StoreK)
	return math.Max(lo, spot*(1-p.StoreDrop)), math.Min(hi, spot*(1+p.StoreDrop))
}

const eps = 1e-9

type entry struct {
	item    Item
	dte     int
	dist    float64
	protect bool // one of the nearest MinPuts puts of its expiry
}

// BuildPlan is the own implementation of PlanFunc.
func BuildPlan(day time.Time, p PlanParams, names []NameInput) Plan {
	ns := append([]NameInput(nil), names...)
	sort.Slice(ns, func(i, j int) bool {
		if ns[i].Symbol != ns[j].Symbol {
			return ns[i].Symbol < ns[j].Symbol
		}
		return ns[i].UnderlyingID < ns[j].UnderlyingID
	})
	var out Plan
	var held []Item
	perName := make([][]*entry, len(ns))
	for i, n := range ns {
		sigma, src := ResolveSigma(n.PriorCloseSigma, n.LastKnownSigma)
		out.Names = append(out.Names, NameSigma{n.UnderlyingID, n.Symbol, n.Spot, sigma, src})
		heldIDs := map[int64]bool{}
		for _, h := range n.Held {
			heldIDs[h.InstrumentID] = true
			held = append(held, Item{UnderlyingID: n.UnderlyingID, Symbol: n.Symbol, Contract: h, Class: ClassHeld})
		}
		if n.Spot > 0 {
			perName[i] = planName(day, p, n, sigma, heldIDs)
			capPerStock(p, perName[i])
		}
	}
	sort.SliceStable(held, func(i, j int) bool {
		return lessContract(held[i].Symbol, held[i].Contract, held[j].Symbol, held[j].Contract)
	})
	out.Items = held

	start := 0
	if len(ns) > 0 {
		start = int(dayNumber(day) % int64(len(ns)))
	}
	var deferred []Item
	for _, c := range []struct {
		class Class
		ceil  int
	}{{ClassAroundSpot, p.CeilAroundSpot}, {ClassPuts, p.CeilPuts}, {ClassCalls, p.CeilCalls}} {
		queues := make([][]*entry, len(ns))
		for i, es := range perName {
			for _, e := range es {
				if e.item.Class == c.class && !e.item.Deferred {
					queues[i] = append(queues[i], e)
				} else if e.item.Class == c.class {
					deferred = append(deferred, e.item)
				}
			}
			sort.SliceStable(queues[i], func(a, b int) bool { return lessEntry(queues[i][a], queues[i][b]) })
		}
		var seq []*entry
		for left := true; left; {
			left = false
			for k := range ns {
				i := (start + k) % len(ns)
				if len(queues[i]) == 0 {
					continue
				}
				left = true
				seq = append(seq, queues[i][0])
				queues[i] = queues[i][1:]
			}
		}
		over := deferTail(seq, c.ceil)
		for _, e := range seq {
			if over[e] {
				e.item.Deferred = true
				deferred = append(deferred, e.item)
			} else {
				out.Items = append(out.Items, e.item)
			}
		}
	}
	out.Items = append(out.Items, deferred...)
	for i := range out.Items {
		out.Items[i].Rank = i + 1
	}
	return out
}

// deferTail picks the entries over the ceiling: from the end of the order,
// the unprotected first; the protected minimum puts only when nothing else is
// left to defer, so they go last.
func deferTail(seq []*entry, ceil int) map[*entry]bool {
	over := map[*entry]bool{}
	need := len(seq) - ceil
	for _, protected := range []bool{false, true} {
		for i := len(seq) - 1; i >= 0 && len(over) < need; i-- {
			if seq[i].protect == protected {
				over[seq[i]] = true
			}
		}
	}
	return over
}

func dayNumber(day time.Time) int64 {
	y, m, d := day.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400
}

func dteOf(day, expiry time.Time) int {
	y, m, d := day.Date()
	ey, em, ed := expiry.Date()
	a := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	b := time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}

func lessContract(sa string, a Contract, sb string, b Contract) bool {
	if sa != sb {
		return sa < sb
	}
	if !a.Expiry.Equal(b.Expiry) {
		return a.Expiry.Before(b.Expiry)
	}
	if a.Strike != b.Strike {
		return a.Strike < b.Strike
	}
	return a.Right < b.Right
}

// lessEntry orders nearest the spot first, then (|DTE-30|, DTE).
func lessEntry(a, b *entry) bool {
	if math.Abs(a.dist-b.dist) > eps {
		return a.dist < b.dist
	}
	if da, db := abs(a.dte-30), abs(b.dte-30); da != db {
		return da < db
	}
	if a.dte != b.dte {
		return a.dte < b.dte
	}
	return lessContract("", a.item.Contract, "", b.item.Contract)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// expiryCtx is what the plan of one expiry of one name needs.
type expiryCtx struct {
	p     PlanParams
	n     NameInput
	dte   int
	calls bool
	five  bool
}

func (x expiryCtx) newEntry(c Contract, class Class) *entry {
	return &entry{item: Item{UnderlyingID: x.n.UnderlyingID, Symbol: x.n.Symbol, Contract: c, Class: class},
		dte: x.dte, dist: math.Abs(c.Strike - x.n.Spot)}
}

func groupByExpiry(day time.Time, p PlanParams, cands []Contract) map[time.Time][]Contract {
	type key struct {
		exp time.Time
		k   float64
		r   Right
	}
	seen := map[key]bool{}
	byExp := map[time.Time][]Contract{}
	for _, c := range cands {
		y, m, d := c.Expiry.Date()
		exp := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		sk := key{exp, c.Strike, c.Right}
		if dte := dteOf(day, exp); seen[sk] || dte < p.MinDTE || dte > p.MaxDTE {
			continue
		}
		seen[sk] = true
		byExp[exp] = append(byExp[exp], c)
	}
	return byExp
}

func planName(day time.Time, p PlanParams, n NameInput, sigma float64, heldIDs map[int64]bool) []*entry {
	x := expiryCtx{p: p, n: n, calls: p.Calls == CallsAll || (p.Calls == CallsHeld && n.HasPosition)}
	for _, s := range p.FiveDollarNames {
		x.five = x.five || strings.EqualFold(s, n.Symbol)
	}
	var out []*entry
	for exp, cs := range groupByExpiry(day, p, n.Candidates) {
		x.dte = dteOf(day, exp)
		out = append(out, x.plan(cs, sigma, heldIDs)...)
	}
	return out
}

// aroundStrikes are the listed strike at or below the spot and the one above.
func aroundStrikes(cs []Contract, spot float64) (below, above, minGap float64) {
	set := map[float64]bool{}
	for _, c := range cs {
		set[c.Strike] = true
	}
	sorted := make([]float64, 0, len(set))
	for s := range set {
		sorted = append(sorted, s)
	}
	sort.Float64s(sorted)
	below, above, minGap = math.Inf(-1), math.Inf(1), math.Inf(1)
	for i, s := range sorted {
		if s <= spot+eps {
			below = s
		} else if math.IsInf(above, 1) {
			above = s
		}
		if i > 0 {
			minGap = math.Min(minGap, s-sorted[i-1])
		}
	}
	return below, above, minGap
}

func (x expiryCtx) plan(cs []Contract, sigma float64, heldIDs map[int64]bool) []*entry {
	p, n := x.p, x.n
	lo, _ := Band(n.Spot, sigma, x.dte, p.PutK)
	if n.DeepPuts {
		lo -= p.DeepPutsDrop * n.Spot
	}
	_, hi := Band(n.Spot, sigma, x.dte, p.CallK)
	below, above, minGap := aroundStrikes(cs, n.Spot)
	oneDollar := math.Abs(minGap-1) < 1e-6

	var out, puts, calls []*entry
	var putsBelow []Contract // listed puts under the around strike
	for _, c := range cs {
		switch {
		case heldIDs[c.InstrumentID]:
		case math.Abs(c.Strike-below) < eps || math.Abs(c.Strike-above) < eps:
			out = append(out, x.newEntry(c, ClassAroundSpot))
		case c.Right == Put && c.Strike < below:
			putsBelow = append(putsBelow, c)
			if c.Strike >= lo-eps {
				puts = append(puts, x.newEntry(c, ClassPuts))
			}
		case c.Right == Call && x.calls && c.Strike > above && c.Strike <= hi+eps:
			calls = append(calls, x.newEntry(c, ClassCalls))
		}
	}
	if x.five && oneDollar {
		puts, calls = x.dropOffFive(puts), x.dropOffFive(calls)
	}
	puts = x.addMinPuts(puts, putsBelow)
	if oneDollar && p.MaxPerExpiry > 0 && len(puts)+len(calls) > p.MaxPerExpiry {
		puts, calls = capExpiry(p.MaxPerExpiry, puts, calls)
	}
	return append(append(out, puts...), calls...)
}

// dropOffFive keeps, beyond FarOTM out of the money, the 5 dollar strikes only.
func (x expiryCtx) dropOffFive(es []*entry) []*entry {
	var kept []*entry
	for _, e := range es {
		if e.dist/x.n.Spot > x.p.FarOTM && math.Mod(e.item.Contract.Strike, 5) != 0 {
			continue
		}
		kept = append(kept, e)
	}
	return kept
}

// addMinPuts adds the nearest listed puts below the spot up to MinPuts and
// marks the MinPuts nearest as protected from every cap.
func (x expiryCtx) addMinPuts(puts []*entry, below []Contract) []*entry {
	sort.Slice(below, func(i, j int) bool { return below[i].Strike > below[j].Strike })
	have := map[float64]bool{}
	for _, e := range puts {
		have[e.item.Contract.Strike] = true
	}
	for _, c := range below {
		if len(puts) >= x.p.MinPuts {
			break
		}
		if !have[c.Strike] {
			puts = append(puts, x.newEntry(c, ClassPuts))
		}
	}
	sort.Slice(puts, func(i, j int) bool { return puts[i].dist < puts[j].dist })
	for i := 0; i < len(puts) && i < x.p.MinPuts; i++ {
		puts[i].protect = true
	}
	return puts
}

// capExpiry drops the farthest puts and calls to the cap; protected puts stay.
func capExpiry(limit int, puts, calls []*entry) ([]*entry, []*entry) {
	all := append(append([]*entry(nil), puts...), calls...)
	sort.SliceStable(all, func(i, j int) bool { return lessEntry(all[i], all[j]) })
	drop := map[*entry]bool{}
	for i := len(all) - 1; i >= 0 && len(all)-len(drop) > limit; i-- {
		if !all[i].protect {
			drop[all[i]] = true
		}
	}
	return keep(puts, drop), keep(calls, drop)
}

func keep(es []*entry, drop map[*entry]bool) []*entry {
	var k []*entry
	for _, e := range es {
		if !drop[e] {
			k = append(k, e)
		}
	}
	return k
}

// capPerStock defers the farthest puts, then the farthest calls, until the
// name is at the per-stock ceiling. around_spot and the protected minimum of
// puts per expiry are never deferred here.
func capPerStock(p PlanParams, es []*entry) {
	if p.CeilPerStock <= 0 {
		return
	}
	total := len(es)
	for _, class := range []Class{ClassPuts, ClassCalls} {
		var cs []*entry
		for _, e := range es {
			if e.item.Class == class && !e.protect {
				cs = append(cs, e)
			}
		}
		sort.SliceStable(cs, func(i, j int) bool { return lessEntry(cs[j], cs[i]) })
		for _, e := range cs {
			if total <= p.CeilPerStock {
				return
			}
			e.item.Deferred = true
			total--
		}
	}
}
