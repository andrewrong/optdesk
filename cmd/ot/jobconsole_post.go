package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// Design: docs/design/job-console.md#Decisions 3 and 11. The post knows no slot: a request before the slot of a
// job is accepted and the scheduler holds it (Decision 8).

// The fixed texts of the refusals; none holds a field value or an error text.
const (
	textBadForm      = "the request is not valid"
	textWithdrawing  = "this request is being withdrawn; press Run again when it shows withdrawn"
	textStoreFailure = "the request store is not available"
)

// jobRun is a validated run action.
type jobRun struct {
	job   jobRow
	day   string
	force bool
}

// postOwnerJobConsole serves POST /owner-job-console: the guard of /owner-settings (guardOwnerPost) first, then
// the action run or withdraw. The poster is opened with the OT_PG_DSN_SETTINGS line per post and closed after it.
func postOwnerJobConsole(a *app, clk clock.Clock, rd siteReaders, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	now := clk.Now().UTC()
	if !guardOwnerPost(w, r, rd.edit, now) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var run jobRun
	var id int64
	action := r.PostForm.Get("action")
	switch action {
	case "run":
		var ok bool
		if run, ok = readJobRun(r, now); !ok {
			http.Error(w, textBadForm, http.StatusUnprocessableEntity)
			return
		}
	case "withdraw":
		var err error
		if id, err = strconv.ParseInt(r.PostForm.Get("id"), 10, 64); err != nil || id <= 0 {
			http.Error(w, textBadForm, http.StatusUnprocessableEntity)
			return
		}
	default:
		http.Error(w, textBadForm, http.StatusUnprocessableEntity)
		return
	}
	ctx := r.Context()
	o, ok := jobRequestOpenersOf(a.deps)
	_, dsn, reason, err := a.ownerEditSecrets()
	if !ok || o.Poster == nil || err != nil || reason != "" {
		storeFailed(a, w, "job console post: no poster")
		return
	}
	st, err := o.Poster(ctx, dsn)
	if err != nil {
		storeFailed(a, w, "job console post: cannot open the store")
		return
	}
	defer func() { _ = st.Close() }()
	if action == "run" {
		postRun(ctx, a, st, run, w, r)
		return
	}
	postWithdraw(ctx, a, st, id, w, r)
}

// storeFailed answers 503 with the fixed text and logs a line that holds no error text (it may hold a DSN).
func storeFailed(a *app, w http.ResponseWriter, msg string) {
	a.log.Error(msg, "component", "ot", "status", "failed")
	http.Error(w, textStoreFailure, http.StatusServiceUnavailable)
}

// readJobRun validates the fields of a run; false is 422 and no row.
func readJobRun(r *http.Request, now time.Time) (jobRun, bool) {
	row, ok := jobRowOf(r.PostForm.Get("job"))
	if !ok || !row.runnable() {
		return jobRun{}, false
	}
	force := false
	switch r.PostForm.Get("force") {
	case "", "off", "0", "false":
	case "on", "1", "true":
		force = true
	default:
		return jobRun{}, false
	}
	if force && !row.Force {
		return jobRun{}, false
	}
	day := r.PostForm.Get("day")
	d, err := time.Parse(time.DateOnly, day)
	if err != nil || d.Format(time.DateOnly) != day {
		return jobRun{}, false
	}
	zone := row.Zone
	if zone == "" {
		zone = marketZone
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return jobRun{}, false
	}
	today := now.In(loc).Format(time.DateOnly)
	if row.Kind == kindTodayOnly && day != today {
		return jobRun{}, false
	}
	if row.Kind == kindAnyDay && day > today {
		return jobRun{}, false
	}
	return jobRun{job: row, day: day, force: force}, true
}

func postRun(ctx context.Context, a *app, st jobrequest.Poster, run jobRun, w http.ResponseWriter, r *http.Request) {
	res, err := st.Enqueue(ctx, jobrequest.Request{JobID: run.job.ID, Day: run.day, Force: run.force})
	switch {
	case err != nil:
		storeFailed(a, w, "job console post: enqueue failed")
	case res.Outcome == jobrequest.EnqueueWithdrawing:
		http.Error(w, textWithdrawing, http.StatusConflict)
	case res.Outcome == jobrequest.EnqueueCreated || res.Outcome == jobrequest.EnqueueExisting:
		redirectToRequest(w, r, res.Row.ID)
	default:
		storeFailed(a, w, "job console post: the queue was busy")
	}
}

func postWithdraw(ctx context.Context, a *app, st jobrequest.Poster, id int64, w http.ResponseWriter, r *http.Request) {
	_, found, err := st.Get(ctx, id)
	switch {
	case err != nil:
		storeFailed(a, w, "job console post: read failed")
		return
	case !found:
		http.Error(w, textBadForm, http.StatusUnprocessableEntity)
		return
	}
	if err := st.Withdraw(ctx, id); err != nil {
		storeFailed(a, w, "job console post: withdraw failed")
		return
	}
	redirectToRequest(w, r, id)
}

// redirectToRequest answers 303 to the page with the request selected, so a reload never repeats the post.
func redirectToRequest(w http.ResponseWriter, r *http.Request, id int64) {
	http.Redirect(w, r, "/"+siteJobConsoleRoute+"?request="+strconv.FormatInt(id, 10), http.StatusSeeOther)
}
