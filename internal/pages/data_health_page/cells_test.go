package data_health_page

import (
	"fmt"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

func dayN(i int) contracts.Day {
	return contracts.Day(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02"))
}

func TestNotRunFromCalendar(t *testing.T) {
	t.Parallel()
	days := []contracts.Day{dayN(0), dayN(1), dayN(2)}
	rows := []coverage.Cell{
		{Day: dayN(1), Dataset: "a", Keys: 2, OK: 2},
		{Day: dayN(2), Dataset: "b", Keys: 1, OK: 1},
	}
	cells := DatasetCells(rows, "a", days)
	want := []string{StateNotRun, "ok", StateNotRun}
	for i, c := range cells {
		if c.State != want[i] || c.Day != days[i] {
			t.Fatalf("cell %d = %+v, want state %s", i, c, want[i])
		}
	}
	if cells[0].MissingKeys == nil {
		t.Fatal("missing_keys must be non-nil")
	}
}

func TestMixedCellPrecedence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		row  coverage.Cell
		want string
		text string
	}{
		{"missing", coverage.Cell{Keys: 500, OK: 499, Missing: 1}, "missing", "499 of 500"},
		{"expired", coverage.Cell{Keys: 500, OK: 499, Expired: 1}, "expired", "499 of 500"},
		{"no_value", coverage.Cell{Keys: 500, OK: 499, NoValue: 1}, "no_value", "499 of 500"},
		{"missing and expired", coverage.Cell{Keys: 2, Missing: 1, Expired: 1}, "missing", "0 of 2"},
		{"all ok", coverage.Cell{Keys: 3, OK: 3}, "ok", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.row.Day, tc.row.Dataset = dayN(0), "d"
			c := DatasetCells([]coverage.Cell{tc.row}, "d", []contracts.Day{dayN(0)})[0]
			if c.State != tc.want || c.CountText() != tc.text {
				t.Fatalf("got %s %q, want %s %q", c.State, c.CountText(), tc.want, tc.text)
			}
		})
	}
}

func TestWindowDaysLast30Sessions(t *testing.T) {
	t.Parallel()
	var ss []coverage.Session
	for i := 44; i >= 0; i-- { // descending input, 45 sessions
		ss = append(ss, coverage.Session{Calendar: "XNYS", Day: dayN(i), IsSession: true})
	}
	ss = append(ss, coverage.Session{Calendar: "XNYS", Day: dayN(46), IsSession: false})
	today := dayN(40) // five sessions after today
	got := WindowDays(ss, today)
	if len(got) != coverage.WindowSessions {
		t.Fatalf("len = %d", len(got))
	}
	if got[0] != dayN(11) || got[len(got)-1] != today {
		t.Fatalf("window %s..%s", got[0], got[len(got)-1])
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("not ascending at %d: %s", i, fmt.Sprint(got))
		}
	}
}
