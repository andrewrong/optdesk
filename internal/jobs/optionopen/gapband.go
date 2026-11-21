package optionopen

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// The live band over the stored chain (Decisions 2 to 5 of docs/design/option-open-band.md): at the open the plan
// function runs over every stored resolved contract of the stock at the live spot, not over the plan items, so a
// stock that gapped is quoted on the stored puts of its new band. Nothing is resolved or written here.

// chainCache reads the stored chain of a stock once per run of a key: the resume, the targets and the live band
// share it.
type chainCache struct {
	src ContractLister
	m   map[int64][]openquote.OptionRef
}

func newChainCache(src ContractLister) *chainCache {
	return &chainCache{src: src, m: map[int64][]openquote.OptionRef{}}
}

// ListOptions is ContractLister, memoised per underlying.
func (c *chainCache) ListOptions(ctx context.Context, u int64) ([]openquote.OptionRef, error) {
	if refs, ok := c.m[u]; ok {
		return refs, nil
	}
	refs, err := c.src.ListOptions(ctx, u)
	if err != nil {
		return nil, err
	}
	c.m[u] = refs
	return refs, nil
}

// liveBand is the outcome of the live band of one stock.
type liveBand struct {
	keep  map[int64]bool // the instrument ids of the plan items to ask
	added []Target       // stored contracts of the live band that the plan did not hold: asked
	over  []Target       // added contracts over the cap of the stock: deferred, not outside the band
}

// liveBand runs the plan function at the open spot. With no Contracts port it is the band over the plan items.
func (c *Core) liveBand(ctx context.Context, day time.Time, spot float64, ts []Target) (liveBand, error) {
	if c.Contracts == nil {
		return liveBand{keep: c.band(day, spot, ts)}, nil
	}
	refs, err := c.Contracts.ListOptions(ctx, int64(ts[0].Item.UnderlyingID))
	if err != nil {
		return liveBand{}, err
	}
	var chain []Contract
	for _, r := range refs {
		if ct, ok := refContract(r); ok {
			chain = append(chain, ct)
		}
	}
	inPlan := map[int64]bool{}
	lb := liveBand{keep: map[int64]bool{}}
	for _, t := range ts {
		inPlan[int64(t.Item.InstrumentID)] = true
		if t.class() == ClassHeld {
			lb.keep[int64(t.Item.InstrumentID)] = true
		}
	}
	live := c.bandItems(day, spot, ts, chain)
	items := make(map[int64]Item, len(live))
	byID := map[int64]openquote.OptionRef{}
	for _, r := range refs {
		byID[r.InstrumentID] = r
	}
	var want []openquote.OptionRef
	for _, it := range live { // in the order of the plan
		id := it.Contract.InstrumentID
		items[id] = it
		switch {
		case inPlan[id]:
			if !it.Deferred {
				lb.keep[id] = true
			}
		case c.Done[id]:
		default:
			want = append(want, byID[id])
		}
	}
	added, err := c.addedTargets(ctx, ts, want, items)
	if err != nil {
		return liveBand{}, err
	}
	for _, t := range added {
		if items[int64(t.Item.InstrumentID)].Deferred {
			lb.over = append(lb.over, t)
		} else {
			lb.added = append(lb.added, t)
		}
	}
	return lb, nil
}

// refContract is the plan contract of a stored one; false when its expiry or strike does not parse.
func refContract(r openquote.OptionRef) (Contract, bool) {
	exp, err := time.Parse(time.DateOnly, string(r.Expiry))
	strike, serr := strconv.ParseFloat(r.Strike, 64)
	if err != nil || serr != nil {
		return Contract{}, false
	}
	return Contract{InstrumentID: r.InstrumentID, Expiry: exp, Strike: strike, Right: Right(r.Right)}, true
}

// addedTargets builds the targets of the stored contracts the live band added: the stock and the item come from the
// first target of the stock, the con_ids from the Instruments reader as pull.targets reads them. Nothing is resolved.
func (c *Core) addedTargets(ctx context.Context, ts []Target, want []openquote.OptionRef, items map[int64]Item) ([]Target, error) {
	if len(want) == 0 || c.Instruments == nil {
		return nil, nil
	}
	tpl := ts[0]
	ids := []int64{int64(tpl.Item.UnderlyingID)}
	for _, r := range want {
		ids = append(ids, r.InstrumentID)
	}
	insts, err := c.Instruments.Instruments(ctx, ids)
	if err != nil {
		return nil, err
	}
	conID := conIDs(insts)
	var out []Target
	for _, r := range want {
		ct, ok := refContract(r)
		it := items[r.InstrumentID]
		t := Target{Item: tpl.Item, Symbol: tpl.Symbol, Under: tpl.Under, Currency: tpl.Currency,
			Expiry: ct.Expiry, Strike: ct.Strike, Right: ct.Right}
		t.Item.InstrumentID, t.Item.Class, t.Item.Rank, t.Item.Deferred = int(r.InstrumentID), gen.OptionOpenItemClass(it.Class), it.Rank, false
		switch {
		case !ok:
			t.Problem = "BAD_CONTRACT"
		case conID[r.InstrumentID] == 0:
			t.Problem = "NO_INSTRUMENT"
		default:
			t.Option = broker.Contract{SecType: "OPT", Symbol: tpl.Symbol, Exchange: r.Exchange, Currency: r.Currency,
				Expiry: strings.ReplaceAll(string(r.Expiry), "-", ""), Strike: r.Strike, Right: r.Right,
				Multiplier: r.Multiplier, TradingClass: r.TradingClass, ConID: conID[r.InstrumentID]}
		}
		out = append(out, t)
	}
	return out, nil
}

// conIDs maps an instrument id to its ibkr_conid.
func conIDs(insts []store.Instrument) map[int64]int64 {
	out := map[int64]int64{}
	for _, in := range insts {
		for _, idf := range in.Identifiers {
			if n, err := strconv.ParseInt(idf.Value, 10, 64); err == nil && idf.Scheme == "ibkr_conid" {
				out[in.ID] = n
			}
		}
	}
	return out
}

// bandItems is the plan function over the chain at the live spot, with the plan's own ceilings: the cap of the stock
// is the plan's. Held contracts of the stored plan are always items.
func (c *Core) bandItems(day time.Time, spot float64, ts []Target, chain []Contract) []Item {
	n := c.nameInput(spot, ts)
	n.Candidates = chain
	return c.plan()(day, c.Params, []NameInput{n}).Items
}

// nameInput is the stock of the live band: the held contracts and the position flag come from the stored plan.
func (c *Core) nameInput(spot float64, ts []Target) NameInput {
	n := NameInput{UnderlyingID: int64(ts[0].Item.UnderlyingID), Symbol: ts[0].Symbol, Spot: spot}
	switch SigmaSource(ts[0].Item.SigmaSource) {
	case SigmaPriorClose:
		n.PriorCloseSigma = ts[0].Item.Sigma
	case SigmaLastKnown:
		n.LastKnownSigma = ts[0].Item.Sigma
	case SigmaDefault: // ResolveSigma gives the default itself
	}
	for _, t := range ts {
		if t.class() == ClassHeld {
			n.Held = append(n.Held, Contract{InstrumentID: int64(t.Item.InstrumentID), Expiry: t.Expiry, Strike: t.Strike, Right: t.Right})
		}
		// Calls are planned only for a name with a held position.
		n.HasPosition = n.HasPosition || t.class() == ClassHeld || t.class() == ClassCalls
	}
	return n
}

// uncapped is the parameter set of the band over the plan items: they are already capped by the plan.
func uncapped(p PlanParams) PlanParams {
	p.CeilAroundSpot, p.CeilPuts, p.CeilCalls, p.CeilPerStock = math.MaxInt32, math.MaxInt32, math.MaxInt32, math.MaxInt32
	return p
}

// splitAdded takes the rows of contracts that are not items of the plan apart: the plan rows and the number of added
// rows (Decision 5: added rows are not in planned and their ok does not count towards the share).
func splitAdded(items []contracts.OptionOpenItem, rows []contracts.OptionOpen) (planRows []contracts.OptionOpen, added int) {
	in := make(map[int]bool, len(items))
	for _, it := range items {
		in[it.InstrumentID] = true
	}
	for _, r := range rows {
		if in[r.InstrumentID] {
			planRows = append(planRows, r)
		} else {
			added++
		}
	}
	return planRows, added
}

// finalIDs is the set of instrument ids of the final rows in hand, and how many of them are not items of the plan.
func finalIDs(items []contracts.OptionOpenItem, final []contracts.OptionOpen) (done map[int64]bool, added int) {
	in := make(map[int]bool, len(items))
	for _, it := range items {
		in[it.InstrumentID] = true
	}
	done = map[int64]bool{}
	for _, r := range final {
		done[int64(r.InstrumentID)] = true
		if !in[r.InstrumentID] {
			added++
		}
	}
	return done, added
}
