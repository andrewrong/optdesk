package option_candidates_page

import (
	"slices"
	"strconv"

	"github.com/andrewrong/optdesk/internal/store"
)

// sourceCounts is the source_counts field: how many names the stored candidates took from the open chain
// and from the last close. none is not written: a name with no candidate has no stored row
// (candidates-after-partial-open.md Decision 7).
type sourceCounts struct {
	Open        int `json:"open"`
	EodFallback int `json:"eod_fallback"`
}

// nameKey identifies a name across the kinds of one currency.
type nameKey struct {
	cur string
	id  int
}

// markTally counts the names of the stored rows by the mark eod_fallback.
type markTally struct {
	fallback map[nameKey]bool
}

func newMarkTally() *markTally { return &markTally{fallback: map[nameKey]bool{}} }

// add reads the rows of one group; a name is a fallback name when any of its rows carries eod_fallback.
func (t *markTally) add(cur string, rows []store.Candidate) {
	for _, r := range rows {
		id := r.UnderlyingID
		if id == 0 {
			id = r.InstrumentID
		}
		k := nameKey{cur, id}
		marks, err := parseMarks(r.Marks)
		fb := err == nil && slices.Contains(marks, markEodFallback)
		t.fallback[k] = t.fallback[k] || fb
	}
}

// counts is nil when no row was read.
func (t *markTally) counts() *sourceCounts {
	if len(t.fallback) == 0 {
		return nil
	}
	var c sourceCounts
	for _, fb := range t.fallback {
		if fb {
			c.EodFallback++
		} else {
			c.Open++
		}
	}
	return &c
}

// line is the text of the counts for the page notes.
func (c sourceCounts) line() string {
	return countsLineOpen + strconv.Itoa(c.Open) + countsLineClose + strconv.Itoa(c.EodFallback) + countsLineTail + countsGloss
}
