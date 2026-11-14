package replay

import (
	"math/big"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

// Reasons of an outcome that is not priced (docs/contracts/strategy.md).
const (
	reasonNoInstrument = "no_instrument"
	reasonGap          = "gap_before_close"
	reasonNoSettle     = "no_settle_price"
	reasonNotExpired   = "not_expired"
	reasonSplit        = "split_in_life"
)

const dayLayout = "2006-01-02"

// lotDays returns the session days after the entry day up to the expiry, in order.
func lotDays(l replayapi.Lot, days []contracts.Day) []contracts.Day {
	var out []contracts.Day
	for _, d := range days {
		if d > l.EntryDay && d <= l.Expiry {
			out = append(out, d)
		}
	}
	return out
}

func ratOf(s string) (*big.Rat, bool) {
	if s == "" {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

// text writes a rational as exact decimal text (every figure here is a decimal, so eight places lose nothing).
func text(r *big.Rat) string {
	s := r.FloatString(8)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

func daysBetween(from, to contracts.Day) (int, bool) {
	a, err1 := time.Parse(dayLayout, string(from))
	b, err2 := time.Parse(dayLayout, string(to))
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return int(b.Sub(a).Hours() / 24), true
}

// intrinsic is the value of the right at the close, floor zero.
func intrinsic(right string, strike, closePx *big.Rat) (*big.Rat, bool) {
	var v *big.Rat
	switch right {
	case "P":
		v = new(big.Rat).Sub(strike, closePx)
	case "C":
		v = new(big.Rat).Sub(closePx, strike)
	default:
		return nil, false
	}
	if v.Sign() < 0 {
		v.SetInt64(0)
	}
	return v, true
}

// splitInLife is true when the adjusted-to-trades close ratio differs between the entry day and the settle day.
// A day or basis the market lacks cannot show a split.
func splitInLife(l replayapi.Lot, mkt replayapi.Market) bool {
	ratio := func(d contracts.Day) (*big.Rat, bool) {
		c, ok1 := ratOf(mkt.Closes[l.Underlying][d])
		a, ok2 := ratOf(mkt.AdjustedCloses[l.Underlying][d])
		if !ok1 || !ok2 || c.Sign() == 0 {
			return nil, false
		}
		return new(big.Rat).Quo(a, c), true
	}
	r1, ok1 := ratio(l.EntryDay)
	r2, ok2 := ratio(l.SettleDay)
	return ok1 && ok2 && r1.Cmp(r2) != 0
}

// holdOutcome prices the lot held to its settle day: credit minus intrinsic value at that day's close. It needs no
// quote.
func holdOutcome(l replayapi.Lot, mkt replayapi.Market, asOf contracts.Day) replayapi.Outcome {
	o := replayapi.Outcome{LotID: l.ID, Variant: HoldVariant}
	switch {
	case l.SettleDay > asOf:
		o.Reason = reasonNotExpired
		return o
	case splitInLife(l, mkt):
		o.Reason = reasonSplit
		return o
	}
	credit, ok1 := ratOf(l.Credit)
	strike, ok2 := ratOf(l.Strike)
	closePx, ok3 := ratOf(mkt.Closes[l.Underlying][l.SettleDay])
	if !ok1 || !ok2 || !ok3 {
		o.Reason = reasonNoSettle
		return o
	}
	in, ok := intrinsic(l.Right, strike, closePx)
	if !ok {
		o.Reason = reasonNoSettle
		return o
	}
	o.Priced, o.ClosedOn, o.CloseReason = true, l.SettleDay, "expiry"
	o.Gross, o.InTheMoney = text(new(big.Rat).Sub(credit, in)), in.Sign() > 0
	return o
}

// walkVariant walks the lot's session days under the variant. A day without a stored quote is a gap and makes the
// lot unpriced, whether the rule closes on it, closes later or never closes: it is never filled.
func walkVariant(l replayapi.Lot, v replayapi.Variant, mkt replayapi.Market, window []contracts.Day,
	hold replayapi.Outcome) replayapi.Outcome {
	o := replayapi.Outcome{LotID: l.ID, Variant: v.Name}
	if !hold.Priced {
		o.Reason = hold.Reason
		return o
	}
	if l.InstrumentID == "" {
		o.Reason = reasonNoInstrument
		return o
	}
	credit, _ := ratOf(l.Credit) // hold priced, so it parses
	skipDTE := earlyDTEWindow(l, v)
	pos := strategy.Position{
		Contract: l.InstrumentID, Right: l.Right, Strike: l.Strike, Expiry: l.Expiry, OpenedOn: l.EntryDay,
		OpenPrice: l.Credit, Quantity: "-1", Multiplier: l.Multiplier,
	}
	for _, d := range window {
		q, has := mkt.Quotes[l.InstrumentID][d]
		snap := strategy.Snapshot{RunDay: d, ChainDay: d, SessionDays: window, Underlying: l.Underlying}
		if has {
			snap.Quotes = []compute.OptionQuote{{InstrumentID: l.InstrumentID, Bid: q.Bid, Ask: q.Ask, QuoteOK: true}}
		}
		act := v.Strategy.Manage(pos, snap)
		ask, askOK := ratOf(q.Ask)
		if !has || !askOK {
			o.Reason = reasonGap
			return o
		}
		if act.Kind != "close" || (act.Reason == "early_dte" && skipDTE) {
			continue
		}
		o.Priced, o.ClosedOn, o.CloseReason = true, d, act.Reason
		o.Gross = text(new(big.Rat).Sub(credit, ask))
		return o
	}
	o.Priced, o.ClosedOn, o.CloseReason, o.Gross, o.InTheMoney = true, hold.ClosedOn, "expiry", hold.Gross, hold.InTheMoney
	return o
}

// earlyDTEWindow is true when the lot was opened with at most early_dte days to expiry: an early_dte close is then
// ignored. A strategy without the key is not affected.
func earlyDTEWindow(l replayapi.Lot, v replayapi.Variant) bool {
	raw, ok := v.Strategy.Params().Values["early_dte"]
	if !ok {
		return false
	}
	limit, ok := ratOf(raw)
	if !ok {
		return false
	}
	n, ok := daysBetween(l.EntryDay, l.Expiry)
	return ok && big.NewRat(int64(n), 1).Cmp(limit) <= 0
}
