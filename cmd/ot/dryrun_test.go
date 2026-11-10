package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	"github.com/andrewrong/optdesk/internal/jobs/quotesnapshot"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// dryJob is a job that refuses before 16:15 New York like option_eod, writes the rows of rows[subject] and
// returns err[subject].
type dryJob struct {
	id   string
	keys []contracts.ExpectedKey
	clk  clock.Clock
	w    store.OptionEodWriter
	rows map[string][]int
	errs map[string]contracts.ErrorKind
	ran  *[]string
	mu   *sync.Mutex
}

func (j dryJob) ID() string { return j.id }
func (j dryJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return j.keys, "h", nil
}

type dryKindErr contracts.ErrorKind

func (e dryKindErr) Error() string             { return string(e) }
func (e dryKindErr) Kind() contracts.ErrorKind { return contracts.ErrorKind(e) }

func (j dryJob) Run(ctx context.Context, _ contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	if len(keys) != 1 {
		return contracts.JobResult{Err: dryKindErr(contracts.Permanent)}
	}
	k := keys[0]
	j.mu.Lock()
	*j.ran = append(*j.ran, j.id+"/"+k.SubjectKey)
	j.mu.Unlock()
	ny, _ := time.LoadLocation("America/New_York")
	if j.clk.Now().In(ny).Hour() < 16 && j.id == jobOptionEod {
		return contracts.JobResult{Err: dryKindErr(contracts.NoValue)}
	}
	var rows []contracts.OptionEod
	for _, s := range j.rows[k.SubjectKey] {
		rows = append(rows, contracts.OptionEod{Status: s})
	}
	if err := j.w.InsertOptionEod(ctx, rows); err != nil {
		return contracts.JobResult{Err: dryKindErr(contracts.Retryable)}
	}
	res := contracts.JobResult{Returned: len(rows) + 1}
	if e, ok := j.errs[k.SubjectKey]; ok {
		res.Err = dryKindErr(e)
	}
	return res
}

// dryApp is an app whose store openers fail the test and whose constructors build dryJobs.
func dryApp(t *testing.T, ran *[]string) (*app, *strings.Builder) {
	t.Helper()
	a, _ := testApp(t, "")
	mu := &sync.Mutex{}
	build := func(id string, keys []contracts.ExpectedKey, rows map[string][]int, errs map[string]contracts.ErrorKind) jobs.NewOptionEodFunc {
		return func(_ broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, clk clock.Clock) (contracts.Job, error) {
			a, err := res.ResolveInstrument(context.Background(), store.Identifier{Scheme: "x", Value: "1"}, store.InstrumentSpec{}, nil)
			b, _ := res.ResolveInstrument(context.Background(), store.Identifier{Scheme: "x", Value: "2"}, store.InstrumentSpec{}, nil)
			c, _ := res.ResolveInstrument(context.Background(), store.Identifier{Scheme: "x", Value: "1"}, store.InstrumentSpec{}, nil)
			if err != nil || a != 1 || b != 2 || c != 1 {
				t.Errorf("resolver ids %d %d %d, want 1 2 1", a, b, c)
			}
			return dryJob{id: id, keys: keys, clk: clk, w: w, rows: rows, errs: errs, ran: ran, mu: mu}, nil
		}
	}
	d := a.deps
	d.dialBroker = func(context.Context, string, broker.Options) (broker.Broker, error) { return unreachableBroker{}, nil }
	d.openLedger = func(context.Context, string) (store.OpsLedger, error) { t.Fatal("openLedger called"); return nil, nil }
	d.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) {
		t.Fatal("openOptionEod called")
		return nil, nil
	}
	d.openQuoteSnapshot = func(context.Context, string) (store.QuoteSnapshotStore, error) {
		t.Fatal("openQuoteSnapshot called")
		return nil, nil
	}
	d.openInstrument = func(context.Context, string) (store.InstrumentStore, error) {
		t.Fatal("openInstrument called")
		return nil, nil
	}
	d.openFence = func(context.Context, string) (store.InstanceFence, error) {
		t.Fatal("openFence called")
		return nil, nil
	}
	eod := build(jobOptionEod, []contracts.ExpectedKey{{DatasetID: "obs.option_eod", SubjectKey: "SPY"}},
		map[string][]int{"SPY": {1, 1, 2, 5}}, nil)
	d.newOptionEod = func(b broker.Broker, r store.InstrumentResolver, w store.OptionEodWriter, c clock.Clock, _ symbol.Translator) (contracts.Job, error) {
		return eod(b, r, w, c)
	}
	d.newOptionEodJP = build(jobOptionEodJP, []contracts.ExpectedKey{{DatasetID: "obs.option_eod", SubjectKey: "N225"}},
		nil, map[string]contracts.ErrorKind{"N225": contracts.Retryable})
	d.newQuoteSnapshot = func(b broker.Broker, res store.InstrumentResolver, _ store.QuoteSnapshotWriter, clk clock.Clock) (contracts.Job, error) {
		return build(jobQuoteSnapshot, []contracts.ExpectedKey{{DatasetID: "obs.quote_snapshot", SubjectKey: "0945"}}, nil, nil)(b, res, &memSink{}, clk)
	}
	a.deps = d
	var out strings.Builder
	a.stdout = &out
	return a, &out
}

func TestDryRunOpensNoStore(t *testing.T) {
	t.Parallel()
	var ran []string
	a, out := dryApp(t, &ran)
	if err := runOnce(a, []string{"--dry-run", "--day", "2026-11-10"}); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 3 || !strings.Contains(out.String(), "keys=3") {
		t.Fatalf("ran %v, out %q", ran, out.String())
	}
}

func TestDryRunBypassesGuardsAndLedger(t *testing.T) {
	t.Parallel()
	// The fixed clock of real jobs is past freeze_after and inside every slot and stop_after: with a broker
	// that answers retryable, a guard would show as no_value or an empty key.
	var ran []string
	a, out := dryApp(t, &ran)
	a.deps.newOptionEod, a.deps.newOptionEodJP, a.deps.newQuoteSnapshot = optioneod.NewWithSymbols, optioneod.NewJP, quotesnapshot.New
	if err := runOnce(a, []string{"--dry-run", "--day", "2026-11-10"}); err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.HasPrefix(l, "dry-run job=") && !strings.HasSuffix(l, "error=retryable") {
			t.Errorf("a guard refused or the key was skipped: %s", l)
		}
	}
	if !strings.Contains(out.String(), "status=failed") {
		t.Fatalf("summary missing: %q", out.String())
	}
	// the fake job refusing before 16:15 sees a clock after it
	var ran2 []string
	b, out2 := dryApp(t, &ran2)
	if err := runOnce(b, []string{"--dry-run", "--job", "option_eod", "--day", "2026-11-10"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2.String(), "error=none") || strings.Contains(out2.String(), "no_value") {
		t.Fatalf("guard refused: %q", out2.String())
	}
}

var lineRe = regexp.MustCompile(`^dry-run job=(\S+) day=2026-11-10 dataset=(\S+) subject=(\S+) ` +
	`requested=(\d+) ok=(\d+) partial=(\d+) no_data=(\d+) elapsed=\S+ error=(\S+)$`)

func TestDryRunPrintsPerKeyAndSummary(t *testing.T) {
	t.Parallel()
	var ran []string
	a, out := dryApp(t, &ran)
	if err := runOnce(a, []string{"--dry-run", "--day", "2026-11-10"}); err != nil {
		t.Fatalf("exit must be 0 for status=failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("lines %q", lines)
	}
	want := [][]string{
		{"option_eod", "obs.option_eod", "SPY", "5", "2", "1", "1", "none"},
		{"option_eod_jp", "obs.option_eod", "N225", "1", "0", "0", "0", "retryable"},
		{"quote_snapshot", "obs.quote_snapshot", "0945", "1", "0", "0", "0", "none"},
	}
	for i, w := range want {
		m := lineRe.FindStringSubmatch(lines[i])
		if m == nil || strings.Join(m[1:], ",") != strings.Join(w, ",") {
			t.Errorf("line %d = %q, want fields %v", i, lines[i], w)
		}
	}
	if lines[3] != "dry-run 2026-11-10 status=failed keys=3 failed=1" {
		t.Errorf("summary %q", lines[3])
	}
}

func TestDryRunPrintsOkSummaryAndDialError(t *testing.T) {
	t.Parallel()
	var ran []string
	a, out := dryApp(t, &ran)
	if err := runOnce(a, []string{"--dry-run", "--job", "option_eod", "--job", "quote_snapshot", "--day", "2026-11-10"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.String(), "dry-run 2026-11-10 status=ok keys=2 failed=0\n") {
		t.Errorf("out %q", out.String())
	}
	boom := errors.New("boom")
	a.deps.dialBroker = func(context.Context, string, broker.Options) (broker.Broker, error) { return nil, boom }
	out.Reset()
	err := runOnce(a, []string{"--dry-run"})
	if !errors.Is(err, boom) || out.Len() != 0 {
		t.Errorf("err %v, out %q", err, out.String())
	}
}

func TestDryRunRejectsOtherJobs(t *testing.T) {
	t.Parallel()
	var ran []string
	a, out := dryApp(t, &ran)
	err := runOnce(a, []string{"--dry-run", "--job", "fred_series"})
	if err == nil {
		t.Fatal("fred_series accepted")
	}
	for _, n := range []string{"option_eod", "option_eod_jp", "quote_snapshot"} {
		if !strings.Contains(err.Error(), n) {
			t.Errorf("error %q does not name %s", err, n)
		}
	}
	if err := runOnce(a, []string{"--job", "option_eod"}); err == nil {
		t.Error("--job without --dry-run accepted")
	}
	if out.Len() != 0 || len(ran) != 0 {
		t.Errorf("something ran: %q %v", out.String(), ran)
	}
}

// zeroGraceSlotJob applies the guard of quote_snapshot with grace_minutes = 0 in New York: the clock must be
// exactly the slot.
type zeroGraceSlotJob struct{ clk clock.Clock }

func (zeroGraceSlotJob) ID() string { return jobQuoteSnapshot }
func (zeroGraceSlotJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: "obs.quote_snapshot", SubjectKey: "0945"}}, "h", nil
}

func (j zeroGraceSlotJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	ny, _ := time.LoadLocation("America/New_York")
	now := j.clk.Now().In(ny)
	at := time.Date(now.Year(), now.Month(), now.Day(), 9, 45, 0, 0, ny)
	if now.Before(at) || now.After(at) {
		return contracts.JobResult{Err: dryKindErr(contracts.NoValue)}
	}
	return contracts.JobResult{Returned: 1}
}

func TestDryRunQuoteSnapshotZeroGraceIsNotRefused(t *testing.T) {
	t.Parallel()
	var ran []string
	a, out := dryApp(t, &ran)
	a.deps.newQuoteSnapshot = func(_ broker.Broker, _ store.InstrumentResolver, _ store.QuoteSnapshotWriter, clk clock.Clock) (contracts.Job, error) {
		return zeroGraceSlotJob{clk: clk}, nil
	}
	if err := runOnce(a, []string{"--dry-run", "--job", "quote_snapshot", "--day", "2026-11-10"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "error=no_value") || !strings.Contains(out.String(), "error=none") {
		t.Fatalf("guard refused a dry-run key: %q", out.String())
	}
}

// tokyoSlotJob applies the quote_snapshot guard in the manifest zone Asia/Tokyo, grace 5 minutes.
type tokyoSlotJob struct{ clk clock.Clock }

func (tokyoSlotJob) ID() string { return jobQuoteSnapshot }
func (tokyoSlotJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: "obs.quote_snapshot", SubjectKey: "0945"}}, "h", nil
}

func (j tokyoSlotJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	tk, _ := time.LoadLocation("Asia/Tokyo")
	now := j.clk.Now().In(tk)
	at := time.Date(now.Year(), now.Month(), now.Day(), 9, 45, 0, 0, tk)
	if now.Format("2006-01-02") != "2026-11-10" || now.Before(at) || now.After(at.Add(5*time.Minute)) {
		return contracts.JobResult{Err: dryKindErr(contracts.NoValue)}
	}
	return contracts.JobResult{Returned: 7}
}

func TestDryRunQuoteSnapshotOtherZoneIsNotRefused(t *testing.T) {
	t.Parallel()
	var ran []string
	a, out := dryApp(t, &ran)
	a.deps.newQuoteSnapshot = func(_ broker.Broker, _ store.InstrumentResolver, _ store.QuoteSnapshotWriter, clk clock.Clock) (contracts.Job, error) {
		return tokyoSlotJob{clk: clk}, nil
	}
	if err := runOnce(a, []string{"--dry-run", "--job", "quote_snapshot", "--day", "2026-11-10"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "error=no_value") || !strings.Contains(out.String(), "requested=7 ") {
		t.Fatalf("guard refused a dry-run key: %q", out.String())
	}
}

// noQuoteSlotJob: NY zone, grace 5m, two slots; inside the window the broker has no quote (NoValue "no quote").
type noQuoteSlotJob struct{ clk clock.Clock }

func (noQuoteSlotJob) ID() string { return jobQuoteSnapshot }
func (noQuoteSlotJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: "obs.quote_snapshot", SubjectKey: "0945"}, {DatasetID: "obs.quote_snapshot", SubjectKey: "1200"}}, "h", nil
}

func (j noQuoteSlotJob) Run(_ context.Context, _ contracts.Day, ks []contracts.ExpectedKey) contracts.JobResult {
	ny, _ := time.LoadLocation("America/New_York")
	now := j.clk.Now().In(ny)
	var hhmm int
	_, _ = fmt.Sscanf(ks[0].SubjectKey, "%04d", &hhmm)
	at := time.Date(now.Year(), now.Month(), now.Day(), hhmm/100, hhmm%100, 0, 0, ny)
	if now.Format("2006-01-02") != "2026-11-10" || now.After(at.Add(5*time.Minute)) {
		return contracts.JobResult{Err: dryKindErr(contracts.NoValue)}
	}
	if now.Before(at) {
		return contracts.JobResult{} // the guard's "not yet"
	}
	return contracts.JobResult{Err: dryKindErr(contracts.NoValue)} // in window, no quote
}

func TestDryRunQuoteSnapshotNoQuoteIsReported(t *testing.T) {
	t.Parallel()
	var ran []string
	a, out := dryApp(t, &ran)
	a.deps.newQuoteSnapshot = func(_ broker.Broker, _ store.InstrumentResolver, _ store.QuoteSnapshotWriter, clk clock.Clock) (contracts.Job, error) {
		return noQuoteSlotJob{clk: clk}, nil
	}
	if err := runOnce(a, []string{"--dry-run", "--job", "quote_snapshot", "--day", "2026-11-10"}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "error=no_value") != 2 || !strings.Contains(out.String(), "status=failed") {
		t.Fatalf("in-window no quote not reported: %q", out.String())
	}
}

const dryWatchTwoUS = `
[[underlying]]
issuer = "cik:0000320193"
market = "US"
symbol = "AAPL"
sec_type = "STK"
exchange = "SMART"
currency = "USD"
con_id = 1001

[[underlying]]
issuer = "cik:0000789019"
market = "US"
symbol = "MSFT"
sec_type = "STK"
exchange = "SMART"
currency = "USD"
con_id = 1002
`

// dryWatchFile writes body as a watch.toml in a temporary directory and returns a readFile that reads it.
func dryWatchFile(t *testing.T, body string) func(string) ([]byte, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "watch.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return func(string) ([]byte, error) { return os.ReadFile(path) }
}

func TestDryRunBuildsOptionEodOverWatchList(t *testing.T) {
	t.Parallel()
	var ran []string
	a, _ := dryApp(t, &ran)
	a.deps.readFile = dryWatchFile(t, dryWatchTwoUS)
	var got []jobs.WatchEntry
	a.deps.newOptionEod = func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock, symbol.Translator) (contracts.Job, error) {
		t.Error("newOptionEod called")
		return nil, errors.New("unexpected")
	}
	a.deps.newOptionEodWatch = func(_ broker.Broker, _ store.InstrumentResolver, w store.OptionEodWriter,
		watch []jobs.WatchEntry, clk clock.Clock, _ symbol.Translator) (contracts.Job, error) {
		got = watch
		return dryJob{id: jobOptionEod, keys: []contracts.ExpectedKey{{DatasetID: "obs.option_eod", SubjectKey: "AAPL"}},
			clk: clk, w: w, rows: map[string][]int{"AAPL": {1}}, ran: &ran, mu: &sync.Mutex{}}, nil
	}
	if err := runOnce(a, []string{"--dry-run", "--job", "option_eod", "--day", "2026-11-10"}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Contract.Symbol != "AAPL" || got[1].Contract.Symbol != "MSFT" {
		t.Fatalf("watch given to option_eod: %+v", got)
	}
}

func TestDryRunRefusesBrokenWatch(t *testing.T) {
	t.Parallel()
	var ran []string
	a, _ := dryApp(t, &ran)
	a.deps.readFile = dryWatchFile(t, "[[underlying]\nbroken")
	a.deps.dialBroker = func(context.Context, string, broker.Options) (broker.Broker, error) {
		t.Error("dialBroker called")
		return unreachableBroker{}, nil
	}
	_, want := loadWatch(a)
	if want == nil {
		t.Fatal("fixture must be refused by loadWatch")
	}
	err := runOnce(a, []string{"--dry-run", "--day", "2026-11-10"})
	if err == nil || err.Error() != want.Error() {
		t.Fatalf("err %v, want %v", err, want)
	}
}
