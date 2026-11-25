package computefigures

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	cuD1 = contracts.Day("2026-11-20")
	cuD2 = contracts.Day("2026-11-23")
	cuD3 = contracts.Day("2026-11-24")
)

// catchupWorld has the sessions of three days and the points of the first.
func catchupWorld() *fx {
	f := newFx()
	for i, d := range []contracts.Day{cuD1, cuD2, cuD3} {
		at := time.Date(2026, 11, []int{20, 23, 24}[i], 21, 0, 0, 0, time.UTC)
		f.addSession(d, true, at.Add(-7*time.Hour), at)
	}
	for _, id := range []string{"VIXCLS", "VXVCLS", "SP500"} {
		f.addPoint(id, cuD1, 30)
	}
	return f
}

func runMarketDay(t *testing.T, f *fx, day contracts.Day, names ...string) {
	t.Helper()
	var keys []contracts.ExpectedKey
	for _, n := range names {
		subj := map[string]string{"market.vix_term_ratio": "VIXCLS", "market.vix_pctile_3y": "VIXCLS", "market.sp500_vs_200d": "SP500"}[n]
		keys = append(keys, keyOf(n, "market", subj))
	}
	res := rvJob(t, f, newFigs()).Run(context.Background(), day, keys)
	if res.Err != nil || res.Returned != len(names) || res.Usable != len(names) {
		t.Fatalf("run %s: %+v, want %d returned and usable, no error", day, res, len(names))
	}
}

func readingOn(t *testing.T, f *fx, name, subj string, day contracts.Day) contracts.ReadingRecord {
	t.Helper()
	rows, _ := f.LatestReadings(context.Background(), name, "market", subj, day, day)
	if len(rows) != 1 {
		t.Fatalf("%s %s: %d readings, want 1", name, day, len(rows))
	}
	return rows[0]
}

func TestMarketLatePointRecomputesEarlierDay(t *testing.T) {
	t.Parallel()
	f := catchupWorld()
	runMarketDay(t, f, cuD2, "market.vix_pctile_3y")
	if r := readingOn(t, f, "market.vix_pctile_3y", "VIXCLS", cuD2); r.Rev != 1 || tagOf(r, "data_day") != string(cuD1) {
		t.Fatalf("before the late point: rev %d data_day %s, want rev 1 of %s", r.Rev, tagOf(r, "data_day"), cuD1)
	}
	f.addPoint("VIXCLS", cuD2, 26.62)
	runMarketDay(t, f, cuD3, "market.vix_pctile_3y")
	r := readingOn(t, f, "market.vix_pctile_3y", "VIXCLS", cuD2)
	if r.Rev != 2 || tagOf(r, "data_day") != string(cuD2) {
		t.Fatalf("rev %d data_day %s, want rev 2 of %s", r.Rev, tagOf(r, "data_day"), cuD2)
	}
}

func TestMarketCatchupSameInputWritesNothing(t *testing.T) {
	t.Parallel()
	f := catchupWorld()
	runMarketDay(t, f, cuD2, "market.vix_pctile_3y")
	runMarketDay(t, f, cuD3, "market.vix_pctile_3y")
	revs := map[contracts.Day]int{}
	for _, d := range []contracts.Day{cuD1, cuD2, cuD3} {
		revs[d] = readingOn(t, f, "market.vix_pctile_3y", "VIXCLS", d).Rev
	}
	before := f.inserts
	runMarketDay(t, f, cuD3, "market.vix_pctile_3y")
	if f.inserts != before {
		t.Fatalf("%d rows written by a run with no new point, want 0", f.inserts-before)
	}
	for d, rev := range revs {
		if got := readingOn(t, f, "market.vix_pctile_3y", "VIXCLS", d).Rev; got != rev {
			t.Fatalf("%s: rev %d, want %d", d, got, rev)
		}
	}
}

func TestMarketCatchupCoversRatioAndMAGap(t *testing.T) {
	t.Parallel()
	f := catchupWorld()
	names := []string{"market.vix_term_ratio", "market.sp500_vs_200d"}
	runMarketDay(t, f, cuD2, names...)
	f.addPoint("VIXCLS", cuD2, 26.62)
	f.addPoint("SP500", cuD2, 5000)
	runMarketDay(t, f, cuD3, names...)
	for name, subj := range map[string]string{names[0]: "VIXCLS", names[1]: "SP500"} {
		if r := readingOn(t, f, name, subj, cuD2); r.Rev != 2 || tagOf(r, "data_day") != string(cuD2) {
			t.Fatalf("%s: rev %d data_day %s, want rev 2 of %s", name, r.Rev, tagOf(r, "data_day"), cuD2)
		}
	}
}

type failSeriesBefore struct {
	*fx
	day contracts.Day // a read that ends before this day fails
}

func (s failSeriesBefore) SeriesPoints(ctx context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	if to < s.day {
		return nil, errors.New("series read failed")
	}
	return s.fx.SeriesPoints(ctx, id, from, to)
}

type failSessions struct{ *fx }

func (failSessions) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	return nil, errors.New("session read failed")
}

func TestMarketCatchupFailureIsRetryableAndKeepsAsOfDay(t *testing.T) {
	t.Parallel()
	cases := map[string]func(f *fx, st *ComputeStores){
		"earlier day series": func(f *fx, st *ComputeStores) { st.Series = failSeriesBefore{fx: f, day: cuD3} },
		"session days":       func(f *fx, st *ComputeStores) { st.SessionDays = failSessions{f} },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := catchupWorld()
			st := f.rvStores()
			setup(f, &st)
			keys := []contracts.ExpectedKey{keyOf("market.vix_pctile_3y", "market", "VIXCLS"),
				keyOf("market.sp500_vs_200d", "market", "SP500")}
			res := rvJobOn(t, st, newFigs()).Run(context.Background(), cuD3, keys)
			if res.Err == nil || res.Err.Kind() != contracts.Retryable {
				t.Fatalf("Err %v, want a retryable error", res.Err)
			}
			if res.Returned != 2 || res.Usable != 2 {
				t.Fatalf("returned %d usable %d, want 2 and 2: the as-of day of every target is stored", res.Returned, res.Usable)
			}
			for _, k := range []struct{ n, s string }{{"market.vix_pctile_3y", "VIXCLS"}, {"market.sp500_vs_200d", "SP500"}} {
				readingOn(t, f, k.n, k.s, cuD3)
			}
		})
	}
}
