package optionopen

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

//go:embed job_resolve.toml
var resolveManifest []byte

//go:embed job_gap.toml
var gapManifest []byte

// ResolveDatasetID is the dataset of the keys of the resolve jobs: one key per stock.
const ResolveDatasetID = "master.option_contract"

// GapDatasetID is the dataset of the keys of the gap pass. It differs from ResolveDatasetID so that the
// reconciler, which keeps the first job per key, does not give the gap keys to the evening resolve.
const GapDatasetID = "master.option_contract.gap"

// stopAfterTimeouts is the number of consecutive timeouts that stops the resolves of a run (option-trader #757).
const stopAfterTimeouts = 2

// ResolveDeps are what the evening resolve and the gap pass use. Pull nil means no mark is made from the pull.
// Log receives the one summary line of a run (nil: discarded). Rand returns a value in [0, 1) for the jitter of
// a mark (nil: math/rand/v2).
type ResolveDeps struct {
	Stored
	Broker broker.Broker
	// History asks the gateway for the last daily bar of a name that has no stored close (nil: nothing is asked).
	History broker.HistoryClient
	Marks   openquote.OptionNotListedStore
	Pull    openquote.OptionOpenReader
	Clock   clock.Clock
	Log     func(line string)
	Rand    func() float64
}

// ResolveOptions are the options of NewResolve and NewGap. MaxNew above 0 overrides max_new of the manifest;
// the wiring reads OT_OPEN_RESOLVE_MAX_NEW and passes it, so one hand run can take 1500.
type ResolveOptions struct {
	MaxNew int
}

type resolveJob struct {
	dataset string // the dataset of the keys
	m       manifest
	d       ResolveDeps
	maxNew  int
	mu      sync.Mutex
	day     contracts.Day // the day the counter below belongs to
	used    int           // new resolves of that day, over every key of the job
}

// NewResolve builds option_open_resolve (US) from the embedded job_resolve.toml. It puts every contract it
// finds into master.option_contract, marks the contracts IBKR answered none for, and turns the not_listed rows of
// the pull into marks. It is the only writer of marks.
func NewResolve(d ResolveDeps, o ResolveOptions) (contracts.Job, error) {
	return newListed(resolveManifest, ResolveDatasetID, d, o)
}

// NewGap builds option_open_resolve_gap (US) from the embedded job_gap.toml: the same code, a gap-only pass that
// reads no pull rows and resolves only what neither the cache nor the marks hold.
func NewGap(d ResolveDeps, o ResolveOptions) (contracts.Job, error) {
	return newListed(gapManifest, GapDatasetID, d, o)
}

func newListed(data []byte, dataset string, d ResolveDeps, o ResolveOptions) (contracts.Job, error) {
	if d.Broker == nil || d.Marks == nil || d.Instruments == nil || d.Bars == nil || d.Options == nil || d.Clock == nil {
		return nil, errors.New("optionopen: broker, marks, instruments, bars, options and clock are required")
	}
	m, err := parseManifest(data)
	if err != nil {
		return nil, err
	}
	if d.Rand == nil {
		d.Rand = rand.Float64
	}
	limit := m.MaxNew
	if o.MaxNew > 0 {
		limit = o.MaxNew
	}
	return &resolveJob{dataset: dataset, m: m, d: d, maxNew: limit}, nil
}

func (j *resolveJob) ID() string { return j.m.ID }

// Schedule is the slot as plain values.
func (j *resolveJob) Schedule() (string, string, string, time.Duration) { return j.m.schedule() }

// Window is the window rule of the manifest.
func (j *resolveJob) Window() (string, time.Duration, time.Duration) { return j.m.window() }

// RunBound is the longest time one run of a key may take.
func (j *resolveJob) RunBound() time.Duration { return j.m.runBound }

// RestartSensitive is false: a restart resumes from the stored cache and marks.
func (j *resolveJob) RestartSensitive() bool { return j.m.Schedule.RestartSensitive }

// Plan returns one key per stock and the hash of day, manifest id and symbols.
func (j *resolveJob) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	t, err := dayTime(day)
	if err != nil {
		return nil, "", err
	}
	stocks, _, err := j.d.stocks(ctx, t)
	if err != nil {
		return nil, "", err
	}
	keys := make([]contracts.ExpectedKey, 0, len(stocks))
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s", day, j.m.ID)
	for _, s := range stocks {
		keys = append(keys, contracts.ExpectedKey{DatasetID: j.dataset, SubjectKey: s.Symbol})
		b.WriteString("\n" + s.Symbol)
	}
	return keys, hashOf(b.String()), nil
}

// counts are the numbers of one run, in the order of its summary line.
type counts struct {
	asked, found, notListed, ambiguous, timeout, disconnect, other int
	deferred, unsent, fromPull                                     int
}

// outcome is the class of one answer of the broker (Decision 6).
type outcome int

const (
	outFound outcome = iota
	outNotListed
	outAmbiguous
	outTimeout
	outDisconnect
	outOther
)

// classify says what an answer is. Only outNotListed is ever marked: an empty list, or error 200 whose text is
// "no security definition", for the one contract that was asked.
func classify(got broker.Resolved, err error, req broker.Contract) outcome {
	if err != nil {
		var te contracts.TypedError
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &te) && te.Kind() == contracts.Retryable) {
			return outTimeout
		}
		return outOther
	}
	switch {
	case got.Listed && got.Contract.ConID > 0 && got.ErrorCode == 0:
		if differs(got.Contract, req) {
			return outAmbiguous
		}
		return outFound
	case got.Listed && got.ErrorCode == 0:
		return outAmbiguous // listed with no con_id: neither a contract nor an empty list
	case got.ErrorCode == 0:
		return outNotListed // an empty list
	case got.ErrorCode == 200:
		if strings.Contains(strings.ToLower(got.ErrorText), "no security definition") {
			return outNotListed
		}
		return outAmbiguous
	case got.ErrorCode == 10197 || (got.ErrorCode >= 1100 && got.ErrorCode <= 1102):
		return outDisconnect
	}
	return outOther // 101, 100, 162, 354, 10089 to 10091 and every unknown code
}

func differs(got, req broker.Contract) bool {
	if got.Expiry != "" && got.Expiry != req.Expiry {
		return true
	}
	if got.Right != "" && got.Right != req.Right {
		return true
	}
	return got.Strike != "" && strikeKey(got.Strike) != strikeKey(req.Strike)
}

// Run resolves the stocks of keys. Each call logs one summary line, and a Retryable end one fault line.
func (j *resolveJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	res := j.run(ctx, day, keys)
	if res.Err != nil && res.Err.Kind() == contracts.Retryable {
		key := ""
		if len(keys) > 0 {
			key = keys[0].SubjectKey
		}
		j.logf("fault stage=%s day=%s key=%s reason=%s", j.m.ID, day, key, res.Err.Error())
	}
	return res
}

func (j *resolveJob) run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	t, err := dayTime(day)
	if err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
	}
	start := j.d.Clock.Now()
	stocks, pos, err := j.d.stocks(ctx, t)
	if err != nil {
		return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
	}
	var cnt counts
	var res contracts.JobResult
	if _, err := j.d.Marks.RemoveNotListedExpired(ctx, day); err != nil {
		return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
	}
	var pulled []contracts.OptionOpen
	if !j.m.GapOnly && j.d.Pull != nil {
		if pulled, err = j.d.Pull.OptionOpen(ctx, day); err != nil {
			return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
		}
	}
	for _, k := range keys {
		st, ok := findStock(stocks, k.SubjectKey)
		if !ok {
			res.Err = firstErr(res.Err, newErr(contracts.Permanent, "unknown stock %q", k.SubjectKey))
			continue
		}
		r := keyRun{j: j, day: day, t: t, start: start, st: st, pos: pos, pulled: pulled, cnt: &cnt}
		if terr := r.run(ctx); terr != nil {
			res.Err = firstErr(res.Err, terr)
		}
	}
	res.Returned, res.Usable = cnt.asked, cnt.found+cnt.notListed
	if res.Err == nil && cnt.unsent > 0 {
		res.Err = newErr(contracts.Retryable, "%d contracts were not sent", cnt.unsent)
	}
	j.logf("%s day=%s cap=%d used=%d asked=%d found=%d not_listed=%d ambiguous=%d timeout=%d disconnect=%d error=%d "+
		"deferred=%d unsent=%d from_pull=%d run_err=%s",
		j.m.ID, day, j.maxNew, j.usedOn(day), cnt.asked, cnt.found, cnt.notListed, cnt.ambiguous, cnt.timeout, cnt.disconnect, cnt.other,
		cnt.deferred, cnt.unsent, cnt.fromPull, runErrText(res.Err))
	return res
}

func runErrText(err contracts.TypedError) string {
	if err == nil {
		return "none"
	}
	return fmt.Sprintf("%s: %s", err.Kind(), err.Error())
}

func (j *resolveJob) logf(format string, a ...any) {
	if j.d.Log != nil {
		j.d.Log(fmt.Sprintf(format, a...))
	}
}

func findStock(ss []stock, symbol string) (stock, bool) {
	for _, s := range ss {
		if s.Symbol == symbol {
			return s, true
		}
	}
	return stock{}, false
}

// firstErr keeps a Retryable error over any other, and the first of equals.
func firstErr(cur, next contracts.TypedError) contracts.TypedError {
	if cur == nil || (cur.Kind() != contracts.Retryable && next.Kind() == contracts.Retryable) {
		return next
	}
	return cur
}

func (j *resolveJob) usedOn(day contracts.Day) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.day != day {
		return 0
	}
	return j.used
}

// take reserves one new resolve of day against the cap.
func (j *resolveJob) take(day contracts.Day) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.day != day {
		j.day, j.used = day, 0
	}
	if j.used >= j.maxNew {
		return false
	}
	j.used++
	return true
}

// candidate is one contract the band holds that the cache may not.
type candidate struct {
	req    broker.Contract
	expiry contracts.Day
	dist   float64
	held   bool
}

func (c candidate) key() string {
	return refKey(c.expiry, c.req.Strike, c.req.Right, c.req.TradingClass)
}

type keyRun struct {
	j      *resolveJob
	day    contracts.Day
	t      time.Time
	start  time.Time
	st     stock
	pos    []store.Position
	pulled []contracts.OptionOpen
	cnt    *counts
}

func (r *keyRun) run(ctx context.Context) contracts.TypedError {
	j := r.j
	if r.st.ConID == 0 {
		return newErr(contracts.Retryable, "no con_id of %s", r.st.Symbol)
	}
	id, err := j.d.underlyingID(ctx, r.st)
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	spot, ok, err := j.d.lastClose(ctx, id, r.t)
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	if !ok {
		// A missing stored input is no answer of the gateway: ask for the last daily bar once.
		if spot, ok, err = fetchClose(ctx, j.d.History, r.st); err != nil {
			return newErr(contracts.Retryable, "%s: no stored close of %s and the fetch failed: %v", reasonFetchFailed, r.st.Symbol, err)
		}
		if !ok {
			return newErr(contracts.Permanent, "%s: no close of %s", reasonNoClose, r.st.Symbol)
		}
	}
	refs, err := j.d.Options.ListOptions(ctx, id)
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	now := j.d.Clock.Now()
	// The chain is listed first, so that the gap is seen against the morning chain.
	chains, err := j.d.Broker.ListChains(ctx, broker.Contract{SecType: "STK", Symbol: r.st.Symbol, Exchange: r.st.Exchange,
		Currency: r.st.Currency, ConID: r.st.ConID})
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	marked, terr := r.marks(ctx, id, spot, refs, chains, now)
	if terr != nil {
		return terr
	}
	prior, last, err := j.d.sigmas(ctx, id, refs, spot, r.t)
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	sigma, _ := ResolveSigma(prior, last)
	cands := r.candidates(chains, spot, sigma, hasPosition(r.pos, r.st.Symbol))
	cached := map[string]bool{}
	for _, ref := range refs {
		cached[refKey(ref.Expiry, ref.Strike, ref.Right, ref.TradingClass)] = true
	}
	return r.resolveAll(ctx, id, cands, cached, marked)
}

// resolveAll asks the candidates the cache and the marks do not hold, one request in flight, and writes the marks
// of the clean not-listed answers at the end.
func (r *keyRun) resolveAll(ctx context.Context, id int64, cands []candidate, cached, marked map[string]bool) contracts.TypedError {
	j := r.j
	var fresh []contracts.OptionNotListed
	timeouts := 0
	stopped := false
	budget := time.Duration(j.m.BudgetMinutes) * time.Minute
	for _, c := range cands {
		if cached[c.key()] || marked[c.key()] {
			continue
		}
		if stopped || j.d.Clock.Now().Sub(r.start) >= budget {
			r.cnt.unsent++
			continue
		}
		if !j.take(r.day) {
			r.cnt.deferred++
			continue
		}
		r.cnt.asked++
		got, rerr := j.d.Broker.ResolveContract(ctx, c.req)
		switch classify(got, rerr, c.req) {
		case outFound:
			timeouts = 0
			if perr := r.store(ctx, id, c, got.Contract); perr != nil {
				return perr
			}
			r.cnt.found++
		case outNotListed:
			timeouts = 0
			r.cnt.notListed++
			fresh = append(fresh, r.mark(id, c, got, j.d.Clock.Now()))
		case outAmbiguous:
			timeouts = 0
			r.cnt.ambiguous++
		case outTimeout:
			r.cnt.timeout++
			timeouts++
		case outDisconnect:
			r.cnt.disconnect++
			timeouts++
		case outOther:
			r.cnt.other++
		}
		stopped = timeouts >= stopAfterTimeouts
	}
	if len(fresh) > 0 {
		if err := j.d.Marks.AddNotListed(ctx, fresh); err != nil {
			return asTyped(err, contracts.Retryable)
		}
	}
	return nil
}

// store puts a found contract into master.instrument and master.option_contract.
func (r *keyRun) store(ctx context.Context, underlyingID int64, c candidate, got broker.Contract) contracts.TypedError {
	j := r.j
	req := c.req
	spec := store.InstrumentSpec{Type: "option", Name: strings.Join([]string{req.Symbol, req.Expiry, req.Right, req.Strike}, " "),
		Currency: req.Currency, CalendarID: "XNYS", Status: "active", PriceIncrement: "0.01", LotSize: req.Multiplier}
	var extra []store.Identifier
	if osi, ok := osiOf(req); ok {
		extra = append(extra, store.Identifier{Scheme: "osi", Value: osi})
	}
	key := store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(got.ConID, 10)}
	iid, err := j.d.Instruments.ResolveInstrument(ctx, key, spec, extra)
	if err != nil {
		return asTyped(err, contracts.Permanent)
	}
	err = j.d.Options.PutOption(ctx, openquote.OptionRef{UnderlyingID: underlyingID, Currency: req.Currency, Exchange: req.Exchange,
		TradingClass: req.TradingClass, Multiplier: req.Multiplier, Expiry: c.expiry, Strike: req.Strike, Right: req.Right, InstrumentID: iid})
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	return nil
}

func osiOf(c broker.Contract) (string, bool) {
	strike, err := strconv.ParseFloat(c.Strike, 64)
	if err != nil || len(c.Expiry) != 8 || len(c.Symbol) > 6 || strike < 0 || strike >= 100000 {
		return "", false
	}
	return fmt.Sprintf("%-6s%s%s%08d", c.Symbol, c.Expiry[2:], c.Right, int64(math.Round(strike*1000))), true
}

// mark is the mark of a contract IBKR answered none for. retry_at is checked_at plus retry_days plus a jitter
// drawn per row, uniform in minus to plus jitter_days.
func (r *keyRun) mark(underlyingID int64, c candidate, got broker.Resolved, now time.Time) contracts.OptionNotListed {
	m := r.j.m
	jitter := time.Duration((r.j.d.Rand()*2 - 1) * float64(m.JitterDays) * float64(24*time.Hour))
	out := contracts.OptionNotListed{
		CheckedAt: now.UTC(), RetryAt: now.UTC().Add(time.Duration(m.RetryDays)*24*time.Hour + jitter),
		Expiry: sessionDate(mustDay(c.expiry)), Strike: c.req.Strike,
		TradingClass: c.req.TradingClass, UnderlyingID: int(underlyingID),
	}
	setEnum(&out.Right, c.req.Right)
	if got.ErrorCode != 0 {
		code, text := strconv.Itoa(got.ErrorCode), got.ErrorText
		out.ErrorCode, out.ErrorText = &code, &text
	}
	return out
}

// statusNotListed is the status of a pull row whose contract is not listed (Decision 8).
const statusNotListed = 4

func mustDay(d contracts.Day) time.Time {
	t, _ := dayTime(d)
	return t
}

// marks reads the marks of the stock, applies the early ends and the pull rows, and returns the active marks by
// contract key. The four triggers of Decision 5 end a mark early: (a) a mark past retry_at is not returned by the
// store; (b) the same strike, right and class is stored listed at another expiry; (c) the morning chain, listed
// by the gap pass first, lists the marked strike at the marked expiry; (d) a stored close that moved by spot_move
// since checked_at ends every mark of the stock.
func (r *keyRun) marks(ctx context.Context, id int64, spot float64, refs []openquote.OptionRef,
	chains []broker.Chain, now time.Time) (map[string]bool, contracts.TypedError) {
	j := r.j
	all, err := j.d.Marks.NotListed(ctx, now)
	if err != nil {
		return nil, asTyped(err, contracts.Retryable)
	}
	var mine []contracts.OptionNotListed
	for _, m := range all {
		if int64(m.UnderlyingID) == id {
			mine = append(mine, m)
		}
	}
	moved := false
	closeAt := map[string]float64{}
	for _, m := range mine {
		at := m.CheckedAt.UTC()
		d := at.Format(time.DateOnly)
		c, seen := closeAt[d]
		if !seen {
			var ok bool
			if c, ok, err = j.d.lastClose(ctx, id, time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)); err != nil {
				return nil, asTyped(err, contracts.Retryable)
			}
			if !ok {
				c = 0
			}
			closeAt[d] = c
		}
		if c > 0 && math.Abs(spot-c)/c >= j.m.SpotMove {
			moved = true
			break
		}
	}
	if moved {
		if _, err := j.d.Marks.RemoveNotListedByUnderlying(ctx, id); err != nil {
			return nil, asTyped(err, contracts.Retryable)
		}
		mine = nil
	}
	mine, terr := r.endMarks(ctx, id, mine, refs, chains)
	if terr != nil {
		return nil, terr
	}
	out := map[string]bool{}
	for _, m := range mine {
		out[markKey(m)] = true
	}
	fresh := r.fromPull(id, refs, now, out)
	if len(fresh) > 0 {
		if err := j.d.Marks.AddNotListed(ctx, fresh); err != nil {
			return nil, asTyped(err, contracts.Retryable)
		}
	}
	return out, nil
}

// endMarks removes the marks that trigger (b) or (c) of Decision 5 ends and returns the others. Trigger (c) is the
// gap pass's: it is the pass that lists the morning chain first, so the evening pass keeps its marks.
func (r *keyRun) endMarks(ctx context.Context, id int64, mine []contracts.OptionNotListed, refs []openquote.OptionRef,
	chains []broker.Chain) ([]contracts.OptionNotListed, contracts.TypedError) {
	listedAt := map[string][]contracts.Day{}
	for _, ref := range refs {
		k := strikeKey(ref.Strike) + "|" + ref.Right + "|" + ref.TradingClass
		listedAt[k] = append(listedAt[k], ref.Expiry)
	}
	var keep []contracts.OptionNotListed
	for _, m := range mine {
		if r.otherExpiry(m, listedAt) || (r.j.m.GapOnly && chainLists(chains, m)) {
			key := openquote.NotListedKey{UnderlyingID: id, Expiry: contracts.Day(m.Expiry.Format(time.DateOnly)),
				Strike: m.Strike, Right: string(m.Right), TradingClass: m.TradingClass}
			if _, err := r.j.d.Marks.RemoveNotListed(ctx, key); err != nil {
				return nil, asTyped(err, contracts.Retryable)
			}
			continue
		}
		keep = append(keep, m)
	}
	return keep, nil
}

// otherExpiry is trigger (b): the strike of the mark is stored listed at an expiry other than the mark's.
func (r *keyRun) otherExpiry(m contracts.OptionNotListed, listedAt map[string][]contracts.Day) bool {
	day := contracts.Day(m.Expiry.Format(time.DateOnly))
	for _, e := range listedAt[strikeKey(m.Strike)+"|"+string(m.Right)+"|"+m.TradingClass] {
		if e != day {
			return true
		}
	}
	return false
}

// chainLists is trigger (c): a chain of the mark's class lists its expiry and its strike.
func chainLists(chains []broker.Chain, m contracts.OptionNotListed) bool {
	exp := m.Expiry.Format("20060102")
	for _, ch := range chains {
		if ch.TradingClass != m.TradingClass || !containsString(ch.Expirations, exp) {
			continue
		}
		for _, s := range ch.Strikes {
			if strikeKey(s) == strikeKey(m.Strike) {
				return true
			}
		}
	}
	return false
}

// fromPull makes a mark of each not_listed row of the pull of this stock whose answer was a clean 200 (no error
// code or 200) and that is not marked yet.
func (r *keyRun) fromPull(id int64, refs []openquote.OptionRef, now time.Time, marked map[string]bool) []contracts.OptionNotListed {
	byID := map[int64]openquote.OptionRef{}
	for _, ref := range refs {
		byID[ref.InstrumentID] = ref
	}
	var out []contracts.OptionNotListed
	for _, row := range r.pulled {
		if row.Status != statusNotListed || int64(row.UnderlyingID) != id {
			continue
		}
		if row.ErrorCode != nil && *row.ErrorCode != "" && *row.ErrorCode != "200" {
			continue
		}
		ref, ok := byID[int64(row.InstrumentID)]
		if !ok || marked[refKey(ref.Expiry, ref.Strike, ref.Right, ref.TradingClass)] {
			continue
		}
		c := candidate{expiry: ref.Expiry, req: broker.Contract{Strike: ref.Strike, Right: ref.Right, TradingClass: ref.TradingClass}}
		var got broker.Resolved
		if row.ErrorCode != nil && *row.ErrorCode == "200" {
			got = broker.Resolved{ErrorCode: 200, ErrorText: "no security definition"}
		}
		out = append(out, r.mark(id, c, got, now))
		marked[refKey(ref.Expiry, ref.Strike, ref.Right, ref.TradingClass)] = true
		r.cnt.fromPull++
	}
	return out
}
