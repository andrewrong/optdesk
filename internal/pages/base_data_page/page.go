// Package base_data_page builds the document of the base data page: it reads the base facts and the coverage,
// has the injected basecheckapi.Func judge them, and lays out groups, rows, checks and Chinese texts
// (docs/design/base-data-page.md, entry 9).
package base_data_page

import (
	"time"

	_ "time/tzdata" // the zone database travels with the binary

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/store/basefact"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

// Input is what Build reads: the clock as a value, the fact reader, the coverage reader and the verdict function.
type Input struct {
	Now   time.Time
	Facts basefact.Reader
	Cov   coverage.Reader
	Eval  basecheckapi.Func
}

type checkVM struct {
	Rule            string `json:"rule"`
	Label           string `json:"label"`
	HintZh          string `json:"hint_zh"`
	Level           string `json:"level"`
	N               int64  `json:"n"`
	Bad             int64  `json:"bad"`
	Share           string `json:"share"`
	WarnAt          string `json:"warn_at"`
	FailAt          string `json:"fail_at"`
	ThresholdSource string `json:"threshold_source"`
	Example         string `json:"example"`
}

type rowVM struct {
	Dataset       string    `json:"dataset"`
	Label         string    `json:"label"`
	LabelZh       string    `json:"label_zh"`
	Level         string    `json:"level"`
	NewestValue   string    `json:"newest_value"`
	NewestDay     *string   `json:"newest_day"`
	NewestSubject string    `json:"newest_subject"`
	AgeText       string    `json:"age_text"`
	FailedSummary string    `json:"failed_summary"`
	Checks        []checkVM `json:"checks"`
}

type groupVM struct {
	Name string  `json:"name"`
	Rows []rowVM `json:"rows"`
}

type countsVM struct {
	Pass    int `json:"pass"`
	Warn    int `json:"warn"`
	Fail    int `json:"fail"`
	Unknown int `json:"unknown"`
}

type topVM struct {
	Level  string   `json:"level"`
	TextZh string   `json:"text_zh"`
	Counts countsVM `json:"counts"`
}

type lastSessionsVM struct {
	XNYS string `json:"XNYS"`
	XTKS string `json:"XTKS"`
}

// GlossaryRow explains one word of the page.
type GlossaryRow struct {
	Term string `json:"term"`
	Text string `json:"text"`
}

// Gap says that part of the page could not be built.
type Gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type viewModel struct {
	Schema       string         `json:"schema"`
	Version      int            `json:"schema_version"`
	RunID        string         `json:"run_id"`
	BuiltAt      string         `json:"built_at"`
	AsOf         string         `json:"as_of"`
	LastSessions lastSessionsVM `json:"last_sessions"`
	Top          topVM          `json:"top"`
	Groups       []groupVM      `json:"groups"`
	Glossary     []GlossaryRow  `json:"glossary"`
	DerivedNote  string         `json:"derived_note"`
	RepairPath   string         `json:"repair_path"`
	Gaps         []Gap          `json:"gaps"`
}
