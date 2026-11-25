package option_candidates_page

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// TestSpotDayFollowsNote: the spot_day of a company is the day its spot_note names, with the rows day 2026-11-25.
func TestSpotDayFollowsNote(t *testing.T) {
	t.Parallel()
	const day = contracts.Day("2026-11-25")
	cases := map[string]struct {
		marks    string
		noSpot   bool
		wantDay  string
		wantNote string
	}{
		"chain price of the day before": {marks: `["spot_chain","chain_day:2026-11-24"]`, wantDay: "2026-11-24", wantNote: "11-24"},
		"chain price without a day":     {marks: `["spot_chain"]`, wantDay: "2026-11-25", wantNote: "11-25"},
		"quote of the rows day":         {marks: `["spot_quote:2026-11-25"]`, wantDay: "2026-11-25", wantNote: "11-25"},
		"quote of the day before":       {marks: `["spot_quote:2026-11-24"]`, wantDay: "2026-11-24", wantNote: "11-24"},
		"no spot mark":                  {marks: `[]`, wantDay: "2026-11-25", wantNote: "11-25"},
		"no spot":                       {marks: `["spot_chain","chain_day:2026-11-24"]`, noSpot: true, wantDay: "2026-11-25"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
			rows[0].Marks = tc.marks
			if tc.noSpot {
				rows[0].Spot = nil
			}
			res := resultOf(fitsVerdict(keyOf(rows, 0)))
			res.Companies = []optiontierapi.CompanyCount{{Company: "7", Fits: 1}}
			c := tierConfig(rows, &fakeTierer{res: res})
			c.Candidates = fakeReader{rows: map[string][]store.Candidate{string(day) + "/short_put": rows}}
			b, err := Build(context.Background(), c, nil, day, time.Date(2026, 11, 25, 22, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			var p struct {
				Tiers *tiersView `json:"short_put_tiers"`
			}
			if err := json.Unmarshal(b, &p); err != nil {
				t.Fatal(err)
			}
			cv := p.Tiers.Companies[0]
			if cv.SpotDay != tc.wantDay {
				t.Errorf("spot_day = %q, want %q", cv.SpotDay, tc.wantDay)
			}
			if tc.noSpot {
				return
			}
			if cv.SpotNote == nil || !strings.Contains(*cv.SpotNote, tc.wantNote) {
				t.Errorf("note = %v, want it to contain %q", cv.SpotNote, tc.wantNote)
			}
		})
	}
}
