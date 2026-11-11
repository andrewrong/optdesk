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
