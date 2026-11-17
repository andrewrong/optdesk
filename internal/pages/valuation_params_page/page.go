// Package valuation_params_page builds the page.valuation_params_page.v1 view model, the market parameters page of
// the valuation: the newest R version, the proposal of R for the latest 10-year yield, the two parameter tables and
// the R history. It computes nothing: every figure is a text of the valuelinesapi.Calculator, shown through its
// RoundText. It reads no store implementation and no clock.
// Design: docs/design/valuation-structure.md#6 valuation-pages.
package valuation_params_page

import (
	"regexp"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// postPath is the owner post path of the page.
const postPath = "/valuation-params-post"

// placesR is the decimal places of a displayed R and yield.
const placesR = 2

const maxText = 200

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Form is the owner token of the form, made by the caller; nil in Input is read-only (form null).
type Form struct{ Expires, MAC string }

// Yield is the newest stored DGS10 point: a decimal text and its day.
type Yield struct {
	Value string
	Day   contracts.Day
}

// Input is everything Build reads.
type Input struct {
	RunID string
	Now   time.Time // built_at; the page reads no clock

	StoreOK  bool                          // false: the store is unavailable
	Versions []valuation.RVersion          // every R version, newest first
	Yield    *Yield                        // nil: no DGS10 point
	RTable   *valuation.RTableVersion      // newest, nil when none
	Margin   *valuation.MarginTableVersion // newest, nil when none
	Calc     valuelinesapi.Calculator

	Form  *Form  // nil: form null
	Error string // empty: none
}

type gapJSON struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type formJSON struct {
	PostPath string `json:"post_path"`
	Expires  string `json:"expires"`
	MAC      string `json:"mac"`
}

type yieldJSON struct {
	Value string `json:"value"`
	Day   string `json:"day"`
}

type rValuesJSON struct {
	US   string `json:"us"`
	JP   string `json:"jp"`
	HKCN string `json:"hkcn"`
}

type rVersionJSON struct {
	EffectiveDate string      `json:"effective_date"`
	YieldUsed     string      `json:"yield_used"`
	R             rValuesJSON `json:"r"`
	AISwitch      bool        `json:"ai_switch"`
	AIExpiry      string      `json:"ai_expiry"`
	Note          string      `json:"note"`
	StepWarnings  []string    `json:"step_warnings"`
}

type proposalJSON struct {
	R            rValuesJSON `json:"r"`
	StepWarnings []string    `json:"step_warnings"`
}

type rTableRowJSON struct {
	YieldUpTo string `json:"yield_up_to"`
	US        string `json:"us"`
	JP        string `json:"jp"`
	HKCN      string `json:"hkcn"`
}

type rTableJSON struct {
	EffectiveDate string          `json:"effective_date"`
	Rows          []rTableRowJSON `json:"rows"`
}

type marginRowJSON struct {
	PFrom string `json:"p_from"`
	M     string `json:"m"`
	W     string `json:"w"`
	H     string `json:"h"`
	S     string `json:"s"`
}

type marginTableJSON struct {
	EffectiveDate string          `json:"effective_date"`
	Rows          []marginRowJSON `json:"rows"`
}

type pageJSON struct {
	Schema        string           `json:"schema"`
	SchemaVersion int              `json:"schema_version"`
	RunID         string           `json:"run_id"`
	BuiltAt       string           `json:"built_at"`
	LatestYield   *yieldJSON       `json:"latest_yield"`
	CurrentR      *rVersionJSON    `json:"current_r"`
	Proposal      *proposalJSON    `json:"proposal"`
	RTable        *rTableJSON      `json:"r_table"`
	MarginTable   *marginTableJSON `json:"margin_table"`
	RHistory      []rVersionJSON   `json:"r_history"`
	Gaps          []gapJSON        `json:"gaps"`
	Error         string           `json:"error"`
	Form          *formJSON        `json:"form"`
}
