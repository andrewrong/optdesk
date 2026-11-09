package option_notices_page

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestLookbackSevenDays(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, capDay string
		found        bool
	}{
		{"same day", "2026-11-09", true},
		{"seven days back", "2026-11-02", true},
		{"eight days back", "2026-11-01", false},
	} {
		a := fakeAccount{positions: map[string][]store.Position{"main/" + tc.capDay: {stock("ACME")}}}
		c, ok, err := findCapture(context.Background(), a, "main", buildDay)
		if err != nil || ok != tc.found {
			t.Fatalf("%s: ok %v err %v", tc.name, ok, err)
		}
		if tc.found && string(c.Day) != tc.capDay {
			t.Errorf("%s: day %s", tc.name, c.Day)
		}
	}
}

func TestPreviousDayFound(t *testing.T) {
	t.Parallel()
	a := fakeAccount{
		positions: map[string][]store.Position{
			"main/2026-11-09": {stock("ACME")},
			"main/2026-11-05": {shortPut("ACME", "2026-11-09")},
			"main/2026-10-20": {stock("OLD")},
		},
		values: map[string][]store.AccountValue{"main/2026-11-09": {{Tag: "Cushion", Value: "0.2"}}},
	}
	c, ok, err := findCapture(context.Background(), a, "main", buildDay)
	if err != nil || !ok {
		t.Fatalf("ok %v err %v", ok, err)
	}
	if c.PrevDay != "2026-11-05" || len(c.Prev) != 1 || c.Prev[0].SecType != "OPT" || len(c.Values) != 1 {
		t.Errorf("capture = %+v", c)
	}
	delete(a.positions, "main/2026-11-05")
	if c, _, _ = findCapture(context.Background(), a, "main", buildDay); c.PrevDay != "" || len(c.Prev) != 0 {
		t.Errorf("capture beyond lookback = %+v", c)
	}
}
