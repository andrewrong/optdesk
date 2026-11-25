package option_candidates_page

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/store"
)

// TestSpotDayFollowsNote: the spot_day of a company is the day its spot_note names (the rows day is 2026-11-09).
func TestSpotDayFollowsNote(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ marks, wantDay, wantNote string }{
		"chain price of an earlier day": {`["spot_chain","chain_day:2026-11-06"]`, "2026-11-06", "11-06"},
		"chain price without a day":     {`["spot_chain"]`, "2026-11-09", "11-09"},
		"quote of the rows day":         {`["spot_quote:2026-11-09"]`, "2026-11-09", "11-09"},
		"quote of an earlier day":       {`["spot_quote:2026-11-06"]`, "2026-11-06", "11-06"},
		"no spot mark":                  {`[]`, "2026-11-09", "11-09"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
			rows[0].Marks = tc.marks
			res := resultOf(fitsVerdict(keyOf(rows, 0)))
			res.Companies = []optiontierapi.CompanyCount{{Company: "7", Fits: 1}}
			cv := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: res})).Companies[0]
			if cv.SpotDay != tc.wantDay {
				t.Errorf("spot_day = %q, want %q", cv.SpotDay, tc.wantDay)
			}
			if cv.SpotNote == nil || !strings.Contains(*cv.SpotNote, tc.wantNote) {
				t.Errorf("note = %v, want it to contain %q", cv.SpotNote, tc.wantNote)
			}
		})
	}
}
