package settings_page

import (
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

const (
	maxText    = 200
	maxHistory = 20
	maxRows    = 1000
	maxNames   = 500
	postPath   = "/owner-settings"
)

type gap struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type formView struct {
	PostPath string `json:"post_path"`
	Expires  string `json:"expires"`
	MAC      string `json:"mac"`
}

type confirmRowView struct {
	Label string `json:"label"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

type confirmView struct {
	Rows []confirmRowView `json:"rows"`
	Form formView         `json:"form"`
}

type paramRow struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Unit    string `json:"unit"`
	Kind    string `json:"kind"`
	Value   string `json:"value"`
	Default string `json:"default"`
	Min     string `json:"min"`
	Max     string `json:"max"`
}

// exclusionRow is one OptionExclusionRow: one underlying and its current flag.
type exclusionRow struct {
	InstrumentID int64  `json:"instrument_id"`
	Symbol       string `json:"symbol"`
	Excluded     bool   `json:"excluded"`
}

type historyRow struct {
	At           string  `json:"at"`
	Kind         string  `json:"kind"`
	Name         string  `json:"name"`
	Value        string  `json:"value"`
	Key          *string `json:"key"`
	InstrumentID *int64  `json:"instrument_id"`
	Excluded     *bool   `json:"excluded"`
}

type noticeView struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type viewModel struct {
	Schema        string         `json:"schema"`
	SchemaVersion int            `json:"schema_version"`
	RunID         string         `json:"run_id"`
	BuildID       string         `json:"build_id"`
	Scope         string         `json:"scope"`
	GeneratedAt   string         `json:"generated_at"`
	DataTimes     []struct{}     `json:"data_times"`
	RunStatus     string         `json:"run_status"`
	Gap           *gap           `json:"gap"`
	Title         string         `json:"title"`
	AsOf          string         `json:"as_of"`
	Rows          []exclusionRow `json:"rows"`
	Form          *formView      `json:"form"`
	Confirm       *confirmView   `json:"confirm"`
	History       []historyRow   `json:"history"`
	Params        []paramRow     `json:"params"`
	Notice        *noticeView    `json:"notice"`
}

func newForm(t settingsapi.FormToken) *formView {
	return &formView{PostPath: postPath, Expires: t.Expires, MAC: t.MAC}
}

func newConfirm(c *Confirm) *confirmView {
	if c == nil {
		return nil
	}
	v := &confirmView{Rows: make([]confirmRowView, 0, len(c.Rows)), Form: *newForm(c.Form)}
	for _, r := range limit(c.Rows, maxRows) {
		v.Rows = append(v.Rows, confirmRowView{Label: clip(r.Label), Old: clip(r.Old), New: clip(r.New)})
	}
	return v
}

func newParamRows(specs []settingsapi.RuleParamSpec, stored []ownerset.Param) []paramRow {
	byKey := make(map[string]string, len(stored))
	for _, p := range stored {
		byKey[p.Key] = p.Value
	}
	rows := make([]paramRow, 0, len(specs))
	for _, s := range specs {
		rows = append(rows, paramRow{
			Key: s.Key, Label: s.Label, Unit: s.Unit, Kind: s.Kind,
			Value: byKey[s.Key], Default: s.Default, Min: s.Min, Max: s.Max,
		})
	}
	return rows
}

// symbolOf is the stored name, else the id as text so a row is never nameless.
func symbolOf(names map[int64]string, id int64) string {
	if n := names[id]; n != "" {
		return clip(n)
	}
	return "#" + strconv.FormatInt(id, 10)
}

func newHistoryRows(h []ownerset.HistoryRow, specs []settingsapi.RuleParamSpec, names map[int64]string, zone *time.Location) []historyRow {
	labels := make(map[string]string, len(specs))
	for _, s := range specs {
		labels[s.Key] = s.Label
	}
	rows := make([]historyRow, 0, len(h))
	for _, r := range limit(h, maxHistory) {
		row := historyRow{At: r.SetAt.In(zone).Format(time.RFC3339), Kind: r.Kind}
		if r.Kind == "param" {
			key := r.Key
			row.Key, row.Value = &key, clip(r.Value)
			row.Name = key
			if l := labels[key]; l != "" {
				row.Name = clip(l)
			}
		} else {
			id, ex := r.InstrumentID, r.Excluded
			row.InstrumentID, row.Excluded = &id, &ex
			row.Name = symbolOf(names, id)
		}
		rows = append(rows, row)
	}
	return rows
}

func limit[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// clip cuts text to the schema's 200 characters.
func clip(s string) string {
	if utf8.RuneCountInString(s) <= maxText {
		return s
	}
	return string([]rune(s)[:maxText])
}
