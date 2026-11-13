package statementcapture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/flexsection"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

var recorded = time.Date(2026, 11, 6, 3, 0, 0, 0, time.UTC)

type attrs map[string]string

func jsonOf(a attrs) string {
	b, _ := json.Marshal(a)
	return string(b)
}

type fakeSections struct {
	rows     map[string][]flexsection.Row // alias|section
	statuses map[string][]flexsection.Status
}

func newSections() *fakeSections {
	return &fakeSections{rows: map[string][]flexsection.Row{}, statuses: map[string][]flexsection.Status{}}
}

func (f *fakeSections) add(alias, section, key string, a attrs, at time.Time) {
	k := alias + "|" + section
	f.rows[k] = append(f.rows[k], flexsection.Row{Alias: alias, Section: section, RowKey: key, Attributes: jsonOf(a), RecordedAt: at})
}

func (f *fakeSections) present(alias string, sections ...string) {
	for _, s := range sections {
		f.statuses[alias] = append(f.statuses[alias], flexsection.Status{Alias: alias, Section: s, State: "present"})
	}
}

func (f *fakeSections) Rows(_ context.Context, alias, section string) ([]flexsection.Row, error) {
	return f.rows[alias+"|"+section], nil
}

func (f *fakeSections) Statuses(_ context.Context, alias string) ([]flexsection.Status, error) {
	return f.statuses[alias], nil
}

type fakeAccount struct {
	mu       sync.Mutex
	captures map[string]store.Capture
	order    []string
	err      error
}

func (w *fakeAccount) WriteCapture(_ context.Context, c store.Capture) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	if _, ok := w.captures[c.ID]; !ok {
		w.captures[c.ID] = c
		w.order = append(w.order, c.ID)
	}
	return nil
}

func (w *fakeAccount) WriteExecutions(context.Context, string, []store.Execution) error {
	return errors.New("not used")
}

type fakeMarks struct {
	mu   sync.Mutex
	rows []mark.PositionMark
	err  error
}

func (m *fakeMarks) WriteMarks(_ context.Context, rows []mark.PositionMark) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.rows = append(m.rows, rows...)
	return nil
}

type fakeCalendar struct{ closed map[string]bool }

func (c fakeCalendar) Session(_ context.Context, id string, d contracts.Day) (clock.Session, error) {
	return clock.Session{CalendarID: id, Day: d, IsSession: !c.closed[string(d)]}, nil
}

func (c fakeCalendar) NextSession(context.Context, string, contracts.Day) (clock.Session, error) {
	return clock.Session{}, errors.New("not used")
}

type rig struct {
	sec *fakeSections
	acc *fakeAccount
	mk  *fakeMarks
	cal fakeCalendar
	log *bytes.Buffer
}

func newRig() *rig {
	return &rig{sec: newSections(), acc: &fakeAccount{captures: map[string]store.Capture{}}, mk: &fakeMarks{},
		cal: fakeCalendar{closed: map[string]bool{}}, log: &bytes.Buffer{}}
}

func (r *rig) run(t *testing.T, aliases []string, day string) contracts.JobResult {
	t.Helper()
	j, err := newJob(Config{Aliases: aliases, Sections: r.sec, Account: r.acc, Marks: r.mk, Calendar: r.cal,
		Clock: clockwork.NewFakeClockAt(time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC))},
		slog.New(slog.NewTextHandler(r.log, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return j.Run(context.Background(), contracts.Day(day), nil)
}

// seed stores a full statement of one stock and one option for alias on date (both date forms by form).
func (r *rig) seed(alias, date string) {
	r.sec.present(alias, sectionPosition, sectionEquity, sectionNAV)
	r.sec.add(alias, sectionPosition, date+"|SUMMARY|1", attrs{
		"reportDate": date, "levelOfDetail": "SUMMARY", "conid": "1", "assetCategory": "STK", "symbol": "XYZ",
		"currency": "USD", "position": "100", "costBasisPrice": "50.5", "markPrice": "52.25",
	}, recorded)
	r.sec.add(alias, sectionPosition, date+"|SUMMARY|2", attrs{
		"reportDate": date, "levelOfDetail": "SUMMARY", "conid": "2", "assetCategory": "OPT", "symbol": "XYZ 261120P00050000",
		"underlyingSymbol": "XYZ", "currency": "USD", "position": "-2", "costBasisPrice": "1.25", "markPrice": "0.9",
		"multiplier": "100", "strike": "50", "expiry": "20261120", "putCall": "P",
	}, recorded.Add(time.Minute))
	r.sec.add(alias, sectionEquity, date, attrs{"reportDate": date, "total": "12345.67", "cash": "800.5"}, recorded)
	r.sec.add(alias, sectionNAV, "a|b", attrs{"fromDate": "20261101", "toDate": "20261106", "currency": "EUR"}, recorded)
}

func TestWritesOneCapturePerReportDay(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261105")
	r.seed("kid", "20261104")
	res := r.run(t, []string{"kid"}, "2026-11-06")
	if res.Err != nil || res.Usable != 2 || res.Returned != 2 {
		t.Fatalf("result %+v", res)
	}
	if len(r.acc.order) != 2 || r.acc.order[0] != "kid:2026-11-05:flex" || r.acc.order[1] != "kid:2026-11-04:flex" {
		t.Fatalf("order newest first, got %v", r.acc.order)
	}
	c := r.acc.captures["kid:2026-11-05:flex"]
	if len(c.Positions) != 2 || len(c.Values) != 2 {
		t.Fatalf("capture %+v", c)
	}
	if !c.At.Equal(recorded.Add(time.Minute)) {
		t.Fatalf("At is the newest RecordedAt, got %v", c.At)
	}
	if c.Values[0] != (store.AccountValue{Tag: "NetLiquidation", Value: "12345.67", Currency: "EUR"}) {
		t.Fatalf("value %+v", c.Values[0])
	}
}

func TestCaptureIdIsAliasDayFlex(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "2026-11-05")
	r.run(t, []string{"kid"}, "2026-11-06")
	c, ok := r.acc.captures["kid:2026-11-05:flex"]
	if !ok || c.Alias != "kid" || c.SessionDay != "2026-11-05" || c.Status != "ok" {
		t.Fatalf("captures %+v", r.acc.captures)
	}
}

func TestWritesOnlyConfiguredAliases(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261105")
	r.seed("main", "20261105")
	r.run(t, []string{"kid"}, "2026-11-06")
	if len(r.acc.captures) != 1 {
		t.Fatalf("captures %v", r.acc.order)
	}
	if _, ok := r.acc.captures["kid:2026-11-05:flex"]; !ok {
		t.Fatal("kid missing")
	}
}

func TestNoSectionPresentWritesNothing(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261105")
	r.sec.statuses["kid"] = []flexsection.Status{
		{Section: sectionPosition, State: "present"}, {Section: sectionEquity, State: "absent"},
	}
	res := r.run(t, []string{"kid"}, "2026-11-06")
	if len(r.acc.captures) != 0 || res.Err != nil {
		t.Fatalf("captures %v err %v", r.acc.order, res.Err)
	}
	if !strings.Contains(r.log.String(), "is not present") || strings.Contains(r.log.String(), "12345") {
		t.Fatalf("log %q", r.log.String())
	}
}

func TestSummaryRowWinsOverLotRows(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261105")
	r.sec.add("kid", sectionPosition, "20261105|LOT|1|a", attrs{
		"reportDate": "20261105", "levelOfDetail": "LOT", "conid": "1", "assetCategory": "STK", "symbol": "XYZ",
		"currency": "USD", "position": "40", "costBasisPrice": "10", "markPrice": "52.25", "openDateTime": "x",
	}, recorded)
	r.run(t, []string{"kid"}, "2026-11-06")
	for _, p := range r.acc.captures["kid:2026-11-05:flex"].Positions {
		if p.ConID == 1 && (p.Quantity != "100" || p.AvgCost != "50.5") {
			t.Fatalf("summary must win, got %+v", p)
		}
	}
}

func lot(date, conid, pos, cost, mark, cur, open string) attrs {
	return attrs{
		"reportDate": date, "levelOfDetail": "LOT", "conid": conid, "assetCategory": "STK", "symbol": "XYZ",
		"currency": cur, "position": pos, "costBasisPrice": cost, "markPrice": mark, "openDateTime": open,
	}
}

func (r *rig) seedLots(cur2 string) {
	r.sec.present("kid", sectionPosition, sectionEquity, sectionNAV)
	r.sec.add("kid", sectionPosition, "k1", lot("20261105", "7", "100", "10", "12", "USD", "a"), recorded)
	r.sec.add("kid", sectionPosition, "k2", lot("20261105", "7", "300", "20", "12", cur2, "b"), recorded)
	r.sec.add("kid", sectionEquity, "20261105", attrs{"reportDate": "20261105", "total": "1", "cash": "2"}, recorded)
	r.sec.add("kid", sectionNAV, "a|b", attrs{"fromDate": "1", "toDate": "2", "currency": "USD"}, recorded)
}

func TestLotOnlyConidIsCombined(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seedLots("USD")
	r.run(t, []string{"kid"}, "2026-11-06")
	ps := r.acc.captures["kid:2026-11-05:flex"].Positions
	if len(ps) != 1 || ps[0].Quantity != "400" || ps[0].AvgCost != "17.5" {
		t.Fatalf("positions %+v", ps)
	}
}

func TestLotsOfTwoCurrenciesRefuseTheDay(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seedLots("EUR")
	res := r.run(t, []string{"kid"}, "2026-11-06")
	if len(r.acc.captures) != 0 || res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("captures %v err %v", r.acc.order, res.Err)
	}
}

func TestBothDateFormsAreRead(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261105")
	r.seed("kid", "2026-11-04")
	r.run(t, []string{"kid"}, "2026-11-06")
	if len(r.acc.captures) != 2 {
		t.Fatalf("captures %v", r.acc.order)
	}
	for _, p := range r.acc.captures["kid:2026-11-05:flex"].Positions {
		if p.SecType == "option" && p.Expiry != "2026-11-20" {
			t.Fatalf("expiry %q", p.Expiry)
		}
	}
}

func TestOtherDateFormRefusesTheDay(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "11/05/2026")
	res := r.run(t, []string{"kid"}, "2026-11-06")
	if len(r.acc.captures) != 0 || res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("captures %v err %v", r.acc.order, res.Err)
	}
	if !strings.Contains(res.Err.Error(), "reportDate") || strings.Contains(res.Err.Error(), "11/05/2026") {
		t.Fatalf("error text %q", res.Err.Error())
	}
}

func TestNonSessionDayIsSkippedNotFailed(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261107")
	r.cal.closed["2026-11-07"] = true
	res := r.run(t, []string{"kid"}, "2026-11-08")
	if len(r.acc.captures) != 0 || res.Err != nil || res.Usable != 0 {
		t.Fatalf("captures %v result %+v", r.acc.order, res)
	}
	if !strings.Contains(r.log.String(), "not_session_day=1") {
		t.Fatalf("log %q", r.log.String())
	}
}

func TestDaysAfterRunDayAreNotCaptured(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261105")
	r.run(t, []string{"kid"}, "2026-11-04")
	if len(r.acc.captures) != 0 {
		t.Fatalf("captures %v", r.acc.order)
	}
}

func TestStoredDayIsLeftAsIs(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261105")
	r.run(t, []string{"kid"}, "2026-11-06")
	first := r.acc.captures["kid:2026-11-05:flex"]
	r.sec.rows["kid|"+sectionEquity][0].Attributes = jsonOf(attrs{"reportDate": "20261105", "total": "1", "cash": "1"})
	r.run(t, []string{"kid"}, "2026-11-06")
	again := r.acc.captures["kid:2026-11-05:flex"]
	if len(r.acc.captures) != 1 || again.Values[0].Value != first.Values[0].Value {
		t.Fatalf("stored capture changed: %+v", again.Values)
	}
}

func TestMarksAreMarkPriceCloseOk(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261105")
	r.run(t, []string{"kid"}, "2026-11-06")
	if len(r.mk.rows) != 2 {
		t.Fatalf("marks %+v", r.mk.rows)
	}
	m := r.mk.rows[1]
	want := mark.PositionMark{CaptureID: "kid:2026-11-05:flex", Alias: "kid", MarkedAt: recorded.Add(time.Minute),
		SessionDay: "2026-11-05", ConID: 2, Attempt: 1, Currency: "USD", Price: "0.9", Basis: "close", Close: "0.9", Status: "ok"}
	if m != want {
		t.Fatalf("mark %+v want %+v", m, want)
	}
}

func TestWriteErrorIsRetryable(t *testing.T) {
	t.Parallel()
	for name, set := range map[string]func(*rig){
		"capture": func(r *rig) { r.acc.err = errors.New("boom") },
		"marks":   func(r *rig) { r.mk.err = errors.New("boom") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig()
			r.seed("kid", "20261105")
			set(r)
			res := r.run(t, []string{"kid"}, "2026-11-06")
			if res.Err == nil || res.Err.Kind() != contracts.Retryable {
				t.Fatalf("err %v", res.Err)
			}
		})
	}
}

func TestMissingAttributeIsPermanentWithoutValue(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261105")
	r.sec.rows["kid|"+sectionPosition][0].Attributes = jsonOf(attrs{
		"reportDate": "20261105", "levelOfDetail": "SUMMARY", "conid": "1", "assetCategory": "STK", "symbol": "XYZ",
		"currency": "USD", "position": "100", "costBasisPrice": "49.99", "markPrice": "n/a",
	})
	res := r.run(t, []string{"kid"}, "2026-11-06")
	if len(r.acc.captures) != 0 || res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("captures %v err %v", r.acc.order, res.Err)
	}
	txt := res.Err.Error()
	if !strings.Contains(txt, "2026-11-05") || !strings.Contains(txt, "markPrice") || strings.Contains(txt, "n/a") || strings.Contains(txt, "49.99") {
		t.Fatalf("error text %q", txt)
	}
}

func TestTwoBaseCurrenciesRefuseTheDay(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.seed("kid", "20261105")
	r.sec.add("kid", sectionNAV, "c|d", attrs{"fromDate": "3", "toDate": "4", "currency": "USD"}, recorded)
	res := r.run(t, []string{"kid"}, "2026-11-06")
	if len(r.acc.captures) != 0 || res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("captures %v err %v", r.acc.order, res.Err)
	}
}

func TestNewRequiresItsConfig(t *testing.T) {
	t.Parallel()
	if _, err := New(Config{}); err == nil {
		t.Fatal("an empty Config must fail")
	}
}

func TestManifestSlot(t *testing.T) {
	t.Parallel()
	r := newRig()
	j, err := newJob(Config{Aliases: []string{"kid"}, Sections: r.sec, Account: r.acc, Marks: r.mk, Calendar: r.cal,
		Clock: clockwork.NewFakeClock()}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	cal, anchor, zone, off := j.Schedule()
	if j.ID() != "statement_capture" || cal != "XNYS" || anchor != "session_close" || zone != "" || off != 95*time.Minute ||
		j.RunBound() != 10*time.Minute || j.RestartSensitive() {
		t.Fatalf("slot %s %s %s %v %v %v", cal, anchor, zone, off, j.RunBound(), j.RestartSensitive())
	}
}
