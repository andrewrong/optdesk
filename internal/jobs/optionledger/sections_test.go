package optionledger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store/flexsection"
)

type fakeSections struct {
	calls    int
	fetchID  string
	rows     []flexsection.Row
	statuses []flexsection.Status
	err      error
}

func (f *fakeSections) RecordSections(_ context.Context, id string, rows []flexsection.Row, st []flexsection.Status) (flexsection.Insert, error) {
	f.calls++
	f.fetchID, f.rows, f.statuses = id, rows, st
	return flexsection.Insert{New: len(rows)}, f.err
}

func fixtureReport(t *testing.T) sources.FlexReport {
	t.Helper()
	b, err := os.ReadFile("testdata/report_sections.json")
	if err != nil {
		t.Fatal(err)
	}
	var rep sources.FlexReport
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	return rep
}

func buildS(t *testing.T, logins []jobs.FlexRun, sec flexsection.Writer) (*job, *fakeWriter, *bytes.Buffer) {
	t.Helper()
	w := &fakeWriter{}
	logs := &bytes.Buffer{}
	cfg := jobs.OptionLedgerConfig{Logins: logins, Accounts: map[string]string{"ACCT1": "a"}, HistoryFrom: "2025-12-01"}
	stores := jobs.OptionLedgerStores{Ledger: w, Read: &fakeReader{aliases: []string{"a"}}, Sections: sec}
	j, err := newJob(stores, cfg, fixedClock{testNow}, slog.New(slog.NewTextHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return j, w, logs
}

func mainLogin(rep sources.FlexReport) []jobs.FlexRun {
	return []jobs.FlexRun{{Name: "main", QueryIDs: []string{"q1"}, Required: true, Source: &fakeSource{rep: rep}}}
}

func stateOf(sts []flexsection.Status) map[string]string {
	m := map[string]string{}
	for _, s := range sts {
		m[s.Section] = s.State
	}
	return m
}

func TestDuplicateKeyBothRefused(t *testing.T) {
	t.Parallel()
	spec, _ := sectionByElement("CashTransaction")
	rows := []map[string]string{{"transactionID": "t1"}, {"transactionID": "t1"}, {"transactionID": "t2"}}
	kept, refused, err := buildSectionRows(spec, "a", rows, AttributesSHA256, testNow)
	if err != nil || len(kept) != 1 || kept[0].RowKey != "t2" || refused != 2 {
		t.Fatalf("kept=%+v refused=%d err=%v", kept, refused, err)
	}
	kept, refused, _ = buildSectionRows(spec, "a", []map[string]string{{"transactionID": ""}, {"x": "1"}}, AttributesSHA256, testNow)
	if len(kept) != 0 || refused != 2 {
		t.Fatalf("missing key: kept=%d refused=%d", len(kept), refused)
	}
}

func TestAccountIdDroppedBeforeHash(t *testing.T) {
	t.Parallel()
	spec, _ := sectionByElement("CashTransaction")
	var seen map[string]string
	hash := func(a map[string]string) (string, error) { seen = a; return "h", nil }
	kept, _, err := buildSectionRows(spec, "a", []map[string]string{{"accountId": "ACCT1", "acctAlias": "x", "transactionID": "t1", "amount": "5"}}, hash, testNow)
	if err != nil || len(kept) != 1 {
		t.Fatal(kept, err)
	}
	if _, ok := seen["accountId"]; ok || seen["transactionID"] != "t1" {
		t.Fatalf("hashed attributes %v", seen)
	}
	if _, ok := seen["acctAlias"]; ok {
		t.Fatalf("hashed attributes %v", seen)
	}
	if kept[0].AttributesSHA256 != "h" || strings.Contains(kept[0].Attributes, "ACCT1") || strings.Contains(kept[0].Attributes, "acctAlias") {
		t.Fatalf("row %+v", kept[0])
	}
}

func TestAbsentSectionWritesStatusAndDegraded(t *testing.T) {
	t.Parallel()
	rep := fixtureReport(t)
	delete(rep.Statements[0].Sections, "CashTransaction")
	sec := &fakeSections{}
	j, _, logs := buildS(t, mainLogin(rep), sec)
	if res := j.Run(context.Background(), "2026-11-10", nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	st := stateOf(sec.statuses)
	if len(st) != 5 || st["CashTransaction"] != "absent" || st["ConversionRate"] != "present" {
		t.Fatalf("statuses %v", st)
	}
	for _, s := range sec.statuses {
		if s.Section == "ConversionRate" && s.RowCount != 1 {
			t.Fatalf("count %d", s.RowCount)
		}
	}
	want := "login=main section=CashTransaction"
	if !strings.Contains(logs.String(), "degraded") || !strings.Contains(logs.String(), want) || strings.Count(logs.String(), "degraded") != 1 {
		t.Fatalf("log %q", logs.String())
	}
	wantID := "main:q1:2025-12-01:2026-11-09:2026-11-10T17:00:00Z"
	if sec.fetchID != wantID || sec.statuses[0].FetchID != wantID || sec.rows[0].FetchID != wantID {
		t.Fatalf("fetch id %q", sec.fetchID)
	}
}

func TestNoSectionsAtAllFiveAbsent(t *testing.T) {
	t.Parallel()
	sec := &fakeSections{}
	j, _, _ := buildS(t, mainLogin(report(oneTrade("ACCT1", "7"))), sec)
	if res := j.Run(context.Background(), "2026-11-10", nil); res.Err != nil {
		t.Fatalf("an absent section must not change the result: %v", res.Err)
	}
	if len(sec.statuses) != 5 || len(sec.rows) != 0 {
		t.Fatalf("statuses %d rows %d", len(sec.statuses), len(sec.rows))
	}
	for _, s := range sec.statuses {
		if s.State != "absent" || s.Alias != "a" {
			t.Fatalf("status %+v", s)
		}
	}
}

func TestNilSectionsStoreLeavesTradePath(t *testing.T) {
	t.Parallel()
	j, w, _ := buildS(t, mainLogin(fixtureReport(t)), nil)
	res := j.Run(context.Background(), "2026-11-10", nil)
	if res.Err != nil || len(w.recs) != 1 || w.recs[0].Status != "ok" {
		t.Fatalf("res %+v recs %+v", res, w.recs)
	}
}

func TestUnmappedStatementWritesNoSection(t *testing.T) {
	t.Parallel()
	rep := fixtureReport(t)
	rep.Statements[0].AccountID = "STRANGER"
	sec := &fakeSections{}
	j, _, logs := buildS(t, mainLogin(rep), sec)
	if res := j.Run(context.Background(), "2026-11-10", nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	if sec.calls != 0 || strings.Contains(logs.String(), "STRANGER") {
		t.Fatalf("calls %d log %q", sec.calls, logs.String())
	}
}

func TestSectionWriteFailureFailsRequiredLogin(t *testing.T) {
	t.Parallel()
	sec := &fakeSections{err: errors.New("db down")}
	j, _, _ := buildS(t, mainLogin(fixtureReport(t)), sec)
	if res := j.Run(context.Background(), "2026-11-10", nil); res.Err == nil {
		t.Fatal("required login must fail")
	}
	sec2 := &fakeSections{err: errors.New("db down")}
	login := []jobs.FlexRun{{Name: "second", QueryIDs: []string{"q1"}, Source: &fakeSource{rep: fixtureReport(t)}}}
	j2, _, logs := buildS(t, login, sec2)
	if res := j2.Run(context.Background(), "2026-11-10", nil); res.Err != nil {
		t.Fatalf("non-required login must stay ok: %v", res.Err)
	}
	if !strings.Contains(logs.String(), "degraded") || strings.Contains(logs.String(), "db down") {
		t.Fatalf("log %q", logs.String())
	}
}

// It moved here from job_test.go: the failed-run result and the section step both carry no figure.
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

	// the section step: a failed write of the rows leaks no figure into the result or the log
	sec2 := &fakeSections{err: errors.New("db down 10.5")}
	j2, _, logs := buildS(t, mainLogin(fixtureReport(t)), sec2)
	res2 := j2.Run(context.Background(), "2026-11-10", nil)
	if res2.Err == nil {
		t.Fatal("want error")
	}
	for _, bad := range []string{"10.5", "150", "100", "ACCT1", "t1", "db down"} {
		if strings.Contains(res2.Err.Error(), bad) || strings.Contains(logs.String(), bad) {
			t.Fatalf("figure %q reached the error or the log: %v %q", bad, res2.Err, logs.String())
		}
	}
}
