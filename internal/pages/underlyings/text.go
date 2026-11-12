package underlyings

import (
	"fmt"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// Every visible string of the page is written here, in English.

const (
	titleText     = "Underlyings"
	readOnlyText  = "Editing is off: start ot serve with --owner-edit to change names."
	refillText    = "Days missed while a group is off cannot be refilled: the daily history of a skipped day is not fetched later."
	followedNone  = "Followed by nothing"
	followedPref  = "Followed by: "
	pendingText   = "Pending: resolves after the close"
	staleText     = "Pending: the resolve job has not settled this name; check ot scheduler"
	unresolvedPre = "Unresolved"
	noNamesText   = "0 names."
)

// statusText is the text of a status; reason is the reason of an unresolved name and stale is a pending name older
// than the settle limit.
func statusText(status, reason string, stale bool) string {
	switch status {
	case underlying.StatusPending:
		if stale {
			return staleText
		}
		return pendingText
	case underlying.StatusActive:
		return "Active"
	case underlying.StatusPaused:
		return "Paused"
	case underlying.StatusRetired:
		return "Retired"
	case underlying.StatusUnresolved:
		if reason != "" {
			return unresolvedPre + ": " + reason
		}
		return unresolvedPre
	}
	return status
}

func followedText(groups []string) string {
	if len(groups) == 0 {
		return followedNone
	}
	return followedPref + strings.Join(groups, ", ")
}

// sentenceText counts the names, then lists the statuses that occur in a fixed order.
func sentenceText(total int, counts map[string]int) string {
	if total == 0 {
		return noNamesText
	}
	noun := "names"
	if total == 1 {
		noun = "name"
	}
	var parts []string
	for _, s := range []string{
		underlying.StatusActive, underlying.StatusPaused, underlying.StatusPending,
		underlying.StatusUnresolved, underlying.StatusRetired,
	} {
		if n := counts[s]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, s))
		}
	}
	return fmt.Sprintf("%d %s: %s.", total, noun, strings.Join(parts, ", "))
}

// stopText is one line of the confirm block.
func stopText(s Stop) string {
	switch {
	case s.Stopped:
		return fmt.Sprintf("%s: stopped for %s", s.Group, s.Symbol)
	case s.Held:
		return fmt.Sprintf("%s: not stopped for %s (held)", s.Group, s.Symbol)
	}
	return fmt.Sprintf("%s: not stopped for %s", s.Group, s.Symbol)
}

func switchText(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// historyText says what one version wrote: the fields that differ from the next older version of the key in the
// list; with no older version listed, a version with a reason is told by its status and reason and any other is "added".
func historyText(h underlying.Row, older []underlying.Row) string {
	prefix := h.Symbol + ": "
	i := slices.IndexFunc(older, func(o underlying.Row) bool { return o.Key == h.Key })
	if i < 0 {
		if h.Reason != "" {
			return prefix + "status " + h.Status + ", reason " + h.Reason
		}
		return prefix + "added, status " + h.Status
	}
	o := older[i]
	var parts []string
	add := func(label, was, now string) {
		if was != now {
			parts = append(parts, label+" "+now)
		}
	}
	add("status", o.Status, h.Status)
	add("reason", o.Reason, h.Reason)
	add("name", o.Name, h.Name)
	add("issuer", o.Issuer, h.Issuer)
	add("options", switchText(o.Options), switchText(h.Options))
	add("fundamentals", switchText(o.Fundamentals), switchText(h.Fundamentals))
	add("buy_price", o.BuyPrice, h.BuyPrice)
	add("sale_price", o.SalePrice, h.SalePrice)
	add("report_date_override", o.ReportDateOverride, h.ReportDateOverride)
	add("note", o.Note, h.Note)
	if len(parts) == 0 {
		return prefix + "no change"
	}
	return prefix + strings.Join(parts, ", ")
}

const (
	heldReadText = "Held names could not be read from the positions; held names may be missing here"
	heldTextMax  = 200
)

// clipRunes cuts s to at most n runes; a negative n gives "".
func clipRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// heldText joins a head and a reason so the whole is at most heldTextMax runes; the reason is cut, not the head.
func heldText(head, reason string) string {
	return clipRunes(head+clipRunes(reason, heldTextMax-len([]rune(head))), heldTextMax)
}

func heldNoRowText(sym string) string { return clipRunes(sym+": held, not on the list", heldTextMax) }

func heldPendingText(sym string) string {
	return clipRunes(sym+": pending; the first option chain comes at least one session day after the resolve", heldTextMax)
}

func heldUnresolvedText(sym, reason string) string {
	return heldText(sym+": unresolved, no option chain: ", reason)
}

func heldRefusedText(sym, reason string) string {
	return heldText(sym+": held, not added to the list: ", reason)
}

// rowNote is the note column: the resolved text for a held-added row with a con_id, else the stored note.
func rowNote(r underlying.Row) string {
	if r.Note == underlying.NoteHeld && r.ConID > 0 {
		return fmt.Sprintf("Resolved as %s stock %s, con_id %d; check the con_id", r.Market, r.Symbol, r.ConID)
	}
	return r.Note
}
