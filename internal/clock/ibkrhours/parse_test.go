package ibkrhours_test

import (
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/clock/ibkrhours"
)

func utc(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func parse(t *testing.T, s, zone string) []clock.HoursDay {
	t.Helper()
	got, err := ibkrhours.New().Parse(s, zone)
	if err != nil {
		t.Fatalf("Parse(%q, %q): %v", s, zone, err)
	}
	return got
}

func TestOneSegmentDay(t *testing.T) {
	t.Parallel()
	got := parse(t, "20261113:0930-20261113:1600", "US/Eastern")
	want := clock.HoursDay{Day: "2026-11-13", IsSession: true, OpenAt: utc(2026, 11, 13, 14, 30), CloseAt: utc(2026, 11, 13, 21, 0)}
	if len(got) != 1 || got[0].Day != want.Day || !got[0].IsSession || !got[0].OpenAt.Equal(want.OpenAt) || !got[0].CloseAt.Equal(want.CloseAt) {
		t.Fatalf("got %+v", got)
	}
	if got[0].OpenAt.Location() != time.UTC {
		t.Fatalf("not UTC: %v", got[0].OpenAt.Location())
	}
}

func TestTokyoLunchBreakGroupsByStartDate(t *testing.T) {
	t.Parallel()
	got := parse(t, "20261113:0900-20261113:1130;20261113:1230-20261113:1530;20261116:0900-20261116:1130;20261116:1230-20261116:1530", "Japan")
	if len(got) != 2 {
		t.Fatalf("want 2 days, got %+v", got)
	}
	if got[0].Day != "2026-11-13" || !got[0].OpenAt.Equal(utc(2026, 11, 13, 0, 0)) || !got[0].CloseAt.Equal(utc(2026, 11, 13, 6, 30)) {
		t.Fatalf("day 1 %+v", got[0])
	}
	if got[1].Day != "2026-11-16" || !got[1].OpenAt.Equal(utc(2026, 11, 16, 0, 0)) || !got[1].CloseAt.Equal(utc(2026, 11, 16, 6, 30)) {
		t.Fatalf("day 2 %+v", got[1])
	}
}

func TestEarlyClose(t *testing.T) {
	t.Parallel()
	got := parse(t, "20261127:0930-20261127:1300", "US/Eastern")
	if len(got) != 1 || !got[0].CloseAt.Equal(utc(2026, 11, 27, 18, 0)) {
		t.Fatalf("got %+v", got)
	}
}

func TestClosedDay(t *testing.T) {
	t.Parallel()
	got := parse(t, "20261126:CLOSED;20261127:0930-20261127:1300", "US/Eastern")
	if len(got) != 2 || got[0].Day != "2026-11-26" || got[0].IsSession || !got[0].OpenAt.IsZero() || !got[0].CloseAt.IsZero() {
		t.Fatalf("got %+v", got)
	}
	if !got[1].IsSession {
		t.Fatalf("second day %+v", got[1])
	}
}

func TestEasternDaylightSavingChangeover(t *testing.T) {
	t.Parallel()
	// DST ended 2026-11-01: Friday is EDT (UTC-4), Monday after is EST (UTC-5).
	got := parse(t, "20261030:0930-20261030:1600;20261102:0930-20261102:1600", "US/Eastern")
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if !got[0].OpenAt.Equal(utc(2026, 10, 30, 13, 30)) || !got[1].OpenAt.Equal(utc(2026, 11, 2, 14, 30)) {
		t.Fatalf("opens %v %v", got[0].OpenAt, got[1].OpenAt)
	}
	// A segment spanning the changeover keeps wall-clock times of its own dates.
	span := parse(t, "20261101:0000-20261101:2359", "US/Eastern")
	if !span[0].OpenAt.Equal(utc(2026, 11, 1, 4, 0)) || !span[0].CloseAt.Equal(utc(2026, 11, 2, 4, 59)) {
		t.Fatalf("changeover day %+v", span[0])
	}
}

func TestTokyoDayEnds1530(t *testing.T) {
	t.Parallel()
	got := parse(t, "20261113:0900-20261113:1530", "Japan")
	if len(got) != 1 || !got[0].CloseAt.Equal(utc(2026, 11, 13, 6, 30)) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, s, zone, token string
	}{
		{"empty zone", "20261113:0930-20261113:1600", "", `time zone ""`},
		{"unknown zone", "20261113:0930-20261113:1600", "Mars/Base", "Mars/Base"},
		{"end before start", "20261113:1600-20261113:0930", "US/Eastern", "20261113:1600-20261113:0930"},
		{"malformed token", "20261113:0930-20261113:1600;garbage", "US/Eastern", "garbage"},
		{"bad time", "20261113:2560-20261113:2600", "US/Eastern", "20261113:2560-20261113:2600"},
		{"closed with junk", "20261113:CLOSED:x", "US/Eastern", "20261113:CLOSED:x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := ibkrhours.New().Parse(c.s, c.zone)
			if err == nil {
				t.Fatalf("want error, got %+v", got)
			}
			if !strings.Contains(err.Error(), c.token) {
				t.Fatalf("error %q does not name %q", err, c.token)
			}
		})
	}
}
