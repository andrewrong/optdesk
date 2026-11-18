// Package valueimportapi declares the valuation import: the types, the constants and the Importer
// that reads the owner's export and gives a plan. Decimal text everywhere, no float, no clock,
// no I/O. Source: docs/design/valuation-import.md, entry 1.
package valueimportapi

import (
	"errors"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	// MaxFileBytes is the largest file the import reads (1 MiB).
	MaxFileBytes = 1 << 20
	// MaxCompanies is the most companies a file may hold.
	MaxCompanies = 500
	// RoundTolerance is the relative rounding of the buy-zone check, as decimal text.
	RoundTolerance = "0.005"
	// RoundToleranceAbs is the absolute rounding of the check; the larger of the two applies.
	RoundToleranceAbs = "0.5"
)

// The actions of a plan.
const (
	ActionInsert      = "insert"
	ActionSame        = "same"
	ActionOlder       = "older"
	ActionRefused     = "refused"
	ActionProfileOnly = "profile_only"
)

// StoredUnderlying is one current settings.underlying row.
type StoredUnderlying struct{ Key, Symbol, Market string }

// StoredProfile is one stored company profile.
type StoredProfile struct{ Key, Name, Symbol, Currency, RMarket, Pool string }

// StoredCurrent is the current valuation of a key.
type StoredCurrent struct {
	Key, V, P   string
	VersionDate contracts.Day
}

// StoredR is the newest stored R version.
type StoredR struct {
	US, JP, HKCN  string
	EffectiveDate contracts.Day
	AISwitch      bool
	AIExpiry      string
}

// Stored is the state of the store the plan is compared with. Latest is the latest stored
// 10-year yield as text, or empty.
type Stored struct {
	Underlying []StoredUnderlying
	Profiles   []StoredProfile
	Current    []StoredCurrent
	R          *StoredR
	Margin     []valuelinesapi.MarginRow
	MarginDate contracts.Day
	Latest     string
}

// Pick is the key and R market the owner chose for an unmatched name.
type Pick struct{ Key, RMarket string }

// Input is what Plan needs. Picks is by ticker and empty in a preview.
type Input struct {
	File   []byte
	Stored Stored
	Picks  map[string]Pick
	Calc   valuelinesapi.Calculator
}

// RPlan is the plan of the R version.
type RPlan struct {
	Action             string
	US, JP, HKCN, Note string
	AISwitch           bool
	AIExpiry, Problem  string
}

// MarginPlan is the plan of the margin table.
type MarginPlan struct {
	Action        string
	Rows          []valuelinesapi.MarginRow
	EffectiveDate contracts.Day
}

// RowPlan is the plan of one company of the file.
type RowPlan struct {
	Key, Ticker, Name, Symbol, Currency, RMarket, Pool string
	NewProfile, Unmatched                              bool
	ProposedKey                                        string
	Action, Reason                                     string
	ReviewType, ReviewID                               string
	V, P                                               string
	VersionDate                                        contracts.Day
	ChangeMemo, ReportText                             string
	FileBuyTop, SiteBuyTop                             string
	NeedsTick                                          bool
}

// Plan is what the import would do.
type Plan struct {
	FileSHA256   string
	AsOf         contracts.Day
	R            RPlan
	Margin       MarginPlan
	Rows         []RowPlan
	NotInFile    []string
	Unvalued     []string
	RemovedNames []string
	FlaggedCount int
}

// Importer is pure: bytes and the stored state in, a plan out. Numbers are read as decimal text.
// A bad file is a *FileError.
type Importer interface {
	Plan(in Input) (Plan, error)
}

// PlanFunc is a function that plans, for a caller that does not hold an Importer.
type PlanFunc func(Input) (Plan, error)

// ErrBadFile is the error of a file the importer cannot read.
var ErrBadFile = errors.New("valueimport: bad file")

// FileError names the path of the key that is missing or wrong, never a value.
type FileError struct{ Path string }

// Error is the text of the error: the path and nothing else of the input.
func (e *FileError) Error() string { return ErrBadFile.Error() + ": " + e.Path }

// Unwrap makes errors.Is(err, ErrBadFile) true.
func (e *FileError) Unwrap() error { return ErrBadFile }
