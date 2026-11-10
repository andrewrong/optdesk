package positionmarks

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

// MarkOf applies Decision 2 of docs/design/position-marks.md: the mid when bid and ask are both above zero
// and ask is not below bid (basis mid), else the last price above zero (BasisLast), else the close above
// zero (BasisClose), else an empty price with BasisNone. Prices are decimal text.
func MarkOf(q broker.Quote) (price, basis string) {
	if positive(q.Bid) && positive(q.Ask) && *q.Ask >= *q.Bid {
		return mid(*q.Bid, *q.Ask), mark.BasisMid
	}
	if positive(q.Last) {
		return dec(*q.Last), mark.BasisLast
	}
	if positive(q.Close) {
		return dec(*q.Close), mark.BasisClose
	}
	return "", mark.BasisNone
}

func positive(v *float64) bool { return v != nil && *v > 0 }

// dec is the shortest decimal text of v, never an exponent.
func dec(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// mid is (a+b)/2 as exact decimal text: the sum of two decimals of n places has at most n+1 places.
func mid(a, b float64) string {
	sa, sb := dec(a), dec(b)
	ra, _ := new(big.Rat).SetString(sa)
	rb, _ := new(big.Rat).SetString(sb)
	sum := new(big.Rat).Add(ra, rb)
	sum.Quo(sum, big.NewRat(2, 1))
	places := max(places(sa), places(sb)) + 1
	out := sum.FloatString(places)
	if strings.Contains(out, ".") {
		out = strings.TrimRight(strings.TrimRight(out, "0"), ".")
	}
	return out
}

func places(s string) int {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}
