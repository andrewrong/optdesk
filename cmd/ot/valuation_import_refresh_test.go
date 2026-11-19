package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/andrewrong/optdesk/internal/store/ops"
	"github.com/andrewrong/optdesk/internal/store/runview"
	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

type failRunStore struct {
	fakeRunStore
}

func (*failRunStore) RunsOn(context.Context, contracts.Day) ([]runview.Row, error) {
	return nil, errors.New("boom")
}

// newRefreshFixture is the import site with a fake poster, a run store and a plan of rows.
func newRefreshFixture(t *testing.T, rs ops.RunStore, rows []valueimportapi.RowPlan) (http.Handler, *fakePoster) {
	t.Helper()
	a, _ := testApp(t, underlyingsEnv+"OT_PG_DSN_PAGES=postgres://pages\n")
	a.deps.newRuleParams = newDeps().newRuleParams
	f := newFakeValuation()
	imp := &fakeImporter{}
	a.deps.planValueImport = func(in valueimportapi.Input) (valueimportapi.Plan, error) {
		pl, err := imp.plan(in)
		pl.Rows = rows
		return pl, err
	}
	a.deps.openValuation = func(string) (valuation.Reader, valuation.Writer, io.Closer, error) {
		return f, f, nopCloser{}, nil
	}
	fp := newFakePoster()
	a.deps.ext = map[string]any{jobConsoleExtKey: jobRequestOpeners{
		Poster: func(context.Context, string) (jobrequest.Poster, error) { return fp, nil },
	}}
	a.deps.openRuns = func(context.Context, string) (ops.RunStore, error) { return rs, nil }
	at := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	urows := []underlying.Row{
		{Key: "US:AAA", Symbol: "AAA", Market: "US", Kind: underlying.KindStock, Status: underlying.StatusActive, Issuer: "AAA Inc", SetAt: at},
		{Key: "JP:7203", Symbol: "7203", Market: "JP", Kind: underlying.KindStock, Status: underlying.StatusActive, Issuer: "Toyota", SetAt: at},
	}
	rd := siteReaders{underlying: &fakeUnderlyingStore{rows: urows}, series: onePoint{}}
	rd.edit = &ownerEdit{form: newDeps().newSettingsForm(nil, ownerTokenTTL, a.deps.newRuleParams()), token: postKey}
	return siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler()), fp
}

func insRow(key, ticker string) valueimportapi.RowPlan {
	return valueimportapi.RowPlan{Key: key, Ticker: ticker, Name: ticker, Symbol: ticker, Currency: "USD", RMarket: "US",
		Pool: "in_pool", NewProfile: true, Action: valueimportapi.ActionInsert, ReviewType: "first", ReviewID: "imp-" + ticker,
		V: "1", P: "1", VersionDate: "2026-11-01"}
}

func sameRow(key, ticker string) valueimportapi.RowPlan {
	r := insRow(key, ticker)
	r.Action = valueimportapi.ActionSame
	return r
}

func finishedRuns(keys ...string) *fakeRunStore {
	st := &fakeRunStore{}
	for _, k := range keys {
		st.runs = append(st.runs, runview.Row{SessionDay: "2026-11-06", Dataset: "candidates.option_candidate", Key: k,
			Status: "ok", FinishedAt: postNow})
	}
	return st
}

func confirmDoc(t *testing.T, h http.Handler, action string) (importDoc, int) {
	t.Helper()
	pv := serve(h, importRequest(t, [][2]string{{"action", "preview"}}, []byte("{}")))
	d := decodeImport(t, pv)
	rec := serve(h, importRequest(t, [][2]string{{"action", action}, {"file_sha256", d.File.SHA256}}, []byte("{}")))
	return decodeImport(t, rec), rec.Code
}

func lastLine(d importDoc) string {
	l := d.Result[len(d.Result)-1]
	return l.Key + "=" + l.Outcome
}

func TestImportConfirmQueuesForcedGateRun(t *testing.T) {
	t.Parallel()
	h, fp := newRefreshFixture(t, finishedRuns("AAA Inc"), []valueimportapi.RowPlan{insRow("US:AAA", "AAA")})
	d, code := confirmDoc(t, h, "confirm")
	want := jobrequest.Request{JobID: "option_candidates", Day: "2026-11-06", Force: true}
	if code != 200 || len(fp.enqueued) != 1 || fp.enqueued[0] != want {
		t.Fatalf("code %d enqueued %v", code, fp.enqueued)
	}
	if got := lastLine(d); got != "option_candidates=inserted" {
		t.Fatal(got)
	}
}

func TestImportJPOnlyQueuesJPJob(t *testing.T) {
	t.Parallel()
	h, fp := newRefreshFixture(t, finishedRuns("AAA Inc", "Toyota"), []valueimportapi.RowPlan{insRow("JP:7203", "7203"), sameRow("US:AAA", "AAA")})
	d, _ := confirmDoc(t, h, "confirm")
	if len(fp.enqueued) != 1 || fp.enqueued[0].JobID != "option_candidates_jp" {
		t.Fatalf("%v", fp.enqueued)
	}
	if got := lastLine(d); got != "option_candidates_jp=inserted" {
		t.Fatal(got)
	}
}

func TestImportNotRunYetQueuesNothing(t *testing.T) {
	t.Parallel()
	h, fp := newRefreshFixture(t, finishedRuns("Toyota"), []valueimportapi.RowPlan{insRow("US:AAA", "AAA")})
	d, _ := confirmDoc(t, h, "confirm")
	if len(fp.enqueued) != 0 || lastLine(d) != "option_candidates=same" ||
		!strings.Contains(d.Result[len(d.Result)-1].Text, "has not run yet; it will use the new valuation") {
		t.Fatalf("%v %+v", fp.enqueued, d.Result)
	}
}

func TestImportRunsUnreadableQueuesNothing(t *testing.T) {
	t.Parallel()
	h, fp := newRefreshFixture(t, &failRunStore{}, []valueimportapi.RowPlan{insRow("US:AAA", "AAA")})
	d, code := confirmDoc(t, h, "confirm")
	if code != 200 || len(fp.enqueued) != 0 || lastLine(d) != "option_candidates=failed" {
		t.Fatalf("%d %v %+v", code, fp.enqueued, d.Result)
	}
}

func TestImportPreviewQueuesNothing(t *testing.T) {
	t.Parallel()
	h, fp := newRefreshFixture(t, finishedRuns("AAA Inc"), []valueimportapi.RowPlan{insRow("US:AAA", "AAA")})
	confirmDoc(t, h, "preview")
	if len(fp.enqueued) != 0 {
		t.Fatalf("%v", fp.enqueued)
	}
}

func TestImportAllSameQueuesNothing(t *testing.T) {
	t.Parallel()
	h, fp := newRefreshFixture(t, finishedRuns("AAA Inc"), []valueimportapi.RowPlan{sameRow("US:AAA", "AAA")})
	d, _ := confirmDoc(t, h, "confirm")
	if len(fp.enqueued) != 0 || strings.HasPrefix(d.Result[len(d.Result)-1].Key, "option_candidates") {
		t.Fatalf("%v %+v", fp.enqueued, d.Result)
	}
}

func TestImportEnqueueFailureKeepsResult(t *testing.T) {
	t.Parallel()
	h, fp := newRefreshFixture(t, finishedRuns("AAA Inc"), []valueimportapi.RowPlan{insRow("US:AAA", "AAA")})
	fp.enqErr = errors.New("down")
	d, code := confirmDoc(t, h, "confirm")
	if code != 200 || len(fp.enqueued) != 1 || lastLine(d) != "option_candidates=failed" {
		t.Fatalf("%d %v %+v", code, fp.enqueued, d.Result)
	}
	found := false
	for _, r := range d.Result {
		found = found || (r.Key == "US:AAA" && r.Outcome == "inserted")
	}
	if !found {
		t.Fatalf("written line lost: %+v", d.Result)
	}
}
