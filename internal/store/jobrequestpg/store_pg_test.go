//go:build pg

package jobrequestpg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

const rolePassword = "pg-test-only"

var testRoles = []string{"ot_owner", "ot_ro_pages", "ot_runner", "ot_w_settings", ""}

// connURL builds the URL of database dbname on the server the libpq variables name; user "" is the administrator.
func connURL(t *testing.T, dbname, user string) string {
	t.Helper()
	cfg, err := pgx.ParseConfig("")
	if err != nil {
		t.Fatalf("read libpq environment: %v", err)
	}
	if cfg.Host == "" || cfg.User == "" {
		t.Fatal("PGHOST and PGUSER name no server")
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))),
		Path:   "/" + dbname,
	}
	if user != "" {
		u.User = url.UserPassword(user, rolePassword)
	}
	mode := "disable"
	if cfg.TLSConfig != nil {
		mode = "prefer"
	}
	u.RawQuery = url.Values{"sslmode": {mode}}.Encode()
	return u.String()
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// newDatabase makes a throwaway database with the roles and the job request migration, dropped in t.Cleanup.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	cfg, err := pgx.ParseConfig("")
	if err != nil || cfg.Database == "" {
		t.Fatalf("PGDATABASE names no database: %v", err)
	}
	admin, err := pgx.Connect(ctx, connURL(t, cfg.Database, ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(962)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range testRoles {
		var have bool
		if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, r).Scan(&have); err != nil {
			t.Fatal(err)
		}
		if !have {
			if _, err := admin.Exec(ctx, "CREATE ROLE "+r+" LOGIN PASSWORD '"+rolePassword+"'"); err != nil {
				t.Fatalf("create role %s: %v", r, err)
			}
		}
	}
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(962)`)
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	name := nonIdent.ReplaceAllString(strings.ToLower(t.Name()), "_")
	if len(name) > 40 {
		name = name[:40]
	}
	name += "_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		defer func() { _ = admin.Close(ctx) }()
		if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS "`+name+`" WITH (FORCE)`); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})
	files, err := filepath.Glob("../../../migrations/*_job_request.sql")
	if err != nil || len(files) != 1 {
		t.Fatalf("job request migration = %v, %v", files, err)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c := pgxAs(t, name)
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply the job request migration: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_settings, ot_ro_pages, ot_runner`); err != nil {
		t.Fatal(err)
	}
	return name
}

func pgxAs(t *testing.T, name string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, ""))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// openAs opens the store with the DSN of role; the web role posts, the runner role consumes, the pages role reads.
func openAs(t *testing.T, name, role string) *Store {
	t.Helper()
	st, err := open(context.Background(), connURL(t, name, role))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

var (
	at0 = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	at1 = at0.Add(time.Minute)
)

func req(job string, force bool) jobrequest.Request {
	return jobrequest.Request{JobID: job, Day: "2026-11-11", Force: force}
}

func enqueue(t *testing.T, p jobrequest.Poster, r jobrequest.Request) jobrequest.Enqueued {
	t.Helper()
	e, err := p.Enqueue(context.Background(), r)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	return e
}

// TestEnqueueSameForceOneRow (new): a double press of the same job, day and force is one request.
func TestEnqueueSameForceOneRow(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	web := openAs(t, name, "ot_w_settings")
	a := enqueue(t, web, req("calendar_refresh", false))
	b := enqueue(t, web, req("calendar_refresh", false))
	if a.Outcome != jobrequest.EnqueueCreated || b.Outcome != jobrequest.EnqueueExisting || a.Row.ID != b.Row.ID {
		t.Fatalf("outcomes = %v %v, ids %d %d", a.Outcome, b.Outcome, a.Row.ID, b.Row.ID)
	}
	rows, err := web.Recent(context.Background(), 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("recent = %d rows, %v; want 1", len(rows), err)
	}
	if a.Row.State != jobrequest.StateQueued || a.Row.Request != req("calendar_refresh", false) {
		t.Errorf("row = %+v", a.Row)
	}
}

// TestEnqueueOtherForceOwnRow (new): the other force value of the same job and day is its own request.
func TestEnqueueOtherForceOwnRow(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	web := openAs(t, name, "ot_w_settings")
	a := enqueue(t, web, req("calendar_refresh", false))
	b := enqueue(t, web, req("calendar_refresh", true))
	if b.Outcome != jobrequest.EnqueueCreated || a.Row.ID == b.Row.ID || !b.Row.Request.Force {
		t.Fatalf("second = %+v", b)
	}
}

// TestEnqueueWithdrawingRefused (new): a press while the open request is withdrawn stores nothing.
func TestEnqueueWithdrawingRefused(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	web := openAs(t, name, "ot_w_settings")
	a := enqueue(t, web, req("calendar_refresh", false))
	if err := web.Withdraw(context.Background(), a.Row.ID); err != nil {
		t.Fatal(err)
	}
	b := enqueue(t, web, req("calendar_refresh", false))
	if b.Outcome != jobrequest.EnqueueWithdrawing || b.Row.ID != a.Row.ID || b.Row.WithdrawnAt.IsZero() {
		t.Fatalf("second = %+v", b)
	}
	if rows, _ := web.Recent(context.Background(), 10); len(rows) != 1 {
		t.Errorf("rows = %d, want 1", len(rows))
	}
	if err := web.Withdraw(context.Background(), a.Row.ID); err != nil {
		t.Errorf("second withdraw: %v", err)
	}
	if err := web.Withdraw(context.Background(), 9999); err == nil {
		t.Error("withdraw of an unknown id succeeded")
	}
}

// TestEnqueueRowFinishedBetweenRetriesOnce (new): the open row finishes between the insert and the read: the insert
// is tried once more and stores the request; when that insert meets another open row, the answer is Busy.
func TestEnqueueRowFinishedBetweenRetriesOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	web := openAs(t, name, "ot_w_settings")
	run := openAs(t, name, "ot_runner")
	a := enqueue(t, web, req("calendar_refresh", false))

	misses := 0
	web.afterMiss = func(ctx context.Context) {
		misses++
		if err := run.Finish(ctx, a.Row.ID, jobrequest.StateDone, "ok", nil, at0); err != nil {
			t.Error(err)
		}
	}
	b := enqueue(t, web, req("calendar_refresh", false))
	if b.Outcome != jobrequest.EnqueueCreated || b.Row.ID == a.Row.ID || misses != 1 {
		t.Fatalf("outcome = %v, id %d, misses %d", b.Outcome, b.Row.ID, misses)
	}

	// Busy: after the empty read another open row takes the place.
	admin := pgxAs(t, name)
	finished := false
	web.afterMiss = func(ctx context.Context) {
		if finished {
			return
		}
		finished = true
		if err := run.Finish(ctx, b.Row.ID, jobrequest.StateDone, "ok", nil, at0); err != nil {
			t.Error(err)
		}
	}
	retries := 0
	web.beforeRetry = func(ctx context.Context) {
		retries++
		if _, err := admin.Exec(ctx, `INSERT INTO settings.job_request (job_id, session_day, force)
			VALUES ('calendar_refresh', '2026-11-11', false)`); err != nil {
			t.Error(err)
		}
	}
	c := enqueue(t, web, req("calendar_refresh", false))
	if c.Outcome != jobrequest.EnqueueBusy || retries != 1 {
		t.Fatalf("outcome = %v, retries %d; want Busy and one retry", c.Outcome, retries)
	}
	var n int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM settings.job_request`).Scan(&n); err != nil || n != 3 {
		t.Errorf("rows = %d, %v; want 3", n, err)
	}
}

// TestClaimTwiceSecondFalse (new): a queued request is claimed once; a withdrawn one is not claimed.
func TestClaimTwiceSecondFalse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	web := openAs(t, name, "ot_w_settings")
	run := openAs(t, name, "ot_runner")
	a := enqueue(t, web, req("calendar_refresh", false))
	first, err := run.Claim(ctx, a.Row.ID, at0)
	second, err2 := run.Claim(ctx, a.Row.ID, at1)
	if err != nil || err2 != nil || !first || second {
		t.Fatalf("claims = %v %v, %v %v", first, second, err, err2)
	}
	got, _, _ := run.Get(ctx, a.Row.ID)
	if got.State != jobrequest.StateRunning || !got.StartedAt.Equal(at0) {
		t.Errorf("row = %+v", got)
	}
	b := enqueue(t, web, req("calendar_refresh", true))
	if err := web.Withdraw(ctx, b.Row.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := run.Claim(ctx, b.Row.ID, at0); ok || err != nil {
		t.Errorf("claim of a withdrawn request = %v, %v", ok, err)
	}
}

// TestFinishStoresReasonForDoneAndWithdrawn (new): Finish stores state, finish time and reason for a running
// request ended done and for a queued one ended withdrawn.
func TestFinishStoresReasonForDoneAndWithdrawn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	web := openAs(t, name, "ot_w_settings")
	run := openAs(t, name, "ot_runner")
	a := enqueue(t, web, req("calendar_refresh", false))
	b := enqueue(t, web, req("calendar_refresh", true))
	if ok, _ := run.Claim(ctx, a.Row.ID, at0); !ok {
		t.Fatal("claim")
	}
	if err := run.Finish(ctx, a.Row.ID, jobrequest.StateDone, "all_present", nil, at1); err != nil {
		t.Fatal(err)
	}
	if err := web.Withdraw(ctx, b.Row.ID); err != nil {
		t.Fatal(err)
	}
	if err := run.Finish(ctx, b.Row.ID, jobrequest.StateWithdrawn, "withdrawn", nil, at1); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[int64][2]string{a.Row.ID: {"done", "all_present"}, b.Row.ID: {"withdrawn", "withdrawn"}} {
		got, found, err := run.Get(ctx, id)
		if err != nil || !found || string(got.State) != want[0] || got.Reason != want[1] || !got.FinishedAt.Equal(at1) {
			t.Errorf("row %d = %+v, %v, %v; want %v", id, got, found, err, want)
		}
	}
	if err := run.Finish(ctx, a.Row.ID, jobrequest.StateFailed, "again", nil, at1); err == nil {
		t.Error("finish of a finished request succeeded")
	}
	// A finished request frees its slot.
	if c := enqueue(t, web, req("calendar_refresh", false)); c.Outcome != jobrequest.EnqueueCreated {
		t.Errorf("enqueue after finish = %v", c.Outcome)
	}
}

// TestFinishThenReadThroughView (new): the lines written by the runner read back through the pages role.
func TestFinishThenReadThroughView(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	web := openAs(t, name, "ot_w_settings")
	run := openAs(t, name, "ot_runner")
	pages := openAs(t, name, "ot_ro_pages")
	a := enqueue(t, web, req("calendar_refresh", false))
	if ok, _ := run.Claim(ctx, a.Row.ID, at0); !ok {
		t.Fatal("claim")
	}
	lines := []jobrequest.KeyLine{
		{Dataset: "calendar", Subject: "XNYS", Group: "rerun", Status: "done", Returned: 5, Usable: 4, Attempt: 1},
		{Dataset: "bars", Subject: "AAPL", Group: "not_due", DueAt: at1},
	}
	if err := run.Finish(ctx, a.Row.ID, jobrequest.StateDone, "ran", lines, at1); err != nil {
		t.Fatal(err)
	}
	got, found, err := pages.Get(ctx, a.Row.ID)
	if err != nil || !found {
		t.Fatalf("get = %v, %v", found, err)
	}
	if len(got.Lines) != 2 || got.Lines[0] != lines[0] || !got.Lines[1].DueAt.Equal(at1) || got.Lines[1].Group != "not_due" {
		t.Errorf("lines = %+v", got.Lines)
	}
	if _, found, err := pages.Get(ctx, 12345); found || err != nil {
		t.Errorf("get unknown = %v, %v", found, err)
	}
}

// TestHoldShownInView (new): Hold shows in the view and in Queued, and a claim of the request clears nothing.
func TestHoldShownInView(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	web := openAs(t, name, "ot_w_settings")
	run := openAs(t, name, "ot_runner")
	pages := openAs(t, name, "ot_ro_pages")
	a := enqueue(t, web, req("calendar_refresh", false))
	if err := run.Hold(ctx, a.Row.ID, "option_open", at1); err != nil {
		t.Fatal(err)
	}
	got, _, err := pages.Get(ctx, a.Row.ID)
	if err != nil || got.HoldJob != "option_open" || !got.HoldAt.Equal(at1) {
		t.Fatalf("row = %+v, %v", got, err)
	}
	q, err := run.Queued(ctx, 10)
	if err != nil || len(q) != 1 || q[0].HoldJob != "option_open" {
		t.Errorf("queued = %+v, %v", q, err)
	}
}

// TestWithdrawShownInView (new): the withdrawal shows as WithdrawnAt for the pages role and in Queued.
func TestWithdrawShownInView(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	web := openAs(t, name, "ot_w_settings")
	run := openAs(t, name, "ot_runner")
	pages := openAs(t, name, "ot_ro_pages")
	a := enqueue(t, web, req("calendar_refresh", false))
	if got, _, _ := pages.Get(ctx, a.Row.ID); !got.WithdrawnAt.IsZero() {
		t.Fatalf("withdrawn before the withdrawal: %+v", got)
	}
	if err := web.Withdraw(ctx, a.Row.ID); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := pages.Get(ctx, a.Row.ID); got.WithdrawnAt.IsZero() || got.State != jobrequest.StateQueued {
		t.Errorf("row = %+v", got)
	}
	if q, _ := run.Queued(ctx, 10); len(q) != 1 || q[0].WithdrawnAt.IsZero() {
		t.Errorf("queued = %+v", q)
	}
}

// TestFailRunningOnStart (new): every running request ends failed with the reason; queued ones stay.
func TestFailRunningOnStart(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	web := openAs(t, name, "ot_w_settings")
	run := openAs(t, name, "ot_runner")
	a := enqueue(t, web, req("calendar_refresh", false))
	b := enqueue(t, web, req("calendar_refresh", true))
	if ok, _ := run.Claim(ctx, a.Row.ID, at0); !ok {
		t.Fatal("claim")
	}
	if err := run.FailRunning(ctx, "host_restart", at1); err != nil {
		t.Fatal(err)
	}
	got, _, _ := run.Get(ctx, a.Row.ID)
	if got.State != jobrequest.StateFailed || got.Reason != "host_restart" || !got.FinishedAt.Equal(at1) {
		t.Errorf("a = %+v", got)
	}
	if got, _, _ := run.Get(ctx, b.Row.ID); got.State != jobrequest.StateQueued {
		t.Errorf("b = %+v", got)
	}
}
