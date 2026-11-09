package option_tracking_page

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// gaps are the named gaps of one gather: a reader that is nil gives its gap, in the order
// they are found. Design: docs/design/option-tracking.md#Decisions (Decisions 2, 3, 6, 7, 8, 11).
type gaps []string

const (
	gapNoReader     = "no_reader"     // Config.Ledger is nil
	gapNoFlexRows   = "no_flex_rows"  // the alias has no stored Flex row
	gapNoSessions   = "no_sessions"   // Config.Sessions is nil
	gapNoEodReaders = "no_eod_reader" // one of Eod, Catalog, Names or Terms is nil
	gapNoCandidates = "no_candidates" // Config.Candidates is nil
	gapNoAccount    = "no_account"    // Config.Account is nil

	dayLayout       = "2006-01-02"
	chunkSize       = 500
	sessionSpanDays = 366
	eventLookback   = 7
	heldLookback    = 6
	quoteExtraDays  = 5
)

var eventKinds = []string{"short_put", "covered_call", "held_call_guard"}

// gather reads everything the tracker needs for one alias on day and maps it to text. It does
// no arithmetic. A nil reader is a gap; a reader error is returned.
func gather(ctx context.Context, c Config, alias string, day contracts.Day) (trackapi.TrackInput, gaps, error) {
	in := trackapi.TrackInput{Alias: alias, AsOf: day}
	var gs gaps
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return in, nil, fmt.Errorf("option_tracking_page: build day %q: %w", day, err)
	}
	if c.Ledger == nil {
		gs = append(gs, gapNoReader)
	}
	if c.Sessions == nil {
		gs = append(gs, gapNoSessions)
	} else if in.SessionDays, err = sessionDays(ctx, c, d, day); err != nil {
		return in, gs, err
	}
	if c.Ledger != nil {
		trades, err := tradesOf(ctx, c.Ledger, alias)
		if err != nil {
			return in, gs, err
		}
		if len(trades) == 0 {
			gs = append(gs, gapNoFlexRows)
		} else {
			if c.Replay == nil {
				return in, gs, fmt.Errorf("option_tracking_page: nil replay")
			}
			in.Positions = c.Replay.Replay(compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: in.SessionDays, AsOf: day},
				compute.LedgerParams{MarkMaxAgeSessions: c.Params.MarkMaxAgeSessions}).Positions
		}
	}
	if c.Eod == nil || c.Catalog == nil || c.Names == nil || c.Terms == nil {
		gs = append(gs, gapNoEodReaders)
	} else if in.Quotes, err = quotesOf(ctx, c, d, in.Positions); err != nil {
		return in, gs, err
	}
	if c.Candidates == nil {
		gs = append(gs, gapNoCandidates)
	} else if in.Events, err = eventsOf(ctx, c.Candidates, alias, d); err != nil {
		return in, gs, err
	}
	if c.Account == nil {
		gs = append(gs, gapNoAccount)
	} else if in.Held, in.HeldKnown, err = heldOf(ctx, c.Account, alias, d); err != nil {
		return in, gs, err
	}
	return in, gs, nil
}

func dayText(d time.Time, back int) contracts.Day {
	return contracts.Day(d.AddDate(0, 0, -back).Format(dayLayout))
}

func sessionDays(ctx context.Context, c Config, d time.Time, day contracts.Day) ([]contracts.Day, error) {
	rows, err := c.Sessions.SessionDays(ctx, c.CalendarID, dayText(d, sessionSpanDays), day)
	if err != nil {
		return nil, fmt.Errorf("option_tracking_page: read session days: %w", err)
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
		return nil, fmt.Errorf("option_tracking_page: read trades of %s: %w", alias, err)
	}
	out := make([]compute.LedgerTrade, 0, len(rows))
	for _, r := range rows {
		var m map[string]string
		if err := json.Unmarshal([]byte(r.Attributes), &m); err != nil {
			return nil, fmt.Errorf("option_tracking_page: attributes of %s: %w", r.TransactionID, err)
		}
		out = append(out, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: m})
	}
	return out, nil
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

// decimalText is the one place a stored float of option_eod becomes decimal text. Nil, NaN and
// infinity give the empty text.
func decimalText(v *float64) string {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// side is C or P for a stored right (C, P, call or put).
func side(r string) string {
	switch strings.ToLower(r) {
	case "c", "call":
		return "C"
	case "p", "put":
		return "P"
	}
	return r
}

// strikeKey is the strike text without trailing zeros of its fraction, so 150.00 and 150 meet.
func strikeKey(s string) string {
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

type contractKey struct{ underlying, expiry, strike, right string }

// underlyingIDs finds the option_eod underlying ids whose name is the symbol of an open position.
func underlyingIDs(ctx context.Context, c Config, since contracts.Day, symbols map[string]bool) ([]int64, map[int64]string, error) {
	latest, err := c.Catalog.OptionEodLatest(ctx, since)
	if err != nil {
		return nil, nil, fmt.Errorf("option_tracking_page: read option_eod catalog: %w", err)
	}
	ids := make([]int64, 0, len(latest))
	for _, l := range latest {
		ids = append(ids, l.UnderlyingID)
	}
	var out []int64
	names := map[int64]string{}
	for _, ch := range chunks(ids) {
		rows, err := c.Names.InstrumentNames(ctx, ch)
		if err != nil {
			return nil, nil, fmt.Errorf("option_tracking_page: read instrument names: %w", err)
		}
		for _, n := range rows {
			if symbols[n.Name] {
				out = append(out, n.InstrumentID)
				names[n.InstrumentID] = n.Name
			}
		}
	}
	return out, names, nil
}

// termsOf reads the current terms of the instruments of rows, in chunks of 500.
func termsOf(ctx context.Context, r store.OptionTermsReader, rows []contracts.OptionEod) (map[int64]store.OptionTerms, error) {
	var instruments []int64
	seen := map[int]bool{}
	for _, row := range rows {
		if !seen[row.InstrumentID] {
			seen[row.InstrumentID] = true
			instruments = append(instruments, int64(row.InstrumentID))
		}
	}
	terms := map[int64]store.OptionTerms{}
	for _, ch := range chunks(instruments) {
		got, err := r.OptionTerms(ctx, ch)
		if err != nil {
			return nil, fmt.Errorf("option_tracking_page: read option terms: %w", err)
		}
		for _, t := range got {
			terms[t.InstrumentID] = t
		}
	}
	return terms, nil
}

func quotesOf(ctx context.Context, c Config, d time.Time, positions []compute.LedgerPosition) ([]trackapi.TrackQuote, error) {
	symbols := map[string]bool{}
	wanted := map[contractKey]bool{}
	for _, p := range positions {
		if p.Status != "open" {
			continue
		}
		symbols[p.Underlying] = true
		wanted[contractKey{p.Underlying, p.Expiry, strikeKey(p.Strike), side(p.Right)}] = true
	}
	if len(symbols) == 0 {
		return nil, nil
	}
	from := dayText(d, c.Params.MarkMaxAgeSessions+quoteExtraDays)
	to := dayText(d, 0)
	ids, names, err := underlyingIDs(ctx, c, from, symbols)
	if err != nil {
		return nil, err
	}
	var rows []contracts.OptionEod
	underOf := map[int]string{}
	for _, id := range ids {
		got, err := c.Eod.OptionEod(ctx, id, from, to)
		if err != nil {
			return nil, fmt.Errorf("option_tracking_page: read option_eod of %d: %w", id, err)
		}
		for _, r := range got {
			underOf[r.InstrumentID] = names[id]
		}
		rows = append(rows, got...)
	}
	terms, err := termsOf(ctx, c.Terms, rows)
	if err != nil {
		return nil, err
	}
	newest := map[contractKey]trackapi.TrackQuote{}
	var order []contractKey
	for _, r := range rows {
		t, ok := terms[int64(r.InstrumentID)]
		if !ok {
			continue
		}
		k := contractKey{underOf[r.InstrumentID], string(t.ExpirationDate), strikeKey(t.Strike), side(t.Right)}
		if !wanted[k] {
			continue
		}
		q := trackapi.TrackQuote{Underlying: k.underlying, Expiry: t.ExpirationDate, Strike: t.Strike, Right: side(t.Right),
			Day: contracts.Day(r.SessionDay.Format(dayLayout)), Bid: decimalText(r.Bid), Ask: decimalText(r.Ask),
			UnderlyingClose: decimalText(r.UnderlyingPrice), Delta: decimalText(r.Delta), Iv: decimalText(r.IvDecimal)}
		old, had := newest[k]
		if !had {
			order = append(order, k)
		}
		if !had || q.Day >= old.Day {
			newest[k] = q
		}
	}
	out := make([]trackapi.TrackQuote, 0, len(order))
	for _, k := range order {
		out = append(out, newest[k])
	}
	return out, nil
}

type gateResult struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// guardReason is the reason of the first gate of the stored JSON text that is not passing.
func guardReason(text string) string {
	var gates []gateResult
	if json.Unmarshal([]byte(text), &gates) != nil {
		return ""
	}
	for _, g := range gates {
		if g.Reason != "" && g.Status != "pass" && g.Status != "ok" {
			return g.Reason
		}
	}
	return ""
}

func deref[T ~string](p *T) T {
	if p == nil {
		return ""
	}
	return *p
}

func eventsOf(ctx context.Context, r store.CandidateReader, alias string, d time.Time) ([]trackapi.TrackEvent, error) {
	var out []trackapi.TrackEvent
	for back := 0; back <= eventLookback; back++ {
		day := dayText(d, back)
		for _, kind := range eventKinds {
			rows, err := r.Candidates(ctx, day, kind)
			if err != nil {
				return nil, fmt.Errorf("option_tracking_page: read %s rows of %s: %w", kind, day, err)
			}
			for _, c := range rows {
				if kind == "held_call_guard" && c.Alias != alias {
					continue
				}
				e := trackapi.TrackEvent{Underlying: c.Issuer, Expiry: c.Expiry, Strike: c.Strike, Right: side(c.Right),
					Kind: c.Kind, Day: c.SessionDay, NextEarnings: deref(c.NextEarnings), EarningsState: c.EarningsState,
					NextExDate: deref(c.NextExDate), ExAmount: deref(c.ExAmount)}
				if kind == "held_call_guard" {
					e.GuardStatus, e.GuardReason = c.Status, guardReason(c.Gates)
				}
				out = append(out, e)
			}
		}
	}
	return out, nil
}

func heldOf(ctx context.Context, r store.AccountReader, alias string, d time.Time) ([]trackapi.TrackHeld, bool, error) {
	for back := 0; back <= heldLookback; back++ {
		day := dayText(d, back)
		rows, err := r.Positions(ctx, alias, string(day))
		if err != nil {
			return nil, false, fmt.Errorf("option_tracking_page: read positions of %s: %w", day, err)
		}
		if len(rows) == 0 {
			continue
		}
		held := []trackapi.TrackHeld{}
		for _, p := range rows {
			if p.SecType == "OPT" {
				held = append(held, trackapi.TrackHeld{Underlying: p.Symbol, Expiry: contracts.Day(p.Expiry), Strike: p.Strike,
					Right: side(p.Right), Quantity: p.Quantity})
			}
		}
		return held, true, nil
	}
	return nil, false, nil
}
