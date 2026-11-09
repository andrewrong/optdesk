package structures_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// The parameters of the page. The compare constants are those of optionstruct (Decision 13); a pages
// package may not import that package, so they are named again here.
const (
	calendarID         = "XNYS"
	sessionLookback    = 60 // calendar days of session days to read
	quoteLookback      = 14 // calendar days of option_eod rows to read
	markMaxAgeSessions = 5
	captureTarget      = "0.50"
	exitDTE            = 21
	commissionDays     = 30
	captureMaxProbe    = 4 // session days probed for the newest capture
	termsBatch         = 500
)

// load reads everything Evaluate needs through the readers of c. Ledger and Sessions are not nil.
func (c Config) load(ctx context.Context, alias string, day contracts.Day) (compute.StructureInput, error) {
	in := compute.StructureInput{Alias: alias, AsOf: day}
	if c.Ledgers == nil || c.Structures == nil {
		return in, errors.New("structures_page: no rule constructors")
	}
	trades, err := c.trades(ctx, alias)
	if err != nil {
		return in, err
	}
	in.SessionDays, err = c.sessionDays(ctx, day)
	if err != nil {
		return in, err
	}
	res := c.Ledgers().Replay(compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: in.SessionDays, AsOf: day},
		compute.LedgerParams{MarkMaxAgeSessions: markMaxAgeSessions})
	in.Positions, in.Cycles = res.Positions, res.Cycles
	stocks := map[string]int64{}
	if err := c.capture(ctx, alias, &in, stocks); err != nil {
		return in, err
	}
	in.Quotes, err = c.quotes(ctx, in.Positions, stocks, day)
	return in, err
}

func (c Config) trades(ctx context.Context, alias string) ([]compute.LedgerTrade, error) {
	rows, err := c.Ledger.FlexTradeRows(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("structures_page: read ledger: %w", err)
	}
	out := make([]compute.LedgerTrade, 0, len(rows))
	for _, r := range rows {
		attrs := map[string]string{}
		if err := json.Unmarshal([]byte(r.Attributes), &attrs); err != nil {
			return nil, fmt.Errorf("structures_page: ledger row %s: %w", r.TransactionID, err)
		}
		out = append(out, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: attrs})
	}
	return out, nil
}

func (c Config) sessionDays(ctx context.Context, day contracts.Day) ([]contracts.Day, error) {
	from, err := shiftDay(day, -sessionLookback)
	if err != nil {
		return nil, fmt.Errorf("structures_page: bad day %q: %w", day, err)
	}
	rows, err := c.Sessions.SessionDays(ctx, calendarID, from, day)
	if err != nil {
		return nil, fmt.Errorf("structures_page: read session days: %w", err)
	}
	out := make([]contracts.Day, 0, len(rows))
	for _, r := range rows {
		if r.IsSession {
			out = append(out, contracts.Day(r.SessionDay.Format(dayLayout)))
		}
	}
	return out, nil
}

// capture reads the newest capture among the last session days: its option rows are the held list, its
// stock rows give the instrument id of each underlying (stocks).
func (c Config) capture(ctx context.Context, alias string, in *compute.StructureInput, stocks map[string]int64) error {
	if c.Account == nil {
		return nil
	}
	days := in.SessionDays
	for i := len(days) - 1; i >= 0 && i >= len(days)-captureMaxProbe; i-- {
		rows, err := c.Account.Positions(ctx, alias, string(days[i]))
		if err != nil {
			return fmt.Errorf("structures_page: read positions: %w", err)
		}
		if len(rows) == 0 {
			continue
		}
		in.HeldDay = days[i]
		for _, p := range rows {
			switch strings.ToUpper(p.SecType) {
			case "OPT":
				in.Held = append(in.Held, compute.HeldOption{Underlying: p.Symbol, Right: rightLetter(p.Right),
					Strike: p.Strike, Expiry: p.Expiry, Quantity: p.Quantity})
			case "STK":
				stocks[p.Symbol] = p.ConID
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

// quotes reads the newest option_eod row of each contract of the held underlyings and matches it to its
// terms. A held underlying with no stock row in the capture has no quotes.
func (c Config) quotes(ctx context.Context, ps []compute.LedgerPosition, stocks map[string]int64, day contracts.Day) ([]compute.StructureQuote, error) {
	if c.Eod == nil || c.Terms == nil {
		return nil, nil
	}
	from, err := shiftDay(day, -quoteLookback)
	if err != nil {
		return nil, err
	}
	newest := map[int]contracts.OptionEod{}
	under := map[int]string{}
	var ids []int // instrument ids in order of first sight
	seen := map[string]bool{}
	for _, p := range ps {
		if p.Status != "open" || seen[p.Underlying] || stocks[p.Underlying] == 0 {
			continue
		}
		seen[p.Underlying] = true
		rows, err := c.Eod.OptionEod(ctx, stocks[p.Underlying], from, day)
		if err != nil {
			return nil, fmt.Errorf("structures_page: read option_eod: %w", err)
		}
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
	var out []compute.StructureQuote
	for lo := 0; lo < len(ids); lo += termsBatch {
		hi := min(lo+termsBatch, len(ids))
		batch := make([]int64, 0, hi-lo)
		for _, id := range ids[lo:hi] {
			batch = append(batch, int64(id))
		}
		terms, err := c.Terms.OptionTerms(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("structures_page: read option terms: %w", err)
		}
		for _, t := range terms {
			out = append(out, quoteOf(under[int(t.InstrumentID)], t, newest[int(t.InstrumentID)]))
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
	if v := r.UnderlyingPrice; v != nil {
		q.UnderlyingClose = strconv.FormatFloat(*v, 'f', -1, 64)
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
