package option_candidates_page

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestSourceCountsFromMarks(t *testing.T) {
	t.Parallel()
	var rows []store.Candidate
	for id := 1; id <= 5; id++ {
		c := cand("short_put", id, "ok")
		if id > 3 {
			c.Marks = `["eod_fallback"]`
		}
		rows = append(rows, c)
	}
	b := buildBytes(t, cfg(fakeReader{rows: map[string][]store.Candidate{"2026-11-09/short_put": rows}}), buildDay)
	var p struct {
		SourceCounts map[string]int `json:"source_counts"`
		Notes        []string       `json:"notes"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if len(p.SourceCounts) != 2 || p.SourceCounts["open"] != 3 || p.SourceCounts["eod_fallback"] != 2 {
		t.Errorf("source_counts = %v", p.SourceCounts)
	}
	want := "Open chain: 3 names, last close: 2 names. " + countsGloss
	if !slices.Contains(p.Notes, want) {
		t.Errorf("notes = %v", p.Notes)
	}
}

func TestSourceCountsAbsentWithoutRows(t *testing.T) {
	t.Parallel()
	b := buildBytes(t, cfg(fakeReader{}), buildDay)
	var p map[string]json.RawMessage
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if _, ok := p["source_counts"]; ok {
		t.Error("source_counts present without rows")
	}
}
