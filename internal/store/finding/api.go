// Package finding declares the findings of the daily data check: one Finding is a rule, a dataset, a day, a
// severity and counts (docs/design/data-check.md, Interface, entry 1). It holds declarations only: the
// implementation is internal/store/findingpg. It is a package of its own because internal/store/api.go is at
// its ceiling.
package finding

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// DSNEnvDataCheck names the line that holds the DSN of the role ot_w_datacheck.
const DSNEnvDataCheck = "OT_PG_DSN_DATACHECK"

// Severity says how bad a finding is.
type Severity string

const (
	// SeverityError: wrong data stored as good.
	SeverityError Severity = "error"
	// SeverityWarn: a gap.
	SeverityWarn Severity = "warn"
	// SeverityUnknown: the rule could not decide.
	SeverityUnknown Severity = "unknown"
)

// Rule ids: fixed strings; a new rule adds one and never changes one.
const (
	// RuleReadingRepeat: the readings of a day repeat the values of the previous day.
	RuleReadingRepeat = "reading_repeat"
	// RuleOpenLateQuote: option open quotes received after their session day.
	RuleOpenLateQuote = "option_open_late_quote"
	// RuleCandidatesStale: the candidates are older than the newest stored setting.
	RuleCandidatesStale = "candidates_older_than_settings"
	// RuleCaptureGap: a ledger cell with missing or expired keys.
	RuleCaptureGap = "capture_gap"
	// RulePlanAbsent: a dataset with no cell for a session day.
	RulePlanAbsent = "plan_absent"
	// RuleGapUnclosed: a job gap still open after the grace.
	RuleGapUnclosed = "gap_unclosed"
	// RuleCandidateRuleVersion: the newest candidate run carries a rule version other than the expected one.
	RuleCandidateRuleVersion = "candidate_rule_version"
	// RuleEarningsCoverage: the earnings check of a market found no dates, or fewer than the day before.
	RuleEarningsCoverage = "earnings_coverage"
	// RuleBarOHLCShape: trades bars of a day that break low <= open, close <= high.
	RuleBarOHLCShape = "bar_ohlc_shape"
)

// Count is one piece of evidence: a name from a fixed vocabulary and a count. The names are ok, same, rows,
// late, hours_behind, keys, missing, expired, cells, age_hours, read_failed, names, listed, errors,
// listed_before and bad.
type Count struct {
	Name string
	N    int
}

// Finding is one problem: a rule, a dataset (or a reading name), a day, a severity and counts. It holds no
// free text. A finding is keyed by (Rule, Dataset, Day, Subject), not by its counts.
type Finding struct {
	Rule, Dataset string
	Day           contracts.Day
	Subject       string // empty in this version
	Severity      Severity
	Evidence      []Count
	IsNew         bool // set by the writer, ignored on input
}

// Run is one run of the check with its findings.
type Run struct {
	RunID                    int64
	CheckDay                 contracts.Day
	StartedAt, FinishedAt    time.Time
	RulesTotal, RulesUnknown int
	Findings                 []Finding
}

// Writer stores one run and says which findings are new (a finding is new when no finding of the same key
// is in an earlier run that has a notice). Insert-only.
type Writer interface {
	// InsertRun stores r; the returned Run has RunID and IsNew set.
	InsertRun(ctx context.Context, r Run) (Run, error)
	// MarkNoticed records that the message of runID was delivered; called after the send, never before.
	MarkNoticed(ctx context.Context, runID int64) error
	Close() error
}

// Reader reads the newest run and its findings as the page role (ot_ro_pages). Slices are never nil; no run
// gives a zero Run.
type Reader interface {
	Latest(ctx context.Context) (Run, error)
	Close() error
}

// Probe answers the questions of the rules with counts, never rows. Window days are inclusive.
type Probe interface {
	// RepeatedReadings counts, per name and day, the ok readings and those equal to the previous reading.
	RepeatedReadings(ctx context.Context, names []string, from, to contracts.Day) ([]ReadingRepeat, error)
	// LateOpenQuotes counts, per day and currency, the rows and those received after the session day.
	LateOpenQuotes(ctx context.Context, from, to contracts.Day) ([]LateQuotes, error)
	// NewestCandidateRuns returns the newest candidate run per kind of the newest day that has rows.
	NewestCandidateRuns(ctx context.Context, from, to contracts.Day) ([]CandidateRun, error)
	// SettingsChangedAt returns the newest set_at of the stored settings; false: no setting stored.
	SettingsChangedAt(ctx context.Context) (time.Time, bool, error)
	// OpenGaps returns the gaps with no cleared_at.
	OpenGaps(ctx context.Context) ([]OpenGap, error)
	// EarningsChecks counts, per market and day, the earnings check rows, those with status listed and those with
	// status error; a day with no rows gives no answer.
	EarningsChecks(ctx context.Context, from, to contracts.Day) ([]EarningsCheck, error)
	// BarShapes counts, per day, the trades bars and those that break low <= open, close <= high.
	BarShapes(ctx context.Context, from, to contracts.Day) ([]BarShape, error)
	Close() error
}

// ReadingRepeat is the answer of RepeatedReadings for one name and day.
type ReadingRepeat struct {
	Name     string
	Day      contracts.Day
	OK, Same int
}

// LateQuotes is the answer of LateOpenQuotes for one day and currency.
type LateQuotes struct {
	Day        contracts.Day
	Currency   string
	Rows, Late int
}

// CandidateRun is the answer of NewestCandidateRuns for one kind.
type CandidateRun struct {
	Day        contracts.Day
	Kind       string
	Rows       int
	ComputedAt time.Time
	// RuleVersion is the rule_version of the rows of the run; empty when the rows disagree.
	RuleVersion string
}

// EarningsCheck is the answer of EarningsChecks for one market ("US" or "JP") and day: the check rows, those
// with status listed, and those with status error.
type EarningsCheck struct {
	Market                string
	Day                   contracts.Day
	Names, Listed, Errors int
}

// BarShape is the answer of BarShapes for one day: the trades bars and those that break low <= open, close <= high.
type BarShape struct {
	Day       contracts.Day
	Rows, Bad int
}

// OpenGap is one gap with no cleared_at.
type OpenGap struct {
	JobID, Dataset string
	Day            contracts.Day
	Reason         string
	FirstSeenAt    time.Time
}

// OpenWriterFunc opens the writer with the line DSNEnvDataCheck (internal/store/findingpg).
type OpenWriterFunc func(ctx context.Context, dsn string) (Writer, error)

// OpenReaderFunc opens the reader with the line DSNEnvPages (internal/store/findingpg).
type OpenReaderFunc func(ctx context.Context, dsn string) (Reader, error)

// OpenProbeFunc opens the probe with the line DSNEnvDataCheck (internal/store/findingpg).
type OpenProbeFunc func(ctx context.Context, dsn string) (Probe, error)
