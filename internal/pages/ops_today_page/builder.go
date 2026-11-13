// Package ops_today_page builds page.ops_today_page.v1, the same-morning page: the newest run of each market key
// of the option-open datasets with its reason code, and one row for each job that is not wired
// (docs/design/option-open-gap-jp.md, Decision 11). It renders rows a producer wrote; it computes nothing.
package ops_today_page

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/store/runview"
)

const (
	pageName    = "ops_today_page"
	pageTitle   = "Today's option-open runs"
	idText      = "ops_today"
	timeLayout  = "2006-01-02T15:04:05-07:00"
	maxText     = 200
	statusFinal = "not_wired"
	suffixJP    = ".jp"
	marketJP    = "jp"
	marketUS    = "us"
	runningText = "running"
)

// listed is the closed set of datasets the page shows (Decision 11); a run of any other dataset is not listed.
var listed = map[string]bool{
	"plan.option_open_item": true, "plan.option_open_item.jp": true,
	"obs.option_open": true, "obs.option_open.jp": true,
	"master.option_list": true, "master.option_list.jp": true,
	"master.option_contract": true, "master.option_contract.jp": true,
}

type row struct {
	Dataset    string  `json:"dataset"`
	Key        string  `json:"key"`
	Market     string  `json:"market"`
	Status     string  `json:"status"`
	ErrorCode  *string `json:"error_code"`
	ReasonCode *string `json:"reason_code"`
	ReturnedN  int     `json:"returned_n"`
	UsableN    int     `json:"usable_n"`
	Detail     *string `json:"detail"`
}

type section struct {
	Rows []row `json:"rows"`
}

type viewModel struct {
	Schema        string     `json:"schema"`
	SchemaVersion int        `json:"schema_version"`
	RunID         string     `json:"run_id"`
	BuildID       string     `json:"build_id"`
	Scope         string     `json:"scope"`
	GeneratedAt   string     `json:"generated_at"`
	DataTimes     []struct{} `json:"data_times"`
	RunStatus     string     `json:"run_status"`
	Stale         *struct{}  `json:"stale"`
	Title         string     `json:"title"`
	Notes         []string   `json:"notes"`
	Gaps          []struct{} `json:"gaps"`
	Day           string     `json:"day"`
	Today         section    `json:"today"`
}

type runKey struct{ dataset, key string }

// market is the one naming rule: jp when the dataset id ends .jp, us otherwise.
func market(dataset string) string {
	if strings.HasSuffix(dataset, suffixJP) {
		return marketJP
	}
	return marketUS
}

// Build returns the JSON of page.ops_today_page.v1: for each (dataset, key) of the listed datasets the newest
// finished run (a running row is skipped), joined to the reasons of the same dataset and key, then one row for
// each reason of the not_wired set (key = its scope, detail = the NAME of the missing key). The session day is the
// day of the first listed run, else the UTC date of now; generated_at is now.
func Build(runs []runview.Row, reasons []runview.Reason, now time.Time) ([]byte, error) {
	newest := map[runKey]runview.Row{}
	var order []runKey
	for _, r := range runs {
		if !listed[r.Dataset] || r.Status == runningText || r.FinishedAt.IsZero() {
			continue
		}
		k := runKey{r.Dataset, r.Key}
		old, seen := newest[k]
		if !seen {
			order = append(order, k)
		}
		if !seen || r.FinishedAt.After(old.FinishedAt) {
			newest[k] = r
		}
	}
	rows := []row{}
	day := now.UTC().Format("2006-01-02")
	for i, k := range order {
		r := newest[k]
		if i == 0 {
			day = string(r.SessionDay)
		}
		rows = append(rows, runRow(r, reasons))
	}
	for _, re := range reasons {
		if re.Dataset != runview.NotWiredDataset {
			continue
		}
		rows = append(rows, row{Dataset: re.Dataset, Key: re.Scope, Market: market(re.Dataset), Status: statusFinal,
			ReasonCode: text(re.Code), Detail: text(re.Detail)})
	}
	vm := viewModel{Schema: pageName, SchemaVersion: 1, RunID: idText, BuildID: idText, Scope: idText,
		GeneratedAt: now.Format(timeLayout), DataTimes: []struct{}{}, RunStatus: "ok", Title: pageTitle,
		Notes: []string{}, Gaps: []struct{}{}, Day: day, Today: section{Rows: rows}}
	return json.Marshal(vm)
}

// runRow joins one run to the reasons of its dataset and key (subject = key), one detail each.
func runRow(r runview.Row, reasons []runview.Reason) row {
	code := r.ReasonCode
	var details []string
	for _, re := range reasons {
		if re.Dataset != r.Dataset || re.Subject != r.Key {
			continue
		}
		if code == "" && re.Code != "field_missing" && re.Code != "fields_complete" {
			code = re.Code
		}
		d := re.Detail
		if re.Scope != "" && d != "" {
			d = re.Scope + ": " + d
		} else if d == "" {
			d = re.Scope
		}
		if d != "" {
			details = append(details, d)
		}
	}
	return row{Dataset: r.Dataset, Key: r.Key, Market: market(r.Dataset), Status: r.Status, ErrorCode: text(r.ErrorCode),
		ReasonCode: text(code), ReturnedN: r.Returned, UsableN: r.Usable, Detail: text(strings.Join(details, "; "))}
}

// text is nil for an empty string and is cut to the schema's 200 runes otherwise.
func text(s string) *string {
	if s == "" {
		return nil
	}
	if rs := []rune(s); len(rs) > maxText {
		s = string(rs[:maxText-1]) + "…"
	}
	return &s
}
