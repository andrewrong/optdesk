package optionnotice

import (
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Severities of a notice.
const (
	sevInfo = "info"
	sevWarn = "warn"
	sevAct  = "act"
)

type rule struct{}

// New returns the rule.
func New() compute.OptionNotices { return rule{} }

// Evaluate returns the open notices of one alias ordered by severity (act, warn, info), then
// expiry (none last), then contract, plus the assignment cash by date. A row whose text does
// not parse is skipped; it never raises a notice and never panics.
func (rule) Evaluate(in compute.NoticeInput, p compute.NoticeParams) compute.NoticeResult {
	day, dayOK := parseDay(string(in.Day))
	cur := parseOptions(in.Positions)
	var ns []compute.Notice
	if dayOK {
		ns = append(ns, expiryNotices(cur, day, p.ExpiryDays)...)
	}
	ns = append(ns, guardNotices(cur, in.Guards)...)
	if dayOK {
		ns = append(ns, earningsNotices(cur, in.Earnings, day)...)
	}
	ns = append(ns, assignmentNotices(in)...)
	cushion, cn := cushionNotice(in.Values, p)
	ns = append(ns, cn...)
	ns = append(ns, staleNotice(in, day, dayOK)...)
	order(ns)
	return compute.NoticeResult{RuleVersion: ruleVersion, Notices: ns, CashByDate: cashByDate(cur, day, dayOK), Cushion: cushion}
}

// held is one parsed option position.
type held struct {
	symbol   string
	expiry   string // YYYY-MM-DD
	expDays  int    // days since the epoch
	strike   *big.Rat
	mult     *big.Rat
	qty      *big.Rat
	right    string // P or C
	currency string
}

func (h held) short() bool { return h.qty.Sign() < 0 }

func (h held) contract() string {
	return h.symbol + " " + h.expiry + " " + formatTrim(h.strike) + " " + h.right
}

// sameContract reports whether two positions are the same contract.
func (h held) sameContract(o held) bool {
	return h.symbol == o.symbol && h.expiry == o.expiry && h.right == o.right && h.strike.Cmp(o.strike) == 0
}

// absQty is the contract count of a position.
func (h held) absQty() *big.Rat { return new(big.Rat).Abs(h.qty) }

// exposure is strike x multiplier x contracts, the cash of one assignment.
func (h held) exposure() *big.Rat {
	v := new(big.Rat).Mul(h.strike, h.mult)
	return v.Mul(v, h.absQty())
}

// parseOptions keeps the option rows whose every figure parses; a stock row or a bad row is dropped.
func parseOptions(ps []compute.NoticePosition) []held {
	var out []held
	for _, p := range ps {
		if h, ok := parseOption(p); ok {
			out = append(out, h)
		}
	}
	return out
}

func parseOption(p compute.NoticePosition) (held, bool) {
	if !strings.EqualFold(p.SecType, "OPT") || p.Symbol == "" {
		return held{}, false
	}
	d, ok := parseDay(p.Expiry)
	if !ok {
		return held{}, false
	}
	strike, ok1 := parseDec(p.Strike)
	mult, ok2 := parseDec(p.Multiplier)
	qty, ok3 := parseDec(p.Quantity)
	right := rightOf(p.Right)
	if !ok1 || !ok2 || !ok3 || right == "" || strike.Sign() <= 0 || mult.Sign() <= 0 || qty.Sign() == 0 {
		return held{}, false
	}
	return held{symbol: p.Symbol, expiry: dayText(d), expDays: d, strike: strike, mult: mult, qty: qty, right: right, currency: p.Currency}, true
}

func rightOf(s string) string {
	switch strings.ToUpper(s) {
	case "P", "PUT":
		return "P"
	case "C", "CALL":
		return "C"
	}
	return ""
}

func rank(sev string) int {
	switch sev {
	case sevAct:
		return 0
	case sevWarn:
		return 1
	}
	return 2
}

// order sorts by severity, then expiry (a notice with none last), then contract, then code.
func order(ns []compute.Notice) {
	sort.SliceStable(ns, func(i, j int) bool {
		a, b := ns[i], ns[j]
		if ra, rb := rank(a.Severity), rank(b.Severity); ra != rb {
			return ra < rb
		}
		if a.Expiry != b.Expiry {
			if a.Expiry == "" || b.Expiry == "" {
				return b.Expiry == ""
			}
			return a.Expiry < b.Expiry
		}
		if a.Contract != b.Contract {
			return a.Contract < b.Contract
		}
		return a.Code < b.Code
	})
}

func newNotice(code, sev string, h held, facts ...compute.NoticeFact) compute.Notice {
	return compute.Notice{Code: code, Severity: sev, Symbol: h.symbol, Contract: h.contract(),
		Expiry: contracts.Day(h.expiry), Facts: facts}
}

func fact(label, value string) compute.NoticeFact {
	return compute.NoticeFact{Label: label, Value: value}
}

func itoa(n int) string { return strconv.Itoa(n) }

// parseDec reads a decimal text (optional sign, digits, at most one dot) as an exact
// rational; an empty or malformed text, an exponent or a fraction reports false.
func parseDec(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	body := strings.TrimLeft(s, "+-")
	if len(s)-len(body) > 1 || body == "" || body == "." || strings.Count(body, ".") > 1 {
		return nil, false
	}
	for _, c := range body {
		if (c < '0' || c > '9') && c != '.' {
			return nil, false
		}
	}
	return new(big.Rat).SetString(s)
}

// formatFixed writes r with exactly places decimals, rounding half to even.
func formatFixed(r *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	abs := new(big.Rat).Abs(r)
	num := new(big.Int).Mul(abs.Num(), scale)
	q, rem := new(big.Int).QuoRem(num, abs.Denom(), new(big.Int))
	twice := new(big.Int).Lsh(rem, 1)
	if c := twice.Cmp(abs.Denom()); c > 0 || (c == 0 && q.Bit(0) == 1) {
		q.Add(q, big.NewInt(1))
	}
	digits := q.String()
	if len(digits) <= places {
		digits = strings.Repeat("0", places-len(digits)+1) + digits
	}
	out := digits
	if places > 0 {
		out = digits[:len(digits)-places] + "." + digits[len(digits)-places:]
	}
	if r.Sign() < 0 && q.Sign() != 0 {
		out = "-" + out
	}
	return out
}

// formatTrim writes r to at most 6 places without trailing zeros.
func formatTrim(r *big.Rat) string {
	s := formatFixed(r, 6)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	return s
}

// parseDay reads YYYY-MM-DD (or YYYYMMDD) as days since 1970-01-01; no time package is used.
func parseDay(s string) (int, bool) {
	if len(s) == 8 {
		s = s[:4] + "-" + s[4:6] + "-" + s[6:]
	}
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return 0, false
	}
	y, e1 := strconv.Atoi(s[:4])
	m, e2 := strconv.Atoi(s[5:7])
	d, e3 := strconv.Atoi(s[8:])
	if e1 != nil || e2 != nil || e3 != nil || m < 1 || m > 12 || d < 1 || d > monthDays(y, m) {
		return 0, false
	}
	return daysFromCivil(y, m, d), true
}

func monthDays(y, m int) int {
	switch m {
	case 2:
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// daysFromCivil is Howard Hinnant's algorithm for the proleptic Gregorian calendar.
func daysFromCivil(y, m, d int) int {
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 {
		era = (y - 399) / 400
	}
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// dayText writes days since the epoch as YYYY-MM-DD.
func dayText(z int) string {
	z += 719468
	era := z / 146097
	if z < 0 {
		era = (z - 146096) / 146097
	}
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := doy - (153*mp+2)/5 + 1
	m := mp + 3
	if m > 12 {
		m -= 12
	}
	if m <= 2 {
		y++
	}
	return pad(y, 4) + "-" + pad(m, 2) + "-" + pad(d, 2)
}

func pad(n, w int) string {
	s := strconv.Itoa(n)
	for len(s) < w {
		s = "0" + s
	}
	return s
}

// sessionDaysBetween counts the weekdays in (from, to]; holidays are not known here.
func sessionDaysBetween(from, to int) int {
	if to-from > 3650 {
		return to - from
	}
	n := 0
	for d := from + 1; d <= to; d++ {
		if w := ((d+4)%7 + 7) % 7; w != 0 && w != 6 { // day 0 is a Thursday
			n++
		}
	}
	return n
}
