package ops_today_page

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/store/runview"
)

const (
	datasetUS    = "obs.option_eod"
	datasetIndex = "obs.option_eod_index"
	maxGaps      = 5
	statusDone   = "done"
)

// coverage is the one line of what the option capture stored today, from the newest finished run of each key of
// obs.option_eod and obs.option_eod_index (docs/design/option-capture-need.md, Decision 12). It is empty when
// there is no run of either dataset. The line is cut to the schema's 200 runes.
func coverage(runs []runview.Row) string {
	us, idx := newestOf(runs, datasetUS), newestOf(runs, datasetIndex)
	if len(us) == 0 && len(idx) == 0 {
		return ""
	}
	contracts := 0
	var gaps []string
	for _, r := range us {
		contracts += r.Usable
	}
	for _, set := range [][]runview.Row{us, idx} {
		for _, r := range set {
			if r.Status != statusDone || r.Usable == 0 {
				gaps = append(gaps, r.Key)
			}
		}
	}
	sort.Strings(gaps)
	list := "none"
	if len(gaps) > 0 {
		shown := gaps
		if len(shown) > maxGaps {
			shown = shown[:maxGaps]
		}
		list = strings.Join(shown, ", ")
		if len(gaps) > maxGaps {
			list += fmt.Sprintf(" and %d more", len(gaps)-maxGaps)
		}
	}
	index := "index: no run yet"
	if len(idx) > 0 {
		parts := make([]string, len(idx))
		for i, r := range idx {
			parts[i] = fmt.Sprintf("%s %d", r.Key, r.Usable)
		}
		index = "index " + strings.Join(parts, ", ") + " contracts"
	}
	line := fmt.Sprintf("Option chains stored today: US %d names, %d contracts; %s; gaps: %s.", len(us), contracts, index, list)
	if rs := []rune(line); len(rs) > maxText {
		line = string(rs[:maxText-1]) + "…"
	}
	return line
}

// newestOf is the newest finished run of each key of one dataset, sorted by key.
func newestOf(runs []runview.Row, dataset string) []runview.Row {
	newest := map[string]runview.Row{}
	for _, r := range runs {
		if r.Dataset != dataset || r.Status == runningText || r.FinishedAt.IsZero() {
			continue
		}
		if old, ok := newest[r.Key]; !ok || r.FinishedAt.After(old.FinishedAt) {
			newest[r.Key] = r
		}
	}
	out := make([]runview.Row, 0, len(newest))
	for _, r := range newest {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
