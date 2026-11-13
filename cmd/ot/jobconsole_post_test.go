package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

const dsnShaped = "postgres://runner:hunter2" + "\x40" + "db.internal:5432/optdesk"

// fakePoster is the poster and the reader in one; it keeps open rows by (job, day, force).
type fakePoster struct {
	mu        sync.Mutex
	opened    int
	closed    int
	dsn       string
	openErr   error
	enqErr    error
	getErr    error
	outcome   jobrequest.EnqueueOutcome // forced outcome when set
	known     map[int64]bool
	open      map[jobrequest.Request]int64
	enqueued  []jobrequest.Request
	withdrawn []int64
	next      int64
}

func newFakePoster() *fakePoster {
	return &fakePoster{known: map[int64]bool{7: true}, open: map[jobrequest.Request]int64{}, next: 100}
}

func (f *fakePoster) Recent(context.Context, int) ([]jobrequest.Row, error) { return nil, f.getErr }
func (f *fakePoster) Get(_ context.Context, id int64) (jobrequest.Row, bool, error) {
	return jobrequest.Row{ID: id}, f.known[id], f.getErr
}
func (f *fakePoster) Close() error { f.mu.Lock(); f.closed++; f.mu.Unlock(); return nil }
func (f *fakePoster) Withdraw(_ context.Context, id int64) error {
	f.withdrawn = append(f.withdrawn, id)
	return nil
}
func (f *fakePoster) Enqueue(_ context.Context, r jobrequest.Request) (jobrequest.Enqueued, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enqueued = append(f.enqueued, r)
	if f.enqErr != nil {
		return jobrequest.Enqueued{}, f.enqErr
	}
	if f.outcome != 0 {
		return jobrequest.Enqueued{Row: jobrequest.Row{ID: 5}, Outcome: f.outcome}, nil
	}
	if id, ok := f.open[r]; ok {
		return jobrequest.Enqueued{Row: jobrequest.Row{ID: id}, Outcome: jobrequest.EnqueueExisting}, nil
	}
	f.next++
	f.open[r] = f.next
	return jobrequest.Enqueued{Row: jobrequest.Row{ID: f.next}, Outcome: jobrequest.EnqueueCreated}, nil
}

// jobConsoleSite is the site over a fake poster at now; withEdit turns the write side on. The buffer is the log.
func jobConsoleSite(t *testing.T, withEdit bool, now time.Time) (http.Handler, *fakePoster, *bytes.Buffer) {
	t.Helper()
	a, logs := testApp(t, underlyingsEnv)
	a.deps.newRuleParams = newDeps().newRuleParams
	fp := newFakePoster()
	a.deps.ext = map[string]any{jobConsoleExtKey: jobRequestOpeners{
		Poster: func(_ context.Context, dsn string) (jobrequest.Poster, error) {
			fp.opened++
			fp.dsn = dsn
			if fp.openErr != nil {
				return nil, fp.openErr
			}
			return fp, nil
		},
	}}
	form := newDeps().newSettingsForm(nil, ownerTokenTTL, a.deps.newRuleParams())
	rd := siteReaders{jobReq: fp}
	if withEdit {
		rd.edit = &ownerEdit{form: form, token: postKey}
	}
	return siteHandler(a, fakeClock{now}, "", rd, echoHandler, http.NotFoundHandler()), fp, logs
}

// jcPost posts the fields with the owner cookie when cookie is not empty.
func jcPost(t *testing.T, h http.Handler, cookie string, fields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/owner-job-console", strings.NewReader(fields.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.RemoteAddr = postPeer
	r.Host = postHost
	if cookie != "" {
		withOwnerCookie(r, cookie)
	}
	return serve(h, r)
}

func runFields(job, day, force string) url.Values {
	v := url.Values{"action": {"run"}, "job": {job}, "day": {day}}
	if force != "" {
		v.Set("force", force)
	}
	return v
}

func TestPostGuardFirst(t *testing.T) {
	t.Parallel()
	h, fp, _ := jobConsoleSite(t, true, postNow)
	// no cookie fails the guard: 403, nothing opened, whatever the fields say.
	rec := jcPost(t, h, "", runFields("fred_series", "2026-11-05", ""))
	if rec.Code != http.StatusForbidden || fp.opened != 0 || len(fp.enqueued) != 0 {
		t.Fatalf("%d, %d opens, %d enqueues", rec.Code, fp.opened, len(fp.enqueued))
	}
	// a field that would be a 422 is still a 403 when the guard fails.
	rec = jcPost(t, h, "", url.Values{"action": {"nonsense"}})
	if rec.Code != http.StatusForbidden || fp.opened != 0 {
		t.Fatalf("%d, %d opens", rec.Code, fp.opened)
	}
	// another method is 405, and without --owner-edit the route is absent.
	rec = settingsGet(t, h, http.MethodGet, "/owner-job-console")
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("GET: %d", rec.Code)
	}
	h2, fp2, _ := jobConsoleSite(t, false, postNow)
	if rec = jcPost(t, h2, postKey, runFields("fred_series", "2026-11-05", "")); rec.Code != http.StatusNotFound || fp2.opened != 0 {
		t.Fatalf("no owner edit: %d", rec.Code)
	}
}

// refused422 posts the fields and wants 422 with no store opened.
func refused422(t *testing.T, now time.Time, fields url.Values) {
	t.Helper()
	h, fp, _ := jobConsoleSite(t, true, now)
	rec := jcPost(t, h, postKey, fields)
	if rec.Code != http.StatusUnprocessableEntity || fp.opened != 0 || len(fp.enqueued) != 0 {
		t.Fatalf("%v: %d, %d opens, %d enqueues", fields, rec.Code, fp.opened, len(fp.enqueued))
	}
}

func TestPostUnknownJob422(t *testing.T) {
	t.Parallel()
	refused422(t, postNow, runFields("no_such_job", "2026-11-05", ""))
	refused422(t, postNow, url.Values{"action": {"other"}})
	refused422(t, postNow, runFields("", "2026-11-05", ""))
}

func TestPostListedOnlyJob422(t *testing.T) {
	t.Parallel()
	refused422(t, postNow, runFields("option_open", "2026-11-06", ""))
	refused422(t, postNow, runFields("option_ledger", "2026-11-06", ""))
}

func TestPostForceOnForceNoJob422(t *testing.T) {
	t.Parallel()
	refused422(t, postNow, runFields("fred_series", "2026-11-05", "on"))
	refused422(t, postNow, runFields("account_capture", "2026-11-06", "on"))
	refused422(t, postNow, runFields("calendar_refresh", "2026-11-06", "maybe"))
}

func TestPostBadDay422(t *testing.T) {
	t.Parallel()
	for _, d := range []string{"", "2026-13-01", "2026-11-5", "11/05/2026", "2026-11-05T00:00:00Z", "2026-02-30"} {
		refused422(t, postNow, runFields("fred_series", d, ""))
	}
}

func TestPostFutureDay422(t *testing.T) {
	t.Parallel()
	refused422(t, postNow, runFields("fred_series", "2026-11-07", ""))
	h, fp, _ := jobConsoleSite(t, true, postNow)
	// today and any earlier day, with no floor, are accepted for an any-day job.
	for _, d := range []string{"2026-11-06", "2026-11-05", "2019-01-02"} {
		if rec := jcPost(t, h, postKey, runFields("fred_series", d, "")); rec.Code != http.StatusSeeOther {
			t.Fatalf("%s: %d", d, rec.Code)
		}
	}
	if len(fp.enqueued) != 3 {
		t.Fatalf("enqueued %v", fp.enqueued)
	}
}

func TestPostTodayOnlyOtherDay422(t *testing.T) {
	t.Parallel()
	refused422(t, postNow, runFields("account_capture", "2026-11-05", ""))
	refused422(t, postNow, runFields("account_capture", "2026-11-07", ""))
	h, fp, _ := jobConsoleSite(t, true, postNow)
	if rec := jcPost(t, h, postKey, runFields("account_capture", "2026-11-06", "")); rec.Code != http.StatusSeeOther || len(fp.enqueued) != 1 {
		t.Fatalf("today: %d", rec.Code)
	}
}

func TestPostTodayOnlyTokyoZone(t *testing.T) {
	t.Parallel()
	// 20:00 UTC is 15:00 in New York (11-06) and 05:00 on 11-07 in Tokyo.
	now := time.Date(2026, 11, 6, 20, 0, 0, 0, time.UTC)
	refused422(t, now, runFields("option_eod_jp", "2026-11-06", ""))
	refused422(t, now, runFields("option_eod", "2026-11-07", ""))
	h, fp, _ := jobConsoleSite(t, true, now)
	if rec := jcPost(t, h, postKey, runFields("option_eod_jp", "2026-11-07", "")); rec.Code != http.StatusSeeOther {
		t.Fatalf("tokyo today: %d", rec.Code)
	}
	if rec := jcPost(t, h, postKey, runFields("option_eod", "2026-11-06", "")); rec.Code != http.StatusSeeOther || len(fp.enqueued) != 2 {
		t.Fatalf("new york today: %d", rec.Code)
	}
}

func TestPostBeforeSlotIsAccepted(t *testing.T) {
	t.Parallel()
	// 02:00 UTC: no slot of any job has come; the post knows none and accepts the request.
	now := time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)
	h, fp, _ := jobConsoleSite(t, true, now)
	rec := jcPost(t, h, postKey, runFields("quote_snapshot", "2026-11-05", ""))
	if rec.Code != http.StatusSeeOther || len(fp.enqueued) != 1 || fp.enqueued[0].JobID != "quote_snapshot" {
		t.Fatalf("%d %v", rec.Code, fp.enqueued)
	}
}

func TestPostSameForceSameRow(t *testing.T) {
	t.Parallel()
	h, fp, _ := jobConsoleSite(t, true, postNow)
	a := jcPost(t, h, postKey, runFields("calendar_refresh", "2026-11-06", "on"))
	b := jcPost(t, h, postKey, runFields("calendar_refresh", "2026-11-06", "on"))
	if a.Code != http.StatusSeeOther || b.Code != http.StatusSeeOther || a.Header().Get("Location") != b.Header().Get("Location") {
		t.Fatalf("%d %q, %d %q", a.Code, a.Header().Get("Location"), b.Code, b.Header().Get("Location"))
	}
	if len(fp.open) != 1 {
		t.Fatalf("open rows %v", fp.open)
	}
}

func TestPostOtherForceOwnRow(t *testing.T) {
	t.Parallel()
	h, fp, _ := jobConsoleSite(t, true, postNow)
	a := jcPost(t, h, postKey, runFields("calendar_refresh", "2026-11-06", "on"))
	b := jcPost(t, h, postKey, runFields("calendar_refresh", "2026-11-06", ""))
	if a.Header().Get("Location") == b.Header().Get("Location") || len(fp.open) != 2 {
		t.Fatalf("%q %q, open %v", a.Header().Get("Location"), b.Header().Get("Location"), fp.open)
	}
	if !fp.enqueued[0].Force || fp.enqueued[1].Force {
		t.Fatalf("force flags %v", fp.enqueued)
	}
}

func TestPostWithdrawingRefused409(t *testing.T) {
	t.Parallel()
	h, fp, _ := jobConsoleSite(t, true, postNow)
	fp.outcome = jobrequest.EnqueueWithdrawing
	rec := jcPost(t, h, postKey, runFields("fred_series", "2026-11-05", ""))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "this request is being withdrawn; press Run again when it shows withdrawn") {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

func TestPostStoreError503(t *testing.T) {
	t.Parallel()
	h, fp, _ := jobConsoleSite(t, true, postNow)
	fp.enqErr = errors.New("boom")
	if rec := jcPost(t, h, postKey, runFields("fred_series", "2026-11-05", "")); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("enqueue error: %d", rec.Code)
	}
	fp.enqErr, fp.outcome = nil, jobrequest.EnqueueBusy
	if rec := jcPost(t, h, postKey, runFields("fred_series", "2026-11-05", "")); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("busy: %d", rec.Code)
	}
	fp.openErr = errors.New("no table")
	if rec := jcPost(t, h, postKey, runFields("fred_series", "2026-11-05", "")); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("open error: %d", rec.Code)
	}
}

func TestPostRedirects303(t *testing.T) {
	t.Parallel()
	h, fp, _ := jobConsoleSite(t, true, postNow)
	rec := jcPost(t, h, postKey, runFields("fred_series", "2026-11-05", ""))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/job-console?request=101" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	// the poster is opened with the settings line, once per post, and closed after it.
	if fp.opened != 1 || fp.closed != 1 || fp.dsn != "sentinel-dsn" {
		t.Fatalf("opened %d, closed %d, dsn %q", fp.opened, fp.closed, fp.dsn)
	}
}

func TestPostWithdraw(t *testing.T) {
	t.Parallel()
	h, fp, _ := jobConsoleSite(t, true, postNow)
	rec := jcPost(t, h, postKey, url.Values{"action": {"withdraw"}, "id": {"7"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/job-console?request=7" || len(fp.withdrawn) != 1 || fp.withdrawn[0] != 7 {
		t.Fatalf("%d %q %v", rec.Code, rec.Header().Get("Location"), fp.withdrawn)
	}
	if fp.closed != 1 {
		t.Fatalf("closed %d", fp.closed)
	}
}

func TestPostWithdrawUnknownId422(t *testing.T) {
	t.Parallel()
	h, fp, _ := jobConsoleSite(t, true, postNow)
	for _, id := range []string{"999", "abc", "", "-1"} {
		if rec := jcPost(t, h, postKey, url.Values{"action": {"withdraw"}, "id": {id}}); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("id %q: %d", id, rec.Code)
		}
	}
	if len(fp.withdrawn) != 0 {
		t.Fatalf("withdrawn %v", fp.withdrawn)
	}
}

func TestPostErrorsCarryNoDSN(t *testing.T) {
	t.Parallel()
	h, fp, logs := jobConsoleSite(t, true, postNow)
	fp.openErr = errors.New("dial " + dsnShaped)
	recs := []*httptest.ResponseRecorder{jcPost(t, h, postKey, runFields("fred_series", "2026-11-05", ""))}
	fp.openErr, fp.enqErr, fp.getErr = nil, errors.New("exec "+dsnShaped), errors.New("read "+dsnShaped)
	recs = append(recs, jcPost(t, h, postKey, runFields("fred_series", "2026-11-05", "")),
		jcPost(t, h, postKey, url.Values{"action": {"withdraw"}, "id": {"7"}}))
	for i, rec := range recs {
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("post %d: %d", i, rec.Code)
		}
		if s := rec.Body.String(); strings.Contains(s, "hunter2") || strings.Contains(s, "postgres://") {
			t.Fatalf("post %d: response %q", i, s)
		}
	}
	if s := logs.String(); !strings.Contains(s, "job console post") || strings.Contains(s, "hunter2") || strings.Contains(s, "postgres://") {
		t.Fatalf("log %q", s)
	}
}
