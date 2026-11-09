package optionledger

import (
	"math/big"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

// ruleVersion is the version of the rule printed with every result.
const ruleVersion = 1

// The kinds of a row (Decision 6).
const (
	kindOpen      = "open"
	kindClose     = "close"
	kindExpire    = "expire"
	kindAssign    = "assign"
	kindExercise  = "exercise"
	kindStockBuy  = "stock_buy"
	kindStockSell = "stock_sell"
)

type ledger struct{}

// New returns the rule: exact (math/big.Rat), pure, no clock and no I/O.
func New() compute.OptionLedger { return ledger{} }

// event is one Flex trade row that enters a figure.
type event struct {
	txID       string
	timeKey    string // 14 digits, YYYYMMDDHHMMSS
	tradeDate  string // 8 digits
	kind       string
	underlying string
	contract   string   // matching key: conid, else symbol:expiry:strike:put_call
	markKeys   []string // every name a mark may carry for the contract
	currency   string
	putCall    string
	expiry     string // digits
	strike     *big.Rat
	qty        *big.Rat
	mult       *big.Rat
	price      *big.Rat // stock rows only
	proceeds   *big.Rat
	commission *big.Rat // a cost, positive
}

func (e event) isStock() bool { return e.kind == kindStockBuy || e.kind == kindStockSell }

// Replay replays the rows of one alias.
func (ledger) Replay(in compute.LedgerInput, p compute.LedgerParams) compute.LedgerResult {
	events, review := parseAll(in.Trades)
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].timeKey != events[j].timeKey {
			return events[i].timeKey < events[j].timeKey
		}
		return txLess(events[i].txID, events[j].txID)
	})
	var opts, stocks []event
	for _, e := range events {
		if e.isStock() {
			stocks = append(stocks, e)
		} else {
			opts = append(opts, e)
		}
	}
	ms := newMarkSet(in, p, opts)
	pieces, unresolved := matchOptions(opts)
	cycles, stockUnresolved := buildCycles(in.Alias, stocks, pieces, ms)
	unresolved = append(unresolved, stockUnresolved...)

	positions := make([]compute.LedgerPosition, 0, len(pieces))
	for _, pc := range pieces {
		positions = append(positions, pc.position(in.Alias, ms))
	}
	return compute.LedgerResult{
		RuleVersion: ruleVersion,
		Positions:   positions,
		Cycles:      cycles,
		Unresolved:  sortedUnique(unresolved),
		Review:      sortedUnique(review),
	}
}

func sortedUnique(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return txLess(out[i], out[j]) })
	return out
}

// txLess orders transaction ids: shorter first (numeric ids), then bytewise.
func txLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func pad14(d string) string {
	if len(d) > 14 {
		return d[:14]
	}
	return d + strings.Repeat("0", 14-len(d))
}

func noteSet(notes string) map[string]bool {
	out := map[string]bool{}
	for _, t := range strings.FieldsFunc(notes, func(r rune) bool { return r == ';' || r == ',' || r == ' ' }) {
		out[t] = true
	}
	return out
}

// parseAll turns rows into events. A cancel or correction row, and a row that cannot be
// read, is listed for review and never guessed. Rows of other asset categories are not
// part of the ledger.
func parseAll(trades []compute.LedgerTrade) (events []event, review []string) {
	for _, t := range trades {
		a := t.Attributes
		cat := strings.ToUpper(strings.TrimSpace(a["assetCategory"]))
		if cat != "OPT" && cat != "STK" {
			continue
		}
		notes := noteSet(a["notes"])
		tt := strings.ToLower(a["transactionType"])
		if strings.Contains(tt, "cancel") || strings.Contains(tt, "correct") || notes["Ca"] || notes["Co"] {
			review = append(review, t.TransactionID)
			continue
		}
		e, ok := parseEvent(t, cat, notes)
		if !ok {
			review = append(review, t.TransactionID)
			continue
		}
		events = append(events, e)
	}
	return events, review
}

func parseEvent(t compute.LedgerTrade, cat string, notes map[string]bool) (event, bool) {
	a := t.Attributes
	e := event{txID: t.TransactionID, currency: strings.TrimSpace(a["currency"])}
	if e.txID == "" || e.currency == "" {
		return e, false
	}
	setTimes(&e, a)
	if e.timeKey == "" {
		return e, false
	}
	var ok bool
	if e.qty, ok = dec(a["quantity"]); !ok || e.qty.Sign() == 0 {
		return e, false
	}
	if e.commission, ok = commissionOf(a["ibCommission"]); !ok {
		return e, false
	}
	if cat == "STK" {
		return parseStock(e, a)
	}
	e.underlying = strings.TrimSpace(a["underlyingSymbol"])
	if e.underlying == "" {
		e.underlying = strings.TrimSpace(a["symbol"])
	}
	e.putCall = strings.ToUpper(strings.TrimSpace(a["putCall"]))
	e.expiry = digits(a["expiry"])
	if e.mult, ok = dec(a["multiplier"]); !ok || e.mult.Sign() <= 0 || e.underlying == "" {
		return e, false
	}
	if e.strike, ok = dec(a["strike"]); !ok {
		return e, false
	}
	composite := strings.Join([]string{strings.TrimSpace(a["symbol"]), e.expiry, trimmed(e.strike), e.putCall}, ":")
	e.contract = composite
	e.markKeys = []string{composite}
	if c := strings.TrimSpace(a["conid"]); c != "" {
		e.contract = c
		e.markKeys = []string{c, composite}
	}
	if e.kind = optionKind(a["openCloseIndicator"], notes); e.kind == "" {
		return e, false
	}
	if strings.TrimSpace(a["proceeds"]) == "" && e.kind != kindOpen && e.kind != kindClose {
		e.proceeds = new(big.Rat)
		return e, true
	}
	e.proceeds, ok = dec(a["proceeds"])
	return e, ok
}

func optionKind(indicator string, notes map[string]bool) string {
	switch strings.ToUpper(strings.TrimSpace(indicator)) {
	case "O":
		return kindOpen
	case "C":
		switch {
		case notes["Ep"]:
			return kindExpire
		case notes["A"]:
			return kindAssign
		case notes["Ex"]:
			return kindExercise
		}
		return kindClose
	}
	return ""
}

func parseStock(e event, a map[string]string) (event, bool) {
	var ok bool
	e.underlying = strings.TrimSpace(a["symbol"])
	e.mult = big.NewRat(1, 1)
	e.kind = kindStockBuy
	if e.qty.Sign() < 0 {
		e.kind = kindStockSell
	}
	if e.price, ok = dec(a["tradePrice"]); !ok || e.underlying == "" {
		return e, false
	}
	e.proceeds, ok = dec(a["proceeds"])
	return e, ok
}

// setTimes sets the time key and the trade date; an unreadable time leaves timeKey empty.
func setTimes(e *event, a map[string]string) {
	e.timeKey = pad14(digits(a["tradeDate"]))
	for _, k := range []string{"orderTime", "dateTime"} {
		if d := digits(a[k]); len(d) >= 8 {
			e.timeKey = pad14(d)
		}
	}
	e.tradeDate = digits(a["tradeDate"])
	if len(e.tradeDate) >= 8 {
		e.tradeDate = e.tradeDate[:8]
	} else {
		e.tradeDate = e.timeKey[:8]
	}
	if strings.Trim(e.timeKey, "0") == "" {
		e.timeKey = ""
	}
}

// commissionOf reads ibCommission (negative in Flex) as a positive cost; empty is zero.
func commissionOf(s string) (*big.Rat, bool) {
	if strings.TrimSpace(s) == "" {
		return new(big.Rat), true
	}
	c, ok := dec(s)
	if !ok {
		return nil, false
	}
	return c.Abs(c), true
}
