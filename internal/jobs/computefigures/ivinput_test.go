package computefigures

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	dayD = contracts.Day("2026-11-06")
	dayE = contracts.Day("2026-12-04")
)

var closeD = time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)

// fx is one fake of every store the job reads and writes.
type fx struct {
	latest     []store.OptionEodLatest
	eod        map[int64][]contracts.OptionEod
	terms      map[int64]store.OptionTerms
	termsCalls [][]int64
	names      map[int64]string
	sessions   []contracts.SessionDay
	series     map[string][]contracts.SeriesPoint
	readings   []contracts.ReadingRecord
	inserts    int
}

func newFx() *fx {
	return &fx{eod: map[int64][]contracts.OptionEod{}, terms: map[int64]store.OptionTerms{}, names: map[int64]string{},
		series: map[string][]contracts.SeriesPoint{}}
}

func (f *fx) stores() jobs.ComputeStores {
	return jobs.ComputeStores{Catalog: f, OptionEod: f, Terms: f, Names: f, SessionDays: f, Series: f, Readings: f}
}

func (f *fx) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	return f.latest, nil
}

func (f *fx) OptionEod(_ context.Context, id int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	var out []contracts.OptionEod
	for _, r := range f.eod[id] {
		if d := dayOf(r.SessionDay); d >= from && d <= to {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fx) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	f.termsCalls = append(f.termsCalls, ids)
	var out []store.OptionTerms
	for _, id := range ids {
		if t, ok := f.terms[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fx) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, id := range ids {
		if n, ok := f.names[id]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Name: n})
		}
	}
	return out, nil
}

func (f *fx) SessionDays(_ context.Context, _ string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	var out []contracts.SessionDay
	for _, s := range f.sessions {
		if d := dayOf(s.SessionDay); d >= from && d <= to {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fx) SeriesPoints(_ context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for _, p := range f.series[id] {
		if d := dayOf(p.ObservationDate); d >= from && d <= to {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fx) LatestReadings(_ context.Context, name, kind, subj string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	newest := map[contracts.Day]contracts.ReadingRecord{}
	for _, r := range f.readings {
		d := dayOf(r.Day)
		if r.Name != name || r.SubjectKind != kind || r.Subject != subj || d < from || d > to {
			continue
		}
		if old, ok := newest[d]; !ok || r.Rev > old.Rev {
			newest[d] = r
		}
	}
	var out []contracts.ReadingRecord
	for _, r := range newest {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return dayOf(out[i].Day) < dayOf(out[j].Day) })
	return out, nil
}

func (f *fx) InsertReadings(_ context.Context, rows []contracts.ReadingRecord) (int, error) {
	f.readings = append(f.readings, rows...)
	f.inserts += len(rows)
	return len(rows), nil
}

func (f *fx) Close() error { return nil }

func (f *fx) addSession(day contracts.Day, isSession bool, open, closeAt time.Time) {
	f.sessions = append(f.sessions, contracts.SessionDay{CalendarID: "XNYS", SessionDay: dateOf(day), IsSession: isSession,
		OpenAt: open, CloseAt: closeAt})
}

func fp(v float64) *float64 { return &v }

func (f *fx) addRate(day contracts.Day, percent float64) {
	f.addPoint("DGS3MO", day, percent)
}

func (f *fx) addPoint(series string, day contracts.Day, v float64) {
	f.series[series] = append(f.series[series], contracts.SeriesPoint{SeriesID: series, ObservationDate: dateOf(day),
		RealtimeStart: dateOf(day), RealtimeEnd: dateOf(day), Status: "present", Value: fp(v)})
}

func eodRow(inst int, bid, ask *float64, iv *float64, at time.Time) contracts.OptionEod {
	return contracts.OptionEod{InstrumentID: inst, UnderlyingID: 1, Status: 1, Bid: bid, Ask: ask, IvDecimal: iv,
		ReceivedAt: at, SessionDay: dateOf(dayD)}
}

func (f *fx) addOption(inst int, right, strike string, expiry contracts.Day, row contracts.OptionEod) {
	f.eod[1] = append(f.eod[1], row)
	f.terms[int64(inst)] = store.OptionTerms{InstrumentID: int64(inst), Right: right, Strike: strike, ExpirationDate: expiry,
		SettlementType: "PM"}
}

// world has SPY (id 1) with a call and a put at 500 expiring on dayE, sessions for both days and a rate of the day.
func world() *fx {
	f := newFx()
	f.latest = []store.OptionEodLatest{{UnderlyingID: 1, SessionDay: dayD}}
	f.names[1] = "SPY"
	f.addSession(dayD, true, closeD.Add(-6*time.Hour-30*time.Minute), closeD)
	f.addSession(dayE, true, closeD.AddDate(0, 0, 28).Add(-6*time.Hour-30*time.Minute), closeD.AddDate(0, 0, 28))
	at := closeD.Add(time.Minute)
	f.addOption(100, "C", "500", dayE, eodRow(100, fp(9), fp(11), fp(0.21), at))
	f.addOption(101, "P", "500", dayE, eodRow(101, fp(8), fp(10), fp(0.19), at))
	f.addRate(dayD, 4)
	return f
}

func testInputs(f *fx) inputs {
	return inputs{ComputeStores: f.stores(), ops: own.NewSeriesOps(), units: own.NewUnits(), rateSeries: "DGS3MO", rateMaxAge: 4}
}

func mustChain(t *testing.T, f *fx) []compute.ExpiryQuotes {
	t.Helper()
	chain, _, err := ivInput(context.Background(), testInputs(f), 1, dayD)
	if err != nil {
		t.Fatalf("ivInput: %v", err)
	}
	return chain
}

func TestIvInputLatestReceivedWins(t *testing.T) {
	t.Parallel()
	f := world()
	f.eod[1] = append(f.eod[1], eodRow(100, fp(19), fp(21), fp(0.30), closeD.Add(time.Hour)))
	f.eod[1] = append(f.eod[1], eodRow(100, fp(1), fp(3), fp(0.10), closeD.Add(-time.Hour)))
	chain := mustChain(t, f)
	q := chain[0].Strikes[0]
	if q.CallMid != 20 || q.CallIV != 0.30 {
		t.Fatalf("call mid %v iv %v, want the row received last (20, 0.30)", q.CallMid, q.CallIV)
	}
}

func TestIvInputOnlyStatusOk(t *testing.T) {
	t.Parallel()
	f := world()
	f.eod[1][0].Status = 4
	chain := mustChain(t, f)
	q := chain[0].Strikes[0]
	if q.HasCall || !q.HasPut {
		t.Fatalf("HasCall %v HasPut %v, want only the put (the call row has status 4)", q.HasCall, q.HasPut)
	}
}

func TestIvInputMidRules(t *testing.T) {
	t.Parallel()
	f := world()
	f.eod[1][0].Bid = fp(0)                           // a zero bid: no call mid
	f.eod[1][1].Bid, f.eod[1][1].Ask = fp(12), fp(10) // crossed: no put mid
	q := mustChain(t, f)[0].Strikes[0]
	if q.HasCall || q.HasPut {
		t.Fatalf("HasCall %v HasPut %v, want both absent", q.HasCall, q.HasPut)
	}
	f = world()
	q = mustChain(t, f)[0].Strikes[0]
	if !q.HasCall || q.CallMid != 10 || !q.HasPut || q.PutMid != 9 {
		t.Fatalf("mids %v %v, want 10 and 9", q.CallMid, q.PutMid)
	}
}

func TestIvInputNoIvIsZero(t *testing.T) {
	t.Parallel()
	f := world()
	f.eod[1][0].IvDecimal = nil
	f.eod[1][1].IvDecimal = fp(-1)
	q := mustChain(t, f)[0].Strikes[0]
	if q.CallIV != 0 || q.PutIV != 0 {
		t.Fatalf("IVs %v %v, want 0 and 0", q.CallIV, q.PutIV)
	}
}

func TestIvInputTermsChunkOf500(t *testing.T) {
	t.Parallel()
	f := world()
	f.eod[1], f.terms = nil, map[int64]store.OptionTerms{}
	for i := 0; i < 501; i++ {
		f.addOption(1000+i, "C", "500", dayE, eodRow(1000+i, fp(9), fp(11), fp(0.2), closeD))
	}
	mustChain(t, f)
	if len(f.termsCalls) != 2 || len(f.termsCalls[0]) != 500 || len(f.termsCalls[1]) != 1 {
		t.Fatalf("terms calls %d, want two of 500 and 1", len(f.termsCalls))
	}
}

func TestIvInputAMSettlementUsesOpen(t *testing.T) {
	t.Parallel()
	f := world()
	tm := f.terms[100]
	tm.SettlementType = "am"
	f.terms[100] = tm
	f.terms[101] = func() store.OptionTerms { x := f.terms[101]; x.SettlementType = "AM"; return x }()
	chain := mustChain(t, f)
	want := 28*1440 - 6*60 - 30 // the open of the expiry day minus the close of the as-of day
	if len(chain) != 1 || chain[0].MinutesToExpiry != want {
		t.Fatalf("minutes %+v, want %d from the open", chain, want)
	}
}

func TestIvInputNonSessionExpiryDropped(t *testing.T) {
	t.Parallel()
	f := world()
	f.sessions[1].IsSession = false
	if chain := mustChain(t, f); len(chain) != 0 {
		t.Fatalf("chain %+v, want the expiry dropped", chain)
	}
}

func TestIvInputMinutesFloored(t *testing.T) {
	t.Parallel()
	f := world()
	f.sessions[1].CloseAt = f.sessions[1].CloseAt.Add(59 * time.Second)
	if got := mustChain(t, f)[0].MinutesToExpiry; got != 28*1440 {
		t.Fatalf("minutes %d, want 40320 (59 seconds floored)", got)
	}
}

func TestIvInputRateIsNewestNotAfterDay(t *testing.T) {
	t.Parallel()
	f := world()
	f.series["DGS3MO"] = nil
	f.addRate("2026-11-04", 3)
	f.addRate("2026-11-05", 5)
	f.addRate("2026-11-09", 9)
	_, rate, err := ivInput(context.Background(), testInputs(f), 1, dayD)
	if err != nil || rate.Day != "2026-11-05" || rate.Value != 0.05 {
		t.Fatalf("rate %+v err %v, want 2026-11-05 at 0.05", rate, err)
	}
}

func TestIvInputStaleRateRules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, tc := range []struct {
		age  int
		want error
	}{{0, nil}, {1, nil}, {4, nil}, {5, errStaleRate}} {
		f := world()
		f.series["DGS3MO"] = nil
		f.addRate(shiftDay(dayD, -tc.age), 4)
		if _, _, err := ivInput(ctx, testInputs(f), 1, dayD); !errors.Is(err, tc.want) {
			t.Fatalf("age %d: err %v, want %v", tc.age, err, tc.want)
		}
	}
	f := world()
	f.series["DGS3MO"] = nil
	if _, _, err := ivInput(ctx, testInputs(f), 1, dayD); !errors.Is(err, errNoRate) {
		t.Fatalf("no point: err %v, want errNoRate", err)
	}
	for _, tc := range []struct {
		age       int
		state     string
		code      int
		staleRate string
	}{{5, "missing_input", 3, "true"}, {4, "ok", 0, "true"}, {1, "ok", 0, "true"}, {0, "ok", 0, "false"}} {
		f := world()
		f.series["DGS3MO"] = nil
		f.addRate(shiftDay(dayD, -tc.age), 4)
		rec := runIv30(t, f)
		if string(rec.State) != tc.state || (tc.code != 0 && (rec.ReasonCode == nil || *rec.ReasonCode != tc.code)) {
			t.Fatalf("age %d: state %s code %v, want %s %d", tc.age, rec.State, rec.ReasonCode, tc.state, tc.code)
		}
		if got := tagOf(rec, "stale_rate"); got != tc.staleRate {
			t.Fatalf("age %d: stale_rate %q, want %q", tc.age, got, tc.staleRate)
		}
	}
}

// runIv30 runs the IV30 key of SPY on dayD and returns its stored reading.
func runIv30(t *testing.T, f *fx) contracts.ReadingRecord {
	t.Helper()
	j := newJob(t, f, newFigs())
	res := j.Run(context.Background(), dayD, []contracts.ExpectedKey{keyOf("market.iv30_own", "underlying", "SPY")})
	if res.Err != nil || res.Usable != 1 {
		t.Fatalf("run: %+v", res)
	}
	rows, _ := f.LatestReadings(context.Background(), "market.iv30_own", "underlying", "SPY", dayD, dayD)
	if len(rows) != 1 {
		t.Fatalf("readings %d, want 1", len(rows))
	}
	return rows[0]
}

func tagOf(r contracts.ReadingRecord, name string) string {
	for _, p := range r.Parameters {
		if p.Param == name && p.ValueTag != nil {
			return *p.ValueTag
		}
	}
	return ""
}

func newJob(t *testing.T, f *fx, figs compute.Figures) contracts.Job {
	t.Helper()
	j, err := New(f.stores(), figs, rules.New(), own.NewUnits(), own.NewSeriesOps(), clockwork.NewFakeClockAt(closeD.Add(20*time.Minute)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return j
}
