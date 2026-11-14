package fxrefresh

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

type fakeSource struct {
	mu    sync.Mutex
	plans []sources.FetchPlan
	pts   []contracts.SeriesPoint
	err   error
}

func (f *fakeSource) Fetch(_ context.Context, p sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.plans = append(f.plans, p)
	return f.pts, f.err
}

func fpt(day string, v *float64) contracts.SeriesPoint {
	d, _ := time.Parse(dayLayout, day)
	return contracts.SeriesPoint{SeriesID: fredSeries, ObservationDate: types.SerializableDate{Time: d}, Value: v}
}

func fv(v float64) *float64 { return &v }

func withSource(t *testing.T, h *fakeHistory, st *memFx, src *fakeSource) contracts.Job {
	t.Helper()
	j := buildJob(t, &fakeBroker{listed: true}, h, st)
	if !AttachSecondary(j, src) {
		t.Fatal("attach refused")
	}
	return j
}

func TestFillInsertsFredPointsWithValue(t *testing.T) {
	t.Parallel()
	src := &fakeSource{pts: []contracts.SeriesPoint{fpt("2026-11-05", fv(151.1)), fpt("2026-11-06", fv(152.25))}}
	st := &memFx{}
	j := withSource(t, &fakeHistory{}, st, src)
	res := j.Run(context.Background(), runDay, usdjpy)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	p := src.plans[0]
	if len(src.plans) != 1 || len(p.SeriesIDs) != 1 || p.SeriesIDs[0] != "DEXJPUS" || p.From != "2026-10-26" || p.To != runDay {
		t.Fatalf("plans %+v", src.plans)
	}
	if len(st.rows) != 2 || st.rows[0].Rate != "151.1" || st.rows[1].Rate != "152.25" {
		t.Fatalf("rows %+v", st.rows)
	}
	for _, r := range st.rows {
		if r.Source != "fred_h10" || r.Pair != "USDJPY" || r.ReceivedAt.IsZero() {
			t.Fatalf("row %+v", r)
		}
	}
	if st.upserts != 0 {
		t.Fatalf("fill must not upsert: %d", st.upserts)
	}
}

func TestFillNeverReplacesStoredRate(t *testing.T) {
	t.Parallel()
	src := &fakeSource{pts: []contracts.SeriesPoint{fpt("2026-11-06", fv(1.5)), fpt("2026-11-05", fv(151.1))}}
	st := &memFx{rows: []contracts.FxRate{stored("2026-11-06", "152.2000")}}
	j := withSource(t, &fakeHistory{}, st, src)
	j.Run(context.Background(), runDay, usdjpy)
	if len(st.rows) != 2 || st.rows[0].Rate != "152.2000" || st.rows[0].Source != sourceMidpoint {
		t.Fatalf("rows %+v", st.rows)
	}
}

func TestNoValuePointSkipped(t *testing.T) {
	t.Parallel()
	src := &fakeSource{pts: []contracts.SeriesPoint{fpt("2026-11-07", nil), fpt("2026-11-06", fv(152.2))}}
	st := &memFx{}
	withSource(t, &fakeHistory{}, st, src).Run(context.Background(), runDay, usdjpy)
	if len(st.rows) != 1 || st.rows[0].RateDay.Format(dayLayout) != "2026-11-06" {
		t.Fatalf("rows %+v", st.rows)
	}
}

func TestNonPositiveFredValueSkipped(t *testing.T) {
	t.Parallel()
	src := &fakeSource{pts: []contracts.SeriesPoint{
		fpt("2026-11-03", nil), fpt("2026-11-04", fv(0)), fpt("2026-11-05", fv(-1.5)), fpt("2026-11-06", fv(153.42)),
	}}
	st := &memFx{}
	withSource(t, &fakeHistory{}, st, src).Run(context.Background(), runDay, usdjpy)
	if len(st.rows) != 1 || st.rows[0].Rate != "153.42" {
		t.Fatalf("rows %+v", st.rows)
	}
}

func TestFillRunsWhenPrimaryFails(t *testing.T) {
	t.Parallel()
	for name, h := range map[string]*fakeHistory{
		"error": {err: errors.New("socket closed")},
		"empty": {},
	} {
		src := &fakeSource{pts: []contracts.SeriesPoint{fpt("2026-11-06", fv(152.2))}}
		st := &memFx{}
		res := withSource(t, h, st, src).Run(context.Background(), runDay, usdjpy)
		if (name == "error") != (res.Err != nil) || res.Returned != 0 {
			t.Fatalf("%s: result %+v", name, res)
		}
		if len(st.rows) != 1 || st.rows[0].Source != "fred_h10" {
			t.Fatalf("%s: rows %+v", name, st.rows)
		}
	}
}

func TestSecondaryFailureKeepsResult(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{bars: []broker.Bar{mkbar("2026-11-09", "153.3000")}}
	want := buildJob(t, &fakeBroker{listed: true}, h, &memFx{}).Run(context.Background(), runDay, usdjpy)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	fetchFail := &fakeSource{err: errors.New("fred down")}
	j := withSource(t, h, &memFx{}, fetchFail)
	j.(*job).log = log
	if got := j.Run(context.Background(), runDay, usdjpy); got != want {
		t.Fatalf("fetch failure: %+v want %+v", got, want)
	}
	if !strings.Contains(buf.String(), "fill source failed") {
		t.Fatalf("not logged: %q", buf.String())
	}

	buf.Reset()
	ins := &failInsert{}
	j2, _ := New(&fakeBroker{listed: true}, h, ins, ins, fixedClock{})
	AttachSecondary(j2, &fakeSource{pts: []contracts.SeriesPoint{fpt("2026-11-06", fv(152.2))}})
	j2.(*job).log = log
	if got := j2.Run(context.Background(), runDay, usdjpy); got.Err != nil || got.Returned != 1 {
		t.Fatalf("insert failure: %+v", got)
	}
	if !strings.Contains(buf.String(), "fill insert failed") {
		t.Fatalf("not logged: %q", buf.String())
	}
}

// failInsert is a store whose insert fails.
type failInsert struct{ memFx }

func (f *failInsert) InsertFxRates(context.Context, []contracts.FxRate) (int, error) {
	return 0, errors.New("db down")
}

func TestNoSourceNoCall(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{bars: []broker.Bar{mkbar("2026-11-09", "153.3000")}}
	st := &memFx{}
	res := buildJob(t, &fakeBroker{listed: true}, h, st).Run(context.Background(), runDay, usdjpy)
	if res.Err != nil || res.Returned != 1 || st.calls != 0 {
		t.Fatalf("result %+v inserts %d", res, st.calls)
	}
}

type foreignJob struct{ contracts.Job }

func TestAttachSecondaryForeignJob(t *testing.T) {
	t.Parallel()
	if AttachSecondary(foreignJob{}, &fakeSource{}) {
		t.Fatal("a foreign job must not take a source")
	}
	if AttachSecondary(nil, &fakeSource{}) {
		t.Fatal("nil job must not take a source")
	}
}

func TestNewRefusesBadSourceList(t *testing.T) {
	t.Parallel()
	const head = "version = 1\nid = \"fx_refresh\"\nduration_days = 5\nfill_lookback_days = 14\n"
	const tail = "[schedule]\ncalendar = \"XNYS\"\nanchor = \"session_close\"\noffset = \"15m\"\nrun_bound = \"10m\"\n"
	for name, list := range map[string]string{
		"missing": "",
		"empty":   "sources = []\n",
		"order":   "sources = [\"fred_h10\", \"ibkr_midpoint\"]\n",
		"unknown": "sources = [\"ibkr_midpoint\", \"stooq\"]\n",
	} {
		st := &memFx{}
		if _, err := build([]byte(head+list+tail), &fakeBroker{}, &fakeHistory{}, st, st, fixedClock{}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	st := &memFx{}
	if _, err := build([]byte(head+"sources = [\"ibkr_midpoint\", \"fred_h10\"]\n"+tail), &fakeBroker{}, &fakeHistory{}, st, st, fixedClock{}); err != nil {
		t.Fatal(err)
	}
	if _, err := New(&fakeBroker{}, &fakeHistory{}, st, st, fixedClock{}); err != nil {
		t.Fatalf("embedded manifest: %v", err)
	}
}
