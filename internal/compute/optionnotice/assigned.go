package optionnotice

import (
	"math/big"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

// assignmentNotices raises assignment_seen (act) for a short put of the previous captured
// day (expiry on or after that day) that expired by the capture day, is no longer held, while the stock of its symbol
// rose. The cash is strike x multiplier x contracts, fixed to two places.
func assignmentNotices(in compute.NoticeInput) []compute.Notice {
	prevDay, ok1 := parseDay(string(in.PrevDay))
	capDay, ok2 := parseDay(string(in.CaptureDay))
	if !ok1 || !ok2 {
		return nil
	}
	prev, cur := parseOptions(in.PrevPositions), parseOptions(in.Positions)
	var out []compute.Notice
	for _, h := range prev {
		if !h.short() || h.right != "P" || h.expDays < prevDay || h.expDays > capDay || stillHeld(cur, h) {
			continue
		}
		rise := stockQty(in.Positions, h.symbol)
		rise.Sub(rise, stockQty(in.PrevPositions, h.symbol))
		if rise.Sign() <= 0 {
			continue
		}
		out = append(out, newNotice("assignment_seen", sevAct, h,
			fact("cash", formatFixed(h.exposure(), 2)), fact("contracts", formatTrim(h.absQty())),
			fact("shares", formatTrim(rise))))
	}
	return out
}

func stillHeld(cur []held, h held) bool {
	for _, c := range cur {
		if c.sameContract(h) {
			return true
		}
	}
	return false
}

// stockQty sums the stock shares of a symbol; a bad quantity counts as none.
func stockQty(ps []compute.NoticePosition, symbol string) *big.Rat {
	sum := new(big.Rat)
	for _, p := range ps {
		if p.Symbol != symbol || !strings.EqualFold(p.SecType, "STK") {
			continue
		}
		if q, ok := parseDec(p.Quantity); ok {
			sum.Add(sum, q)
		}
	}
	return sum
}
