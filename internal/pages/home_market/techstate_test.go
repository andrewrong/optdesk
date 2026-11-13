package home_market

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
)

func techRecord(state, g1, g3, dataDay string) contracts.ReadingRecord {
	tag := func(n, v string) gen.ReadingRecordParametersElem {
		return gen.ReadingRecordParametersElem{Param: n, ValueTag: &v}
	}
	r := reading("2026-11-06", "ok", "-1")
	r.Parameters = []gen.ReadingRecordParametersElem{
		tag("state", state), tag("gap_1m", g1), tag("gap_3m", g3), tag("data_day", dataDay),
	}
	return r
}

// withTech answers market.tech_state with rows (or err) and every other reading as the base fake does.
func withTech(rows []contracts.ReadingRecord, err error) Config {
	c := config(nil)
	base := c.Readings
	c.Readings = compute.ReadingFunc(func(ctx context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
		if name == figTech {
			if kind != "market" || subject != "NDX_SPX" {
				return nil, errors.New("unexpected tech key " + kind + " " + subject)
			}
			return rows, err
		}
		return base(ctx, name, kind, subject, from, to)
	})
	return c
}

type techDoc struct {
	RunStatus string `json:"run_status"`
	Card      struct {
		Notes []string `json:"notes"`
		Gaps  []gap    `json:"gaps"`
	} `json:"card"`
}

func buildTech(t *testing.T, c Config) (techDoc, []byte) {
	t.Helper()
	b := build(t, c, fullSeries())
	var d techDoc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d, b
}

func techGapCount(d techDoc) (n int) {
	for _, g := range d.Card.Gaps {
		if strings.HasPrefix(g.Text, "Tech state") {
			n++
		}
	}
	return n
}

func TestTechNotesAreFirstAndInOrder(t *testing.T) {
	t.Parallel()
	d, _ := buildTech(t, withTech([]contracts.ReadingRecord{techRecord("LAGGING", "-3.12", "-6.40", "2026-11-06")}, nil))
	if len(d.Card.Notes) != 3 {
		t.Fatalf("notes = %q", d.Card.Notes)
	}
	if d.Card.Notes[0] != sayLagging {
		t.Errorf("notes[0] = %q", d.Card.Notes[0])
	}
	if want := "NDX minus SPX: 1 month -3.12 pt, 3 months -6.40 pt; trial thresholds, not a forecast"; d.Card.Notes[1] != want {
		t.Errorf("notes[1] = %q", d.Card.Notes[1])
	}
	if d.Card.Notes[2] != notes[0] {
		t.Errorf("notes[2] = %q", d.Card.Notes[2])
	}
	if techGapCount(d) != 0 {
		t.Errorf("a shown state left a tech gap: %v", d.Card.Gaps)
	}
}

func TestTechNotesForEachState(t *testing.T) {
	t.Parallel()
	for state, want := range map[string]string{"LEADING": sayLeading, "IN LINE": sayInLine, "LAGGING": sayLagging, "LAGGING HARD": sayLaggingHrd} {
		d, _ := buildTech(t, withTech([]contracts.ReadingRecord{techRecord(state, "1.00", "2.00", "2026-11-06")}, nil))
		if d.Card.Notes[0] != want {
			t.Errorf("%s: notes[0] = %q", state, d.Card.Notes[0])
		}
	}
}

func TestTechNoReadingIsNoDataGap(t *testing.T) {
	t.Parallel()
	d, _ := buildTech(t, withTech(nil, nil))
	if len(d.Card.Notes) != 1 || techGapCount(d) != 1 {
		t.Fatalf("notes = %q, gaps = %v", d.Card.Notes, d.Card.Gaps)
	}
	for _, g := range d.Card.Gaps {
		if strings.HasPrefix(g.Text, "Tech state") && g.Reason != "no_data" {
			t.Errorf("reason = %s", g.Reason)
		}
	}
}

func TestTechStaleReadingIsStaleGap(t *testing.T) {
	t.Parallel()
	d, _ := buildTech(t, withTech([]contracts.ReadingRecord{techRecord("LEADING", "1.00", "2.00", "2026-10-30")}, nil))
	if len(d.Card.Notes) != 1 || techGapCount(d) != 1 {
		t.Fatalf("notes = %q, gaps = %v", d.Card.Notes, d.Card.Gaps)
	}
	for _, g := range d.Card.Gaps {
		if strings.HasPrefix(g.Text, "Tech state") && g.Reason != "stale_input" {
			t.Errorf("reason = %s", g.Reason)
		}
	}
}

func TestTechGapKeepsRunStatus(t *testing.T) {
	t.Parallel()
	withGap, _ := buildTech(t, withTech(nil, nil))
	withState, _ := buildTech(t, withTech([]contracts.ReadingRecord{techRecord("LEADING", "1.00", "2.00", "2026-11-06")}, nil))
	if techGapCount(withGap) != 1 || techGapCount(withState) != 0 {
		t.Fatalf("gaps: %v / %v", withGap.Card.Gaps, withState.Card.Gaps)
	}
	if withGap.RunStatus != withState.RunStatus {
		t.Fatalf("run_status %s with the gap, %s without", withGap.RunStatus, withState.RunStatus)
	}
}

func TestTechReaderErrorIsError(t *testing.T) {
	t.Parallel()
	_, err := Build(context.Background(), withTech(nil, errors.New("boom")), fullSeries(), "2026-11-06", at)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestTechNoteTextsFitTheSchema(t *testing.T) {
	t.Parallel()
	for _, rows := range [][]contracts.ReadingRecord{
		{techRecord("LAGGING", "-3.12", "-6.40", "2026-11-06")},
		nil,
		{techRecord("LEADING", "1.00", "2.00", "2026-10-30")},
	} {
		d, _ := buildTech(t, withTech(rows, nil)) // build validates the whole card against page.market_summary.v3.json
		for _, n := range d.Card.Notes {
			if c := utf8.RuneCountInString(n); c < 1 || c > 200 {
				t.Errorf("note of %d characters", c)
			}
		}
		for _, g := range d.Card.Gaps {
			if c := utf8.RuneCountInString(g.Text); c < 1 || c >= 200 {
				t.Errorf("gap text of %d characters", c)
			}
		}
	}
}
