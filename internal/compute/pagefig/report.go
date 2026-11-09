package pagefig

import (
	"math/big"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// staleWeekdays is the age, in weekdays behind the newest series of a request, past which a
// close is the gap stale_input (3 session days, builders-wave Decision 2).
const staleWeekdays = 3

// trades is the figure report.trade_kind: one line per execution, in input order. The kind comes
// from the signed quantity of the contract before the fill (PriorQuantity) and the fill's own sign
// (BOT positive, SLD negative): a fill that grows the position, or starts one, is open; one that
// moves it toward zero is close, or expire when an option closes at price 0. A fill without a
// readable prior has no kind (Kind is empty) and is never guessed. The rows are the day's
// executions already; day is the data day of the page.
func trades(_ contracts.Day, in []blockapi.ExecIn) []blockapi.TradeLine {
	out := make([]blockapi.TradeLine, 0, len(in))
	for _, e := range in {
		out = append(out, blockapi.TradeLine{
			Time: e.Time, Kind: tradeKind(e), Instrument: instrument(e), Quantity: e.Quantity, Price: e.Price,
		})
	}
	return out
}

func tradeKind(e blockapi.ExecIn) string {
	prior, ok := parse(e.PriorQuantity)
	q, qok := parse(e.Quantity)
	if !ok || !qok {
		return ""
	}
	delta := new(big.Rat).Abs(q)
	switch strings.ToUpper(e.Side) {
	case "BOT":
	case "SLD":
		delta.Neg(delta)
	default:
		return ""
	}
	if prior.Sign() == 0 || prior.Sign() == delta.Sign() {
		return "open"
	}
	if p, pok := parse(e.Price); pok && p.Sign() == 0 && e.SecType == "option" {
		return "expire"
	}
	return "close"
}

func instrument(e blockapi.ExecIn) string {
	if e.SecType != "option" {
		return e.Symbol
	}
	return strings.Join([]string{e.Symbol, e.Expiry, e.Strike, e.Right}, " ")
}

// accountMove is the figures report.account_day and report.margin_buffer of one account: the net
// liquidation, its change from the prior capture (amount and percent of the prior), the margin
// buffer (ExcessLiquidity as a percent of NetLiquidation) and the cash left after assignment of
// every short put. Each is the gap missing_input when an input is absent or unreadable, and
// unsupported_currency when two inputs differ in currency.
func accountMove(in blockapi.AccountMoveIn) blockapi.AccountMoveOut {
	net, netR, netOK := tagValue(in.Values, "NetLiquidation")
	prior, priorR, priorOK := tagValue(in.Prior, "NetLiquidation")
	excess, excessR, excessOK := tagValue(in.Values, "ExcessLiquidity")
	cash, cashR, cashOK := tagValue(in.Values, "TotalCashValue")
	out := blockapi.AccountMoveOut{
		Net: gap("missing_input"), DayAmount: gap("missing_input"), DayPct: gap("missing_input"),
		MarginBuffer: gap("missing_input"), CashAfterAssignment: gap("missing_input"),
	}
	if netOK {
		out.Net = compute.FigureValue{Value: money(netR)}
	}
	if netOK && priorOK {
		switch {
		case prior.Currency != net.Currency:
			out.DayAmount, out.DayPct = gap("unsupported_currency"), gap("unsupported_currency")
		default:
			diff := new(big.Rat).Sub(netR, priorR)
			out.DayAmount = compute.FigureValue{Value: money(diff)}
			out.DayPct = percent(diff, priorR)
		}
	}
	if netOK && excessOK {
		if excess.Currency != net.Currency {
			out.MarginBuffer = gap("unsupported_currency")
		} else {
			out.MarginBuffer = percent(excessR, netR)
		}
	}
	if cashOK {
		out.CashAfterAssignment = cashLeft(cash, cashR, in.Obligations)
	}
	return out
}

// percent is 100 * num / den rounded half-even to 2 places; a den of zero or less is no_data.
func percent(num, den *big.Rat) compute.FigureValue {
	if den.Sign() <= 0 {
		return gap("no_data")
	}
	p := new(big.Rat).Quo(new(big.Rat).Mul(num, big.NewRat(100, 1)), den)
	return compute.FigureValue{Value: money(p)}
}

// cashLeft is the cash minus the gross put notional of every obligation, signed.
func cashLeft(cash blockapi.ValueIn, cashR *big.Rat, obs []blockapi.ObligationIn) compute.FigureValue {
	left := new(big.Rat).Set(cashR)
	for _, o := range obs {
		n, ok := notional(o)
		if !ok {
			return gap("missing_input")
		}
		if o.Currency != cash.Currency {
			return gap("unsupported_currency")
		}
		left.Sub(left, n)
	}
	return compute.FigureValue{Value: money(left)}
}

// moves is the figure report.market_move: per series the one-day change (newest close over the one
// before) and the five-day change (newest over the sixth close, so six closes), in percent
// rounded half-even to 2 places, in input order. Fewer closes is the gap no_data, an unreadable
// or non-positive close missing_input; a series whose newest day is more than three weekdays
// behind the newest day of the request is stale_input in both.
func moves(in []blockapi.CloseIn) []blockapi.MoveLine {
	var newest contracts.Day
	for _, s := range in {
		if len(s.Days) > 0 && s.Days[0] > newest {
			newest = s.Days[0]
		}
	}
	out := make([]blockapi.MoveLine, 0, len(in))
	for _, s := range in {
		l := blockapi.MoveLine{Name: s.Name, Symbol: s.Symbol}
		if len(s.Days) > 0 {
			l.DataDay = s.Days[0]
		}
		if len(s.Closes) == 0 || len(s.Days) != len(s.Closes) {
			l.D1, l.D5 = gap("no_data"), gap("no_data")
		} else if weekdaysBehind(s.Days[0], newest) > staleWeekdays {
			l.D1, l.D5 = gap("stale_input"), gap("stale_input")
		} else {
			l.D1, l.D5 = change(s.Closes, 1), change(s.Closes, 5)
		}
		out = append(out, l)
	}
	return out
}

// change is the percent change of closes[0] over closes[back]; no_data without that many closes.
func change(closes []string, back int) compute.FigureValue {
	if len(closes) <= back {
		return gap("no_data")
	}
	now, ok1 := parse(closes[0])
	then, ok2 := parse(closes[back])
	if !ok1 || !ok2 || then.Sign() <= 0 {
		return gap("missing_input")
	}
	return percent(new(big.Rat).Sub(now, then), then)
}

// weekdaysBehind counts the weekdays after day up to and including newest; an unreadable day is
// treated as stale.
func weekdaysBehind(day, newest contracts.Day) int {
	a, err1 := time.Parse("2006-01-02", string(day))
	b, err2 := time.Parse("2006-01-02", string(newest))
	if err1 != nil || err2 != nil {
		return staleWeekdays + 1
	}
	n := 0
	for d := a.AddDate(0, 0, 1); !d.After(b); d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			n++
		}
	}
	return n
}
