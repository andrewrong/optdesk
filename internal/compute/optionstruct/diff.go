package optionstruct

import (
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

// maxCaptureAgeSessions is the age, in session days, past which the capture is not compared
// (Decision 2).
const maxCaptureAgeSessions = 3

// The reasons a capture is not compared.
const (
	skipNoCapture = "no_capture"
	skipOld       = "capture_older_than_3_sessions"
)

// captureDiff compares the option rows of the newest capture with the open positions of the
// ledger by contract: CaptureOnly are held and not open in the ledger, LedgerOnly the reverse.
// A capture older than 3 session days, or none, gives no diff and a reason.
func captureDiff(in compute.StructureInput, units []unit) compute.CaptureDiff {
	if in.HeldDay == "" {
		return compute.CaptureDiff{Skipped: skipNoCapture}
	}
	age := 0
	for _, d := range in.SessionDays {
		if d > in.HeldDay && (in.AsOf == "" || d <= in.AsOf) {
			age++
		}
	}
	if age > maxCaptureAgeSessions {
		return compute.CaptureDiff{Skipped: skipOld}
	}
	held := map[string]bool{}
	for _, h := range in.Held {
		if q, ok := parseDec(h.Quantity); ok && q.Sign() != 0 {
			held[contractKey(h.Underlying, h.Expiry, h.Strike, h.Right)] = true
		}
	}
	open := map[string]bool{}
	for _, u := range units {
		for _, l := range u.legs {
			open[contractKey(l.p.Underlying, l.p.Expiry, l.p.Strike, l.p.Right)] = true
		}
	}
	return compute.CaptureDiff{CaptureOnly: missingFrom(held, open), LedgerOnly: missingFrom(open, held)}
}

// missingFrom lists, in order, the keys of a that b does not hold.
func missingFrom(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// contractKey names a contract with its strike written without trailing zeros.
func contractKey(underlying, expiry, strike, right string) string {
	if r, ok := parseDec(strike); ok {
		strike = r.FloatString(6)
		strike = strings.TrimSuffix(strings.TrimRight(strike, "0"), ".")
	}
	return underlying + " " + expiry + " " + strike + " " + right
}
