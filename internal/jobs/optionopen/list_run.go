package optionopen

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// The reasons of a run row (Decision 6 of docs/design/option-contract-lists.md) that this job writes.
const (
	reasonNotFinished = "not_finished"
	reasonNone        = "none"
	reasonNoExpiry    = "no_expiry_in_window"
	reasonChainsFail  = "listchains_failed"
	reasonBudget      = "budget"
	reasonTimeouts    = "timeouts"
	reasonUnserved    = "unserved"
	reasonBrokerDown  = "broker_down"
	reasonMaxNew      = "list_max_new"
	reasonAllShort    = "all_short"
	reasonSomeShort   = "some_short"
	reasonNoSpot      = "no_spot"
)

// The codes of the reason rows of ReasonWriter.
const (
	codeHeldStale    = "held_stale"
	codeOutsideRange = "outside_range"
	codeNoSpot       = "no_spot"
)

const (
	spotUnknown = iota
	spotKnown
	spotNone
)

// listName is one stock of a run.
type listName struct {
	st       stock
	id       int64
	rank     int      // 0 held, 1 priority (BuyPrice), 2 the rest
	expected []string // "expiry|class"; nil until known
	reason   string
	mult     string // the multiplier of the chain; empty when this run did not ask ListChains
	listed   int    // fresh groups, recounted from master.option_list
	done     bool
	pending  int
	lists    map[string]bool // fresh groups (listed in the freshness window, zone inside the stored range)
	recs     map[string]openquote.ListRecord
	answered map[string]bool // groups the broker answered in this run
	chainsOK bool            // ListChains answered in this run
	refs     []openquote.OptionRef
	refSet   map[string]bool // refKey of every stored contract
	refCount map[string]int  // stored contracts per "expiry|class"
	loaded   bool
	spot     float64
	spotSt   int
	sigma    float64
	sigSrc   string
	sigmaSet bool
}

type listGroup struct {
	n      *listName
	expiry contracts.Day
	class  string
	sigma  float64 // zone mode: the stored sigma the group is listed with
	source string
}

func (g listGroup) key() string { return string(g.expiry) + "|" + g.class }

type listRun struct {
	j       *listJob
	day     contracts.Day
	t       time.Time
	start   time.Time
	stopAt  time.Time
	names   []*listName
	stop    string
	abort   contracts.TypedError
	touts   int
	written int
	groups  int
	zone    bool
	noSpot  bool
	relist  map[string]bool // zone mode: "id|expiry|class" listed again in this run
}

// Run lists the stocks of the rules of the job. The one key of the job is the only key it runs.
func (j *listJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	t, err := dayTime(day)
	if err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
	}
	if len(keys) == 0 || keys[0].DatasetID != j.m.Dataset || keys[0].SubjectKey != j.m.Subject {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "unknown key")}
	}
	now := j.d.Clock.Now()
	r := &listRun{j: j, day: day, t: t, start: now, zone: j.m.Mode == modeZone, relist: map[string]bool{},
		stopAt: time.Date(t.Year(), t.Month(), t.Day(), j.m.stopH, j.m.stopM, 0, 0, j.m.loc)}
	if r.zone {
		return r.runZone(ctx)
	}
	if terr := r.run(ctx); terr != nil {
		return contracts.JobResult{Returned: r.groups, Usable: r.groups, Err: terr}
	}
	res := contracts.JobResult{Returned: r.groups, Usable: r.groups}
	listed, err := r.ownListed(ctx)
	stale := r.heldStale()
	switch {
	case err != nil:
		res.Err = asTyped(err, contracts.Retryable)
	case r.stop == reasonUnserved:
	case len(stale) > 0 && j.m.TTLDays > 1:
		res.Err = newErr(contracts.Retryable, "list_preopen: %d held or priority group(s) are neither fresh nor answered", len(stale))
	case !listed:
		res.Err = newErr(contracts.Retryable, "list_preopen: nothing was listed (%s)", r.mainReason())
	case r.noSpot:
		res.Err = newErr(contracts.Retryable, "list_preopen: a name has no prior close")
	}
	if err == nil && j.m.TTLDays > 1 {
		if werr := r.writeReasons(ctx, r.heldStaleReasons(stale)); werr != nil && res.Err == nil {
			res.Err = asTyped(werr, contracts.Retryable)
		}
	}
	r.summary(res.Err)
	return res
}

func (r *listRun) run(ctx context.Context) contracts.TypedError {
	guard, terr := r.load(ctx)
	if terr != nil {
		return terr
	}
	for _, n := range r.names {
		if terr := r.settle(ctx, n, guard); terr != nil {
			return terr
		}
	}
	if terr := r.chainsPhase(ctx, guard); terr != nil {
		return terr
	}
	if r.stop == "" {
		if terr := r.callsPhase(ctx); terr != nil {
			return terr
		}
	}
	if r.abort != nil {
		return r.abort
	}
	for _, n := range r.names {
		if !n.done && r.stop != "" {
			n.reason = r.stop
			if terr := r.put(ctx, n); terr != nil {
				return terr
			}
		}
	}
	return nil
}

// load builds the names, reads the rows of the session and writes a row for each name without one. guard is true
// when nothing is listed for the session.
func (r *listRun) load(ctx context.Context) (guard bool, terr contracts.TypedError) {
	d := r.j.d
	stocks, pos, err := r.j.rules.Names(ctx, d.Stored, r.t)
	if err != nil {
		return false, asTyped(err, contracts.Retryable)
	}
	rows, err := d.ListStore.ListRuns(ctx, r.day)
	if err != nil {
		return false, asTyped(err, contracts.Retryable)
	}
	prior := map[int64]openquote.ListRunRecord{}
	type own struct {
		st stock
		id int64
	}
	var owned []own
	ids := map[int64]bool{}
	for _, st := range stocks {
		id, err := d.underlyingID(ctx, st)
		if err != nil {
			return false, asTyped(err, contracts.Retryable)
		}
		owned = append(owned, own{st, id})
		ids[id] = true
	}
	guard = true
	for _, row := range rows {
		if !ids[row.UnderlyingID] {
			continue
		}
		prior[row.UnderlyingID] = row
		guard = guard && row.Listed == 0
	}
	var fresh []openquote.ListRunRecord
	for _, o := range owned {
		st, id := o.st, o.id
		n := &listName{st: st, id: id, rank: 2, reason: reasonNotFinished, mult: r.j.rules.Multiplier, answered: map[string]bool{}}
		if hasPosition(pos, st.Symbol) {
			n.rank = 0
		} else if r.priority(st.Symbol) {
			n.rank = 1
		}
		if row, has := prior[id]; has {
			n.reason, n.expected = row.Reason, row.Expected
		} else {
			fresh = append(fresh, openquote.ListRunRecord{UnderlyingID: id, Session: r.day, Reason: n.reason, UpdatedAt: r.start.UTC()})
		}
		if terr := r.recount(ctx, n); terr != nil {
			return false, terr
		}
		guard = guard && n.listed == 0
		r.names = append(r.names, n)
	}
	if len(fresh) > 0 {
		if err := d.ListStore.PutListRuns(context.WithoutCancel(ctx), fresh); err != nil {
			return false, asTyped(err, contracts.Retryable)
		}
	}
	return guard, nil
}

// settle ends, before any call, a name that has nothing to ask: its groups are all listed for the session (a
// reason none is written only over another reason) or its answer held no expiry in the window.
func (r *listRun) settle(ctx context.Context, n *listName, guard bool) contracts.TypedError {
	switch {
	case n.expected == nil:
	case len(n.expected) == 0:
		n.done = !guard
	case n.listed >= len(n.expected):
		if n.reason == reasonNone {
			n.done = true
			return nil
		}
		return r.end(ctx, n, reasonNone)
	}
	return nil
}

func (r *listRun) priority(symbol string) bool {
	for _, e := range r.j.d.Watch {
		if e.Contract.Symbol == symbol && e.BuyPrice != "" {
			return true
		}
	}
	return false
}

// isFresh says whether a stored group is fresh for the day: outcome listed, listed on or after D - (ttl - 1)
// days (a band of the default sigma only on the day it was listed).
func (r *listRun) isFresh(row openquote.ListRecord) bool {
	first := r.t.AddDate(0, 0, -(r.j.m.TTLDays - 1)).Format(time.DateOnly)
	if row.Outcome != "listed" || string(row.ListedSession) < first || row.ListedSession > r.day {
		return false
	}
	return row.SigmaSource != "default" || row.ListedSession == r.day
}

// recount reads the groups of the name from master.option_list: the fresh ones whose target zone from the prior
// close is inside the stored range (a group with no range, or a name with no spot, is stale).
func (r *listRun) recount(ctx context.Context, n *listName) contracts.TypedError {
	rows, err := r.j.d.ListStore.Lists(ctx, n.id)
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	want := map[string]bool{}
	for _, e := range n.expected {
		want[e] = true
	}
	n.lists, n.recs, n.listed = map[string]bool{}, map[string]openquote.ListRecord{}, 0
	var fresh []string
	for _, row := range rows {
		k := string(row.Expiry) + "|" + row.Class
		n.recs[k] = row
		if r.isFresh(row) && (n.expected == nil || want[k]) {
			fresh = append(fresh, k)
		}
	}
	if len(fresh) == 0 {
		return nil
	}
	spot, ok, terr := r.spotOf(ctx, n)
	if terr != nil {
		return terr
	}
	p := r.j.m
	for _, k := range fresh {
		row := n.recs[k]
		lo, hi := strikeValue(row.BandLo), strikeValue(row.BandHi)
		if ok && row.BandLo != "" && row.BandHi != "" && row.Sigma > 0 &&
			zoneInside(spot, row.Sigma, daysTo(r.t, row.Expiry), p.ZoneNear, p.ZoneFar, lo, hi) {
			n.lists[k] = true
			n.listed++
		}
	}
	return nil
}

// put writes the run row of a name with the listed count recounted from the table.
func (r *listRun) put(ctx context.Context, n *listName) contracts.TypedError {
	if terr := r.recount(ctx, n); terr != nil {
		return terr
	}
	row := openquote.ListRunRecord{UnderlyingID: n.id, Session: r.day, Expected: n.expected, Listed: n.listed,
		Reason: n.reason, UpdatedAt: r.j.d.Clock.Now().UTC()}
	if err := r.j.d.ListStore.PutListRuns(context.WithoutCancel(ctx), []openquote.ListRunRecord{row}); err != nil {
		return asTyped(err, contracts.Retryable)
	}
	return nil
}

// end gives a name its reason and writes the row; done names are not touched at the end of the run.
func (r *listRun) end(ctx context.Context, n *listName, reason string) contracts.TypedError {
	n.reason, n.done = reason, true
	return r.put(ctx, n)
}

// halt says why the run must not make another call, or "".
func (r *listRun) halt() string {
	now := r.j.d.Clock.Now()
	switch {
	case r.abort != nil:
		return "aborted"
	case now.Sub(r.start) >= time.Duration(r.j.m.BudgetMinutes)*time.Minute:
		return reasonBudget
	}
	return ""
}

// callErr turns the error of a call into the reason that stops the run, or "" to go on. A timeout is followed by
// the 30 s wait; two in a row stop the run.
func (r *listRun) callErr(ctx context.Context, err error) string {
	var te contracts.TypedError
	switch {
	case errors.Is(err, broker.ErrNotServed):
		r.log("list_unserved: the pyside service has not been restarted")
		return reasonUnserved
	case errors.Is(err, context.DeadlineExceeded):
		r.touts++
		if perr := r.pause(ctx); perr != nil {
			r.abort = asTyped(perr, contracts.Retryable)
			return "aborted"
		}
		if r.touts >= stopAfterTimeouts {
			return reasonTimeouts
		}
		return ""
	case errors.As(err, &te) && te.Kind() == contracts.Retryable:
		return reasonBrokerDown
	}
	r.touts = 0
	return ""
}

func (r *listRun) pause(ctx context.Context) error {
	if p := r.j.d.Pause; p != nil {
		return p(ctx, listPause)
	}
	tm := time.NewTimer(listPause)
	defer tm.Stop()
	select {
	case <-tm.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *listRun) log(line string) {
	if r.j.d.Log != nil {
		r.j.d.Log(line)
	}
}

func (r *listRun) underlying(n *listName) broker.Contract {
	return broker.Contract{SecType: "STK", Symbol: n.st.Symbol, Exchange: r.exchange(n), Currency: n.st.Currency, ConID: n.st.ConID}
}

// chainsPhase asks ListChains of each name whose expected groups are unknown (and, when nothing is listed for the
// session, of the names whose answer held no expiry in the window).
func (r *listRun) chainsPhase(ctx context.Context, guard bool) contracts.TypedError {
	var todo []*listName
	for _, n := range r.names {
		if n.expected == nil || (len(n.expected) == 0 && guard) {
			todo = append(todo, n)
		}
	}
	sort.SliceStable(todo, func(a, b int) bool {
		if todo[a].rank != todo[b].rank {
			return todo[a].rank < todo[b].rank
		}
		return todo[a].st.Symbol < todo[b].st.Symbol
	})
	for _, n := range todo {
		if r.stop = r.halt(); r.stop != "" {
			return nil
		}
		chains, err := r.j.chains.ListChains(ctx, r.underlying(n))
		if err != nil {
			if r.stop = r.callErr(ctx, err); r.stop != "" {
				return nil
			}
			r.log("list_chains_failed: " + n.st.Symbol + " " + err.Error())
			if terr := r.end(ctx, n, reasonChainsFail); terr != nil {
				return terr
			}
			continue
		}
		r.touts = 0
		n.chainsOK = true
		if terr := r.chosen(ctx, n, chains); terr != nil {
			return terr
		}
	}
	return nil
}

// chosen sets expected from the chains of a name and ends a name that has nothing to list.
func (r *listRun) chosen(ctx context.Context, n *listName, chains []broker.Chain) contracts.TypedError {
	var expiries []contracts.Day
	class := r.j.rules.Class(n.st)
	answered := false
	for _, ch := range chains {
		answered = answered || len(ch.Expirations) > 0
		if ch.TradingClass != class {
			continue
		}
		if n.mult == "" {
			n.mult = ch.Multiplier
		}
		for _, e := range ch.Expirations {
			if t, err := time.Parse("20060102", e); err == nil {
				expiries = append(expiries, contracts.Day(t.Format(time.DateOnly)))
			}
		}
	}
	if !answered {
		if len(chains) == 0 {
			r.log("list_chains_empty: " + n.st.Symbol + " answered no chains")
		} else {
			r.log("list_chains_empty: " + n.st.Symbol + " answered chains with no expiration")
		}
		return r.end(ctx, n, reasonChainsFail)
	}
	n.expected = []string{}
	seen := map[contracts.Day]bool{}
	for _, w := range r.j.m.Windows {
		for _, e := range ChooseExpiries(r.day, expiries, w.MinDTE, w.MaxDTE) {
			if !seen[e] {
				seen[e] = true
				n.expected = append(n.expected, string(e)+"|"+class)
			}
		}
	}
	if len(n.expected) == 0 {
		return r.end(ctx, n, reasonNoExpiry)
	}
	if terr := r.recount(ctx, n); terr != nil {
		return terr
	}
	if n.listed >= len(n.expected) {
		return r.end(ctx, n, reasonNone)
	}
	return r.put(ctx, n)
}

// sortGroups orders calls: held names, priority names, the rest; nearest expiry first.
func sortGroups(calls []listGroup) {
	sort.SliceStable(calls, func(a, b int) bool {
		x, y := calls[a], calls[b]
		switch {
		case x.n.rank != y.n.rank:
			return x.n.rank < y.n.rank
		case x.expiry != y.expiry:
			return x.expiry < y.expiry
		case x.n.st.Symbol != y.n.st.Symbol:
			return x.n.st.Symbol < y.n.st.Symbol
		}
		return x.class < y.class
	})
}

// callsPhase asks the groups no row lists for the session, sorted once across the names.
func (r *listRun) callsPhase(ctx context.Context) contracts.TypedError {
	var calls []listGroup
	for _, n := range r.names {
		if n.done || len(n.expected) == 0 {
			continue
		}
		for _, e := range n.expected {
			expiry, class, _ := strings.Cut(e, "|")
			if !n.lists[e] {
				calls = append(calls, listGroup{n: n, expiry: contracts.Day(expiry), class: class})
			}
		}
	}
	for _, g := range calls {
		g.n.pending++
	}
	sortGroups(calls)
	for _, g := range calls {
		if r.stop = r.halt(); r.stop != "" {
			return nil
		}
		if r.written >= r.j.m.MaxNew {
			r.stop = reasonMaxNew
			return nil
		}
		if terr := r.ask(ctx, g); terr != nil || r.stop != "" {
			return terr
		}
	}
	return nil
}

// finish gives a name whose groups were all asked its reason.
func (r *listRun) finish(ctx context.Context, n *listName) contracts.TypedError {
	if terr := r.recount(ctx, n); terr != nil {
		return terr
	}
	switch {
	case n.listed >= len(n.expected):
		return r.end(ctx, n, reasonNone)
	case n.spotSt == spotNone:
		return r.end(ctx, n, reasonNoSpot)
	case n.listed == 0:
		return r.end(ctx, n, reasonAllShort)
	}
	return r.end(ctx, n, reasonSomeShort)
}

// ask makes one ListContracts call and stores its answer; r.stop is set when the run must end. In the zone mode the
// group is listed with the stored sigma and no run row is written.
func (r *listRun) ask(ctx context.Context, g listGroup) contracts.TypedError {
	n := g.n
	if terr := r.loadRefs(ctx, n); terr != nil {
		return terr
	}
	spot, ok, terr := r.spotOf(ctx, n)
	if terr != nil {
		return terr
	}
	if !ok {
		return r.asked(ctx, n)
	}
	sigma, source := g.sigma, g.source
	if sigma <= 0 {
		if sigma, source, terr = r.sigmaOf(ctx, n, spot); terr != nil {
			return terr
		}
	}
	got, err := r.j.d.Lister.ListContracts(ctx, r.underlying(n), g.expiry, "", g.class)
	now := r.j.d.Clock.Now().UTC()
	outcome, accepted := "", []broker.Contract(nil)
	if err != nil {
		if r.stop = r.callErr(ctx, err); r.stop != "" {
			return nil
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			outcome = "error"
			n.answered[g.key()] = true
		}
	} else {
		r.touts = 0
		n.answered[g.key()] = true
		outcome, accepted = r.classify(n, g, got)
	}
	rec := openquote.ListRecord{UnderlyingID: n.id, Expiry: g.expiry, Class: g.class, TriedAt: now, Outcome: outcome}
	if outcome == "listed" {
		var inBand []broker.Contract
		lo, hi := r.band(g, spot, sigma, accepted)
		for _, c := range accepted {
			if v := strikeValue(c.Strike); v >= lo && v <= hi {
				inBand = append(inBand, c)
			}
		}
		capped, terr := r.write(ctx, n, g, inBand)
		if terr != nil {
			return terr
		}
		if capped {
			r.stop = reasonMaxNew
			return nil
		}
		rec.ListedAt, rec.ListedSession, rec.Count = now, r.day, len(inBand)
		rec.BandLo, rec.BandHi, rec.Sigma, rec.SigmaSource, rec.InBand = fmtStrike(lo), fmtStrike(hi), sigma, source, len(inBand)
		r.groups++
		r.relist[fmt.Sprintf("%d|%s", n.id, g.key())] = true
	}
	if outcome != "" {
		if perr := r.j.d.ListStore.PutLists(context.WithoutCancel(ctx), []openquote.ListRecord{rec}); perr != nil {
			return asTyped(perr, contracts.Retryable)
		}
	}
	return r.asked(ctx, n)
}

// band is the stored range of a listed group of the contracts.
func (r *listRun) band(g listGroup, spot, sigma float64, list []broker.Contract) (lo, hi float64) {
	var strikes []float64
	for _, c := range list {
		strikes = append(strikes, strikeValue(c.Strike))
	}
	m := r.j.m
	return ListBand(spot, sigma, daysTo(r.t, g.expiry), strikes, BandParams{K: m.BandK, FloorPct: m.BandFloorPct, MinSide: m.MinSide})
}

// asked counts one group of the name as handled; the last one ends the name (not in the zone mode).
func (r *listRun) asked(ctx context.Context, n *listName) contracts.TypedError {
	if r.zone {
		return nil
	}
	if n.pending--; n.pending == 0 {
		return r.finish(ctx, n)
	}
	return nil
}

func (r *listRun) loadRefs(ctx context.Context, n *listName) contracts.TypedError {
	if n.loaded {
		return nil
	}
	refs, err := r.j.d.Options.ListOptions(ctx, n.id)
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	n.refs, n.refSet, n.refCount, n.loaded = refs, map[string]bool{}, map[string]int{}, true
	for _, ref := range refs {
		n.refSet[refKey(ref.Expiry, ref.Strike, ref.Right, ref.TradingClass)] = true
		n.refCount[string(ref.Expiry)+"|"+ref.TradingClass]++
	}
	return nil
}
