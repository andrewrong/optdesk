package data_health_page

import (
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

// Input is what Build reads: the clock as a value (Now), the day, the coverage reader, the jobs and the owner's form.
type Input struct {
	Now   time.Time
	Today contracts.Day
	Cov   coverage.Reader
	Jobs  []Job
	Form  *Form // nil without owner edit
}

// Newest is the newest observation of one FRED series and its age in days at Today.
type Newest struct {
	SeriesID string        `json:"series_id"`
	Day      contracts.Day `json:"day"`
	AgeDays  int           `json:"age_days"`
}

// Dataset is one row of the page.
type Dataset struct {
	DatasetID string   `json:"dataset_id"`
	Label     string   `json:"label"`
	JobID     *string  `json:"job_id"`
	Calendar  string   `json:"calendar"`
	GapDays   int      `json:"gap_days"`
	RepairAll string   `json:"repair_all"`
	Newest    []Newest `json:"newest"`
	Cells     []Cell   `json:"cells"`
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

type formVM struct {
	PostPath string `json:"post_path"`
	Expires  string `json:"expires"`
	MAC      string `json:"mac"`
}

type viewModel struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	BuiltAt       string        `json:"built_at"`
	Form          *formVM       `json:"form"`
	Datasets      []Dataset     `json:"datasets"`
	Glossary      []GlossaryRow `json:"glossary"`
	OverwriteNote string        `json:"overwrite_note"`
	Gaps          []Gap         `json:"gaps"`
}
