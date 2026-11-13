package calendarhours

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/hoursnotice"
)

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC) }

type fakeBroker struct {
	broker.Broker
	res   broker.Resolved
	err   error
	calls int
	got   broker.Contract
}

func (b *fakeBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	b.calls++
	b.got = c
	return b.res, b.err
}

type fakeSource struct{ rows []contracts.SessionDay }

func (s fakeSource) SessionDays(_ context.Context, cal string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	var out []contracts.SessionDay
	for _, r := range s.rows {
		d := contracts.Day(r.SessionDay.Format(dayLayout))
		if r.CalendarID == cal && d >= from && d <= to {
			out = append(out, r)
		}
	}
	return out, nil
}

type fakeParser struct {
	days []clock.HoursDay
	err  error
}

func (p fakeParser) Parse(string, string) ([]clock.HoursDay, error) { return p.days, p.err }

type fakeStore struct {
	rows     map[string][]hoursnotice.Notice
	replaces int
}

func (s *fakeStore) Notices(context.Context) ([]hoursnotice.Notice, error) { return nil, nil }
func (s *fakeStore) Close() error                                          { return nil }
func (s *fakeStore) Replace(_ context.Context, m string, rows []hoursnotice.Notice) error {
	s.replaces++
	s.rows[m] = rows
	return nil
}

var ny = func() *time.Location { l, _ := time.LoadLocation("America/New_York"); return l }()

func at(day string, h, m int) time.Time {
	d, _ := time.ParseInLocation(dayLayout, day, ny)
	return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, ny).UTC()
}

func ourDay(day string, open bool, oh, om, ch, cm int) contracts.SessionDay {
	d, _ := time.Parse(dayLayout, day)
	r := contracts.SessionDay{CalendarID: "XNYS", SessionDay: types.SerializableDate{Time: d}, IsSession: open}
	if open {
		r.OpenAt, r.CloseAt = at(day, oh, om), at(day, ch, cm)
	}
	return r
}

func ibkrDay(day string, open bool, oh, om, ch, cm int) clock.HoursDay {
	h := clock.HoursDay{Day: contracts.Day(day), IsSession: open}
	if open {
		h.OpenAt, h.CloseAt = at(day, oh, om), at(day, ch, cm)
	}
	return h
}

type rig struct {
	j    contracts.Job
	br   *fakeBroker
	st   *fakeStore
	logs *bytes.Buffer
}

func newRig(t *testing.T, mkt string, ours []contracts.SessionDay, p fakeParser) *rig {
	t.Helper()
	r := &rig{br: &fakeBroker{res: broker.Resolved{Listed: true, LiquidHours: "x", TimeZoneID: "US/Eastern"}},
		st: &fakeStore{rows: map[string][]hoursnotice.Notice{}}, logs: &bytes.Buffer{}}
	log := slog.New(slog.NewTextHandler(r.logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	j, err := New(mkt, r.br, fakeSource{ours}, p, r.st, fakeClock{}, log)
	if err != nil {
		t.Fatal(err)
	}
	r.j = j
	return r
}

func (r *rig) run() contracts.JobResult {
	return r.j.Run(context.Background(), "2026-11-25", nil)
}

func TestEqualDaysWriteNothingAndLogNothing(t *testing.T) {
	t.Parallel()
	r := newRig(t, "XNYS", []contracts.SessionDay{ourDay("2026-11-25", true, 9, 30, 16, 0)},
		fakeParser{days: []clock.HoursDay{ibkrDay("2026-11-25", true, 9, 30, 16, 0)}})
	if res := r.run(); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(r.st.rows["XNYS"]) != 0 || r.logs.Len() != 0 {
		t.Fatalf("rows %v logs %q; want none", r.st.rows, r.logs)
	}
	if r.br.got.Symbol != "SPY" || r.br.calls != 1 {
		t.Fatalf("resolved %+v %d times", r.br.got, r.br.calls)
	}
}

func TestDifferencesLogOneLineAndWriteOneRow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ibkr clock.HoursDay
		line string
		want hoursnotice.Notice
	}{
		{"closed", ibkrDay("2026-11-27", false, 0, 0, 0, 0), "ours=0930-1600 ibkr=CLOSED",
			hoursnotice.Notice{Market: "XNYS", Day: "2026-11-27", Ours: "0930-1600", IBKR: "CLOSED"}},
		{"early close", ibkrDay("2026-11-27", true, 9, 30, 13, 0), "ours=0930-1600 ibkr=0930-1300",
			hoursnotice.Notice{Market: "XNYS", Day: "2026-11-27", Ours: "0930-1600", IBKR: "0930-1300"}},
		{"open time", ibkrDay("2026-11-27", true, 10, 0, 16, 0), "ours=0930-1600 ibkr=1000-1600",
			hoursnotice.Notice{Market: "XNYS", Day: "2026-11-27", Ours: "0930-1600", IBKR: "1000-1600"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, "XNYS", []contracts.SessionDay{ourDay("2026-11-27", true, 9, 30, 16, 0)}, fakeParser{days: []clock.HoursDay{c.ibkr}})
			if res := r.run(); res.Err != nil {
				t.Fatal(res.Err)
			}
			if got := r.st.rows["XNYS"]; len(got) != 1 || got[0] != c.want {
				t.Fatalf("rows = %v; want [%v]", got, c.want)
			}
			out := strings.TrimSpace(r.logs.String())
			if strings.Count(out, "\n") != 0 || !strings.Contains(out, "calendar hours differ") ||
				!strings.Contains(out, "market=XNYS day=2026-11-27") || !strings.Contains(out, c.line) {
				t.Fatalf("log = %q", out)
			}
		})
	}
}

func TestDayWithoutOurRowIsNotCompared(t *testing.T) {
	t.Parallel()
	r := newRig(t, "XNYS", nil, fakeParser{days: []clock.HoursDay{ibkrDay("2026-11-27", false, 0, 0, 0, 0)}})
	if res := r.run(); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(r.st.rows["XNYS"]) != 0 || r.logs.Len() != 0 || r.st.replaces != 1 {
		t.Fatalf("rows %v logs %q replaces %d", r.st.rows, r.logs, r.st.replaces)
	}
}

func TestDayThatNoLongerDiffersIsGone(t *testing.T) {
	t.Parallel()
	p := &fakeParser{days: []clock.HoursDay{ibkrDay("2026-11-27", false, 0, 0, 0, 0)}}
	r := &rig{br: &fakeBroker{res: broker.Resolved{Listed: true}}, st: &fakeStore{rows: map[string][]hoursnotice.Notice{}}, logs: &bytes.Buffer{}}
	ours := []contracts.SessionDay{ourDay("2026-11-27", true, 9, 30, 16, 0)}
	pp := &switchParser{p: p}
	j, err := New("XNYS", r.br, fakeSource{ours}, pp, r.st, fakeClock{}, slog.New(slog.NewTextHandler(r.logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), "d", nil); res.Err != nil || len(r.st.rows["XNYS"]) != 1 {
		t.Fatalf("first run: %v %v", res.Err, r.st.rows)
	}
	p.days = []clock.HoursDay{ibkrDay("2026-11-27", true, 9, 30, 16, 0)}
	if res := j.Run(context.Background(), "d", nil); res.Err != nil || len(r.st.rows["XNYS"]) != 0 {
		t.Fatalf("second run: %v %v", res.Err, r.st.rows)
	}
}

type switchParser struct{ p *fakeParser }

func (s *switchParser) Parse(a, b string) ([]clock.HoursDay, error) { return s.p.Parse(a, b) }

type kindErr struct{}

func (kindErr) Error() string             { return "boom" }
func (kindErr) Kind() contracts.ErrorKind { return contracts.Retryable }

func TestFailedResolveKeepsRowsAndNamesMarketAndCode(t *testing.T) {
	t.Parallel()
	r := newRig(t, "XTKS", nil, fakeParser{})
	r.br.err = kindErr{}
	res := r.run()
	if res.Err == nil || !strings.Contains(res.Err.Error(), "XTKS") || !strings.Contains(res.Err.Error(), "code "+string(contracts.Retryable)) {
		t.Fatalf("err = %v", res.Err)
	}
	if r.st.replaces != 0 || r.br.got.Symbol != "7203" || r.br.got.Exchange != "TSEJ" {
		t.Fatalf("replaces %d contract %+v", r.st.replaces, r.br.got)
	}
}

func TestUnparsableStringFailsRun(t *testing.T) {
	t.Parallel()
	r := newRig(t, "XNYS", nil, fakeParser{err: errors.New("bad token")})
	res := r.run()
	if res.Err == nil || !strings.Contains(res.Err.Error(), "XNYS") || res.Err.Kind() != contracts.Permanent || r.st.replaces != 0 {
		t.Fatalf("err %v replaces %d", res.Err, r.st.replaces)
	}
}

func TestPlanIsOwnKeyAndManifests(t *testing.T) {
	t.Parallel()
	for mkt, id := range map[string]string{"XNYS": "calendar_hours_us", "XTKS": "calendar_hours_jp"} {
		r := newRig(t, mkt, nil, fakeParser{})
		keys, _, err := r.j.Plan(context.Background(), "2026-11-25")
		if err != nil || len(keys) != 1 || keys[0].DatasetID != "master.session_hours_notice" || keys[0].SubjectKey != mkt {
			t.Fatalf("%s: keys %v err %v", mkt, keys, err)
		}
		if r.j.ID() != id {
			t.Fatalf("id = %s; want %s", r.j.ID(), id)
		}
		s := r.j.(interface {
			Schedule() (string, string, string, time.Duration)
			RunBound() time.Duration
			RestartSensitive() bool
		})
		cal, anchor, _, off := s.Schedule()
		if cal != mkt || anchor != "session_open" || off != -20*time.Minute || s.RunBound() != 5*time.Minute || s.RestartSensitive() {
			t.Fatalf("%s schedule %s %s %v", mkt, cal, anchor, off)
		}
	}
	if _, err := New("XLON", nil, nil, nil, nil, nil, nil); err == nil {
		t.Fatal("unknown market accepted")
	}
}
