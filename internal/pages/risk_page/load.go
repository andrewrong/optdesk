package risk_page

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/shared"
	"github.com/andrewrong/optdesk/internal/store"
)

// The parameters are those of structures_page, which loads the same input (Decision 13); a pages package may
// not import another, so they are named again here.
const (
	calendarID         = "XNYS"
	dayLayout          = "2006-01-02"
	sessionLookback    = 60 // calendar days of session days to read
	quoteLookback      = 14 // calendar days of option_eod rows to read
	markMaxAgeSessions = 5
	captureTarget      = "0.50"
	exitDTE            = 21
	commissionDays     = 30
	captureMaxProbe    = 10 // session days probed for the newest capture
	staleSessions      = 3  // a capture older than this many session days is the gap stale_input
	termsBatch         = 500
)

// stock is one stock row of the capture.
type stock struct{ symbol, currency, quantity string }

// loaded is the structure input and the stocks of the newest capture.
type loaded struct {
	in     compute.StructureInput
	stocks []stock
	at     time.Time // when the capture was taken, zero when unknown
	ledger time.Time // newest ledger record time, zero when none
}

func (c Config) load(ctx context.Context, alias string, day contracts.Day) (loaded, error) {
	var l loaded
	l.in = compute.StructureInput{Alias: alias, AsOf: day}
	rows, err := c.Ledger.FlexTradeRows(ctx, alias)
	if err != nil {
		return l, fmt.Errorf("risk_page: read ledger: %w", err)
	}
	trades := make([]compute.LedgerTrade, 0, len(rows))
	for _, r := range rows {
		attrs := map[string]string{}
		if err := json.Unmarshal([]byte(r.Attributes), &attrs); err != nil {
			return l, fmt.Errorf("risk_page: ledger row %s: %w", r.TransactionID, err)
		}
		trades = append(trades, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: attrs})
		if r.RecordedAt.After(l.ledger) {
			l.ledger = r.RecordedAt
		}
	}
	l.in.SessionDays, err = c.sessionDays(ctx, day)
	if err != nil {
		return l, err
	}
	res := c.Ledgers().Replay(compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: l.in.SessionDays, AsOf: day},
		compute.LedgerParams{MarkMaxAgeSessions: markMaxAgeSessions})
	l.in.Positions, l.in.Cycles = res.Positions, res.Cycles
	if err := c.capture(ctx, alias, &l); err != nil {
		return l, err
	}
	l.in.Quotes, err = c.quotes(ctx, l.in.Positions, day)
	return l, err
}

func (c Config) sessionDays(ctx context.Context, day contracts.Day) ([]contracts.Day, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("risk_page: bad day %q: %w", day, err)
	}
	from := contracts.Day(d.AddDate(0, 0, -sessionLookback).Format(dayLayout))
	rows, err := c.Sessions.SessionDays(ctx, calendarID, from, day)
	if err != nil {
		return nil, fmt.Errorf("risk_page: read session days: %w", err)
	}
	out := make([]contracts.Day, 0, len(rows))
	for _, r := range rows {
		if r.IsSession {
			out = append(out, contracts.Day(r.SessionDay.Format(dayLayout)))
		}
	}
	return out, nil
}

// capture reads the newest capture among the last session days: its option rows are the held list, its stock
// rows are the stocks.
func (c Config) capture(ctx context.Context, alias string, l *loaded) error {
	days := l.in.SessionDays
	for i := len(days) - 1; i >= 0 && i >= len(days)-captureMaxProbe; i-- {
		rows, err := c.Account.Positions(ctx, alias, string(days[i]))
		if err != nil {
			return fmt.Errorf("risk_page: read positions: %w", err)
		}
		if len(rows) == 0 {
			continue
		}
		l.in.HeldDay = days[i]
		l.at = rows[0].CapturedAt
		for _, p := range rows {
			switch strings.ToUpper(p.SecType) {
			case "OPTION":
				l.in.Held = append(l.in.Held, compute.HeldOption{Underlying: p.Symbol, Right: rightLetter(p.Right),
					Strike: p.Strike, Expiry: p.Expiry, Quantity: p.Quantity})
			case "STOCK":
				l.stocks = append(l.stocks, stock{symbol: p.Symbol, currency: p.Currency, quantity: p.Quantity})
			}
		}
		return nil
	}
	return nil
}

func rightLetter(r string) string {
	if r == "" {
		return ""
	}
	return strings.ToUpper(r[:1])
}

// canQuote is true when every reader the quotes need is set.
func (c Config) canQuote() bool { return c.Eod != nil && c.Terms != nil && c.StockIDs != nil }

// quotes reads the newest option_eod row of each contract of the held underlyings and matches it to its
// terms. The underlying id is the stock instrument id read by name; a symbol with no row or with two rows has
// no quotes, and so has every symbol when StockIDs is nil.
func (c Config) quotes(ctx context.Context, ps []compute.LedgerPosition, day contracts.Day) ([]compute.StructureQuote, error) {
	if !c.canQuote() {
		return nil, nil
	}
	stockID, err := c.stockIDs(ctx, ps)
	if err != nil {
		return nil, err
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("risk_page: bad day %q: %w", day, err)
	}
	from := contracts.Day(d.AddDate(0, 0, -quoteLookback).Format(dayLayout))
	newest := map[int]contracts.OptionEod{}
	under := map[int]string{}
	var ids []int // instrument ids in order of first sight
	seen := map[string]bool{}
	ids0 := map[string]int64{}
	eodRows := map[string][]contracts.OptionEod{}
	for _, p := range ps {
		if p.Status != "open" || seen[p.Underlying] || stockID[p.Underlying] == 0 {
			continue
		}
		seen[p.Underlying] = true
		rows, err := c.Eod.OptionEod(ctx, stockID[p.Underlying], from, day)
		if err != nil {
			return nil, fmt.Errorf("risk_page: read option_eod: %w", err)
		}
		ids0[p.Underlying], eodRows[p.Underlying] = stockID[p.Underlying], rows
		for _, r := range rows {
			old, ok := newest[r.InstrumentID]
			if !ok {
				ids = append(ids, r.InstrumentID)
			}
			if !ok || old.SessionDay.Format(dayLayout) <= r.SessionDay.Format(dayLayout) {
				newest[r.InstrumentID] = r
				under[r.InstrumentID] = p.Underlying
			}
		}
	}
	closes, err := c.closes(ctx, ids0, eodRows, day)
	if err != nil {
		return nil, err
	}
	var out []compute.StructureQuote
	for lo := 0; lo < len(ids); lo += termsBatch {
		hi := min(lo+termsBatch, len(ids))
		batch := make([]int64, 0, hi-lo)
		for _, id := range ids[lo:hi] {
			batch = append(batch, int64(id))
		}
		terms, err := c.Terms.OptionTerms(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("risk_page: read option terms: %w", err)
		}
		for _, t := range terms {
			q := quoteOf(under[int(t.InstrumentID)], t, newest[int(t.InstrumentID)])
			q.UnderlyingClose = closes[q.Underlying]
			out = append(out, q)
		}
	}
	return out, nil
}

// closes is the close of each underlying symbol on day: the stored bar of its instrument id, else the print.
func (c Config) closes(ctx context.Context, ids map[string]int64, eodRows map[string][]contracts.OptionEod, day contracts.Day) (map[string]string, error) {
	closes := map[string]string{}
	for sym, id := range ids {
		src, err := shared.SourceOf(ctx, c.Bars, id, eodRows[sym], day)
		if err != nil {
			return nil, fmt.Errorf("risk_page: read price bars of %s: %w", sym, err)
		}
		if cl, ok := shared.Pick(src, day); ok {
			closes[sym] = cl.Price
		}
	}
	return closes, nil
}

// stockIDs is the instrument id of each symbol with an open position, by name; a name with no row or with two
// rows is absent.
func (c Config) stockIDs(ctx context.Context, ps []compute.LedgerPosition) (map[string]int64, error) {
	var names []string
	for _, p := range ps {
		if p.Status == "open" && !slices.Contains(names, p.Underlying) {
			names = append(names, p.Underlying)
		}
	}
	out := map[string]int64{}
	if len(names) == 0 {
		return out, nil
	}
	rows, err := c.StockIDs.StockIDs(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("risk_page: read stock ids: %w", err)
	}
	count := map[string]int{}
	for _, r := range rows {
		count[r.Name]++
		out[r.Name] = r.InstrumentID
	}
	for n, k := range count {
		if k != 1 || out[n] == 0 {
			delete(out, n)
		}
	}
	return out, nil
}

// quoteOf is the stored row as decimal text, shortest exact form; an absent value stays empty.
func quoteOf(underlying string, t store.OptionTerms, r contracts.OptionEod) compute.StructureQuote {
	q := compute.StructureQuote{Underlying: underlying, Right: rightLetter(t.Right), Strike: t.Strike,
		Expiry: string(t.ExpirationDate), Day: contracts.Day(r.SessionDay.Format(dayLayout))}
	if v := r.Bid; v != nil {
		q.Bid = strconv.FormatFloat(*v, 'f', -1, 64)
	}
	if v := r.Ask; v != nil {
		q.Ask = strconv.FormatFloat(*v, 'f', -1, 64)
	}
	if v := r.Delta; v != nil {
		q.Delta = strconv.FormatFloat(*v, 'f', -1, 64)
	}
	if v := r.Gamma; v != nil {
		q.Gamma = strconv.FormatFloat(*v, 'f', -1, 64)
	}
	if v := r.ThetaPerCalendarDay; v != nil {
		q.Theta = strconv.FormatFloat(*v, 'f', -1, 64)
	}
	if v := r.VegaPerVolPoint; v != nil {
		q.Vega = strconv.FormatFloat(*v, 'f', -1, 64)
	}
	return q
}
