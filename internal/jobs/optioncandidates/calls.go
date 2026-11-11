package optioncandidates

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

var _ NewOptionCandidatesCallsFunc = NewWithCalls

// callsJob is the job of New plus a dividend reader: one run writes the short_put, covered_call and
// held_call_guard rows of the same run id. ID, Schedule, RunBound, RestartSensitive, Plan and Skipped are the
// ones of the embedded job.
type callsJob struct {
	*job
	div dividend.Reader
}

// NewWithCalls builds the same job as New and adds the covered-call and held-call-guard rows to its run. A nil
// dividend reader is an error; the other requirements are those of New.
func NewWithCalls(
	st CandidateStores, div dividend.Reader, rule compute.OptionCandidates, entries []jobs.WatchEntry, clk clock.Clock,
) (contracts.Job, error) {
	if div == nil {
		return nil, errors.New("optioncandidates: the dividend reader is required")
	}
	base, err := New(st, rule, entries, clk)
	if err != nil {
		return nil, err
	}
	return &callsJob{job: base.(*job), div: div}, nil
}

// Run reads the stores and the newest dividend row of each watched US underlying and writes the rows of the day
// under a new run id: puts first, then covered calls, then held-call guards. Nothing is written when a read fails.
func (j *callsJob) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	us, skipped := j.split()
	defer func() {
		j.mu.Lock()
		j.skipped = skipped
		j.mu.Unlock()
	}()
	days, err := sessionDays(ctx, j.st.SessionDays, day)
	if err != nil {
		return fail("read_session_days_failed", err)
	}
	ulist, missing, err := j.locate(ctx, day, us)
	if err != nil {
		return fail("read_chain_catalog_failed", err)
	}
	for _, s := range missing {
		skipped = append(skipped, "US entry "+s+": no chain day of option_eod, no row")
	}
	rate, rateDay, err := j.rate(ctx, day)
	if err != nil {
		return fail("read_rate_failed", err)
	}
	rc := runCtx{day: day, days: days, rate: rate, rateDay: rateDay, runID: uuid.NewString(), now: j.clk.Now().UTC()}
	var puts, calls, guards []store.Candidate
	for _, ul := range ulist {
		rows, failed := j.underlying(ctx, rc, entryOf(us, ul.issuer), ul)
		if failed != nil {
			return *failed
		}
		for _, row := range rows {
			switch compute.CandidateKind(row.Kind) {
			case compute.KindShortPut:
				puts = append(puts, row)
			case compute.KindCoveredCall:
				calls = append(calls, row)
			case compute.KindHeldCallGuard:
				guards = append(guards, row)
			}
		}
	}
	// Ranks are per kind; a guard row has no yield and no rank.
	if err := rankRun(puts); err != nil {
		return fail("rank_candidates_failed", err)
	}
	if err := rankRun(calls); err != nil {
		return fail("rank_candidates_failed", err)
	}
	rows := append(append(puts, calls...), guards...)
	if _, err := j.st.Candidates.InsertCandidates(ctx, rows); err != nil {
		return fail("write_candidates_failed", err)
	}
	res := contracts.JobResult{Returned: len(rows)}
	for _, r := range rows {
		if r.Status == "ok" {
			res.Usable++
		}
	}
	return res
}

// runCtx is what one run reads once and every underlying shares.
type runCtx struct {
	day     contracts.Day
	days    []contracts.Day
	rate    string
	rateDay contracts.Day
	runID   string
	now     time.Time
}

// underlying reads the stores of one underlying, evaluates the three kinds and maps the rows; a failure is the
// result the run returns.
func (j *callsJob) underlying(ctx context.Context, rc runCtx, entry jobs.WatchEntry, ul underlyingRef) ([]store.Candidate, *contracts.JobResult) {
	failWith := func(code string, err error) ([]store.Candidate, *contracts.JobResult) {
		res := fail(code, err)
		return nil, &res
	}
	spot, quotes, err := j.chainQuotes(ctx, ul)
	if err != nil {
		return failWith("read_chain_failed", err)
	}
	info, ev, err := j.earnings(ctx, entry, ul.chainDay)
	if err != nil {
		return failWith("read_earnings_failed", err)
	}
	accts, err := j.accounts(ctx, ul.symbol, rc.days)
	if err != nil {
		return failWith("read_account_failed", err)
	}
	held, err := j.heldShortCalls(ctx, ul.symbol, rc.days)
	if err != nil {
		return failWith("read_account_failed", err)
	}
	div, err := j.dividendOf(ctx, ul, rc.day)
	if err != nil {
		return failWith("read_dividend_failed", err)
	}
	inputs := []contracts.Day{ul.chainDay}
	if info != nil {
		inputs = append(inputs, info.RowDay)
	}
	if div != nil {
		inputs = append(inputs, div.RowDay)
	}
	for _, a := range accts {
		inputs = append(inputs, a.CaptureDay)
	}
	in := compute.CandidateInput{
		RunDay: rc.day, ChainDay: ul.chainDay, SessionDays: trimSessionDays(rc.days, inputs...), Underlying: ul.symbol,
		Spot: spot, Quotes: quotes, BuyPrice: entry.BuyPrice, SalePrice: entry.SalePrice, Earnings: info,
		Dividend: div, Rate: rc.rate, RateDay: rc.rateDay, Accounts: accts, OpenShortIDs: held,
		Kinds: []compute.CandidateKind{compute.KindShortPut, compute.KindCoveredCall, compute.KindHeldCallGuard},
	}
	var rows []store.Candidate
	for _, c := range j.rule.Evaluate(in, j.params) {
		row, err := toStoreRow(c, rc.day, rc.runID, ul, ev, rc.now)
		if err != nil {
			return failWith("map_candidate_failed", err)
		}
		if div != nil && div.State == "present" && c.Kind != compute.KindShortPut {
			if div.NextExDate != "" {
				d := div.NextExDate
				row.NextExDate = &d
			}
			row.ExAmount = optPtr(div.Amount)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// dividendOf reads the dividend rows of the look-back window and answers the newest as the rule's input; no row
// gives nil. Only a present row carries the date and the amount.
func (j *callsJob) dividendOf(ctx context.Context, ul underlyingRef, day contracts.Day) (*compute.DividendInfo, error) {
	rows, err := j.div.Dividends(ctx, int(ul.id), addDays(day, -lookbackDays), day)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	last := rows[len(rows)-1]
	info := &compute.DividendInfo{State: string(last.Status), RowDay: contracts.Day(last.SessionDay.Format(time.DateOnly))}
	if info.State == "present" {
		if last.NextExDate != nil {
			info.NextExDate = contracts.Day(last.NextExDate.Format(time.DateOnly))
		}
		if last.NextAmount != nil {
			info.Amount = *last.NextAmount
		}
	}
	return info, nil
}

// heldShortCalls lists the instrument ids of the short calls on symbol in the newest capture of each alias,
// sorted and without duplicates. A nil account reader gives none.
func (j *callsJob) heldShortCalls(ctx context.Context, symbol string, days []contracts.Day) ([]string, error) {
	if j.st.Accounts == nil {
		return nil, nil
	}
	seen := map[string]bool{}
	for _, alias := range j.st.Aliases {
		for tries, i := 0, len(days)-1; i >= 0 && tries < captureLookup; tries, i = tries+1, i-1 {
			values, err := j.st.Accounts.Values(ctx, alias, string(days[i]))
			if err != nil {
				return nil, err
			}
			if len(values) == 0 {
				continue
			}
			positions, err := j.st.Accounts.Positions(ctx, alias, string(days[i]))
			if err != nil {
				return nil, err
			}
			for _, p := range positions {
				if isShortCall(p, symbol) {
					seen[strconv.FormatInt(p.ConID, 10)] = true
				}
			}
			break
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

func isShortCall(p store.Position, symbol string) bool {
	q, ok := new(big.Rat).SetString(strings.TrimSpace(p.Quantity))
	return ok && q.Sign() < 0 && strings.EqualFold(p.Symbol, symbol) && p.SecType == "option" && p.Right == "call"
}

// chainQuotes is quotes of input.go for both rights: the rows of the chain day, the terms of their contracts
// and the join of the two.
func (j *callsJob) chainQuotes(ctx context.Context, ul underlyingRef) (spot string, out []compute.OptionQuote, err error) {
	rows, err := j.st.OptionEod.OptionEod(ctx, ul.id, ul.chainDay, ul.chainDay)
	if err != nil {
		return "", nil, err
	}
	best := map[int]contracts.OptionEod{}
	for _, r := range rows {
		if old, ok := best[r.InstrumentID]; !ok || r.Generation > old.Generation ||
			(r.Generation == old.Generation && r.CaptureID >= old.CaptureID) {
			best[r.InstrumentID] = r
		}
		if spot == "" {
			spot = optText(r.UnderlyingPrice)
		}
	}
	ids := make([]int64, 0, len(best))
	for id := range best {
		ids = append(ids, int64(id))
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	for start := 0; start < len(ids); start += termsChunk {
		end := min(start+termsChunk, len(ids))
		terms, err := j.st.Terms.OptionTerms(ctx, ids[start:end])
		if err != nil {
			return "", nil, err
		}
		for _, t := range terms {
			r := best[int(t.InstrumentID)]
			out = append(out, compute.OptionQuote{
				InstrumentID: strconv.FormatInt(t.InstrumentID, 10), Expiry: t.ExpirationDate, Right: t.Right,
				Strike: t.Strike, Multiplier: t.Multiplier, Bid: optText(r.Bid), Ask: optText(r.Ask),
				BidSize: intOf(r.BidSize), OpenInterest: intOf(r.OpenInterest), Delta: optText(r.Delta),
				IV: optText(r.IvDecimal), MarketDataType: r.MarketDataType, QuoteOK: r.Status == 1,
			})
		}
	}
	return spot, out, nil
}
