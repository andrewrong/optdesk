// Package job_console builds the page.job_console.v1 view model from the request rows of a jobrequest.Reader and a
// table of jobs given by the caller. It reads no clock (the caller passes the instant), imports no command or runner
// package, and writes every visible string through text.go.
// Design: docs/design/job-console.md#6 job-console-builder.
package job_console

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	_ "time/tzdata" // the zone database travels with the binary

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
)

const (
	recentLimit    = 20
	refreshSeconds = 5
	gapKind        = "gap"
	gapUnavailable = "job_requests_unavailable"
	gapNoSuchReq   = "no_such_request"
	dayLayout      = "2006-01-02"
)

// Form is the token of the forms, made by the caller; a nil *Form in Build means the route is off.
type Form struct{ PostPath, Expires, MAC string }

// JobInfo is one row of the job table of the caller. Kind is any_day, today_only or listed_only; Zone is the zone
// name in which Today is read (empty is UTC); WhyCode, WhyText and NeedsText are for a listed-only job; ForceOK is
// true for a force-yes job and ForceWhy is the text for a force-no job.
type JobInfo struct {
	ID, Kind, Zone, Note        string
	WhyCode, WhyText, NeedsText string
	ForceOK                     bool
	ForceWhy                    string
}

type pageJSON struct {
	Schema         string        `json:"schema"`
	SchemaVersion  int           `json:"schema_version"`
	GeneratedAt    string        `json:"generated_at"`
	Title          string        `json:"title"`
	Sentence       string        `json:"sentence"`
	Form           *formJSON     `json:"form"`
	Gaps           []gapJSON     `json:"gaps"`
	Jobs           []jobJSON     `json:"jobs"`
	Requests       []requestJSON `json:"requests"`
	Selected       *int64        `json:"selected"`
	RefreshSeconds int           `json:"refresh_seconds"`
	Running        []runningJSON `json:"running,omitempty"`
}

// runningJSON is one line of "Running now": the figures are copied from compute.FiguresFunc, Line is text.go's.
type runningJSON struct {
	Line           string `json:"line"`
	Job            string `json:"job"`
	StartedAt      string `json:"started_at"`
	KeysDone       int    `json:"keys_done"`
	KeysTotal      int    `json:"keys_total"`
	Step           string `json:"step"`
	ElapsedMinutes int    `json:"elapsed_minutes"`
	UsualMinutes   int    `json:"usual_minutes"`
	Slow           bool   `json:"slow"`
}

// waitingJSON is the line of a request that waits for its own job: since is hold_at.
type waitingJSON struct {
	Line  string `json:"line"`
	Job   string `json:"job"`
	Since string `json:"since"`
}

// Runs is what the running-now and already-running lines read: the open runs and the usual durations, and the
// function that turns them into figures (compute.FiguresFunc). The zero Runs gives a page without those lines.
type Runs struct {
	Reader  jobrun.JobRunReader
	Figures jobrunapi.FiguresFunc
}

type formJSON struct {
	PostPath string `json:"post_path"`
	Expires  string `json:"expires"`
	MAC      string `json:"mac"`
}

type gapJSON struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type jobJSON struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Today     string  `json:"today"`
	Note      *string `json:"note"`
	Force     bool    `json:"force"`
	ForceWhy  string  `json:"force_why"`
	WhyText   string  `json:"why_text"`
	NeedsText string  `json:"needs_text"`
}

type requestJSON struct {
	ID        int64     `json:"id"`
	Job       string    `json:"job"`
	Day       string    `json:"day"`
	Force     bool      `json:"force"`
	State     string    `json:"state"`
	StateText string    `json:"state_text"`
	AtText    string    `json:"at_text"`
	Reason    string    `json:"reason"`
	Keys      []keyJSON `json:"keys"`

	WaitingFor *waitingJSON `json:"waiting_for,omitempty"`
}

type keyJSON struct {
	Dataset     string `json:"dataset"`
	Subject     string `json:"subject"`
	Outcome     string `json:"outcome"`
	OutcomeText string `json:"outcome_text"`
	Present     string `json:"present"`
	Reason      string `json:"reason"`
}

// Build returns the JSON of contracts/schemas/page.job_console.v1.json. r may be nil (the page then carries the gap
// job_requests_unavailable and no requests); a reader error gives the same gap. selected is the id of ?request= (0
// for none): it is listed even when older than the newest 20 (which then give up their last place), and an unknown id
// gives the gap no_such_request. form is nil when the route is off. generatedAt is the page's only clock.
func Build(ctx context.Context, r jobrequest.Reader, generatedAt time.Time, form *Form, selected int64, jobs []JobInfo) ([]byte, error) {
	return BuildWithRuns(ctx, r, generatedAt, form, selected, jobs, Runs{})
}

// BuildWithRuns is Build with the running-now lines and the already-running line of a waiting request. A request
// carries waiting_for only when its hold_job is its own job and that job has an open run. A slow run only sets the
// slow flag and its line; nothing here stops or hides a job. A failed read of the runs leaves the lines out.
func BuildWithRuns(ctx context.Context, r jobrequest.Reader, generatedAt time.Time, form *Form, selected int64, jobs []JobInfo, runs Runs) ([]byte, error) {
	now := generatedAt.UTC()
	p := pageJSON{
		Schema: "job_console", SchemaVersion: 1,
		GeneratedAt: now.Format(time.RFC3339),
		Title:       titleText, Sentence: sentenceText,
		Gaps: []gapJSON{}, Jobs: []jobJSON{}, Requests: []requestJSON{},
	}
	if form != nil {
		p.Form = &formJSON{PostPath: form.PostPath, Expires: form.Expires, MAC: form.MAC}
	}
	for _, j := range jobs {
		jj, err := jobRow(j, now)
		if err != nil {
			return nil, err
		}
		p.Jobs = append(p.Jobs, jj)
	}

	open := readRuns(ctx, runs, now)
	p.Running = open.rows

	rows, sel, gap := readRows(ctx, r, selected)
	if gap != nil {
		p.Gaps = append(p.Gaps, *gap)
	}
	for _, row := range rows {
		rj, err := requestRow(row, now, open.waiting(row))
		if err != nil {
			return nil, err
		}
		p.Requests = append(p.Requests, rj)
		if row.State == jobrequest.StateQueued || row.State == jobrequest.StateRunning {
			p.RefreshSeconds = refreshSeconds
		}
	}
	if sel != nil {
		id := sel.ID
		p.Selected = &id
	}
	p.Sentence = sentenceFor(p.Requests, sel)
	return json.MarshalIndent(p, "", "  ")
}

// readRows returns the requests to list (newest first, the selected one last when it is older than the list), the
// selected row when it is shown, and the gap that explains a missing read.
func readRows(ctx context.Context, r jobrequest.Reader, selected int64) (rows []jobrequest.Row, sel *jobrequest.Row, gap *gapJSON) {
	unavailable := &gapJSON{Kind: gapKind, Reason: gapUnavailable, Text: unavailableText}
	if r == nil {
		return nil, nil, unavailable
	}
	recent, err := r.Recent(ctx, recentLimit)
	if err != nil {
		return nil, nil, unavailable
	}
	if len(recent) > recentLimit {
		recent = recent[:recentLimit]
	}
	rows = recent
	if selected <= 0 {
		return rows, nil, nil
	}
	for i := range rows {
		if rows[i].ID == selected {
			return rows, &rows[i], nil
		}
	}
	row, found, err := r.Get(ctx, selected)
	switch {
	case err != nil:
		return rows, nil, unavailable
	case !found:
		return rows, nil, &gapJSON{Kind: gapKind, Reason: gapNoSuchReq, Text: noSuchText}
	}
	if len(rows) >= recentLimit {
		rows = rows[:recentLimit-1]
	}
	rows = append(rows, row)
	return rows, &rows[len(rows)-1], nil
}

func jobRow(j JobInfo, now time.Time) (jobJSON, error) {
	switch j.Kind {
	case "any_day", "today_only", "listed_only":
	default:
		return jobJSON{}, fmt.Errorf("job_console: job %q has kind %q", j.ID, j.Kind)
	}
	loc := time.UTC
	if j.Zone != "" {
		var err error
		if loc, err = time.LoadLocation(j.Zone); err != nil {
			return jobJSON{}, fmt.Errorf("job_console: job %q zone: %w", j.ID, err)
		}
	}
	jj := jobJSON{
		ID: j.ID, Kind: j.Kind, Today: now.In(loc).Format(dayLayout),
		Force: j.ForceOK, ForceWhy: j.ForceWhy, WhyText: j.WhyText, NeedsText: j.NeedsText,
	}
	if j.Note != "" {
		n := j.Note
		jj.Note = &n
	}
	return jj, nil
}

func requestRow(row jobrequest.Row, now time.Time, waiting *waitingJSON) (requestJSON, error) {
	keys := make([]keyJSON, 0, len(row.Lines))
	var due time.Time
	for _, l := range row.Lines {
		lt, ok := lineOutcome(l)
		if !ok {
			return requestJSON{}, fmt.Errorf("job_console: request %d: line %s/%s has group %q status %q",
				row.ID, l.Dataset, l.Subject, l.Group, l.Status)
		}
		if due.IsZero() {
			due = l.DueAt
		}
		keys = append(keys, keyJSON{Dataset: l.Dataset, Subject: l.Subject, Outcome: lt.Outcome,
			OutcomeText: lt.Text, Present: lt.Present, Reason: lt.Reason})
	}
	started := row.StartedAt
	if started.IsZero() {
		started = row.RequestedAt
	}
	return requestJSON{
		ID: row.ID, Job: row.Request.JobID, Day: row.Request.Day, Force: row.Request.Force,
		WaitingFor: waiting,
		State:      string(row.State), AtText: row.RequestedAt.UTC().Format(atLayout), Reason: row.Reason, Keys: keys,
		StateText: stateText(stateIn{
			State: row.State, Reason: row.Reason, HoldJob: row.HoldJob, HoldAt: row.HoldAt,
			Withdrawing: !row.WithdrawnAt.IsZero(), Age: now.Sub(row.RequestedAt),
			StartedAt: started.UTC().Format(atLayout), DueAt: due, Waiting: waiting != nil,
		}),
	}, nil
}

// sentenceFor is the page sentence: the request-level line "nothing was produced" when the request in focus (the
// selected one, else the newest) is done and none of its lines is produced.
func sentenceFor(reqs []requestJSON, sel *jobrequest.Row) string {
	if len(reqs) == 0 {
		return sentenceText
	}
	focus := reqs[0]
	if sel != nil {
		for _, q := range reqs {
			if q.ID == sel.ID {
				focus = q
			}
		}
	}
	if focus.State != string(jobrequest.StateDone) {
		return sentenceText
	}
	for _, k := range focus.Keys {
		if k.Outcome == outProduced {
			return sentenceText
		}
	}
	return fmt.Sprintf("Request %d (%s, %s): %s.", focus.ID, focus.Job, focus.Day, nothingProducedText)
}

// openRuns is the open runs as lines, and the jobs that have one.
type openRuns struct {
	rows []runningJSON
	jobs map[string]bool
}

// readRuns copies the figures of runs.Figures for the open runs of runs.Reader at now. It does no arithmetic.
func readRuns(ctx context.Context, runs Runs, now time.Time) openRuns {
	out := openRuns{jobs: map[string]bool{}}
	if runs.Reader == nil || runs.Figures == nil {
		return out
	}
	open, err := runs.Reader.Running(ctx)
	if err != nil {
		return out
	}
	usuals, err := runs.Reader.Usuals(ctx)
	if err != nil {
		return out
	}
	in := make([]jobrunapi.Input, len(open))
	for i, o := range open {
		in[i] = jobrunapi.Input{JobID: o.JobID, StartedAt: o.StartedAt, KeysDone: o.KeysDone, KeysTotal: o.KeysTotal, Step: o.Step}
	}
	us := make([]jobrunapi.Usual, len(usuals))
	for i, u := range usuals {
		us[i] = jobrunapi.Usual{JobID: u.JobID, Median: u.Median, Samples: u.Samples}
	}
	for i, f := range runs.Figures(now, in, us) {
		if i >= len(open) {
			break
		}
		out.jobs[f.JobID] = true
		out.rows = append(out.rows, runningJSON{
			Line: runningLine(f, open[i].StartedAt), Job: f.JobID, StartedAt: open[i].StartedAt.UTC().Format(time.RFC3339),
			KeysDone: f.KeysDone, KeysTotal: f.KeysTotal, Step: f.Step,
			ElapsedMinutes: f.ElapsedMinutes, UsualMinutes: f.UsualMinutes, Slow: f.Slow,
		})
	}
	return out
}

// waiting is the waiting_for of a queued request whose hold_job is its own job and that job has an open run.
func (o openRuns) waiting(row jobrequest.Row) *waitingJSON {
	job := row.Request.JobID
	if row.State != jobrequest.StateQueued || row.HoldJob != job || !o.jobs[job] {
		return nil
	}
	return &waitingJSON{Line: waitingLine(row.HoldAt), Job: job, Since: row.HoldAt.UTC().Format(time.RFC3339)}
}
