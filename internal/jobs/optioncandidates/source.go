package optioncandidates

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// Statuses of an obs.option_open row that the source reads (docs/design/candidates-at-open.md Decision 1).
const (
	openStatusOK        = 1
	openStatusPartial   = 2
	openStatusNotListed = 4
	openStatusDeferred  = 7

	markEodFallback    = "eod_fallback"
	markOIPriorSession = "oi_prior_session"
)

// sourceParams are the values of the [source] table of job.toml.
type sourceParams struct {
	OpenMinRows        int    `toml:"open_min_rows"`
	NeedHigh           string `toml:"need_high"`
	OILookbackSessions int    `toml:"oi_lookback_sessions"`
}

// chainResult is the chain of one underlying as the run reads it: one source and one chain day for both rights.
type chainResult struct {
	chainDay contracts.Day
	spot     string
	quotes   []compute.OptionQuote
	fallback bool            // the eod path of a run that holds an open reader
	oiPrior  map[string]bool // instrument ids whose open interest came from an earlier eod session
}

// addMarks appends the marks the source owes to the row.
func (r chainResult) addMarks(c *compute.Candidate) {
	if r.fallback {
		c.Marks = append(c.Marks, markEodFallback)
	}
	if c.Right == "P" && r.oiPrior[c.InstrumentID] {
		c.Marks = append(c.Marks, markOIPriorSession)
	}
}

// sourceRun is the source of one Run: the open snapshot of the day, read once, and the counts of the run.
type sourceRun struct {
	j      *job
	day    contracts.Day
	days   []contracts.Day
	begun  bool
	quiet  bool // the read succeeded and the day has no open row: the job of today, no mark and no line
	open   map[int][]openRow
	nOpen  int
	nEod   int
	nNone  int
	noTerm int
	puts   []string // "SYMBOL=n": usable puts at or below need_high times the buy price, per underlying on the open
	lines  []string // Skipped lines of the underlyings that fell back
}

type openRow = contracts.OptionOpen

func newSourceRun(j *job, day contracts.Day) *sourceRun { return &sourceRun{j: j, day: day} }

func (j *job) logger() *slog.Logger {
	if j.st.log != nil {
		return j.st.log
	}
	return slog.Default()
}

// begin reads the open rows of the day once. A nil reader reads nothing; a read error is one warning and leaves
// every underlying on the eod path.
func (s *sourceRun) begin(ctx context.Context, days []contracts.Day) {
	s.days, s.begun = days, true
	if s.j.st.Open == nil {
		return
	}
	rows, err := s.j.st.Open.OptionOpen(ctx, s.day)
	if err != nil {
		s.j.logger().Warn("option_open read failed, every underlying takes the option_eod chain",
			"component", "jobs", "status", "degraded", "job", s.j.m.ID, "err", err)
		return
	}
	s.quiet = len(rows) == 0
	s.open = map[int][]openRow{}
	for _, r := range rows {
		if r.Currency == "USD" {
			s.open[r.UnderlyingID] = append(s.open[r.UnderlyingID], r)
		}
	}
}

// finish returns the Skipped lines of the fallbacks and logs the counts of the run (only with an open reader).
func (s *sourceRun) finish() []string {
	if !s.begun || s.j.st.Open == nil {
		return nil
	}
	s.j.logger().Info("option_candidates source", "component", "jobs", "job", s.j.m.ID, "open", s.nOpen,
		"eod_fallback", s.nEod, "none", s.nNone, "open_rows_without_terms", s.noTerm,
		"puts_at_need_high", strings.Join(s.puts, ","))
	return s.lines
}

// chain is the one function that gives the chain of an underlying, for puts only or for both rights.
func (s *sourceRun) chain(ctx context.Context, ul underlyingRef, entry jobs.WatchEntry, calls bool) (chainResult, error) {
	if rows := s.open[int(ul.id)]; len(rows) > 0 {
		res, usable, err := s.fromOpen(ctx, ul, entry, calls, rows)
		if err != nil {
			return chainResult{}, err
		}
		if usable {
			s.nOpen++
			return res, nil
		}
	}
	spot, q, err := s.j.eodQuotes(ctx, ul, calls)
	if err != nil {
		return chainResult{}, err
	}
	res := chainResult{chainDay: ul.chainDay, spot: spot, quotes: q}
	if s.j.st.Open != nil {
		s.nEod++
		res.fallback = !s.quiet
	}
	if res.fallback {
		s.lines = append(s.lines, fmt.Sprintf("US entry %s: option_open not usable, fell back to option_eod chain day %s", ul.symbol, ul.chainDay))
	}
	return res, nil
}

// eodQuotes reads the rows of the chain day of option_eod, the terms of their contracts (in chunks of 500 ids)
// and joins them; calls false keeps the puts only. spot is the underlying price of the chain, empty when no row
// carries one.
func (j *job) eodQuotes(ctx context.Context, ul underlyingRef, calls bool) (spot string, out []compute.OptionQuote, err error) {
	rows, err := j.st.OptionEod.OptionEod(ctx, ul.id, ul.chainDay, ul.chainDay)
	if err != nil {
		return "", nil, err
	}
	// The last row of an instrument (by generation, then capture) is its newest capture of the day.
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
	terms, err := j.termsOf(ctx, ids)
	if err != nil {
		return "", nil, err
	}
	for _, t := range terms {
		if !calls && t.Right != "P" {
			continue
		}
		r := best[int(t.InstrumentID)]
		out = append(out, compute.OptionQuote{
			InstrumentID: strconv.FormatInt(t.InstrumentID, 10), Expiry: t.ExpirationDate, Right: t.Right,
			Strike: t.Strike, Multiplier: t.Multiplier, Bid: optText(r.Bid), Ask: optText(r.Ask),
			BidSize: intOf(r.BidSize), OpenInterest: intOf(r.OpenInterest), Delta: optText(r.Delta),
			IV: optText(r.IvDecimal), MarketDataType: r.MarketDataType, QuoteOK: r.Status == 1,
		})
	}
	return spot, out, nil
}

// termsOf reads the terms of the ids in chunks of 500, ids ascending.
func (j *job) termsOf(ctx context.Context, ids []int64) ([]store.OptionTerms, error) {
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	var out []store.OptionTerms
	for start := 0; start < len(ids); start += termsChunk {
		end := min(start+termsChunk, len(ids))
		terms, err := j.st.Terms.OptionTerms(ctx, ids[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, terms...)
	}
	return out, nil
}

// fromOpen builds the chain of an underlying from its open rows and says whether it is usable.
func (s *sourceRun) fromOpen(
	ctx context.Context, ul underlyingRef, entry jobs.WatchEntry, calls bool, rows []openRow,
) (chainResult, bool, error) {
	res := chainResult{chainDay: s.day}
	byID := map[int]openRow{}
	for _, r := range rows {
		if res.spot == "" {
			res.spot = optText(r.UnderlyingPrice)
		}
		if r.Status == openStatusDeferred || r.Status == openStatusNotListed {
			continue
		}
		byID[r.InstrumentID] = r
	}
	ids := make([]int64, 0, len(byID))
	for id := range byID {
		ids = append(ids, int64(id))
	}
	terms, err := s.j.termsOf(ctx, ids)
	if err != nil {
		return res, false, err
	}
	s.noTerm += len(byID) - len(terms)
	usable := 0
	for _, t := range terms {
		if !calls && t.Right != "P" {
			continue
		}
		r := byID[int(t.InstrumentID)]
		q := openQuote(t, r)
		res.quotes = append(res.quotes, q)
		if t.Right == "P" && s.countsAsUsable(q, entry.BuyPrice) {
			usable++
		}
	}
	if usable < s.j.m.Source.OpenMinRows {
		return res, false, nil
	}
	s.puts = append(s.puts, ul.symbol+"="+strconv.Itoa(usable))
	if err := s.joinOpenInterest(ctx, ul, &res); err != nil {
		return res, false, err
	}
	return res, true, nil
}

// openQuote maps one open row and its terms to the quote of the rule. QuoteOK is status 1, or status 2 with a bid
// and an ask; a partial row has no model values, so its delta and IV are empty text.
func openQuote(t store.OptionTerms, r openRow) compute.OptionQuote {
	q := compute.OptionQuote{
		InstrumentID: strconv.FormatInt(t.InstrumentID, 10), Expiry: t.ExpirationDate, Right: t.Right,
		Strike: t.Strike, Multiplier: t.Multiplier, Bid: optText(r.Bid), Ask: optText(r.Ask),
		BidSize: intOf(r.BidSize), Delta: optText(r.Delta), IV: optText(r.IvDecimal), MarketDataType: r.MarketDataType,
	}
	switch r.Status {
	case openStatusOK:
		q.QuoteOK = true
	case openStatusPartial:
		q.QuoteOK = r.Bid != nil && r.Ask != nil
		q.Delta, q.IV = "", ""
	}
	return q
}

// countsAsUsable is a put quote inside the DTE window whose strike is at most need_high times the buy price.
func (s *sourceRun) countsAsUsable(q compute.OptionQuote, buy string) bool {
	if !q.QuoteOK {
		return false
	}
	exp, err := time.Parse(time.DateOnly, string(q.Expiry))
	day, ok := parseDay(s.day)
	if err != nil || !ok {
		return false
	}
	dte := int(exp.Sub(day).Hours() / 24)
	if dte < s.j.params.MinDTE || dte > s.j.params.MaxDTE {
		return false
	}
	b, hasBuy := new(big.Rat).SetString(strings.TrimSpace(buy))
	if !hasBuy || b.Sign() <= 0 {
		return true
	}
	high, _ := new(big.Rat).SetString(s.j.m.Source.NeedHigh)
	strike, ok := new(big.Rat).SetString(q.Strike)
	return ok && strike.Cmp(new(big.Rat).Mul(high, b)) <= 0
}

// joinOpenInterest sets the open interest of each put of the open from the newest option_eod row of the same
// instrument within the last oi_lookback_sessions session days; a put with none keeps 0.
func (s *sourceRun) joinOpenInterest(ctx context.Context, ul underlyingRef, res *chainResult) error {
	from := s.day
	if n := len(s.days); n > 0 {
		from = s.days[max(0, n-s.j.m.Source.OILookbackSessions)]
	}
	rows, err := s.j.st.OptionEod.OptionEod(ctx, ul.id, from, s.day)
	if err != nil {
		return err
	}
	best := map[int]contracts.OptionEod{}
	for _, r := range rows {
		if r.OpenInterest == nil || intOf(r.OpenInterest) == 0 {
			continue
		}
		old, ok := best[r.InstrumentID]
		if !ok || eodNewer(r, old) {
			best[r.InstrumentID] = r
		}
	}
	for i := range res.quotes {
		q := &res.quotes[i]
		if q.Right != "P" {
			continue
		}
		id, _ := strconv.Atoi(q.InstrumentID)
		if r, ok := best[id]; ok {
			q.OpenInterest = intOf(r.OpenInterest)
			if res.oiPrior == nil {
				res.oiPrior = map[string]bool{}
			}
			res.oiPrior[q.InstrumentID] = true
		}
	}
	return nil
}

func eodNewer(a, b contracts.OptionEod) bool {
	if !a.SessionDay.Equal(b.SessionDay.Time) {
		return a.SessionDay.After(b.SessionDay.Time)
	}
	if a.Generation != b.Generation {
		return a.Generation > b.Generation
	}
	return a.CaptureID >= b.CaptureID
}
