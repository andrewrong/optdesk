package optionledger_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/jobs/optionledger"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

var (
	flexNow = time.Date(2026, 11, 12, 8, 30, 0, 0, time.UTC)
	flexRef = optionledger.FlexFileRef{RequestID: "0a1b2c3d", Ordinal: 1}
	flexAcc = map[string]string{"U111": "main", "U222": "kid"}
)

type flexWriter struct {
	mu    sync.Mutex
	recs  []store.FlexFetchRecord
	rows  []store.FlexTradeRow
	calls int
	err   error
}

func (f *flexWriter) RecordFlexFetch(_ context.Context, rec store.FlexFetchRecord, rows []store.FlexTradeRow) (store.FlexInsert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return store.FlexInsert{}, f.err
	}
	f.recs = append(f.recs, rec)
	f.rows = append(f.rows, rows...)
	return store.FlexInsert{New: len(rows)}, nil
}

func (f *flexWriter) ImportFlexTradeRows(context.Context, []store.FlexTradeRow) (store.FlexImportResult, error) {
	panic("not used")
}

// flexReader serves the rows the writer holds, as the store would: one row per key, by alias.
type flexReader struct {
	w   *flexWriter
	err error
}

func (r flexReader) FlexTradeRows(_ context.Context, alias string) ([]store.FlexTradeRow, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.w.mu.Lock()
	defer r.w.mu.Unlock()
	var out []store.FlexTradeRow
	for _, x := range r.w.rows {
		if x.Alias == alias {
			out = append(out, x)
		}
	}
	return out, nil
}

func (r flexReader) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	panic("not used")
}

func trade(id string, extra ...string) map[string]string {
	m := map[string]string{"transactionID": id, "levelOfDetail": "EXECUTION", "accountId": "U111", "acctAlias": "Bob", "tradeDate": "20250310"}
	for i := 0; i+1 < len(extra); i += 2 {
		m[extra[i]] = extra[i+1]
	}
	return m
}

func stmt(acct string, from, to string, trades ...map[string]string) sources.FlexStatement {
	return sources.FlexStatement{AccountID: acct, FromDate: from, ToDate: to, Trades: trades}
}

func run(t *testing.T, w *flexWriter, rep sources.FlexReport) (optionledger.FlexImportSummary, error) {
	t.Helper()
	return optionledger.ImportFlexReport(context.Background(), rep, flexAcc, w, flexReader{w: w}, flexRef, flexNow)
}

func rep(sts ...sources.FlexStatement) sources.FlexReport {
	return sources.FlexReport{RawSHA256: "abc", Statements: sts}
}

var queryIDRE = regexp.MustCompile(`^file:[0-9a-f]{8}:[1-9][0-9]*:[^:\s]+$`)

func TestImportFlexTwoAliases(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	s, err := run(t, w, rep(
		stmt("U222", "20250101", "20250131", trade("3")),
		stmt("U111", "20250201", "20250228", trade("1"), trade("2")),
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Records) != 2 || s.Records[0].QueryID == s.Records[1].QueryID {
		t.Fatalf("records %+v", s.Records)
	}
	if s.Records[0].QueryID != "file:0a1b2c3d:1:kid" || s.Records[1].QueryID != "file:0a1b2c3d:1:main" {
		t.Fatalf("sorted aliases: %s %s", s.Records[0].QueryID, s.Records[1].QueryID)
	}
	if s.Aliases[0].RowsNew != 1 || s.Aliases[1].RowsNew != 2 || s.Records[1].RowsParsed != 2 {
		t.Fatalf("counts %+v", s.Aliases)
	}
	if s.Records[1].FirstDay != "2025-02-01" || s.Records[1].LastDay != "2025-02-28" {
		t.Fatalf("period %s %s", s.Records[1].FirstDay, s.Records[1].LastDay)
	}
}

func TestImportFlexSameReportTwice(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	r := rep(stmt("U111", "20250201", "20250228", trade("1"), trade("2")))
	first, _ := run(t, w, r)
	second, err := run(t, w, r)
	if err != nil {
		t.Fatal(err)
	}
	if second.Aliases[0].RowsNew != 0 || second.Records[0].RowsParsed != first.Aliases[0].RowsNew || second.Aliases[0].RowsPresent != 2 {
		t.Fatalf("first %+v second %+v", first.Aliases, second.Aliases)
	}
	if len(w.rows) != 2 || len(w.recs) != 2 {
		t.Fatalf("rows %d records %d", len(w.rows), len(w.recs))
	}
}

func TestImportFlexOtherFieldSetIsPresent(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	if _, err := run(t, w, rep(stmt("U111", "20250201", "20250228", trade("1")))); err != nil {
		t.Fatal(err)
	}
	before := len(w.rows)
	s, err := run(t, w, rep(stmt("U111", "20250201", "20250228", trade("1", "price", "9.9", "tradeDate", "20250311"))))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.rows) != before || s.Aliases[0].RowsPresent != 1 || s.Aliases[0].RowsNew != 0 {
		t.Fatalf("rows %d counts %+v", len(w.rows), s.Aliases)
	}
	for _, x := range w.rows {
		if x.Revision > 1 {
			t.Fatalf("revision written: %+v", x)
		}
	}
}

func TestImportFlexRepeatedKey(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	s, err := run(t, w, rep(stmt("U111", "20250201", "20250228", trade("1"), trade("1", "price", "2"))))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Aliases[0]
	if c.RowsParsed != 2 || c.RowsNew != 1 || c.RowsPresent != 1 || len(w.rows) != 1 {
		t.Fatalf("%+v rows %d", c, len(w.rows))
	}
}

func TestImportFlexUnmappedHoldsNoAccount(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	s, err := run(t, w, rep(
		stmt("U999", "20250201", "20250228", trade("9", "accountId", "U999")),
		stmt("U111", "20250201", "20250228", trade("1")),
	))
	if err != nil {
		t.Fatal(err)
	}
	if s.Unmapped != 1 || s.Records[0].UnmappedStatement != 1 || len(w.rows) != 1 {
		t.Fatalf("%+v rows %d", s, len(w.rows))
	}
	all := ""
	for _, x := range w.rows {
		all += x.Attributes + x.Alias + x.TransactionID
	}
	for _, rc := range s.Records {
		all += rc.QueryID + rc.Login + rc.RawSHA256
	}
	for _, bad := range []string{"U999", "U111", "accountId", "acctAlias", "Bob"} {
		if strings.Contains(all, bad) {
			t.Fatalf("%q leaked", bad)
		}
	}
}

func TestImportFlexNoAliasMapped(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	s, err := run(t, w, rep(stmt("U999", "20250201", "20250228", trade("9")), stmt("U998", "", "", trade("8"))))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Records) != 1 || s.Records[0].QueryID != "file:0a1b2c3d:1:-" || len(w.rows) != 0 || s.Records[0].UnmappedStatement != 2 {
		t.Fatalf("%+v", s.Records)
	}
	st, _ := optionledger.DeriveImportStatus(s.Records[0])
	if st != optionledger.ImportUnconfigured {
		t.Fatal(st)
	}
}

func TestImportFlexNothingFound(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	s, err := run(t, w, rep(stmt("U111", "20250201", "20250228")))
	if err != nil {
		t.Fatal(err)
	}
	r := s.Records[0]
	if len(s.Records) != 1 || r.QueryID != "file:0a1b2c3d:1:-" || r.Statements != 0 || r.UnmappedStatement != 0 || r.RowsParsed != 0 || r.RowsNew != 0 {
		t.Fatalf("%+v", s.Records)
	}
	if st, _ := optionledger.DeriveImportStatus(r); st != optionledger.ImportNothingFound {
		t.Fatal(st)
	}
}

func TestImportFlexMappedWithoutTradeBesideUnmapped(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	s, err := run(t, w, rep(stmt("U111", "20250201", "20250228"), stmt("U999", "20250201", "20250228", trade("9"))))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Records) != 1 || s.Records[0].QueryID != "file:0a1b2c3d:1:-" || s.Records[0].UnmappedStatement != 1 {
		t.Fatalf("%+v", s.Records)
	}
}

func TestImportFlexUnreadablePeriod(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	s, err := run(t, w, rep(stmt("U111", "garbage", "", trade("1", "tradeDate", "20250310"), trade("2", "tradeDate", "20250105"), trade("3", "tradeDate", "x"))))
	if err != nil {
		t.Fatal(err)
	}
	if s.Records[0].FirstDay != "2025-01-05" || s.Records[0].LastDay != "2025-03-10" || len(w.rows) != 3 {
		t.Fatalf("%+v rows %d", s.Records[0], len(w.rows))
	}
	w2 := &flexWriter{}
	s, err = run(t, w2, rep(stmt("U111", "garbage", "", trade("1", "tradeDate", ""))))
	if err != nil {
		t.Fatal(err)
	}
	if s.Records[0].FirstDay != "2026-11-12" || s.Records[0].LastDay != "2026-11-12" || len(w2.rows) != 1 {
		t.Fatalf("%+v", s.Records[0])
	}
	w3 := &flexWriter{}
	if err := optionledger.RecordImportFailure(context.Background(), w3, flexRef, optionledger.ReasonNotReport, flexNow); err != nil {
		t.Fatal(err)
	}
	if w3.recs[0].FirstDay != "2026-11-12" || w3.recs[0].LastDay != "2026-11-12" {
		t.Fatalf("%+v", w3.recs[0])
	}
}

func TestImportFlexRecordShape(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	if _, err := run(t, w, rep(stmt("U111", "20250201", "20250228", trade("1")), stmt("U999", "", ""))); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, w, rep()); err != nil {
		t.Fatal(err)
	}
	if err := optionledger.RecordImportFailure(context.Background(), w, flexRef, optionledger.ReasonNotReport, flexNow); err != nil {
		t.Fatal(err)
	}
	for _, r := range w.recs {
		if r.StartedAt.IsZero() || r.FinishedAt.IsZero() || r.FirstDay == "" || r.LastDay == "" || !queryIDRE.MatchString(r.QueryID) {
			t.Fatalf("%+v", r)
		}
		if r.Status != "ok" && r.Status != "failed" || r.Login != "import" || r.ParserVersion != "1" || !r.StartedAt.Equal(flexNow) || !r.FinishedAt.Equal(flexNow) {
			t.Fatalf("%+v", r)
		}
	}
}

func TestImportFlexWriterErrorAndFailureRecord(t *testing.T) {
	t.Parallel()
	w := &flexWriter{err: errors.New("pq: secret value")}
	_, err := run(t, w, rep(stmt("U111", "20250201", "20250228", trade("1"))))
	var ie *optionledger.ImportError
	if !errors.As(err, &ie) || ie.Code != "store_error" || strings.Contains(err.Error(), "secret") {
		t.Fatalf("%v", err)
	}
	w2 := &flexWriter{}
	if err := optionledger.RecordImportFailure(context.Background(), w2, flexRef, "store_error", flexNow); err != nil {
		t.Fatal(err)
	}
	r := w2.recs[0]
	if r.Status != "failed" || r.ErrorKind != "import" || r.ErrorCode != "store_error" ||
		r.RawSHA256 != "" || len(w2.rows) != 0 || r.QueryID != "file:0a1b2c3d:1:-" {
		t.Fatalf("%+v", r)
	}
	if err := optionledger.RecordImportFailure(context.Background(), w, flexRef, "x", flexNow); err == nil {
		t.Fatal("failed write must be an error")
	}
	rd := flexReader{w: w2, err: errors.New("down")}
	_, err = optionledger.ImportFlexReport(context.Background(), rep(stmt("U111", "", "", trade("1"))), flexAcc, w2, rd, flexRef, flexNow)
	if !errors.As(err, &ie) || ie.Code != "store_error" {
		t.Fatalf("%v", err)
	}
}

func TestRecordImportFailureOrdinalsDiffer(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	for n := 1; n <= 2; n++ {
		ref := optionledger.FlexFileRef{RequestID: "0a1b2c3d", Ordinal: n}
		if err := optionledger.RecordImportFailure(context.Background(), w, ref, "not_report", flexNow); err != nil {
			t.Fatal(err)
		}
	}
	if w.recs[0].QueryID == w.recs[1].QueryID {
		t.Fatal("same query id")
	}
}

func TestImportFlexBadRowIsNotReport(t *testing.T) {
	t.Parallel()
	w := &flexWriter{}
	_, err := run(t, w, rep(stmt("U111", "", "", map[string]string{"levelOfDetail": "EXECUTION"})))
	var ie *optionledger.ImportError
	if !errors.As(err, &ie) || ie.Code != "not_report" || w.calls != 0 {
		t.Fatalf("%v calls %d", err, w.calls)
	}
}

func TestDeriveImportStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		rec            store.FlexFetchRecord
		status, reason string
	}{
		{"failed", store.FlexFetchRecord{Status: "failed", ErrorCode: "too_large", RowsNew: 3}, "failed", "too_large"},
		{"stored", store.FlexFetchRecord{Status: "ok", RowsParsed: 2, RowsNew: 1}, "stored", ""},
		{"revised", store.FlexFetchRecord{Status: "ok", RowsParsed: 2, RowsRevised: 1}, "stored", ""},
		{"present", store.FlexFetchRecord{Status: "ok", RowsParsed: 2}, "present", ""},
		{"unconfigured", store.FlexFetchRecord{Status: "ok", UnmappedStatement: 2}, "unconfigured", ""},
		{"nothing", store.FlexFetchRecord{Status: "ok"}, "nothing_found", ""},
	}
	for _, c := range cases {
		st, rs := optionledger.DeriveImportStatus(c.rec)
		if st != c.status || rs != c.reason {
			t.Errorf("%s: %s %s", c.name, st, rs)
		}
	}
	if optionledger.ImportAlias("file:0a1b2c3d:1:main") != "main" || optionledger.ImportAlias("file:0a1b2c3d:1:-") != "" {
		t.Fatal("alias")
	}
}

func (r flexReader) StoredAliases(context.Context) ([]string, error) { panic("not used") }
