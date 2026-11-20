package barsrefresh

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

const (
	sourceYfinance = "yfinance"

	codeBasisDiffers = "fallback_basis_differs"
	codeIncomplete   = "incomplete"
	codeUnmapped     = "unmapped"
	codeSourceFailed = "source_failed"
	codeCalendar     = "calendar"
	codeBadStored    = "stored_unreadable"

	// histDownAfter is the number of keys whose history call ends Retryable before the rest of the run
	// skips the IB pull: each further call would wait its whole timeout.
	histDownAfter = 2
)

// WithFallback sets the yfinance source and the session calendar of a job built by New and returns it:
// a key whose history call ends Retryable then gets its daily trades bars from src (design
// docs/design/bars-fallback.md, Decisions 1 to 5, 7 and 8). A nil src or cal, or a job of another
// package, is an error.
func WithFallback(j contracts.Job, src sources.BarSource, cal clock.Calendar) (contracts.Job, error) {
	if src == nil || cal == nil {
		return nil, errors.New("barsrefresh: the fallback needs a bar source and a calendar")
	}
	jb, ok := j.(*job)
	if !ok {
		return nil, errors.New("barsrefresh: the fallback attaches only to a job built by New")
	}
	jb.src, jb.cal = src, cal
	return jb, nil
}

// historyFailure marks a Retryable end of a history call: the key goes to the fallback.
type historyFailure struct{ contracts.TypedError }

var errHistDown = newErr(contracts.Retryable, "history pull skipped: %d keys of the run already failed", histDownAfter)

// historyFailed classifies the error of a Bars call. With a fallback attached, a Retryable one is marked
// and counted; the second of the run sets histDown. Any other error is returned unchanged.
func (r *runState) historyFailed(err error) error {
	te := asTyped(err, contracts.Retryable)
	if r.j.src == nil || te.Kind() != contracts.Retryable {
		return err
	}
	r.histFails++
	if r.histFails >= histDownAfter {
		r.histDown = true
	}
	return &historyFailure{te}
}

// pending is a key whose history call failed and that waits for the one DailyBars call of the run.
type pending struct {
	key    string
	id     int64
	c      broker.Contract
	stored []contracts.PriceBar
	days   int
	cause  error
}

// deferToFallback queues the key when terr is a marked history failure and reports true.
func (r *runState) deferToFallback(key string, id int64, c broker.Contract, stored []contracts.PriceBar, days int, terr error) bool {
	var hf *historyFailure
	if r.j.src == nil || !errors.As(terr, &hf) {
		return false
	}
	r.pending = append(r.pending, pending{key: key, id: id, c: c, stored: stored, days: days, cause: hf.TypedError})
	return true
}

// pairOf maps the contract of a key to the pair the source is asked: an index, and a currency other
// than USD and JPY, have none.
func pairOf(c broker.Contract) (sources.BarPair, bool) {
	if c.SecType == "IND" || c.Symbol == "" || (c.Currency != "USD" && c.Currency != "JPY") {
		return sources.BarPair{}, false
	}
	return sources.BarPair{Symbol: c.Symbol, Currency: c.Currency}, true
}

func calendarOf(currency string) string {
	if currency == "JPY" {
		return "XTKS"
	}
	return "XNYS"
}

// runFallback asks the source once for every queued key, then writes each key's rows; add receives the
// outcome of each key.
func (r *runState) runFallback(ctx context.Context, add func(returned, usable int, err contracts.TypedError)) {
	if len(r.pending) == 0 {
		return
	}
	var pairs []sources.BarPair
	seen := map[sources.BarPair]bool{}
	widest := 0
	for _, p := range r.pending {
		if pr, ok := pairOf(p.c); ok && !seen[pr] {
			seen[pr] = true
			pairs = append(pairs, pr)
			widest = max(widest, p.days)
		}
	}
	var res sources.BarResult
	var srcErr error
	if len(pairs) > 0 {
		res, srcErr = r.j.src.DailyBars(ctx, sources.BarRequest{
			Pairs: pairs, From: contracts.Day(r.dayTime.AddDate(0, 0, -widest).Format(dayLayout)), To: r.day,
		})
	}
	bars := map[sources.BarPair][]sources.RefBar{}
	for _, b := range res.Bars {
		bars[b.Pair] = append(bars[b.Pair], b)
	}
	codes := map[sources.BarPair]string{}
	for _, u := range res.Unanswered {
		codes[u.Pair] = u.Code
	}
	tally := map[string]int{}
	rows := 0
	for _, p := range r.pending {
		n, usable, code, terr := r.fallbackKey(ctx, p, bars, codes, srcErr)
		if code != "" {
			tally[code]++
		}
		rows += n
		add(n, usable, terr)
	}
	var names []string
	for c, n := range tally {
		names = append(names, fmt.Sprintf("%s=%d", c, n))
	}
	sort.Strings(names)
	r.j.log.Info("bars_refresh fallback", "day", string(r.day), "keys", len(r.pending), "rows", rows, "codes", strings.Join(names, ","))
}

// fallbackKey writes the rows of one queued key. code is the one code the key ended with, or "".
func (r *runState) fallbackKey(
	ctx context.Context, p pending, bars map[sources.BarPair][]sources.RefBar, codes map[sources.BarPair]string, srcErr error,
) (written, usable int, code string, terr contracts.TypedError) {
	fail := func(code string) (int, int, string, contracts.TypedError) {
		return 0, 0, code, newErr(contracts.Retryable, "%s: history failed (%v); fallback %s", p.key, p.cause, code)
	}
	pair, ok := pairOf(p.c)
	switch {
	case !ok:
		return fail(codeUnmapped)
	case srcErr != nil:
		return fail(codeSourceFailed)
	case codes[pair] != "":
		return fail(codes[pair])
	case len(bars[pair]) == 0:
		return fail(sources.BarCodeNoData)
	}
	from := r.dayTime.AddDate(0, 0, -p.days).Format(dayLayout)
	kept, incomplete := 0, 0
	var rows []contracts.PriceBar
	for _, b := range bars[pair] {
		if string(b.Day) < from || string(b.Day) > string(r.day) {
			continue
		}
		if closed, err := r.sessionClosed(ctx, p.c.Currency, b.Day); err != nil {
			return fail(codeCalendar)
		} else if !closed {
			continue
		}
		row, ok := r.yfinanceRow(b, p)
		if !ok {
			incomplete++
			continue
		}
		rows = append(rows, row)
		kept++
	}
	note, over, err := basisCheck(p.stored, rows, r.j.m.FallbackTolerance)
	if err != nil {
		return fail(codeBadStored)
	}
	r.j.log.Info("bars_refresh fallback basis", "key", p.key, "note", note)
	if over {
		return fail(codeBasisDiffers)
	}
	if len(rows) > 0 {
		cnt, err := r.up.UpsertPriceBars(ctx, rows, "bars_refresh:"+string(r.day))
		if err != nil {
			return 0, 0, "", asTyped(err, contracts.Retryable)
		}
		r.j.log.Info("bars_refresh fallback written", "key", p.key, "written", cnt.Written, "unchanged", cnt.Unchanged)
	}
	if incomplete > 0 {
		return kept, kept, codeIncomplete, newErr(contracts.Retryable,
			"%s: history failed (%v); fallback %s: %d bars without a full price", p.key, p.cause, codeIncomplete, incomplete)
	}
	return kept, kept, "", nil
}

// yfinanceRow maps a bar to a trades row with source yfinance and a NULL volume (Decisions 3 and 4);
// ok is false when open, high, low or close is empty or not a positive decimal.
func (r *runState) yfinanceRow(b sources.RefBar, p pending) (contracts.PriceBar, bool) {
	for _, v := range []string{b.Open, b.High, b.Low, b.Close} {
		if x, err := parseRat(v); v == "" || err != nil || x.Sign() <= 0 {
			return contracts.PriceBar{}, false
		}
	}
	d, err := time.Parse(dayLayout, string(b.Day))
	if err != nil {
		return contracts.PriceBar{}, false
	}
	return contracts.PriceBar{
		Basis: basisTrades, Close: b.Close, Currency: p.c.Currency, High: b.High, Low: b.Low, Open: b.Open,
		InstrumentID: int(p.id), ReceivedAt: r.j.clk.Now().UTC(), SessionDay: types.SerializableDate{Time: d},
		Source: sourceYfinance,
	}, true
}

// sessionClosed reports whether the session of day in the calendar of currency is over: a day that is
// no session, or whose close is still ahead of the clock (a half day closes at its early close), is not.
func (r *runState) sessionClosed(ctx context.Context, currency string, day contracts.Day) (bool, error) {
	id := calendarOf(currency)
	s, err := r.j.cal.Session(ctx, id, day)
	if err != nil {
		return false, err
	}
	return s.IsSession && !r.j.clk.Now().Before(s.CloseAt), nil
}

// basisCheck compares the closes of rows with the stored ibkr rows of the same day (Decision 5). differs
// is true when one is off by more than tol, relative to the stored one. note names the largest
// difference, or "no overlap".
func basisCheck(stored, rows []contracts.PriceBar, tol *big.Rat) (note string, over bool, err error) {
	ib := map[string]*big.Rat{}
	for _, s := range stored {
		if s.Source != sourceIbkr {
			continue
		}
		v, perr := parseRat(s.Close)
		if perr != nil {
			return "", false, perr
		}
		ib[s.SessionDay.Format(dayLayout)] = v
	}
	largest := new(big.Rat)
	overlap := 0
	for _, row := range rows {
		old, ok := ib[row.SessionDay.Format(dayLayout)]
		if !ok {
			continue
		}
		overlap++
		y, perr := parseRat(row.Close)
		if perr != nil {
			return "", false, perr
		}
		if old.Sign() == 0 {
			return "stored close is zero", true, nil
		}
		d := new(big.Rat).Sub(y, old)
		d.Abs(d).Quo(d, old)
		if d.Cmp(largest) > 0 {
			largest = d
		}
	}
	if overlap == 0 {
		return "no overlap", false, nil
	}
	return "largest difference " + largest.FloatString(6), largest.Cmp(tol) > 0, nil
}

// hasSource reports whether a row of rows has the given source.
func hasSource(rows []contracts.PriceBar, source string) bool {
	for _, r := range rows {
		if string(r.Source) == source {
			return true
		}
	}
	return false
}
