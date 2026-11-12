package optionopen

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
	"github.com/google/uuid"
)

// The final statuses of a row of obs.option_open (Decision 8).
const (
	stOK = iota + 1
	stPartial
	stNoData
	stNotListed
	stNoSubscription
	stDeadline
	stDeferred
	stError
	stDisconnected
)

// skewMove is the move of the spot within a group that sets the skew flag (1 percent).
const skewMove = 0.01

// refreshDeadline cancels a refresh call; the rows not yet received are dropped (Decision 14).
const refreshDeadline = 15 * time.Second

// Target is one planned item with the contracts needed to ask it. Problem is non-empty when the contract could not
// be built from the stores: the item then ends `error` with Problem as its error code and nothing is asked.
type Target struct {
	Item     contracts.OptionOpenItem
	Symbol   string
	Under    broker.Contract
	Option   broker.Contract
	Currency string
	Expiry   time.Time
	Strike   float64
	Right    Right
	Problem  string
}

func (t Target) class() Class { return Class(t.Item.Class) }

// Core is the shared core of the pull and of Refresh (Decisions 7, 8, 10 and 12). It sends
// broker.KindOptionOpen in wave 1 (Decisions 9 and 10); the one constant kindOf names the kind.
type Core struct {
	B      broker.Broker
	Clk    clock.Clock
	Plan   PlanFunc   // nil: BuildPlan
	Params PlanParams // the band parameters of the plan
	Slot   string
	Chunk  int
	// Retry is the set of classes whose timeout is asked once more.
	Retry map[Class]bool
	// Budget bounds a run; an item not asked when it is spent ends deferred. Zero means no bound.
	Budget time.Duration
	// Beat is the interval of the heartbeat line; the clock is read after each broker call. Zero means none.
	Beat time.Duration
	Log  *slog.Logger
}

// kindOf is the kind of snapshot the core sends: wave 1 asks for the option_open kind (Decisions 9 and 10), with
// its 15 s timeout and no open-interest grace.
const kindOf = broker.KindOptionOpen

// Result is the rows of a run: exactly one terminal row per target, in the order of the targets by stock.
type Result struct {
	Rows    []contracts.OptionOpen
	Summary Summary
}

type runState struct {
	start    time.Time
	dead     bool // a whole-snapshot UNAVAILABLE: every item left ends disconnected
	rows     []contracts.OptionOpen
	progress map[int64]*StockProgress
	order    []int64
	lastBeat time.Time
}

// Pull quotes the targets: per stock the first underlying snapshot, the band at the open spot, then the groups
// (stock, expiry). Every target gets exactly one row.
func (c *Core) Pull(ctx context.Context, targets []Target) Result {
	st := &runState{start: c.Clk.Now(), progress: map[int64]*StockProgress{}}
	st.lastBeat = st.start
	var stocks [][]Target
	idx := map[int64]int{}
	for _, t := range targets {
		u := int64(t.Item.UnderlyingID)
		i, ok := idx[u]
		if !ok {
			i = len(stocks)
			idx[u] = i
			stocks = append(stocks, nil)
			st.order = append(st.order, u)
			st.progress[u] = &StockProgress{Symbol: t.Symbol}
		}
		stocks[i] = append(stocks[i], t)
		st.progress[u].Total++
	}
	for _, ts := range stocks {
		c.stock(ctx, st, ts)
	}
	return Result{Rows: st.rows, Summary: c.summary(st)}
}

func (c *Core) logger() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.Default()
}

func (c *Core) plan() PlanFunc {
	if c.Plan != nil {
		return c.Plan
	}
	return BuildPlan
}

func (c *Core) budgetSpent(st *runState) bool {
	return c.Budget > 0 && c.Clk.Now().Sub(st.start) >= c.Budget
}

// beat writes the heartbeat line when Beat has passed since the last one: the progress of the run so far.
func (c *Core) beat(st *runState) {
	if c.Beat <= 0 || c.Clk.Now().Sub(st.lastBeat) < c.Beat {
		return
	}
	st.lastBeat = c.Clk.Now()
	c.logger().Info("option_open heartbeat", "progress", c.summary(st).String())
}

func (c *Core) add(st *runState, t Target, r contracts.OptionOpen) {
	st.rows = append(st.rows, r)
	if p := st.progress[int64(t.Item.UnderlyingID)]; p != nil {
		p.Done++
		p.LastRowT = r.ReceivedAt
	}
}

// stock handles the targets of one underlying.
func (c *Core) stock(ctx context.Context, st *runState, all []Target) {
	var ts []Target
	for _, t := range all {
		if t.Problem != "" {
			c.add(st, t, c.unasked(t, stError, t.Problem, false, 0, nil))
			continue
		}
		ts = append(ts, t)
	}
	if len(ts) == 0 {
		return
	}
	if st.dead {
		c.finish(st, ts, stDisconnected, "UNAVAILABLE", 0, nil)
		return
	}
	if c.budgetSpent(st) {
		c.finish(st, ts, stDeferred, "BUDGET", 0, nil)
		return
	}
	spot, spotTs, code, err := c.spot(ctx, st, ts[0].Under)
	if err != nil {
		c.callFailed(st, ts, err)
		return
	}
	if spot <= 0 {
		// Never a quote against a stale spot: the band is built from the open spot.
		c.finish(st, ts, stDeferred, code, 0, nil)
		return
	}
	day := ts[0].Item.SessionDay.Time
	keep := c.band(day, spot, ts)
	groups := map[time.Time][]Target{}
	var exps []time.Time
	for _, t := range ts {
		switch {
		case t.Item.Deferred:
			c.add(st, t, c.unasked(t, stDeferred, "", false, spot, spotTs))
		case !keep[int64(t.Item.InstrumentID)]:
			c.add(st, t, c.unasked(t, stDeferred, "", true, spot, spotTs))
		default:
			if _, ok := groups[t.Expiry]; !ok {
				exps = append(exps, t.Expiry)
			}
			groups[t.Expiry] = append(groups[t.Expiry], t)
		}
	}
	sort.Slice(exps, func(i, j int) bool { return exps[i].Before(exps[j]) })
	for _, e := range exps {
		g := groups[e]
		sort.SliceStable(g, func(i, j int) bool { return g[i].Item.Rank < g[j].Item.Rank })
		spot, spotTs = c.group(ctx, st, g, spot, spotTs)
	}
}

// finish writes one unasked row per target with the status and code.
func (c *Core) finish(st *runState, ts []Target, status int, code string, spot float64, spotTs *time.Time) {
	for _, t := range ts {
		c.add(st, t, c.unasked(t, status, code, false, spot, spotTs))
	}
}

// callFailed ends the targets after the error of a whole call: a deadline is `deadline`, a retryable error is a
// whole-snapshot UNAVAILABLE (every item left ends disconnected), any other error is `error`.
func (c *Core) callFailed(st *runState, ts []Target, err error) {
	status, code := callStatus(err)
	if status == stDisconnected {
		st.dead = true
	}
	c.finish(st, ts, status, code, 0, nil)
}

func callStatus(err error) (int, string) {
	var te contracts.TypedError
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "deadline") {
		return stDeadline, "DEADLINE_EXCEEDED"
	}
	if errors.As(err, &te) && te.Kind() == contracts.Retryable {
		return stDisconnected, "UNAVAILABLE"
	}
	return stError, "BROKER_ERROR"
}

// spot takes the underlying snapshot. A call error is returned; an answer with no usable price returns spot 0 and
// the error code of the answer (the IBKR code, else the name of the status).
func (c *Core) spot(ctx context.Context, st *runState, u broker.Contract) (float64, *time.Time, string, error) {
	qs, err := c.B.Snapshot(ctx, broker.KindSnapshot, []broker.Contract{u})
	c.beat(st)
	if err != nil {
		return 0, nil, "", err
	}
	if len(qs) != 1 {
		return 0, nil, "NO_DATA", nil
	}
	q := qs[0]
	for _, p := range []*float64{q.Last, q.UndPrice} {
		if p != nil && *p > 0 {
			return *p, q.EventTs, "", nil
		}
	}
	if q.ErrorCode != 0 {
		return 0, nil, strconv.Itoa(q.ErrorCode), nil
	}
	return 0, nil, statusName(q.Status), nil
}

func statusName(s broker.Status) string {
	switch s {
	case broker.StatusNoSubscription:
		return "NO_SUBSCRIPTION"
	case broker.StatusTimeout:
		return "TIMEOUT"
	case broker.StatusDisconnected:
		return "DISCONNECTED"
	case broker.StatusError:
		return "ERROR"
	case broker.StatusOK, broker.StatusPartial, broker.StatusNoData:
	}
	return "NO_DATA"
}

// band runs the plan function over the stored candidates with the open spot: the instrument ids it keeps are asked,
// the others are outside the band (Decisions 3 and 12). Held contracts are always kept; a ceiling never drops one.
func (c *Core) band(day time.Time, spot float64, ts []Target) map[int64]bool {
	n := NameInput{UnderlyingID: int64(ts[0].Item.UnderlyingID), Symbol: ts[0].Symbol, Spot: spot}
	switch SigmaSource(ts[0].Item.SigmaSource) {
	case SigmaPriorClose:
		n.PriorCloseSigma = ts[0].Item.Sigma
	case SigmaLastKnown:
		n.LastKnownSigma = ts[0].Item.Sigma
	case SigmaDefault: // ResolveSigma gives the default itself
	}
	keep := map[int64]bool{}
	for _, t := range ts {
		ct := Contract{InstrumentID: int64(t.Item.InstrumentID), Expiry: t.Expiry, Strike: t.Strike, Right: t.Right}
		if t.class() == ClassHeld {
			n.Held = append(n.Held, ct)
			keep[ct.InstrumentID] = true
		} else if !t.Item.Deferred {
			n.Candidates = append(n.Candidates, ct)
		}
		// Calls are planned only for a name with a held position.
		n.HasPosition = n.HasPosition || t.class() == ClassHeld || t.class() == ClassCalls
	}
	p := c.Params
	p.CeilAroundSpot, p.CeilPuts, p.CeilCalls, p.CeilPerStock = math.MaxInt32, math.MaxInt32, math.MaxInt32, math.MaxInt32
	for _, it := range c.plan()(day, p, []NameInput{n}).Items {
		if !it.Deferred {
			keep[it.Contract.InstrumentID] = true
		}
	}
	return keep
}

// answer is the outcome of asking one target.
type answer struct {
	status int
	code   *string
	q      *broker.Quote
	sent   time.Time
}

// group asks the targets of one (stock, expiry) group, retries the timeouts of the retry classes once, samples the
// spot again at the end of the group and writes the rows. It returns the spot at the end of the group, which is
// the start of the next one.
func (c *Core) group(ctx context.Context, st *runState, g []Target, start float64, startTs *time.Time) (float64, *time.Time) {
	requested := c.Clk.Now()
	as := c.ask(ctx, st, g)
	var again []int
	for i, a := range as {
		if a.q != nil && c.Retry[g[i].class()] && retryable(a) && !st.dead && !c.budgetSpent(st) {
			again = append(again, i)
		}
	}
	if len(again) > 0 {
		sub := make([]Target, len(again))
		for k, i := range again {
			sub[k] = g[i]
		}
		for k, a := range c.ask(ctx, st, sub) {
			if better(a, as[again[k]]) {
				as[again[k]] = a
			}
		}
	}
	end, endTs := start, startTs
	if !st.dead && !c.budgetSpent(st) {
		if s, ts, _, err := c.spot(ctx, st, g[0].Under); err == nil && s > 0 {
			end, endTs = s, ts
		}
	}
	skew := start > 0 && end > 0 && math.Abs(end/start-1) >= skewMove
	for i, t := range g {
		r := c.row(t, as[i], requested)
		r.Skew, r.UnderlyingPrice, r.UnderlyingEventTs = skew, &end, endTs
		c.add(st, t, r)
	}
	return end, endTs
}

// retryable is a timeout, or a partial that ended at the timeout.
func retryable(a answer) bool {
	return a.q.Status == broker.StatusTimeout || (a.q.Status == broker.StatusPartial && a.status == stPartial)
}

// better is whether the answer of the retry replaces the first one: a quote with data beats a timeout, and an ok
// beats a partial.
func better(retry, first answer) bool {
	rank := func(a answer) int {
		switch a.status {
		case stOK:
			return 3
		case stPartial:
			return 2
		case stDeadline:
			return 0
		}
		return 1
	}
	return rank(retry) > rank(first)
}

// ask sends the targets in chunks, one call in flight. A whole call that fails ends its chunk, and for a
// retryable error every target left, with the status of the error; the budget ends the rest as deferred.
func (c *Core) ask(ctx context.Context, st *runState, ts []Target) []answer {
	out := make([]answer, len(ts))
	chunk := max(c.Chunk, 1)
	for i := 0; i < len(ts); i += chunk {
		end := min(i+chunk, len(ts))
		switch {
		case st.dead:
			fill(out[i:], stDisconnected, "UNAVAILABLE")
			return out
		case c.budgetSpent(st):
			fill(out[i:], stDeferred, "BUDGET")
			return out
		}
		cs := make([]broker.Contract, end-i)
		for k := i; k < end; k++ {
			cs[k-i] = ts[k].Option
		}
		sent := c.Clk.Now()
		qs, err := c.B.Snapshot(ctx, kindOf, cs)
		c.beat(st)
		if err == nil && len(qs) != len(cs) {
			err = errors.New("optionopen: broker answered a wrong number of quotes")
		}
		if err != nil {
			status, code := callStatus(err)
			if status == stDisconnected {
				st.dead = true
				fill(out[i:], status, code)
				return out
			}
			fill(out[i:end], status, code)
			continue
		}
		for k, q := range qs {
			q := q
			s, code := rowStatus(q)
			out[i+k] = answer{status: s, code: code, q: &q, sent: sent}
		}
	}
	return out
}

func fill(as []answer, status int, code string) {
	for i := range as {
		as[i] = answer{status: status, code: &code}
	}
}

// rowStatus maps a quote to the final status and the error code (nil when none). The statuses of the broker
// follow Decision 8: a clean 200 "no security definition" is not_listed, 354 is no_subscription, a connection
// error is disconnected, any other error code is error.
func rowStatus(q broker.Quote) (int, *string) {
	code := func(name string) *string {
		if q.ErrorCode != 0 {
			s := strconv.Itoa(q.ErrorCode)
			return &s
		}
		return &name
	}
	switch {
	case q.ErrorCode == 200 && strings.Contains(strings.ToLower(q.ErrorText), "no security definition"):
		return stNotListed, code("")
	case q.ErrorCode == 354:
		return stNoSubscription, code("")
	case q.ErrorCode == 1100 || q.ErrorCode == 1101 || q.ErrorCode == 1102 || q.ErrorCode == 10197:
		return stDisconnected, code("")
	}
	switch q.Status {
	case broker.StatusOK, broker.StatusPartial:
		return contentStatus(q), nil
	case broker.StatusNoData:
		return stNoData, nil
	case broker.StatusNoSubscription:
		return stNoSubscription, code("NO_SUBSCRIPTION")
	case broker.StatusTimeout:
		return stDeadline, code("TIMEOUT")
	case broker.StatusDisconnected:
		return stDisconnected, code("DISCONNECTED")
	case broker.StatusError:
		return stError, code("ERROR")
	}
	return stError, code("ERROR")
}

// contentStatus is no_data for a frozen (2), delayed (3) or delayed frozen (4) quote, whatever its fields: it is not
// today's open. Otherwise ok with bid, ask and the model IV, partial with any other part of them, else no_data.
func contentStatus(q broker.Quote) int {
	switch {
	case q.MarketDataType >= 2 && q.MarketDataType <= 4:
		return stNoData
	case q.Bid != nil && q.Ask != nil && q.ImpliedVol != nil:
		return stOK
	case q.Bid != nil || q.Ask != nil || q.Last != nil || q.ImpliedVol != nil:
		return stPartial
	}
	return stNoData
}

// captureID is deterministic per (session day, slot, instrument): a retry of a key meets the primary key.
func (c *Core) captureID(t Target) string {
	day := t.Item.SessionDay.Format(time.DateOnly)
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(day+"|"+c.Slot+"|"+strconv.Itoa(t.Item.InstrumentID))).String()
}

func (c *Core) base(t Target, status int, code *string) contracts.OptionOpen {
	now := c.Clk.Now().UTC()
	return contracts.OptionOpen{
		SessionDay: t.Item.SessionDay, CaptureID: c.captureID(t), Slot: c.Slot,
		InstrumentID: t.Item.InstrumentID, UnderlyingID: t.Item.UnderlyingID, PlanID: t.Item.PlanID,
		RequestedAt: now, ReceivedAt: now, Status: status, ErrorCode: code,
		Currency: t.Currency, UnderlyingCcy: t.Currency,
	}
}

// unasked is the row of a target that was not asked.
func (c *Core) unasked(t Target, status int, code string, outside bool, spot float64, spotTs *time.Time) contracts.OptionOpen {
	var cp *string
	if code != "" {
		cp = &code
	}
	r := c.base(t, status, cp)
	r.OutsideBand = outside
	if spot > 0 {
		r.UnderlyingPrice, r.UnderlyingEventTs = &spot, spotTs
	}
	return r
}

// row is the row of an asked target.
func (c *Core) row(t Target, a answer, requested time.Time) contracts.OptionOpen {
	r := c.base(t, a.status, a.code)
	r.RequestedAt = requested.UTC()
	if !a.sent.IsZero() {
		s := a.sent.UTC()
		r.SentAt = &s
	}
	q := a.q
	if q == nil {
		return r
	}
	if !q.ReceivedAt.IsZero() {
		r.ReceivedAt = q.ReceivedAt.UTC()
	}
	if q.EventTs != nil {
		e := q.EventTs.UTC()
		r.EventTs = &e
	}
	r.Generation, r.ConnectionGeneration, r.MarketDataType = int(q.Generation), int(q.Generation), q.MarketDataType
	r.Bid, r.Ask, r.BidSize, r.AskSize, r.Last, r.Volume = q.Bid, q.Ask, q.BidSize, q.AskSize, q.Last, q.Volume
	r.IvDecimal, r.Delta, r.Gamma = q.ImpliedVol, roundDelta(q.Delta), q.Gamma
	r.ThetaPerCalendarDay, r.VegaPerVolPoint = q.Theta, q.Vega
	r.UnderlyingPrice = q.UndPrice
	return r
}

// RefreshResult is the answer of Refresh: the fresh rows and how many contracts were asked.
type RefreshResult struct {
	Rows  []contracts.OptionOpen
	Asked int
}

// Refresh asks the newest quotes of the targets for the button of the option pages: the stored plan only, no band
// and no underlying snapshot, no retry, no write. The call is cancelled at 15 s; a row of any status but ok or
// partial is dropped, and the rows of a refresh carry the slot openquote.SlotRefresh. chunk is the number of
// contracts in flight at once (Decision 15).
func Refresh(ctx context.Context, b broker.Broker, clk clock.Clock, targets []Target, chunk int) RefreshResult {
	ctx, cancel := context.WithTimeout(ctx, refreshDeadline)
	defer cancel()
	c := &Core{B: b, Clk: clk, Slot: openquote.SlotRefresh, Chunk: chunk}
	st := &runState{start: clk.Now(), progress: map[int64]*StockProgress{}}
	var ts []Target
	for _, t := range targets {
		if t.Problem == "" {
			ts = append(ts, t)
		}
	}
	res := RefreshResult{Asked: len(ts)}
	requested := clk.Now()
	for i, a := range c.ask(ctx, st, ts) {
		if a.status == stOK || a.status == stPartial {
			res.Rows = append(res.Rows, c.row(ts[i], a, requested))
		}
	}
	return res
}

// roundDelta rounds a delta to 4 places, half-even on the exact value of the float, through its decimal text so
// that no float arithmetic runs and the shortest text of the result has at most 4 places. NaN and infinities
// pass through; nil stays nil.
func roundDelta(v *float64) *float64 {
	if v == nil {
		return nil
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) {
		return v
	}
	r, err := strconv.ParseFloat(strconv.FormatFloat(*v, 'f', 4, 64), 64)
	if err != nil {
		return v
	}
	return &r
}
