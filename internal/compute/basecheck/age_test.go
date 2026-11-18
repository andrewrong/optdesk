package basecheck

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// ageOf sets the newest day of every subject of a row and returns the row.
func ageOf(t *testing.T, in basecheckapi.Input, row string, newest contracts.Day) basecheckapi.Row {
	t.Helper()
	set(&in, row, "newest", func(f *basecheckapi.Fact) { f.Day = newest })
	return rowOf(t, Evaluate(in), row)
}

func TestAgeInSessions(t *testing.T) {
	t.Parallel()
	// Days XNYS 2026-11-17; sessions 11-09..11-13, 11-16, 11-17.
	for _, tc := range []struct {
		newest   contracts.Day
		sessions int
		level    basecheckapi.Level
	}{
		{"2026-11-17", 0, basecheckapi.Pass},
		{"2026-11-16", 1, basecheckapi.Warn},
		{"2026-11-13", 2, basecheckapi.Fail},
		{"2026-11-10", 5, basecheckapi.Fail},
	} {
		r := ageOf(t, load(t), "bars", tc.newest)
		c := checkOf(t, r, "age")
		if r.AgeSessions != tc.sessions || c.Level != tc.level {
			t.Errorf("newest %s: sessions %d level %s, want %d %s", tc.newest, r.AgeSessions, c.Level, tc.sessions, tc.level)
		}
	}
}

func TestAgeAcrossHoliday(t *testing.T) {
	t.Parallel()
	// 2026-11-14 and 15 are a weekend and 11-16 a holiday here: the calendar days are 4, the
	// sessions after 11-13 up to 11-17 are one.
	in := load(t)
	in.Sessions["XNYS"] = []contracts.Day{"2026-11-12", "2026-11-13", "2026-11-17"}
	r := ageOf(t, in, "quotes", "2026-11-13")
	if r.AgeDays != 4 || r.AgeSessions != 1 {
		t.Errorf("days %d sessions %d, want 4 and 1", r.AgeDays, r.AgeSessions)
	}
	if got := checkOf(t, r, "age").Level; got != basecheckapi.Warn {
		t.Errorf("level = %s, want warn", got)
	}
}

func TestJpAgeUsesJpCalendar(t *testing.T) {
	t.Parallel()
	in := load(t)
	in.Days = map[string]contracts.Day{"XTKS": "2026-11-18", "XNYS": "2026-11-17"}
	in.Sessions["XTKS"] = append(in.Sessions["XTKS"], "2026-11-18")
	set(&in, "bars", "newest", func(f *basecheckapi.Fact) {
		if f.Subject == "JPY" {
			f.Day = "2026-11-18"
		}
	})
	r := rowOf(t, Evaluate(in), "bars")
	if r.AgeSessions != 0 {
		t.Errorf("age = %d, want 0 (the JP day is the last closed XTKS session)", r.AgeSessions)
	}
	if got := checkOf(t, r, "age").Level; got != basecheckapi.Pass {
		t.Errorf("level = %s, want pass", got)
	}
}

func TestAgeNeverNegative(t *testing.T) {
	t.Parallel()
	r := ageOf(t, load(t), "bars", "2026-11-18")
	if r.AgeSessions != 0 || r.AgeDays != 0 {
		t.Errorf("age = %d sessions %d days, want 0", r.AgeSessions, r.AgeDays)
	}
	if got := checkOf(t, r, "age").Level; got != basecheckapi.Pass {
		t.Errorf("level = %s, want pass", got)
	}
}

func TestSeveralSubjectsTakeTheWorst(t *testing.T) {
	t.Parallel()
	in := load(t)
	set(&in, "bars", "newest", func(f *basecheckapi.Fact) {
		if f.Subject == "JPY" {
			f.Day = "2026-11-13"
		}
	})
	r := rowOf(t, Evaluate(in), "bars")
	if r.AgeSessions != 2 || checkOf(t, r, "age").Level != basecheckapi.Fail {
		t.Errorf("sessions %d level %s, want 2 fail", r.AgeSessions, checkOf(t, r, "age").Level)
	}
}

func TestCalendarlessRowCountsDays(t *testing.T) {
	t.Parallel()
	// rates.age is (1 day, 2 days): newest 2026-11-15 is 2 calendar days before 11-17.
	r := ageOf(t, load(t), "rates", "2026-11-15")
	if r.AgeDays != 2 || checkOf(t, r, "age").Level != basecheckapi.Fail {
		t.Errorf("days %d level %s, want 2 fail", r.AgeDays, checkOf(t, r, "age").Level)
	}
}
