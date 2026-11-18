// Package basecheck turns counts, thresholds and sessions into verdicts. This file holds the
// thresholds, the only place a threshold number is written; tuning after the first live read
// changes this file only (docs/design/base-data-page.md#7).
package basecheck

import (
	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// HistoryStart is the first day the coverage ledger recorded presence. Days before it hold
// expected keys but no record of what was stored, so coverage_gap does not count them.
const HistoryStart contracts.Day = "2026-11-07"

// Kind says how a Limit reads its Value.
type Kind string

const (
	// None: no limit. A fail of None means the rule is at most warn.
	None Kind = ""
	// Any: reached when Bad >= 1.
	Any Kind = "any"
	// Empty: reached when N is 0.
	Empty Kind = "empty"
	// Count: reached when Bad >= Value (decimal text).
	Count Kind = "count"
	// Share: reached when Bad / N >= Value (decimal text).
	Share Kind = "share"
	// Days: an age in calendar days, reached at Value.
	Days Kind = "days"
	// Sessions: an age in sessions of the subject's calendar, reached at Value.
	Sessions Kind = "sessions"
)

// Limit is one threshold. Value is decimal text, empty for None, Any and Empty.
type Limit struct {
	Kind  Kind
	Value string
}

// Threshold is the (warn at, fail at) pair of one rule and where it comes from: "def" for a
// definitional limit, "trial" for our choice, tuned on live numbers.
type Threshold struct {
	Warn, Fail Limit
	Source     string
}

// Thresholds is the table of thresholds keyed by "<row>.<rule>". Every rule of
// basecheckapi.Rules marked sql, ledger or derived has an entry; a note rule has none.
var Thresholds = buildThresholds()

func buildThresholds() map[string]Threshold {
	anyBad := Limit{Kind: Any}
	none := Limit{}
	share := func(v string) Limit { return Limit{Share, v} }
	days := func(v string) Limit { return Limit{Days, v} }
	sessions := func(v string) Limit { return Limit{Sessions, v} }
	pair := func(w, f Limit, src string) Threshold { return Threshold{w, f, src} }

	t := map[string]Threshold{
		"bars.ohlc_order": pair(anyBad, anyBad, "def"),
		"bars.bar_jump":   pair(anyBad, none, "trial"),

		"quotes.crossed":  pair(share("0.001"), share("0.02"), "trial"),
		"quotes.empty":    pair(share("0.1"), share("0.4"), "trial"),
		"quotes.errors":   pair(share("0.05"), share("0.3"), "trial"),
		"quotes.non_live": pair(share("0.1"), share("0.5"), "trial"),

		"rates.series_age": pair(anyBad, share("0.1"), "trial"),

		"fx.fx_band": pair(anyBad, anyBad, "trial"),
		"fx.fx_jump": pair(anyBad, anyBad, "trial"),

		"events.earnings_past":   pair(anyBad, share("0.1"), "trial"),
		"events.dividend_past":   pair(anyBad, share("0.1"), "trial"),
		"events.macro_exhausted": pair(anyBad, anyBad, "trial"),

		"calendar.calendar_horizon": pair(anyBad, anyBad, "trial"),

		"facts.period_order":  pair(anyBad, anyBad, "def"),
		"facts.subject_stale": pair(share("0.2"), share("0.5"), "trial"),

		"account.capture_age":      pair(sessions("1"), sessions("3"), "trial"),
		"account.no_netliq":        pair(anyBad, anyBad, "def"),
		"account.position_expired": pair(anyBad, anyBad, "def"),

		"flex.flex_age": pair(days("3"), days("7"), "trial"),

		// Rows with no subject calendar count calendar days (Decision 6).
		"rates.age":    pair(days("1"), days("2"), "trial"),
		"calendar.age": pair(days("1"), days("2"), "trial"),

		"contracts.age": pair(days("7"), days("30"), "trial"),
		"events.age":    pair(days("3"), days("10"), "trial"),
		"facts.age":     pair(days("30"), days("100"), "trial"),
	}

	// chains and open read as each other: the same five option rules, and bad_iv.
	for _, row := range []string{"chains", "open"} {
		t[row+".crossed"] = pair(share("0.01"), share("0.05"), "trial")
		t[row+".bad_iv"] = pair(anyBad, share("0.01"), "trial")
		t[row+".empty"] = pair(share("0.5"), share("0.9"), "trial")
		t[row+".errors"] = pair(share("0.05"), share("0.3"), "trial")
		t[row+".non_live"] = pair(share("0.1"), share("0.5"), "trial")
	}

	// The generic rules apply to every row.
	for _, row := range basecheckapi.Rows {
		t[row.ID+".rows"] = pair(none, Limit{Kind: Empty}, "def")
		t[row.ID+".newest"] = pair(none, none, "def") // shown, not judged
		t[row.ID+".coverage_gap"] = pair(anyBad, Limit{Count, "5"}, "trial")
		t[row.ID+".coverage_latest"] = pair(anyBad, share("0.05"), "trial")
	}
	// An age rule with no limit of its own counts sessions (Decision 6: 1 warn, 2 fail).
	for _, r := range basecheckapi.Rules {
		id := r.Row + "." + r.Rule
		if _, named := t[id]; !named && r.Rule == "age" {
			t[id] = pair(sessions("1"), sessions("2"), "trial")
		}
	}
	return t
}
