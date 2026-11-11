package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/optionopen"
	"github.com/andrewrong/optdesk/internal/pages/option_boards"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// Design: docs/design/option-open-pull.md#10 (Decisions 13 to 17).

// The refresh button of the option pages under `ot serve --live-quotes`: the POST of options/refresh asks pyside for
// the plan contracts of the symbols of a page and keeps the rows in memory; the overlay reader puts them over the
// stored obs.option_open rows. The flag is accepted for a loopback address only (serve.go). The client is the
// snapshot port of broker.Broker: it holds no way to send an instruction to the exchange, writes no row and reads no
// account, and the contracts it asks are those of the stored plan only.

// liveRefreshRoute is the route of the POST under the root of the site.
const liveRefreshRoute = "options/refresh"

const (
	liveMaxSymbols   = 8
	liveMaxContracts = 40
	liveReuse        = 60 * time.Second
	liveWait         = 3 * time.Second
	liveChunk        = 20
	liveWindowChunk  = 10 // a call inside the pull window asks one chunk of this size at most
	liveMaxPerHour   = 30
	liveBodyLimit    = 64 << 10
	liveDialTimeout  = 5 * time.Second
	liveCardKind     = "short_put"
	liveHeldClass    = "held"
)

// pull window of the US job in the zone of the exchange: 09:44 to 10:10 (Decision 17).
const (
	liveWindowFrom = 9*60 + 44
	liveWindowTo   = 10*60 + 10
)

// liveMarket is one market of the plan: the slot of its pull job, its calendar and the contract fields.
type liveMarket struct {
	name, slot, calendar, exchange, currency string
	zone                                     *time.Location
	lunch                                    [2]string // HHMM; empty: no break
}

// liveMarkets are the US and the Japan markets (the slots and fields of job_pull.toml and job_pull_jp.toml).
func liveMarkets() ([]liveMarket, error) {
	ny, err := time.LoadLocation(marketZone)
	if err != nil {
		return nil, err
	}
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return nil, err
	}
	return []liveMarket{
		{name: "US", slot: "0945", calendar: "XNYS", exchange: "SMART", currency: "USD", zone: ny},
		{name: "JP", slot: "0905", calendar: "XTKS", exchange: "TSEJ", currency: "JPY", zone: tokyo, lunch: optionopen.JPLunchBreak()},
	}, nil
}

// liveDeps are the ports of liveQuotes; each is narrow: reads, the clock and one dial.
type liveDeps struct {
	Clock     clock.Clock
	Calendar  clock.Calendar // nil: the session check answers closed
	Plan      optionopen.PlanReader
	Contracts optionopen.ContractLister
	Insts     store.InstrumentReader
	Stored    openquote.OptionOpenReader // the stored rows the overlay sits over; nil: none
	Cands     store.CandidateReader      // the card contract of each symbol; nil: none
	Names     store.InstrumentNameReader // the ticker of a card; nil: none
	Dial      func(ctx context.Context) (broker.Broker, error)
	After     func(time.Duration) <-chan time.Time // nil: time.After
}

// liveAnswer is the answer of one refresh call.
type liveAnswer struct {
	Notice string `json:"notice"`
	Fresh  int    `json:"fresh_count"`
	Total  int    `json:"total_count"`
}

type liveEntry struct {
	at   time.Time
	rows map[int]contracts.OptionOpen // by instrument id
}

// liveQuotes holds the in-memory rows of the refresh calls and the limits of Decisions 15 and 17.
type liveQuotes struct {
	d       liveDeps
	markets []liveMarket
	sem     chan struct{} // one call at a time per process

	mu      sync.Mutex
	running string // symbol set of the call that holds sem
	cache   map[string]*liveEntry
	last    map[string]liveAnswer // by symbol set
	calls   []time.Time
	brk     broker.Broker
}

func newLive(d liveDeps) (*liveQuotes, error) {
	ms, err := liveMarkets()
	if err != nil {
		return nil, err
	}
	if d.After == nil {
		d.After = time.After
	}
	return &liveQuotes{d: d, markets: ms, sem: make(chan struct{}, 1), cache: map[string]*liveEntry{}, last: map[string]liveAnswer{}}, nil
}

// newLiveQuotes builds the refresh machinery of `ot serve --live-quotes` over the readers the site opened and the
// instrument store of the pages line; the client of the pyside socket is dialled at the first refresh. It returns a
// close function. A missing store gives an error, never a half-wired route.
func newLiveQuotes(ctx context.Context, a *app, clk clock.Clock, rd siteReaders) (*liveQuotes, func(), error) {
	if rd.plan == nil || rd.optContracts == nil || a.deps.openInstrument == nil || a.deps.dialBroker == nil {
		return nil, nil, errors.New("serve: --live-quotes needs the option open stores and the instrument store")
	}
	env, err := a.envFile()
	if err != nil {
		return nil, nil, err
	}
	insts, err := a.deps.openInstrument(ctx, env[pagesDSNName])
	if err != nil {
		return nil, nil, errors.New("serve: --live-quotes could not open the instrument store")
	}
	sock := filepath.Join(a.getenv("OT_HOME"), "run", "ibkr.sock")
	d := liveDeps{
		Clock: clk, Plan: rd.plan, Contracts: rd.optContracts, Insts: insts, Stored: rd.open, Cands: rd.cands, Names: rd.names,
		Dial: func(ctx context.Context) (broker.Broker, error) {
			return a.deps.dialBroker(ctx, sock, broker.Options{})
		},
	}
	if rd.sessions != nil && a.deps.newCalendar != nil {
		d.Calendar = a.deps.newCalendar(rd.sessions)
	}
	l, err := newLive(d)
	if err != nil {
		_ = insts.Close()
		return nil, nil, err
	}
	return l, func() {
		l.mu.Lock()
		if l.brk != nil {
			_ = l.brk.Close()
		}
		l.mu.Unlock()
		_ = insts.Close()
	}, nil
}

// dayOf is the session day of now in the zone of m.
func (m liveMarket) dayOf(now time.Time) contracts.Day {
	return contracts.Day(now.In(m.zone).Format(time.DateOnly))
}

// open reports whether m is inside its regular session at now: between the open and the close of the calendar (an
// early close included) and outside the lunch break.
func (l *liveQuotes) open(ctx context.Context, m liveMarket, now time.Time) bool {
	if l.d.Calendar == nil {
		return false
	}
	s, err := l.d.Calendar.Session(ctx, m.calendar, m.dayOf(now))
	if err != nil || !s.IsSession || now.Before(s.OpenAt) || !now.Before(s.CloseAt) {
		return false
	}
	if m.lunch[0] != "" {
		hm := now.In(m.zone).Format("1504")
		if hm >= m.lunch[0] && hm < m.lunch[1] {
			return false
		}
	}
	return true
}

func (l *liveQuotes) anyOpen(ctx context.Context, now time.Time) bool {
	for _, m := range l.markets {
		if l.open(ctx, m, now) {
			return true
		}
	}
	return false
}

// inPullWindow reports whether now is inside 09:44 to 10:10 in the zone of the US exchange.
func (l *liveQuotes) inPullWindow(now time.Time) bool {
	n := now.In(l.markets[0].zone)
	m := n.Hour()*60 + n.Minute()
	return m >= liveWindowFrom && m <= liveWindowTo
}

// OptionOpen is the overlay reader: the stored rows of the day with the in-memory rows of the refresh calls over
// them (the newer of the two per instrument), each added row with the slot openquote.SlotRefresh; by instrument id.
func (l *liveQuotes) OptionOpen(ctx context.Context, day contracts.Day) ([]contracts.OptionOpen, error) {
	var stored []contracts.OptionOpen
	if l.d.Stored != nil {
		var err error
		if stored, err = l.d.Stored.OptionOpen(ctx, day); err != nil {
			return nil, err
		}
	}
	by := make(map[int]contracts.OptionOpen, len(stored))
	for _, r := range stored {
		by[r.InstrumentID] = r
	}
	l.mu.Lock()
	for _, e := range l.cache {
		for id, r := range e.rows {
			if r.SessionDay.Format(time.DateOnly) != string(day) {
				continue
			}
			if old, ok := by[id]; ok && old.ReceivedAt.After(r.ReceivedAt) {
				continue
			}
			r.Slot = openquote.SlotRefresh
			by[id] = r
		}
	}
	l.mu.Unlock()
	out := make([]contracts.OptionOpen, 0, len(by))
	for _, r := range by {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstrumentID < out[j].InstrumentID })
	return out, nil
}

// state is the RefreshState of a page whose cards are symbols: Enabled when a market is inside its session.
func (l *liveQuotes) state(ctx context.Context, symbols []string) option_boards.RefreshState {
	if l == nil {
		return option_boards.RefreshState{Reason: "live quotes are off (ot serve --live-quotes)"}
	}
	s := option_boards.RefreshState{Symbols: symbols}
	switch {
	case len(symbols) == 0:
		s.Reason = "no symbols on this page"
	case !l.anyOpen(ctx, l.d.Clock.Now()):
		s.Reason = "market closed"
	default:
		s.Enabled = true
	}
	l.mu.Lock()
	a, ok := l.last[symbolKey(cleanSymbols(symbols))]
	l.mu.Unlock()
	if ok {
		s.Notice, s.Fresh, s.Total = a.Notice, a.Fresh, a.Total
	}
	return s
}

// cleanSymbols upper-cases, drops empty and repeated symbols and keeps the first liveMaxSymbols.
func cleanSymbols(in []string) []string {
	var out []string
	for _, s := range in {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	if len(out) > liveMaxSymbols {
		out = out[:liveMaxSymbols]
	}
	return out
}

func symbolKey(syms []string) string {
	s := slices.Clone(syms)
	sort.Strings(s)
	return strings.Join(s, ",")
}

// acquire takes the one-call slot. A call that finds it taken waits up to 3 s; it gets the slot only when the call
// it waited for asked the same symbols (its rows are then reused), else the answer is the busy notice.
func (l *liveQuotes) acquire(ctx context.Context, key string) bool {
	select {
	case l.sem <- struct{}{}:
	default:
		l.mu.Lock()
		same := l.running == key
		l.mu.Unlock()
		select {
		case l.sem <- struct{}{}:
			if !same {
				<-l.sem
				return false
			}
		case <-l.d.After(liveWait):
			return false
		case <-ctx.Done():
			return false
		}
	}
	l.mu.Lock()
	l.running = key
	l.mu.Unlock()
	return true
}

func (l *liveQuotes) release() {
	l.mu.Lock()
	l.running = ""
	l.mu.Unlock()
	<-l.sem
}

// refresh runs one call for the symbols and answers with a notice; it never fails: the stored rows stay what the
// pages show whenever nothing fresh came (Decision 16).
func (l *liveQuotes) refresh(ctx context.Context, symbols []string) liveAnswer {
	syms := cleanSymbols(symbols)
	if len(syms) == 0 {
		return liveAnswer{Notice: "no symbols"}
	}
	key := symbolKey(syms)
	ans := l.run(ctx, syms, key)
	l.mu.Lock()
	l.last[key] = ans
	l.mu.Unlock()
	return ans
}

func (l *liveQuotes) run(ctx context.Context, syms []string, key string) liveAnswer {
	if !l.acquire(ctx, key) {
		return liveAnswer{Notice: "another refresh is running, try again"}
	}
	defer l.release()
	now := l.d.Clock.Now()
	var open []liveMarket
	for _, m := range l.markets {
		if l.open(ctx, m, now) {
			open = append(open, m)
		}
	}
	if len(open) == 0 {
		return liveAnswer{Notice: l.closedNotice(ctx, now)}
	}
	reused, reusedRows, oldest, ask := l.reuse(syms, now)
	if len(ask) == 0 {
		return liveAnswer{Notice: fmt.Sprintf("reused, %d s old", int(oldest.Seconds())), Fresh: reusedRows, Total: reusedRows}
	}
	targets, err := l.targets(ctx, open, now, ask)
	if err != nil {
		return liveAnswer{Notice: "stored quotes shown: the plan could not be read"}
	}
	if len(targets) == 0 {
		return liveAnswer{Notice: "no plan today"}
	}
	limit, chunk := liveMaxContracts, liveChunk
	if l.inPullWindow(now) {
		limit, chunk = liveWindowChunk, liveWindowChunk
	}
	if len(targets) > limit {
		targets = targets[:limit]
	}
	if !l.countCall(now) {
		return liveAnswer{Notice: fmt.Sprintf("at most %d refreshes an hour; stored quotes shown", liveMaxPerHour)}
	}
	b, err := l.broker(ctx)
	if err != nil {
		return liveAnswer{Notice: "pyside is not reachable; stored quotes shown"}
	}
	res := optionopen.Refresh(ctx, b, l.d.Clock, targets, chunk)
	l.keep(targets, res.Rows, ask, l.d.Clock.Now())
	total, fresh := len(targets)+reusedRows, len(res.Rows)+reusedRows
	notice := fmt.Sprintf("%d of %d quotes fresh", fresh, total)
	if n := total - fresh; n > 0 {
		notice += fmt.Sprintf(", %d from %s", n, l.storedLabel(ctx, open[0], now))
	}
	if len(reused) > 0 {
		notice += fmt.Sprintf("; reused, %d s old", int(oldest.Seconds()))
	}
	return liveAnswer{Notice: notice, Fresh: fresh, Total: total}
}

// reuse splits syms into those whose newest refresh is under 60 s old (reused) and those to ask.
func (l *liveQuotes) reuse(syms []string, now time.Time) (reused []string, rows int, oldest time.Duration, ask []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range syms {
		e, ok := l.cache[s]
		if !ok || now.Sub(e.at) >= liveReuse || len(e.rows) == 0 {
			ask = append(ask, s)
			continue
		}
		reused, rows = append(reused, s), rows+len(e.rows)
		oldest = max(oldest, now.Sub(e.at))
	}
	return reused, rows, oldest, ask
}

// countCall records a call that asks the gateway and refuses the 31st within an hour.
func (l *liveQuotes) countCall(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.calls[:0]
	for _, t := range l.calls {
		if now.Sub(t) < time.Hour {
			kept = append(kept, t)
		}
	}
	l.calls = kept
	if len(l.calls) >= liveMaxPerHour {
		return false
	}
	l.calls = append(l.calls, now)
	return true
}

// broker dials the pyside client once and keeps it.
func (l *liveQuotes) broker(ctx context.Context) (broker.Broker, error) {
	l.mu.Lock()
	b := l.brk
	l.mu.Unlock()
	if b != nil {
		return b, nil
	}
	if l.d.Dial == nil {
		return nil, errors.New("no dial")
	}
	ctx, cancel := context.WithTimeout(ctx, liveDialTimeout)
	defer cancel()
	b, err := l.d.Dial(ctx)
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	l.brk = b
	l.mu.Unlock()
	return b, nil
}

// keep stores the rows of a call by symbol; a symbol that got no row keeps its older rows and its older time.
func (l *liveQuotes) keep(targets []optionopen.Target, rows []contracts.OptionOpen, ask []string, at time.Time) {
	sym := map[int]string{}
	for _, t := range targets {
		sym[t.Item.InstrumentID] = strings.ToUpper(t.Symbol)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, r := range rows {
		s, ok := sym[r.InstrumentID]
		if !ok {
			continue
		}
		e := l.cache[s]
		if e == nil {
			e = &liveEntry{rows: map[int]contracts.OptionOpen{}}
			l.cache[s] = e
		}
		e.rows[r.InstrumentID], e.at = r, at
	}
}

// storedLabel is the time of the newest stored row of the day of m ("09:45"), else the close archive.
func (l *liveQuotes) storedLabel(ctx context.Context, m liveMarket, now time.Time) string {
	if t, ok := l.newestStored(ctx, m, now); ok {
		return t.In(m.zone).Format("15:04")
	}
	return "the close archive"
}

func (l *liveQuotes) newestStored(ctx context.Context, m liveMarket, now time.Time) (time.Time, bool) {
	if l.d.Stored == nil {
		return time.Time{}, false
	}
	rows, err := l.d.Stored.OptionOpen(ctx, m.dayOf(now))
	var newest time.Time
	for _, r := range rows {
		if err == nil && r.ReceivedAt.After(newest) {
			newest = r.ReceivedAt
		}
	}
	return newest, !newest.IsZero()
}

// closedNotice is "market closed; stored quotes of <time>" with the newest stored row of any market.
func (l *liveQuotes) closedNotice(ctx context.Context, now time.Time) string {
	for _, m := range l.markets {
		if t, ok := l.newestStored(ctx, m, now); ok {
			return "market closed; stored quotes of " + t.In(m.zone).Format("15:04")
		}
	}
	return "market closed; no stored quotes"
}

// cardIDs are the instrument ids of the card contracts (the ok rank 1 short put of each underlying) of the day.
func (l *liveQuotes) cardIDs(ctx context.Context, day contracts.Day) map[int]bool {
	out := map[int]bool{}
	if l.d.Cands == nil {
		return out
	}
	rows, err := l.d.Cands.Candidates(ctx, day, liveCardKind)
	if err != nil {
		return out
	}
	for _, r := range rows {
		if isCardRow(r) {
			out[r.InstrumentID] = true
		}
	}
	return out
}

// targets reads today's plan of the open markets for syms and returns the contracts to ask in the order of
// Decision 14: held first, then the card contract of each symbol, then the rest by plan rank.
func (l *liveQuotes) targets(ctx context.Context, open []liveMarket, now time.Time, syms []string) ([]optionopen.Target, error) {
	var out []optionopen.Target
	for _, m := range open {
		day := m.dayOf(now)
		ts, err := l.marketTargets(ctx, m, day, syms)
		if err != nil {
			return nil, err
		}
		cards := l.cardIDs(ctx, day)
		rank := func(t optionopen.Target) int {
			switch {
			case string(t.Item.Class) == liveHeldClass:
				return 0
			case cards[t.Item.InstrumentID]:
				return 1
			}
			return 2
		}
		sort.SliceStable(ts, func(i, j int) bool {
			if ri, rj := rank(ts[i]), rank(ts[j]); ri != rj {
				return ri < rj
			}
			return ts[i].Item.Rank < ts[j].Item.Rank
		})
		out = append(out, ts...)
	}
	return out, nil
}

// ServeHTTP is the POST of options/refresh: a JSON body {"symbols": [...]} and the answer {"notice", "fresh_count",
// "total_count"} with status 200 whatever the gateway did.
func (l *liveQuotes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Symbols []string `json:"symbols"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, liveBodyLimit)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	b, err := json.Marshal(l.refresh(r.Context(), req.Symbols))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
