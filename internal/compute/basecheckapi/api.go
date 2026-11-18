// Package basecheckapi declares the verdicts of the base data page and the function that computes
// them, and the one table of its rules and rows. Declarations only: no function body; the only
// variables with a value are Rules and Rows. The implementation is internal/compute/basecheck.
// Design: docs/design/base-data-page.md#Interface (entry 2).
package basecheckapi

import "github.com/andrewrong/optdesk/internal/contracts"

// Level is the verdict of a check, a row or the page.
type Level string

// The four levels. A row takes the worst of its checks in the order fail, unknown, warn, pass.
const (
	Pass    Level = "pass"
	Warn    Level = "warn"
	Fail    Level = "fail"
	Unknown Level = "unknown" // no fact for the check, or the view that holds it failed
)

// Fact is one row of a store view: one per row id, rule and subject. It is a copy of
// internal/store/basefact.Fact (compute imports no store); a field change is made in both and in
// the converter of the page package. Dataset is the row id of Rows; Subject is empty for a rule
// of the whole row; Day and Value are set for the rule newest; rule rows uses N.
type Fact struct {
	Dataset, Rule, Subject string
	N, Bad                 int64
	Day                    contracts.Day
	Value                  string
}

// Cell is the sum of the coverage ledger cells of the datasets of one row on one day. Dataset is
// the row id.
type Cell struct {
	Day                    contracts.Day
	Dataset                string
	Keys, Missing, Expired int
}

// Input is everything the verdicts need. NotRead are the row ids of a view that failed: their
// rows are unknown. Days maps a calendar id (XNYS, XTKS, both always present) to its last closed
// session; Sessions maps it to its session days, ascending.
type Input struct {
	Facts    []Fact
	NotRead  []string
	Cells    []Cell
	Days     map[string]contracts.Day
	Sessions map[string][]contracts.Day
}

// Check is the result of one rule on one row. Share, WarnAt and FailAt are decimal text written
// by compute; ThresholdSource is "def", "trial" or a source name.
type Check struct {
	Rule            string
	Level           Level
	N, Bad          int64
	Share           string
	Example         string
	WarnAt, FailAt  string
	ThresholdSource string
}

// Row is the verdict of one dataset row. AgeDays is calendar days, AgeSessions is sessions of the
// calendar of the subject.
type Row struct {
	Dataset, Group                     string
	Level                              Level
	Newest, NewestValue, NewestSubject string
	AgeDays, AgeSessions               int
	Checks                             []Check
}

// Output is the verdict of the page: the worst level of its rows, a count per level, the rows in
// the order of Rows.
type Output struct {
	Level  Level
	Counts map[Level]int
	Rows   []Row
}

// Func turns the facts, the ledger cells and the sessions into the verdicts. The one
// implementation is internal/compute/basecheck.
type Func func(Input) Output

// RuleInfo names one rule of one row. Source is sql (a view emits it), ledger (from the coverage
// cells), derived (compute works it out) or note (shown on the row, never judged).
type RuleInfo struct {
	Row, Rule, Source string
}

// RowInfo is one row of the page. ViewGroup is the store view that holds it: bars, option, other
// or account. LedgerDatasets are the coverage ledger dataset ids summed into the row. Calendars
// maps a subject to its calendar id; empty means calendar days.
type RowInfo struct {
	ID, Group, ViewGroup string
	LedgerDatasets       []string
	Calendars            map[string]string
}

// Rules is the table of rules: ids and source marks only, no threshold. The two generic ledger
// rules and the generic sql rules apply to every row.
var Rules = []RuleInfo{
	{"bars", "rows", "sql"},
	{"bars", "newest", "sql"},
	{"bars", "coverage_gap", "ledger"},
	{"bars", "coverage_latest", "ledger"},
	{"bars", "ohlc_order", "sql"},
	{"bars", "bar_jump", "sql"},
	{"bars", "age", "derived"},
	{"quotes", "rows", "sql"},
	{"quotes", "newest", "sql"},
	{"quotes", "coverage_gap", "ledger"},
	{"quotes", "coverage_latest", "ledger"},
	{"quotes", "crossed", "sql"},
	{"quotes", "empty", "sql"},
	{"quotes", "errors", "sql"},
	{"quotes", "non_live", "sql"},
	{"quotes", "age", "derived"},
	{"chains", "rows", "sql"},
	{"chains", "newest", "sql"},
	{"chains", "coverage_gap", "ledger"},
	{"chains", "coverage_latest", "ledger"},
	{"chains", "crossed", "sql"},
	{"chains", "bad_iv", "sql"},
	{"chains", "empty", "sql"},
	{"chains", "errors", "sql"},
	{"chains", "non_live", "sql"},
	{"chains", "age", "derived"},
	{"open", "rows", "sql"},
	{"open", "newest", "sql"},
	{"open", "coverage_gap", "ledger"},
	{"open", "coverage_latest", "ledger"},
	{"open", "crossed", "sql"},
	{"open", "bad_iv", "sql"},
	{"open", "empty", "sql"},
	{"open", "errors", "sql"},
	{"open", "non_live", "sql"},
	{"open", "age", "derived"},
	{"contracts", "rows", "sql"},
	{"contracts", "newest", "sql"},
	{"contracts", "coverage_gap", "ledger"},
	{"contracts", "coverage_latest", "ledger"},
	{"contracts", "age", "derived"},
	{"rates", "rows", "sql"},
	{"rates", "newest", "sql"},
	{"rates", "coverage_gap", "ledger"},
	{"rates", "coverage_latest", "ledger"},
	{"rates", "series_age", "sql"},
	{"rates", "age", "derived"},
	{"fx", "rows", "sql"},
	{"fx", "newest", "sql"},
	{"fx", "coverage_gap", "ledger"},
	{"fx", "coverage_latest", "ledger"},
	{"fx", "fx_band", "sql"},
	{"fx", "fx_jump", "sql"},
	{"fx", "age", "derived"},
	{"events", "rows", "sql"},
	{"events", "newest", "sql"},
	{"events", "coverage_gap", "ledger"},
	{"events", "coverage_latest", "ledger"},
	{"events", "earnings_past", "sql"},
	{"events", "dividend_past", "sql"},
	{"events", "macro_exhausted", "sql"},
	{"events", "age", "derived"},
	{"calendar", "rows", "sql"},
	{"calendar", "newest", "sql"},
	{"calendar", "coverage_gap", "ledger"},
	{"calendar", "coverage_latest", "ledger"},
	{"calendar", "calendar_horizon", "sql"},
	{"calendar", "age", "derived"},
	{"facts", "rows", "sql"},
	{"facts", "newest", "sql"},
	{"facts", "coverage_gap", "ledger"},
	{"facts", "coverage_latest", "ledger"},
	{"facts", "period_order", "sql"},
	{"facts", "subject_stale", "sql"},
	{"facts", "age", "derived"},
	{"account", "rows", "sql"},
	{"account", "newest", "sql"},
	{"account", "coverage_gap", "ledger"},
	{"account", "coverage_latest", "ledger"},
	{"account", "capture_age", "derived"},
	{"account", "no_netliq", "sql"},
	{"account", "position_expired", "sql"},
	{"flex", "rows", "sql"},
	{"flex", "newest", "sql"},
	{"flex", "coverage_gap", "ledger"},
	{"flex", "coverage_latest", "ledger"},
	{"flex", "flex_age", "derived"},
	{"flex", "flex_section_absent", "note"},
}

// Rows is the table of the rows of the page, in page order.
var Rows = []RowInfo{
	{"bars", "Market data", "bars", []string{"obs.price_bar"}, map[string]string{"USD": "XNYS", "JPY": "XTKS"}},
	{"quotes", "Market data", "option", []string{"obs.quote_snapshot"}, map[string]string{"USD": "XNYS"}},
	{"chains", "Market data", "option", []string{"obs.option_eod", "obs.option_eod_jp", "obs.option_eod_index"}, map[string]string{"USD": "XNYS", "JPY": "XTKS"}},
	{"open", "Market data", "option", []string{"obs.option_open", "obs.option_open.jp"}, map[string]string{"USD": "XNYS", "JPY": "XTKS"}},
	{"contracts", "Market data", "other", []string{"master.option_list", "master.option_list.jp", "master.option_contract", "master.option_contract.jp", "master.option_contract.gap"}, nil},
	{"rates", "Rates and events", "other", []string{"obs.series_point"}, nil},
	{"fx", "Rates and events", "other", []string{"obs.fx_rate"}, map[string]string{"USDJPY": "XNYS"}},
	{"events", "Rates and events", "other", []string{"obs.earnings_date", "obs.dividend_expectation", "obs.macro_event"}, nil},
	{"calendar", "Rates and events", "other", []string{"master.session_day", "master.session_hours_notice"}, nil},
	{"facts", "Company facts", "other", []string{"obs.fundamental_fact"}, nil},
	{"account", "Account facts", "account", []string{"account.capture", "account.position_mark"}, map[string]string{"USD": "XNYS"}},
	{"flex", "Account facts", "account", []string{"optionledger.flex_trade_row"}, nil},
}
