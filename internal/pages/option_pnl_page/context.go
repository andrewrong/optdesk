package option_pnl_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

const (
	dayLayout       = "2006-01-02"
	chunkSize       = 500
	sessionSpanDays = 366
	rateLeadDays    = 14
	rateSeries      = "DGS3MO"
	basisTrades     = "trades"
)

// gathered is what one alias gives the builder. gap names a missing reader or record of the alias.
type gathered struct {
	lots  []pnlLot
	rates []optionattrapi.Rate
	calls int
	longs int
	gap   string
}

func dayText(d time.Time, back int) contracts.Day {
	return contracts.Day(d.AddDate(0, 0, -back).Format(dayLayout))
}

// chunks splits ids into slices of at most chunkSize.
func chunks(ids []int64) [][]int64 {
	var out [][]int64
	for len(ids) > chunkSize {
		out = append(out, ids[:chunkSize])
		ids = ids[chunkSize:]
	}
	if len(ids) > 0 {
		out = append(out, ids)
	}
	return out
}

// gather reads everything the lots of one alias need and maps it to text; a nil reader is a gap and a
// reader error is returned. Two replays: the first without marks learns the positions, the second
// takes the marks of marks.go (Decision 6).
func gather(ctx context.Context, c Config, alias string, day contracts.Day, series store.SeriesReader) (gathered, error) {
	var g gathered
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return g, fmt.Errorf("option_pnl_page: build day %q: %w", day, err)
	}
	if c.Sessions == nil {
		g.gap = codeNoSessions
		return g, nil
	}
	if c.Days == nil {
		g.gap = codeNoQuoteReder
		return g, nil
	}
	sessions, err := sessionDays(ctx, c, d, day)
	if err != nil {
		return g, err
	}
	trades, err := tradesOf(ctx, c.Ledger, alias)
	if err != nil {
		return g, err
	}
	if len(trades) == 0 {
		g.gap = codeNoFlexRows
		return g, nil
	}
	params := compute.LedgerParams{MarkMaxAgeSessions: c.MarkMaxAgeSessions}
	in := compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: sessions, AsOf: day}
	first := c.Replay.Replay(in, params).Positions
	shown, from := selectLots(c, first, d, &g)
	rows, err := rowsOf(ctx, c, shown, from, day)
	if err != nil {
		return g, err
	}
	for _, p := range shown {
		if m, ok := markOf(p, rows[p.Contract]); ok && p.Status == statusOpen {
			in.Marks = append(in.Marks, m)
		}
	}
	second := c.Replay.Replay(in, params).Positions
	byID := map[string]compute.LedgerPosition{}
	for _, p := range second {
		byID[p.PositionID] = p
	}
	for _, p := range shown {
		if q, ok := byID[p.PositionID]; ok {
			p = q
		}
		g.lots = append(g.lots, pnlLot{pos: p, rows: rows[p.Contract]})
	}
	if len(g.lots) == 0 {
		return g, nil
	}
	if g.rates, err = ratesOf(ctx, series, from, d); err != nil {
		return g, err
	}
	return g, closesOf(ctx, c, g.lots)
}

// selectLots keeps the short puts that are open or ended within RecentDays, counts the calls and the long puts it
// skips, and returns the oldest open day of the kept lots.
func selectLots(c Config, positions []compute.LedgerPosition, d time.Time, g *gathered) ([]compute.LedgerPosition, contracts.Day) {
	since := dayText(d, c.RecentDays)
	var out []compute.LedgerPosition
	var from contracts.Day
	for _, p := range positions {
		if p.Status != statusOpen && p.ClosedOn < since {
			continue
		}
		if strings.ToUpper(p.Right) != "P" {
			g.calls++
			continue
		}
		if q := strings.TrimSpace(p.Quantity); q != "" && !strings.HasPrefix(q, "-") {
			g.longs++
			continue
		}
		out = append(out, p)
		if from == "" || p.OpenedOn < from {
			from = p.OpenedOn
		}
	}
	return out, from
}

// rowsOf reads the option_day rows of the conids of the lots in chunks of 500, keyed by contract text.
func rowsOf(ctx context.Context, c Config, lots []compute.LedgerPosition, from, to contracts.Day) (map[string][]optionday.Day, error) {
	out := map[string][]optionday.Day{}
	var ids []int64
	seen := map[int64]bool{}
	for _, p := range lots {
		if id, err := strconv.ParseInt(p.Contract, 10, 64); err == nil && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, ch := range chunks(ids) {
		got, err := c.Days.Days(ctx, ch, from, to)
		if err != nil {
			return nil, fmt.Errorf("option_pnl_page: read option_day rows: %w", err)
		}
		for _, r := range got {
			k := strconv.FormatInt(r.InstrumentID, 10)
			out[k] = append(out[k], r)
		}
	}
	return out, nil
}

// ratesOf reads the DGS3MO points from rateLeadDays before from up to day, through the argument of Build.
func ratesOf(ctx context.Context, r store.SeriesReader, from contracts.Day, d time.Time) ([]optionattrapi.Rate, error) {
	if r == nil {
		return nil, nil
	}
	start, err := time.Parse(dayLayout, string(from))
	if err != nil {
		return nil, fmt.Errorf("option_pnl_page: open day %q: %w", from, err)
	}
	pts, err := r.SeriesPoints(ctx, rateSeries, dayText(start, rateLeadDays), contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("option_pnl_page: read %s: %w", rateSeries, err)
	}
	var out []optionattrapi.Rate
	for _, p := range pts {
		if p.Value == nil {
			continue
		}
		// the one place a stored float becomes decimal text; NaN and infinity do not parse and are dropped
		text := strconv.FormatFloat(*p.Value, 'f', -1, 64)
		if _, err := decimal.NewFromString(text); err != nil {
			continue
		}
		out = append(out, optionattrapi.Rate{Day: contracts.Day(p.ObservationDate.Format(dayLayout)), Percent: text})
	}
	return out, nil
}

// closesOf sets endClose of each lot that expired, was assigned or exercised: the stored trades close
// of its underlying on ClosedOn. No reader, a shared name or no bar leaves it empty.
func closesOf(ctx context.Context, c Config, lots []pnlLot) error {
	if c.StockIDs == nil || c.Bars == nil {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	for _, l := range lots {
		if endedKind(l.pos.Status) && !seen[l.pos.Underlying] {
			seen[l.pos.Underlying] = true
			names = append(names, l.pos.Underlying)
		}
	}
	if len(names) == 0 {
		return nil
	}
	got, err := c.StockIDs.StockIDs(ctx, names)
	if err != nil {
		return fmt.Errorf("option_pnl_page: read stock ids: %w", err)
	}
	count := map[string]int{}
	id := map[string]int64{}
	for _, n := range got {
		count[n.Name]++
		id[n.Name] = n.InstrumentID
	}
	for i, l := range lots {
		p := l.pos
		if !endedKind(p.Status) || count[p.Underlying] != 1 {
			continue
		}
		bars, err := c.Bars.PriceBars(ctx, id[p.Underlying], basisTrades, p.ClosedOn, p.ClosedOn)
		if err != nil {
			return fmt.Errorf("option_pnl_page: read close of %s: %w", p.Underlying, err)
		}
		if len(bars) > 0 {
			lots[i].endClose = bars[len(bars)-1].Close
		}
	}
	return nil
}

func sessionDays(ctx context.Context, c Config, d time.Time, day contracts.Day) ([]contracts.Day, error) {
	rows, err := c.Sessions.SessionDays(ctx, c.CalendarID, dayText(d, sessionSpanDays), day)
	if err != nil {
		return nil, fmt.Errorf("option_pnl_page: read session days: %w", err)
	}
	var out []contracts.Day
	for _, s := range rows {
		if s.IsSession {
			out = append(out, contracts.Day(s.SessionDay.Format(dayLayout)))
		}
	}
	return out, nil
}

func tradesOf(ctx context.Context, r store.OptionLedgerReader, alias string) ([]compute.LedgerTrade, error) {
	rows, err := r.FlexTradeRows(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("option_pnl_page: read trades of %s: %w", alias, err)
	}
	out := make([]compute.LedgerTrade, 0, len(rows))
	for _, r := range rows {
		var m map[string]string
		if err := json.Unmarshal([]byte(r.Attributes), &m); err != nil {
			return nil, fmt.Errorf("option_pnl_page: attributes of %s: %w", r.TransactionID, err)
		}
		out = append(out, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: m})
	}
	return out, nil
}
