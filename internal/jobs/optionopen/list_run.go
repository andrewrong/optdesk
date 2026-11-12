package optionopen

import (
	"context"
	"errors"
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
)

// listName is one stock of a run.
type listName struct {
	st       stock
	id       int64
	rank     int      // 0 held, 1 priority (BuyPrice), 2 the rest
	expected []string // "expiry|class"; nil until known
	reason   string
	mult     string // the multiplier of the chain; empty when this run did not ask ListChains
	listed   int    // groups listed for the session, recounted from master.option_list
	done     bool
	pending  int
	lists    map[string]bool // groups listed for the session
	refSet   map[string]bool // refKey of every stored contract
	refCount map[string]int  // stored contracts per "expiry|class"
	loaded   bool
}

type listGroup struct {
	n      *listName
	expiry contracts.Day
	class  string
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
}

// Run lists the stocks of Stored.Watch. The one key of the job is the only key it runs.
func (j *listJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	t, err := dayTime(day)
	if err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
	}
	if len(keys) == 0 || keys[0].DatasetID != ListDatasetID || keys[0].SubjectKey != ListSubject {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "unknown key")}
	}
	now := j.d.Clock.Now()
	r := &listRun{j: j, day: day, t: t, start: now,
		stopAt: time.Date(t.Year(), t.Month(), t.Day(), j.m.stopH, j.m.stopM, 0, 0, j.m.loc)}
	if terr := r.run(ctx); terr != nil {
		return contracts.JobResult{Returned: r.groups, Usable: r.groups, Err: terr}
	}
	res := contracts.JobResult{Returned: r.groups, Usable: r.groups}
	listed, err := r.anyListed(ctx)
	if err != nil {
		res.Err = asTyped(err, contracts.Retryable)
	} else if !listed && r.stop != reasonUnserved {
		res.Err = newErr(contracts.Retryable, "list_preopen: nothing was listed (%s)", r.mainReason())
	}
	r.summary(res.Err)
	return res
}

// Postcondition is true when some run row of the day has listed above 0.
func (j *listJob) Postcondition(ctx context.Context, day, _, _ string) (bool, error) {
	rows, err := j.d.ListStore.ListRuns(ctx, contracts.Day(day))
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Listed > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (r *listRun) anyListed(ctx context.Context) (bool, error) {
	return r.j.Postcondition(ctx, string(r.day), ListDatasetID, ListSubject)
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
	stocks, pos, err := d.stocks(ctx, r.t)
	if err != nil {
		return false, asTyped(err, contracts.Retryable)
	}
	rows, err := d.ListStore.ListRuns(ctx, r.day)
	if err != nil {
		return false, asTyped(err, contracts.Retryable)
	}
	prior := map[int64]openquote.ListRunRecord{}
	guard = true
	for _, row := range rows {
		prior[row.UnderlyingID] = row
		guard = guard && row.Listed == 0
	}
	var fresh []openquote.ListRunRecord
	for _, st := range stocks {
		id, err := d.underlyingID(ctx, st)
		if err != nil {
			return false, asTyped(err, contracts.Retryable)
		}
		n := &listName{st: st, id: id, rank: 2, reason: reasonNotFinished}
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

// recount reads the groups listed for the session from master.option_list.
func (r *listRun) recount(ctx context.Context, n *listName) contracts.TypedError {
	rows, err := r.j.d.ListStore.Lists(ctx, n.id)
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	n.lists, n.listed = map[string]bool{}, 0
	for _, row := range rows {
		if row.ListedSession == r.day {
			n.lists[string(row.Expiry)+"|"+row.Class] = true
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
	return broker.Contract{SecType: "STK", Symbol: n.st.Symbol, Exchange: n.st.Exchange, Currency: n.st.Currency, ConID: n.st.ConID}
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
			if terr := r.end(ctx, n, reasonChainsFail); terr != nil {
				return terr
			}
			continue
		}
		r.touts = 0
		if terr := r.chosen(ctx, n, chains); terr != nil {
			return terr
		}
	}
	return nil
}

// chosen sets expected from the chains of a name and ends a name that has nothing to list.
func (r *listRun) chosen(ctx context.Context, n *listName, chains []broker.Chain) contracts.TypedError {
	var expiries []contracts.Day
	answered := false
	for _, ch := range chains {
		answered = answered || len(ch.Expirations) > 0
		if ch.TradingClass != n.st.Symbol {
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
		return r.end(ctx, n, reasonChainsFail)
	}
	n.expected = []string{}
	for _, e := range ChooseExpiries(r.day, expiries, r.j.m.DTEMin, r.j.m.DTEMax) {
		n.expected = append(n.expected, string(e)+"|"+n.st.Symbol)
	}
	if len(n.expected) == 0 {
		return r.end(ctx, n, reasonNoExpiry)
	}
	return r.put(ctx, n)
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
	case n.listed == 0:
		return r.end(ctx, n, reasonAllShort)
	}
	return r.end(ctx, n, reasonSomeShort)
}

// ask makes one ListContracts call and stores its answer; r.stop is set when the run must end.
func (r *listRun) ask(ctx context.Context, g listGroup) contracts.TypedError {
	n := g.n
	if terr := r.loadRefs(ctx, n); terr != nil {
		return terr
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
		}
	} else {
		r.touts = 0
		outcome, accepted = r.classify(n, g, got)
	}
	rec := openquote.ListRecord{UnderlyingID: n.id, Expiry: g.expiry, Class: g.class, TriedAt: now, Outcome: outcome}
	if outcome == "listed" {
		capped, terr := r.write(ctx, n, g, accepted)
		if terr != nil {
			return terr
		}
		if capped {
			r.stop = reasonMaxNew
			return nil
		}
		rec.ListedAt, rec.ListedSession, rec.Count = now, r.day, len(accepted)
		r.groups++
	}
	if outcome != "" {
		if perr := r.j.d.ListStore.PutLists(context.WithoutCancel(ctx), []openquote.ListRecord{rec}); perr != nil {
			return asTyped(perr, contracts.Retryable)
		}
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
	n.refSet, n.refCount, n.loaded = map[string]bool{}, map[string]int{}, true
	for _, ref := range refs {
		n.refSet[refKey(ref.Expiry, ref.Strike, ref.Right, ref.TradingClass)] = true
		n.refCount[string(ref.Expiry)+"|"+ref.TradingClass]++
	}
	return nil
}
