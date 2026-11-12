package main

import (
	"context"
	"io"
	"strconv"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages/job_console"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// Design: docs/design/job-console.md#Decisions 3, 10 and 11.

// siteJobConsoleRoute is the address of the page job_console under the site root.
const siteJobConsoleRoute = "job-console"

// jobConsolePostRoute is the address of the form post, under the site root. The page builder carries the same
// path as the post_path of the form.
const jobConsolePostRoute = "owner-job-console"

func init() {
	registerPage(pageFamily{route: siteJobConsoleRoute,
		build: func(ctx context.Context, _ *app, clk clock.Clock, _ string, rd siteReaders) ([]byte, error) {
			return buildJobConsolePage(ctx, clk, rd, selectedRequest(ctx))
		}})
}

// selectedRequest is the id of ?request= (0 when absent or not a positive number).
func selectedRequest(ctx context.Context) int64 {
	id, err := strconv.ParseInt(queryOf(ctx).Get("request"), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// openJobRequestReader opens the page's reader with the pages line when deps holds the openers, else it is nil
// (the page shows the gap job_requests_unavailable). A failed open leaves it nil with one log line; the site
// still starts.
func openJobRequestReader(ctx context.Context, a *app, dsn string, track func(io.Closer, error) bool) jobrequest.Reader {
	o, ok := jobRequestOpenersOf(a.deps)
	if !ok || o.Reader == nil {
		return nil
	}
	st, err := o.Reader(ctx, dsn)
	if !track(st, err) {
		a.log.Warn("job request store not opened", "component", "ot", "status", "failed")
		return nil
	}
	return st
}

// buildJobConsolePage builds page.job_console.v1 per request over the reader of the site. With rd.edit the page
// carries the token of the form; without it the form is null.
func buildJobConsolePage(ctx context.Context, clk clock.Clock, rd siteReaders, selected int64) ([]byte, error) {
	now := clk.Now().UTC()
	var form *job_console.Form
	if rd.edit != nil {
		tok := rd.edit.form.Token(now)
		form = &job_console.Form{PostPath: "/" + jobConsolePostRoute, Expires: tok.Expires, MAC: tok.MAC}
	}
	return job_console.Build(ctx, rd.jobReq, now, form, selected, jobInfos(jobTable))
}

// jobInfos converts the table of the console to the rows the page builder takes.
func jobInfos(rows []jobRow) []job_console.JobInfo {
	out := make([]job_console.JobInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, job_console.JobInfo{
			ID: r.ID, Kind: r.Kind, Zone: r.Zone, Note: r.Note,
			WhyCode: r.Reason, WhyText: r.Text, NeedsText: r.Needs,
			ForceOK: r.Force, ForceWhy: r.ForceText,
		})
	}
	return out
}
