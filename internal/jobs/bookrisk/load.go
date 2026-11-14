package bookrisk

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/strategy"
)

const (
	classEquity = strategy.RiskClass("equity")
	classHigh   = strategy.RiskClass("broad_high")
	classOther  = strategy.RiskClass("broad_other")
)

// The parameters of risk_page/load.go, whose sequence this loader repeats (a job may not import a page).
const (
	dayLayout       = "2006-01-02"
	sessionLookback = 60 // calendar days of session days to read
	quoteLookback   = 14 // calendar days of option_eod rows to read
	captureTarget   = "0.50"
	exitDTE         = 21
	commissionDays  = 30
	captureMaxProbe = 10 // session days probed for the newest capture
	termsBatch      = 500
	rateSeries      = "DGS3MO"
	rateLookback    = 14
)

// loaded is the book of one alias and what the stress cannot see: the capture options absent from the ledger.
type loaded struct {
	book        strategy.RiskBook
	notInLedger []strategy.RiskExcluded
	empty       bool // no ledger row and no capture: the alias has no book
}

// termKey names a contract by its underlying, right, expiry and strike without trailing zeros.
type termKey struct{ under, right, expiry, strike string }

func keyOf(under, right, expiry, strike string) termKey {
	if r, ok := new(big.Rat).SetString(strike); ok {
		strike = strings.TrimSuffix(strings.TrimRight(r.FloatString(6), "0"), ".")
	}
	return termKey{under, right, expiry, strike}
}

// classOf is the class of a symbol: the table of the manifest, else equity.
func (j *job) classOf(symbol string) strategy.RiskClass {
	if c, ok := j.classes[symbol]; ok {
		return c
	}
	return classEquity
}

func rightLetter(r string) string {
	if r == "" {
		return ""
	}
	return strings.ToUpper(r[:1])
}

func (j *job) load(ctx context.Context, alias string, day contracts.Day) (loaded, error) {
	var l loaded
	rows, err := j.st.Ledger.FlexTradeRows(ctx, alias)
	if err != nil {
		return l, fmt.Errorf("read ledger: %w", err)
	}
	trades := make([]compute.LedgerTrade, 0, len(rows))
	for _, r := range rows {
		attrs := map[string]string{}
		if err := json.Unmarshal([]byte(r.Attributes), &attrs); err != nil {
			return l, fmt.Errorf("ledger row %s: %w", r.TransactionID, err)
		}
		trades = append(trades, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: attrs})
	}
	days, err := j.sessionDays(ctx, day)
	if err != nil {
		return l, err
	}
	in := compute.StructureInput{Alias: alias, AsOf: day, SessionDays: days}
	res := j.st.Ledgers().Replay(compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: days, AsOf: day},
		compute.LedgerParams{MarkMaxAgeSessions: j.m.Params.MaxMarkAgeSessions})
	in.Positions, in.Cycles = res.Positions, res.Cycles
	cp, err := j.capture(ctx, alias, days)
	if err != nil {
		return l, err
	}
	for _, p := range cp.options {
		in.Held = append(in.Held, compute.HeldOption{Underlying: p.Symbol, Right: rightLetter(p.Right),
			Strike: p.Strike, Expiry: p.Expiry, Quantity: p.Quantity})
	}
	in.HeldDay = cp.day
	conIDs := map[string]int64{}
	for _, p := range cp.stocks {
		conIDs[p.Symbol] = p.ConID
	}
	quotes, terms, err := j.quotes(ctx, in.Positions, conIDs, day)
	if err != nil {
		return l, err
	}
	in.Quotes = quotes
	l.empty = len(rows) == 0 && cp.day == ""
	if l.empty {
		return l, nil
	}
	sr := j.st.Structures().Evaluate(in, compute.StructureParams{MarkMaxAgeSessions: j.m.Params.MaxMarkAgeSessions,
		CaptureTarget: captureTarget, ExitDTE: exitDTE, CommissionDays: commissionDays})
	funds, err := j.funds(ctx, alias, day, days)
	if err != nil {
		return l, err
	}
	rate, err := j.rate(ctx, day)
	if err != nil {
		return l, err
	}
	book := strategy.RiskBook{Alias: alias, Currency: defaultCurrency, RunDay: day, Rate: rate, Funds: funds,
		HasCapture: cp.day != "", SessionsOld: sessionsOld(days, day)}
	if funds != nil && funds.Currency != "" {
		book.Currency = funds.Currency
	}
	book.Holdings = append(book.Holdings, j.optionHoldings(sr, quotes, terms, day)...)
	stocks, err := j.stockHoldings(ctx, alias, day, in, cp, quotes)
	if err != nil {
		return l, err
	}
	book.Holdings = append(book.Holdings, stocks...)
	l.book = book
	l.notInLedger = j.captureOnly(sr.Diff, cp)
	return l, nil
}

func (j *job) sessionDays(ctx context.Context, day contracts.Day) ([]contracts.Day, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("bad day %q: %w", day, err)
	}
	from := contracts.Day(d.AddDate(0, 0, -sessionLookback).Format(dayLayout))
	rows, err := j.st.SessionDays.SessionDays(ctx, "XNYS", from, day)
	if err != nil {
		return nil, fmt.Errorf("read session days: %w", err)
	}
	out := make([]contracts.Day, 0, len(rows))
	for _, r := range rows {
		if r.IsSession {
			out = append(out, contracts.Day(r.SessionDay.Format(dayLayout)))
		}
	}
	return out, nil
}

// sessionsOld counts the session days after d up to and including the run day.
func sessionsOld(days []contracts.Day, run contracts.Day) func(contracts.Day) int {
	return func(d contracts.Day) int {
		n := 0
		for _, s := range days {
			if s > d && s <= run {
				n++
			}
		}
		return n
	}
}

// captured is the newest capture among the last session days: its OPTION and STOCK rows. day is empty when none.
type captured struct {
	day             contracts.Day
	options, stocks []store.Position
}

func (j *job) capture(ctx context.Context, alias string, days []contracts.Day) (captured, error) {
	var c captured
	for i := len(days) - 1; i >= 0 && i >= len(days)-captureMaxProbe; i-- {
		rows, err := j.st.Account.Positions(ctx, alias, string(days[i]))
		if err != nil {
			return c, fmt.Errorf("read positions: %w", err)
		}
		if len(rows) == 0 {
			continue
		}
		c.day = days[i]
		for _, p := range rows {
			switch strings.ToUpper(p.SecType) {
			case "OPTION":
				c.options = append(c.options, p)
			case "STOCK":
				c.stocks = append(c.stocks, p)
			}
		}
		return c, nil
	}
	return c, nil
}

// quotes reads the newest option_eod row of each contract of the held underlyings and matches it to its terms, as
// risk_page does; the terms of each quote are returned beside, by contract.
func (j *job) quotes(ctx context.Context, ps []compute.LedgerPosition, conIDs map[string]int64,
	day contracts.Day) ([]compute.StructureQuote, map[termKey]store.OptionTerms, error) {
	if j.st.OptionEod == nil || j.st.Terms == nil {
		return nil, nil, nil
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, nil, fmt.Errorf("bad day %q: %w", day, err)
	}
	from := contracts.Day(d.AddDate(0, 0, -quoteLookback).Format(dayLayout))
	newest := map[int]contracts.OptionEod{}
	under := map[int]string{}
	var ids []int
	seen := map[string]bool{}
	for _, p := range ps {
		if p.Status != "open" || seen[p.Underlying] || conIDs[p.Underlying] == 0 {
			continue
		}
		seen[p.Underlying] = true
		rows, err := j.st.OptionEod.OptionEod(ctx, conIDs[p.Underlying], from, day)
		if err != nil {
			return nil, nil, fmt.Errorf("read option_eod: %w", err)
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
	byContract := map[termKey]store.OptionTerms{}
	for lo := 0; lo < len(ids); lo += termsBatch {
		hi := min(lo+termsBatch, len(ids))
		batch := make([]int64, 0, hi-lo)
		for _, id := range ids[lo:hi] {
			batch = append(batch, int64(id))
		}
		terms, err := j.st.Terms.OptionTerms(ctx, batch)
		if err != nil {
			return nil, nil, fmt.Errorf("read option terms: %w", err)
		}
		for _, t := range terms {
			q := quoteOf(under[int(t.InstrumentID)], t, newest[int(t.InstrumentID)])
			out = append(out, q)
			byContract[keyOf(q.Underlying, q.Right, q.Expiry, q.Strike)] = t
		}
	}
	return out, byContract, nil
}

func decimal(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// quoteOf is the stored row as decimal text, shortest exact form; an absent value stays empty.
func quoteOf(underlying string, t store.OptionTerms, r contracts.OptionEod) compute.StructureQuote {
	return compute.StructureQuote{Underlying: underlying, Right: rightLetter(t.Right), Strike: t.Strike,
		Expiry: string(t.ExpirationDate), Day: contracts.Day(r.SessionDay.Format(dayLayout)),
		Bid: decimal(r.Bid), Ask: decimal(r.Ask), UnderlyingClose: decimal(r.UnderlyingPrice), Delta: decimal(r.Delta),
		Gamma: decimal(r.Gamma), Theta: decimal(r.ThetaPerCalendarDay), Vega: decimal(r.VegaPerVolPoint)}
}

// newestQuote is the quote of a contract with the newest day not after day.
func newestQuote(qs []compute.StructureQuote, under, right, expiry, strike string, day contracts.Day) *compute.StructureQuote {
	want := keyOf(under, right, expiry, strike)
	var best *compute.StructureQuote
	for i := range qs {
		q := &qs[i]
		if q.Day > day || keyOf(q.Underlying, q.Right, q.Expiry, q.Strike) != want {
			continue
		}
		if best == nil || q.Day > best.Day {
			best = q
		}
	}
	return best
}

// newestSpot is the UnderlyingClose of the newest quote of a symbol that has one, with its day.
func newestSpot(qs []compute.StructureQuote, symbol string, day contracts.Day) (string, contracts.Day, bool) {
	var spot string
	var at contracts.Day
	for _, q := range qs {
		v, ok := new(big.Rat).SetString(q.UnderlyingClose)
		if q.Underlying != symbol || q.Day > day || !ok || v.Sign() <= 0 {
			continue
		}
		if spot == "" || q.Day > at {
			spot, at = q.UnderlyingClose, q.Day
		}
	}
	return spot, at, spot != ""
}

// optionHoldings is one holding per leg of the structures; its mark is the leg's mid and its spot and days those of
// the quote of the contract. A leg of a cash-settled European option, or of an underlying of an index class, is
// cash settled (design Decision 6).
func (j *job) optionHoldings(sr compute.StructureResult, qs []compute.StructureQuote, terms map[termKey]store.OptionTerms,
	day contracts.Day) []strategy.RiskHolding {
	var out []strategy.RiskHolding
	for _, s := range append(append([]compute.Structure(nil), sr.Structures...), sr.Other...) {
		class := j.classOf(s.Underlying)
		for _, l := range s.Legs {
			h := strategy.RiskHolding{Underlying: s.Underlying, Currency: s.Currency, Class: class,
				Quantity: l.Quantity, Multiplier: l.Multiplier, Mark: l.Mid, MarkUsable: l.Mid != ""}
			t := terms[keyOf(s.Underlying, l.Right, l.Expiry, l.Strike)]
			h.Option = &strategy.RiskLeg{Right: l.Right, Strike: l.Strike, Expiry: l.Expiry,
				CashSettled: (strings.EqualFold(t.ExerciseStyle, "european") && strings.EqualFold(t.SettlementType, "cash")) ||
					class == classHigh || class == classOther}
			if q := newestQuote(qs, s.Underlying, l.Right, l.Expiry, l.Strike, day); q != nil {
				h.Spot, h.SpotDay, h.MarkDay = q.UnderlyingClose, q.Day, q.Day
			}
			out = append(out, h)
		}
	}
	return out
}

// stockHoldings are the STOCK rows of the capture; an alias with no capture has the share lines its ledger
// carries (the cycles), with a spot only when a quote of the symbol gives one. The price of a share is the
// UnderlyingClose of a quote of its symbol, else the stored mark of its ConID.
func (j *job) stockHoldings(ctx context.Context, alias string, day contracts.Day, in compute.StructureInput, cp captured,
	qs []compute.StructureQuote) ([]strategy.RiskHolding, error) {
	var marks map[int64]mark.PositionMark
	var out []strategy.RiskHolding
	for _, p := range cp.stocks {
		if v, ok := new(big.Rat).SetString(p.Quantity); !ok || v.Sign() == 0 {
			continue
		}
		h := strategy.RiskHolding{Underlying: p.Symbol, Currency: p.Currency, Class: j.classOf(p.Symbol),
			Quantity: p.Quantity, Multiplier: "1"}
		if spot, at, ok := newestSpot(qs, p.Symbol, day); ok {
			h.Spot, h.Mark, h.SpotDay, h.MarkDay, h.MarkUsable = spot, spot, at, at, true
		} else if j.st.Marks != nil {
			if marks == nil {
				ms, err := j.st.Marks.Marks(ctx, alias, string(day))
				if err != nil {
					return nil, fmt.Errorf("read marks: %w", err)
				}
				marks = map[int64]mark.PositionMark{}
				for _, m := range ms {
					marks[m.ConID] = m
				}
			}
			if m, ok := marks[p.ConID]; ok {
				usable := m.Status == "ok" && m.Basis != mark.BasisNone && m.Price != ""
				h.MarkUsable = usable
				if usable {
					h.Spot, h.Mark = m.Price, m.Price
				}
				h.SpotDay, h.MarkDay = contracts.Day(m.SessionDay), contracts.Day(m.SessionDay)
			}
		}
		out = append(out, h)
	}
	if cp.day != "" {
		return out, nil
	}
	for _, c := range in.Cycles {
		if v, ok := new(big.Rat).SetString(c.Shares); !ok || v.Sign() == 0 {
			continue
		}
		h := strategy.RiskHolding{Underlying: c.Underlying, Currency: c.Currency, Class: j.classOf(c.Underlying),
			Quantity: c.Shares, Multiplier: "1"}
		if spot, at, ok := newestSpot(qs, c.Underlying, day); ok {
			h.Spot, h.Mark, h.SpotDay, h.MarkDay, h.MarkUsable = spot, spot, at, at, true
		}
		out = append(out, h)
	}
	return out, nil
}

// captureOnly lists the capture options the ledger holds no open position for as not_in_ledger, in the currency
// of their capture row.
func (j *job) captureOnly(d compute.CaptureDiff, cp captured) []strategy.RiskExcluded {
	cur := map[string]string{}
	for _, p := range append(append([]store.Position(nil), cp.options...), cp.stocks...) {
		if _, ok := cur[p.Symbol]; !ok || strings.ToUpper(p.SecType) == "OPTION" {
			cur[p.Symbol] = p.Currency
		}
	}
	var out []strategy.RiskExcluded
	for _, k := range d.CaptureOnly {
		under, _, _ := strings.Cut(k, " ")
		c := cur[under]
		if c == "" {
			c = defaultCurrency
		}
		out = append(out, strategy.RiskExcluded{Underlying: under, Currency: c, Reason: "not_in_ledger"})
	}
	return out
}

// funds are the funds of the newest account.value capture of the day or the session day before; nil when neither
// has a row with one of the three tags.
func (j *job) funds(ctx context.Context, alias string, day contracts.Day, days []contracts.Day) (*strategy.RiskFunds, error) {
	try := []contracts.Day{day}
	if i := slices.Index(days, day); i > 0 {
		try = append(try, days[i-1])
	} else if d, err := time.Parse(dayLayout, string(day)); err == nil {
		try = append(try, contracts.Day(d.AddDate(0, 0, -1).Format(dayLayout)))
	}
	for _, d := range try {
		rows, err := j.st.Account.Values(ctx, alias, string(d))
		if err != nil {
			return nil, fmt.Errorf("read values: %w", err)
		}
		var newest time.Time
		for _, r := range rows {
			if r.CapturedAt.After(newest) {
				newest = r.CapturedAt
			}
		}
		var f strategy.RiskFunds
		found := false
		for _, r := range rows {
			if !r.CapturedAt.Equal(newest) {
				continue
			}
			switch r.Tag {
			case "NetLiquidation":
				f.NetLiquidation = r.Value
			case "AvailableFunds":
				f.Available = r.Value
			case "ExcessLiquidity":
				f.ExcessLiquidity = r.Value
			default:
				continue
			}
			found = true
			if f.Currency == "" || r.Tag == "NetLiquidation" {
				f.Currency = r.Currency
			}
		}
		if found {
			return &f, nil
		}
	}
	return nil, nil
}

// rate is the newest DGS3MO point of the last two weeks as stored (percent text); empty without a reader or a point.
func (j *job) rate(ctx context.Context, day contracts.Day) (string, error) {
	if j.st.Series == nil {
		return "", nil
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return "", fmt.Errorf("bad day %q: %w", day, err)
	}
	pts, err := j.st.Series.SeriesPoints(ctx, rateSeries, contracts.Day(d.AddDate(0, 0, -rateLookback).Format(dayLayout)), day)
	if err != nil {
		return "", fmt.Errorf("read rate: %w", err)
	}
	var bestDay string
	var text string
	for _, p := range pts {
		od := p.ObservationDate.Format(dayLayout)
		if p.Value == nil || od < bestDay {
			continue
		}
		bestDay, text = od, strconv.FormatFloat(*p.Value, 'f', -1, 64)
	}
	return text, nil
}
