package data_health_page

import (
	"time"

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
)

// Input is what Build reads: the clock as a value (Now), the day, the coverage reader, the jobs and the owner's form.
type Input struct {
	Now   time.Time
	Today contracts.Day
	Cov   coverage.Reader
	Jobs  []Job
	Form  *Form // nil without owner edit

	Gateways []GatewayInput // one per gateway; empty: no gateways block

	Runs    jobrun.JobRunReader   // nil: no running lines
	Figures jobrunapi.FiguresFunc // the figures of the open runs at Now; nil: no running lines
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

	FailuresTotal      int          `json:"failures_total"`
	FailuresHidden     int          `json:"failures_hidden"`
	FailuresHiddenText string       `json:"failures_hidden_text"`
	Repeated           []FailureRow `json:"repeated_failures"`
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
	Running       []runningVM   `json:"running,omitempty"`
	Gateways      []GatewayVM   `json:"gateways,omitempty"`
	Datasets      []Dataset     `json:"datasets"`
	Glossary      []GlossaryRow `json:"glossary"`
	OverwriteNote string        `json:"overwrite_note"`
	Gaps          []Gap         `json:"gaps"`
	FailuresNote  string        `json:"failures_note"`
	FailuresNone  string        `json:"failures_none_text"`
}
