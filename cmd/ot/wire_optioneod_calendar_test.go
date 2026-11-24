package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
)

// closeCal is a calendar of ordinary session days that close at the instant of its calendar id.
type closeCal struct{ closes map[string]time.Time }

func (c closeCal) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	return clock.Session{CalendarID: id, Day: day, IsSession: true, CloseAt: c.closes[id]}, nil
}

func (c closeCal) NextSession(ctx context.Context, id string, day contracts.Day) (clock.Session, error) {
	return c.Session(ctx, id, day)
}

// calendarRun builds job id through a jobFactory holding cal and clk, plans 2026-11-19 and runs its first key.
func calendarRun(t *testing.T, id string, cal clock.Calendar, now time.Time) contracts.JobResult {
	t.Helper()
	d := newDeps()
	d.newOptionEodWatch = optioneod.NewWithWatchSymbols
	d.newOptionEodJPWatch = optioneod.NewJPWithWatch
	var (
		f   jobFactory
		b   broker.Broker
		err error
		j   contracts.Job
	)
	if id == jobOptionEodJP {
		f = jobFactory{d: d, clk: fakeClock{now}, cal: cal, inst: conidInst{r: &conidResolver{}}, eod: conidEod{}}.
			withList(listRowsOf(jpConIDEntry("1111", 910001))...)
		b = &conidBroker{ids: map[string]int64{"1111": 910001}}
		j, err = f.optionEodJP(b)
	} else {
		f = jobFactory{d: d, clk: fakeClock{now}, cal: cal, inst: storedInst{}, eod: storedEodWriter{}}.
			withList(listRowsOf(jobs.WatchEntry{Market: "US", Contract: broker.Contract{Symbol: "AAA", SecType: "STK",
				Exchange: "SMART", Currency: "USD", ConID: 1}})...)
		b = &storedBroker{}
		j, err = f.optionEodWith(b, nil)
	}
	if err != nil || j == nil {
		t.Fatalf("build %s: %v %v", id, j, err)
	}
	day := contracts.Day("2026-11-19")
	keys, _, err := j.Plan(context.Background(), day)
	if err != nil || len(keys) == 0 {
		t.Fatalf("plan %s: %v %v", id, keys, err)
	}
	key := keys[0]
	if id == jobOptionEodJP {
		key = contracts.ExpectedKey{DatasetID: "obs.option_eod_jp", SubjectKey: "1111"}
	}
	return j.Run(context.Background(), day, []contracts.ExpectedKey{key})
}

func TestOptionEodJobsGetCalendar(t *testing.T) {
	t.Parallel()
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	tk, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	cal := closeCal{closes: map[string]time.Time{
		"XNYS": time.Date(2026, 11, 19, 16, 0, 0, 0, ny),
		"XTKS": time.Date(2026, 11, 19, 15, 30, 0, 0, tk),
	}}
	cases := map[string]time.Time{
		jobOptionEod:   time.Date(2026, 11, 19, 13, 30, 0, 0, ny),
		jobOptionEodJP: time.Date(2026, 11, 19, 14, 30, 0, 0, tk),
	}
	for id, now := range cases {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			res := calendarRun(t, id, cal, now)
			if res.Err == nil || res.Err.Kind() != contracts.Retryable || !strings.Contains(res.Err.Error(), "before_slot") {
				t.Fatalf("run with the calendar: %+v", res)
			}
			if res := calendarRun(t, id, nil, now); res.Err != nil {
				t.Fatalf("run without the calendar is early: %v", res.Err)
			}
		})
	}
}
