package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/hoursnotice"
)

// init is the one write of openSiteHours, before any test runs: a fake reader for every line.
func init() {
	openSiteHours = func(context.Context, string) (hoursnotice.Reader, error) { return fakeHoursReader{}, nil }
}

// fakeHoursReader returns two fixed notices; err, when set, fails the read.
type fakeHoursReader struct{ err error }

func (f fakeHoursReader) Notices(context.Context) ([]hoursnotice.Notice, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []hoursnotice.Notice{
		{Market: "XNYS", Day: "2026-11-27", Ours: "0930-1300", IBKR: "closed"},
		{Market: "XTKS", Day: "2026-12-30", Ours: "0900-1500", IBKR: "0900-1130"},
	}, nil
}
func (fakeHoursReader) Close() error { return nil }

func homeOverviewGaps(t *testing.T, rd siteReaders) []string {
	t.Helper()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Overview struct {
			Gaps []struct {
				Card string `json:"card"`
				Text string `json:"text"`
			} `json:"gaps"`
		} `json:"overview"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, g := range p.Overview.Gaps {
		out = append(out, g.Card+": "+g.Text)
	}
	return out
}

func TestHomeFillsHoursNotices(t *testing.T) {
	t.Parallel()
	got := homeOverviewGaps(t, siteReaders{hours: fakeHoursReader{}})
	want := []string{
		"calendar: XNYS 2026-11-27 ours 0930-1300, IBKR closed",
		"calendar: XTKS 2026-12-30 ours 0900-1500, IBKR 0900-1130",
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("gaps %v, want %v", got, want)
	}
}

func TestHomeNilHoursReader(t *testing.T) {
	t.Parallel()
	if got := homeOverviewGaps(t, siteReaders{}); len(got) != 0 {
		t.Fatalf("gaps %v with a nil reader, want none", got)
	}
	if got := homeOverviewGaps(t, siteReaders{hours: fakeHoursReader{err: errors.New("down")}}); len(got) != 0 {
		t.Fatalf("gaps %v with a failed read, want none", got)
	}
}
