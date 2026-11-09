package optionledger

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// 2026-11-10 in New York: the last day is 2026-11-09.
var testNow = time.Date(2026, 11, 10, 17, 0, 0, 0, time.UTC)

type fakeWriter struct {
	recs []store.FlexFetchRecord
	rows [][]store.FlexTradeRow
}

func (w *fakeWriter) RecordFlexFetch(_ context.Context, rec store.FlexFetchRecord, rows []store.FlexTradeRow) (store.FlexInsert, error) {
	w.recs = append(w.recs, rec)
	w.rows = append(w.rows, rows)
	return store.FlexInsert{New: len(rows)}, nil
}

func (w *fakeWriter) ImportFlexTradeRows(context.Context, []store.FlexTradeRow) (store.FlexImportResult, error) {
	return store.FlexImportResult{}, nil
}

type fakeReader struct {
	aliases []string
	okFetch map[string]store.FlexFetchRecord // by login|query
}

func (r *fakeReader) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return nil, nil
}

func (r *fakeReader) LatestOkFetch(_ context.Context, login, q string) (store.FlexFetchRecord, bool, error) {
	rec, ok := r.okFetch[login+"|"+q]
	return rec, ok, nil
}

func (r *fakeReader) StoredAliases(context.Context) ([]string, error) { return r.aliases, nil }

type permErr struct{}

func (permErr) Error() string             { return "flex_error: 1018" }
func (permErr) Kind() contracts.ErrorKind { return contracts.Permanent }

type fakeSource struct {
	calls []sources.FlexRequest
	fail  bool
	rep   sources.FlexReport
}

func (s *fakeSource) Fetch(_ context.Context, r sources.FlexRequest) (sources.FlexReport, error) {
	s.calls = append(s.calls, r)
	if s.fail {
		return sources.FlexReport{}, permErr{}
	}
	return s.rep, nil
}

func oneTrade(acct, id string) sources.FlexStatement {
	return sources.FlexStatement{AccountID: acct, Trades: []map[string]string{
		{"accountId": acct, "levelOfDetail": "EXECUTION", "transactionID": id, "symbol": "ABC"},
	}}
}

func report(sts ...sources.FlexStatement) sources.FlexReport {
	return sources.FlexReport{FetchedAt: testNow, RawSHA256: "ab", Statements: sts}
}

type env struct {
	w   *fakeWriter
	r   *fakeReader
	log *bytes.Buffer
}

func build(t *testing.T, logins []jobs.FlexRun, r *fakeReader) (*job, env) {
	t.Helper()
	if r == nil {
		r = &fakeReader{aliases: []string{"a"}}
	}
	e := env{w: &fakeWriter{}, r: r, log: &bytes.Buffer{}}
	cfg := jobs.OptionLedgerConfig{Logins: logins, Accounts: map[string]string{"ACCT1": "a"}, HistoryFrom: "2025-12-01"}
	j, err := newJob(jobs.OptionLedgerStores{Ledger: e.w, Read: r}, cfg, fixedClock{testNow}, slog.New(slog.NewTextHandler(e.log, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return j, e
}

func TestOneFetchRecordPerRequest(t *testing.T) {
	t.Parallel()
	src := &fakeSource{rep: report(oneTrade("ACCT1", "7"))}
	j, e := build(t, []jobs.FlexRun{{Name: "main", QueryIDs: []string{"q1", "q2"}, Required: true, Source: src}}, nil)
	res := j.Run(context.Background(), "2026-11-10", nil)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(src.calls) != 2 || len(e.w.recs) != 2 {
		t.Fatalf("calls %d records %d", len(src.calls), len(e.w.recs))
	}
	rec := e.w.recs[0]
	if rec.Login != "main" || rec.QueryID != "q1" || rec.Status != "ok" || rec.FirstDay != "2025-12-01" || rec.LastDay != "2026-11-09" ||
		rec.Statements != 1 || rec.RowsParsed != 1 || rec.RawSHA256 != "ab" || rec.ErrorKind != "" {
		t.Fatalf("record %+v", rec)
	}
	if len(e.w.rows[0]) != 1 || e.w.rows[0][0].Alias != "a" {
		t.Fatalf("rows %+v", e.w.rows[0])
	}
}

func TestUnmappedStatementCountedNotStored(t *testing.T) {
	t.Parallel()
	src := &fakeSource{rep: report(oneTrade("ACCT1", "7"), oneTrade("STRANGER99", "8"))}
	j, e := build(t, []jobs.FlexRun{{Name: "main", QueryIDs: []string{"q1"}, Required: true, Source: src}}, nil)
	res := j.Run(context.Background(), "2026-11-10", nil)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	rec := e.w.recs[0]
	if rec.Statements != 2 || rec.UnmappedStatement != 1 || rec.RowsParsed != 1 || len(e.w.rows[0]) != 1 {
		t.Fatalf("record %+v rows %d", rec, len(e.w.rows[0]))
	}
	if strings.Contains(e.log.String(), "STRANGER99") {
		t.Fatal("account number logged")
	}
}

func TestFailedWindowStopsThatQueryOnly(t *testing.T) {
	t.Parallel()
	bad := &fakeSource{fail: true}
	good := &fakeSource{rep: report(oneTrade("ACCT1", "7"))}
	// 2024-01-01 gives three windows for the failing login.
	j, e := build(t, []jobs.FlexRun{
		{Name: "second", QueryIDs: []string{"q1", "q2"}, Source: bad},
		{Name: "main", QueryIDs: []string{"q3"}, Required: true, Source: good},
	}, nil)
	j.cfg.HistoryFrom = "2024-01-01"
	res := j.Run(context.Background(), "2026-11-10", nil)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(bad.calls) != 2 { // one per query, each stopped at its first window
		t.Fatalf("bad calls %d", len(bad.calls))
	}
	if len(good.calls) != 3 {
		t.Fatalf("good calls %d", len(good.calls))
	}
	if e.w.recs[0].Status != "failed" || e.w.recs[0].ErrorKind != "permanent" || e.w.recs[0].ErrorCode == "" {
		t.Fatalf("failed record %+v", e.w.recs[0])
	}
}

func TestNoRetryAfterFailure(t *testing.T) {
	t.Parallel()
	src := &fakeSource{fail: true}
	j, _ := build(t, []jobs.FlexRun{{Name: "main", QueryIDs: []string{"q1"}, Required: true, Source: src}}, nil)
	j.Run(context.Background(), "2026-11-10", nil)
	if len(src.calls) != 1 {
		t.Fatalf("calls %d, want 1", len(src.calls))
	}
}

func TestResultCarriesNoFigures(t *testing.T) {
	t.Parallel()
	src := &fakeSource{fail: true}
	j, _ := build(t, []jobs.FlexRun{{Name: "main", QueryIDs: []string{"q1"}, Required: true, Source: src}}, nil)
	res := j.Run(context.Background(), "2026-11-10", nil)
	if res.Err == nil {
		t.Fatal("want error")
	}
	for _, bad := range []string{"ACCT1", "q1", "symbol", "ABC", "1018"} {
		if strings.Contains(res.Err.Error(), bad) {
			t.Fatalf("error text holds %q: %v", bad, res.Err)
		}
	}
	keys, _, err := j.Plan(context.Background(), "2026-11-10")
	if err != nil || len(keys) != 1 || keys[0].SubjectKey != "a" {
		t.Fatalf("plan %v %v", keys, err)
	}
}

func TestRequiredLoginFailsResult(t *testing.T) {
	t.Parallel()
	j, e := build(t, []jobs.FlexRun{
		{Name: "main", QueryIDs: []string{"q1"}, Required: true, Source: &fakeSource{fail: true}},
		{Name: "second", QueryIDs: []string{"q2"}, Source: &fakeSource{rep: report(oneTrade("ACCT1", "7"))}},
	}, nil)
	res := j.Run(context.Background(), "2026-11-10", nil)
	var te contracts.TypedError
	if !errors.As(res.Err, &te) || te.Kind() != contracts.Permanent {
		t.Fatalf("err %v", res.Err)
	}
	if strings.Contains(e.log.String(), "degraded") {
		t.Fatal("required failure logged as degraded")
	}
	if res.Returned != 1 { // the other login still ran
		t.Fatalf("returned %d", res.Returned)
	}
	// a required login with no source fails too
	j2, _ := build(t, []jobs.FlexRun{{Name: "main", QueryIDs: []string{"q1"}, Required: true}}, nil)
	if res := j2.Run(context.Background(), "2026-11-10", nil); res.Err == nil {
		t.Fatal("missing required login must fail")
	}
}

func TestNonRequiredLoginDegradedOk(t *testing.T) {
	t.Parallel()
	j, e := build(t, []jobs.FlexRun{
		{Name: "main", QueryIDs: []string{"q1"}, Required: true, Source: &fakeSource{rep: report(oneTrade("ACCT1", "7"))}},
		{Name: "second", QueryIDs: []string{"q2", "q3"}, Source: &fakeSource{fail: true}},
		{Name: "third", QueryIDs: []string{"q4"}}, // no token
	}, nil)
	res := j.Run(context.Background(), "2026-11-10", nil)
	if res.Err != nil {
		t.Fatalf("result must be ok: %v", res.Err)
	}
	lines := strings.Split(strings.TrimSpace(e.log.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("log lines %d: %q", len(lines), e.log.String())
	}
	if !strings.Contains(lines[0], "degraded") || !strings.Contains(lines[0], "login=second") || !strings.Contains(lines[0], "status=failed") ||
		!strings.Contains(lines[1], "login=third") || !strings.Contains(lines[1], "status=not_configured") {
		t.Fatalf("log: %q", e.log.String())
	}
	if strings.Contains(e.log.String(), "q2") || strings.Contains(e.log.String(), "1018") {
		t.Fatal("log holds more than login and status")
	}
}

func TestNonRequiredFailureNoGap(t *testing.T) {
	t.Parallel()
	j, e := build(t, []jobs.FlexRun{{Name: "second", QueryIDs: []string{"q2"}, Source: &fakeSource{fail: true}}}, nil)
	res := j.Run(context.Background(), "2026-11-10", nil)
	if res.Err != nil || res.Returned != 0 || res.Usable != 0 {
		t.Fatalf("result %+v", res)
	}
	// only the failed fetch record is written; no trade row and no other record
	if len(e.w.recs) != 1 || len(e.w.rows[0]) != 0 || e.w.recs[0].Status != "failed" {
		t.Fatalf("writes %+v", e.w.recs)
	}
}

func TestOneFetchPerDayAcrossPlanKeys(t *testing.T) {
	t.Parallel()
	src := &fakeSource{rep: report(oneTrade("ACCT1", "7"))}
	j, _ := build(t, []jobs.FlexRun{{Name: "main", QueryIDs: []string{"q1"}, Required: true, Source: src}}, nil)
	j.cfg.Accounts = map[string]string{"ACCT1": "a", "ACCT2": "b"}
	keys, _, err := j.Plan(context.Background(), "2026-11-10")
	if err != nil || len(keys) != 2 {
		t.Fatalf("keys %v %v", keys, err)
	}
	for _, k := range keys { // as runner/reconcile/runkey.go:72 does
		j.Run(context.Background(), "2026-11-10", []contracts.ExpectedKey{k})
	}
	if len(src.calls) != 1 {
		t.Fatalf("Flex fetched %d times for one window in one day, want 1", len(src.calls))
	}
}

func TestRetriedKeyDoesNotFetchAgainAndIsPresent(t *testing.T) {
	t.Parallel()
	// a non-required login fails; every row of the other is unchanged (Usable 0)
	bad := &fakeSource{fail: true}
	j, _ := build(t, []jobs.FlexRun{{Name: "second", QueryIDs: []string{"q2"}, Source: bad}}, nil)
	ctx := context.Background()
	if ok, _ := j.Postcondition(ctx, "2026-11-10", datasetID, "a"); ok {
		t.Fatal("present before any run")
	}
	for i := 0; i < 3; i++ { // the runner retries a missing key
		if res := j.Run(ctx, "2026-11-10", nil); res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	if len(bad.calls) != 1 {
		t.Fatalf("fetched %d times, want 1", len(bad.calls))
	}
	if ok, _ := j.Postcondition(ctx, "2026-11-10", datasetID, "a"); !ok {
		t.Fatal("key must be present after a day that failed no required login")
	}
}

func TestRequiredFailureKeyStaysMissingWithoutRefetch(t *testing.T) {
	t.Parallel()
	src := &fakeSource{fail: true}
	j, _ := build(t, []jobs.FlexRun{{Name: "main", QueryIDs: []string{"q1"}, Required: true, Source: src}}, nil)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if res := j.Run(ctx, "2026-11-10", nil); res.Err == nil {
			t.Fatal("want error")
		}
	}
	if len(src.calls) != 1 {
		t.Fatalf("fetched %d times, want 1", len(src.calls))
	}
	if ok, _ := j.Postcondition(ctx, "2026-11-10", datasetID, "a"); ok {
		t.Fatal("a failed required login must not mark the key present")
	}
}
