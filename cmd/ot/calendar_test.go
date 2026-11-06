package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const syntheticFile = "testdata/calendar_synthetic.json"

// memStore keeps rows in a map keyed by calendar id and session day, as the real upsert does.
type memStore struct {
	store.SessionDayStore
	mu      sync.Mutex
	rows    map[string]contracts.SessionDay
	batches [][]contracts.SessionDay
	closed  int
	fail    error
}

func (m *memStore) UpsertSessionDays(_ context.Context, rows []contracts.SessionDay) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return m.fail
	}
	m.batches = append(m.batches, rows)
	for _, r := range rows {
		m.rows[r.CalendarID+"|"+r.SessionDay.Format(dayLayout)] = r
	}
	return nil
}

func (m *memStore) SessionDays(_ context.Context, id string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []contracts.SessionDay
	for _, r := range m.rows {
		d := contracts.Day(r.SessionDay.Format(dayLayout))
		if r.CalendarID == id && d >= from && d <= to {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memStore) Close() error { m.closed++; return nil }

type calEnv struct {
	a         *app
	log       *bytes.Buffer
	store     *memStore
	opens     int
	envLines  map[string]string
	readPaths []string
}

// newCalEnv builds an app over the process environment env with a fake readEnv that serves e.envLines
// (the .env lines) and a fake openSessionDay that accepts only the loader DSN.
func newCalEnv(t *testing.T, env map[string]string, profileName string) *calEnv {
	t.Helper()
	var out bytes.Buffer
	e := &calEnv{store: &memStore{rows: map[string]contracts.SessionDay{}}, log: &bytes.Buffer{},
		envLines: map[string]string{store.DSNEnvLoader: "loader-dsn"}}
	a, err := newApp(func(k string) string { return env[k] }, &out, e.log)
	if err != nil {
		t.Fatal(err)
	}
	a.profile.Name = profileName
	a.deps = newDeps()
	a.deps.readEnv = func(path string) (map[string]string, error) {
		e.readPaths = append(e.readPaths, path)
		return e.envLines, nil
	}
	a.deps.openSessionDay = func(_ context.Context, dsn string) (store.SessionDayStore, error) {
		e.opens++
		if dsn != "loader-dsn" {
			return nil, fmt.Errorf("unexpected dsn %q", dsn)
		}
		return e.store, nil
	}
	e.a = a
	return e
}

func realCal(t *testing.T) *calEnv {
	t.Helper()
	return newCalEnv(t, map[string]string{"OT_HOME": "home", "OT_ZONE": "UTC"}, "real")
}

const goodProv = `{"generator":"g","generator_version":"1","source_library":"s","source_library_version":"2","generated_at":"2026-01-01T00:00:00Z"}`

func rowJSON(id, day string, session bool, early bool, open, closeAt string) string {
	return fmt.Sprintf(`{"calendar_id":%q,"session_day":%q,"is_session":%t,"early_close":%t,"open_at":%q,"close_at":%q}`,
		id, day, session, early, open, closeAt)
}

func writeCal(t *testing.T, prov string, rows ...string) string {
	t.Helper()
	body := `{"rows":[` + strings.Join(rows, ",") + `]`
	if prov != "" {
		body += `,"provenance":` + prov
	}
	body += `}`
	return writeRaw(t, body)
}

func writeRaw(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cal.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func ts(s string) time.Time {
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return v
}

func TestCalendarUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"load"}, {"dump", "f"}} {
		e := realCal(t)
		err := runCalendar(e.a, args)
		if err == nil || !strings.Contains(err.Error(), "usage") || e.opens != 0 {
			t.Errorf("args %v: err %v opens %d", args, err, e.opens)
		}
	}
}

func TestCalendarLoadDemoRefused(t *testing.T) {
	t.Parallel()
	e := newCalEnv(t, map[string]string{"OT_HOME": "home"}, "demo")
	err := runCalendar(e.a, []string{"load", filepath.Join(t.TempDir(), "missing.json")})
	if err == nil || strings.Contains(err.Error(), "missing.json") || e.opens != 0 {
		t.Fatalf("err %v opens %d", err, e.opens)
	}
}

func TestCalendarLoadNoLoaderLine(t *testing.T) {
	t.Parallel()
	for name, lines := range map[string]map[string]string{"absent": {}, "empty": {store.DSNEnvLoader: ""}} {
		e := realCal(t)
		e.envLines = lines
		err := runCalendar(e.a, []string{"load", syntheticFile})
		if err == nil || !strings.Contains(err.Error(), store.DSNEnvLoader) || !strings.Contains(err.Error(), "ot setup pg") || e.opens != 0 {
			t.Errorf("%s: err %v opens %d", name, err, e.opens)
		}
	}
}

func TestCalendarLoadNeverFallsBack(t *testing.T) {
	t.Parallel()
	e := newCalEnv(t, map[string]string{"OT_HOME": "home", "OT_ZONE": "UTC", store.AdminDSNEnv: "admin-dsn"}, "real")
	e.envLines = map[string]string{}
	err := runCalendar(e.a, []string{"load", syntheticFile})
	if err == nil || e.opens != 0 || len(e.store.batches) != 0 {
		t.Fatalf("err %v opens %d", err, e.opens)
	}
}

func TestCalendarLoadChecksBeforeEnv(t *testing.T) {
	t.Parallel()
	e := realCal(t)
	p := writeCal(t, "", rowJSON("TEST", "2026-03-02", true, false, "2026-03-02T14:30:00Z", "2026-03-02T21:00:00Z"))
	err := runCalendar(e.a, []string{"load", p})
	if err == nil || !strings.Contains(err.Error(), "provenance") || len(e.readPaths) != 0 || e.opens != 0 {
		t.Fatalf("err %v reads %v opens %d", err, e.readPaths, e.opens)
	}
}

func TestCalendarLoadOpenFailureWritesNothing(t *testing.T) {
	t.Parallel()
	e := realCal(t)
	e.a.deps.openSessionDay = func(context.Context, string) (store.SessionDayStore, error) {
		e.opens++
		return nil, errors.New("open-secret-text")
	}
	err := runCalendar(e.a, []string{"load", syntheticFile})
	if err == nil || strings.Contains(err.Error(), "open-secret-text") || e.opens != 1 || len(e.store.batches) != 0 || e.store.closed != 0 {
		t.Fatalf("err %v opens %d", err, e.opens)
	}
}

func TestCalendarLoadProvenanceRequired(t *testing.T) {
	t.Parallel()
	e := realCal(t)
	if err := runCalendar(e.a, []string{"load", syntheticFile}); err != nil {
		t.Fatal(err)
	}
	if len(e.store.rows) != 3 || e.opens != 1 {
		t.Fatalf("rows %d opens %d", len(e.store.rows), e.opens)
	}
	e = realCal(t)
	p := writeCal(t, "", rowJSON("TEST", "2026-03-02", true, false, "2026-03-02T14:30:00Z", "2026-03-02T21:00:00Z"))
	err := runCalendar(e.a, []string{"load", p})
	if err == nil || !strings.Contains(err.Error(), p) || !strings.Contains(err.Error(), "provenance") || e.opens != 0 {
		t.Fatalf("err %v opens %d", err, e.opens)
	}
}

func TestCalendarLoadProvenanceFields(t *testing.T) {
	t.Parallel()
	row := rowJSON("TEST", "2026-03-02", true, false, "2026-03-02T14:30:00Z", "2026-03-02T21:00:00Z")
	for _, key := range []string{"generator", "generator_version", "source_library", "source_library_version", "generated_at"} {
		prov := strings.Replace(goodProv, `"`+key+`":"`, `"`+key+`":"`, 1)
		// empty the value of key
		i := strings.Index(prov, `"`+key+`":"`) + len(key) + 4
		j := i + strings.Index(prov[i:], `"`)
		prov = prov[:i] + prov[j:]
		e := realCal(t)
		p := writeCal(t, prov, row)
		err := runCalendar(e.a, []string{"load", p})
		if err == nil || !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), p) || e.opens != 0 {
			t.Errorf("%s: err %v opens %d", key, err, e.opens)
		}
	}
	e := realCal(t)
	p := writeCal(t, strings.Replace(goodProv, "2026-01-01T00:00:00Z", "yesterday", 1), row)
	if err := runCalendar(e.a, []string{"load", p}); err == nil || !strings.Contains(err.Error(), "generated_at") || e.opens != 0 {
		t.Fatalf("err %v opens %d", err, e.opens)
	}
}

func TestCalendarLoadRowOrder(t *testing.T) {
	t.Parallel()
	a := rowJSON("TEST", "2026-03-02", true, false, "2026-03-02T14:30:00Z", "2026-03-02T21:00:00Z")
	b := rowJSON("TEST", "2026-03-03", true, false, "2026-03-03T14:30:00Z", "2026-03-03T21:00:00Z")
	other := rowJSON("OTHER", "2026-03-01", true, false, "2026-03-01T14:30:00Z", "2026-03-01T21:00:00Z")
	for name, rows := range map[string][]string{"descending": {b, a}, "duplicate": {a, a}} {
		e := realCal(t)
		err := runCalendar(e.a, []string{"load", writeCal(t, goodProv, rows...)})
		if err == nil || !strings.Contains(err.Error(), "ascending") || e.opens != 0 {
			t.Errorf("%s: err %v opens %d", name, err, e.opens)
		}
	}
	e := realCal(t)
	if err := runCalendar(e.a, []string{"load", writeCal(t, goodProv, a, b, other)}); err != nil {
		t.Fatalf("order is per calendar id: %v", err)
	}
}

func TestCalendarLoadRowHours(t *testing.T) {
	t.Parallel()
	const d = "2026-03-02"
	o, c := d+"T14:30:00Z", d+"T21:00:00Z"
	midnight := d + "T00:00:00Z"
	cases := []struct {
		name    string
		row     string
		wantErr bool
	}{
		{"session open before close", rowJSON("TEST", d, true, false, o, c), false},
		{"closed day open equals close", rowJSON("TEST", d, false, false, midnight, midnight), false},
		{"session open equals close", rowJSON("TEST", d, true, false, o, o), true},
		{"session open after close", rowJSON("TEST", d, true, false, c, o), true},
		{"closed day open differs", rowJSON("TEST", d, false, false, o, c), true},
		{"closed day early close", rowJSON("TEST", d, false, true, midnight, midnight), true},
	}
	for _, tc := range cases {
		e := realCal(t)
		err := runCalendar(e.a, []string{"load", writeCal(t, goodProv, tc.row)})
		if (err != nil) != tc.wantErr || (tc.wantErr && e.opens != 0) {
			t.Errorf("%s: err %v opens %d", tc.name, err, e.opens)
		}
	}
}

func TestCalendarLoadClosedDayShape(t *testing.T) {
	t.Parallel()
	const day = "2026-03-03"
	e := realCal(t)
	p := writeCal(t, goodProv, rowJSON("TEST", day, false, true, day+"T00:00:00Z", day+"T00:00:00Z"))
	err := runCalendar(e.a, []string{"load", p})
	if err == nil || !strings.Contains(err.Error(), p) || strings.Contains(err.Error(), "T00:00") || e.opens != 0 {
		t.Fatalf("err %v opens %d", err, e.opens)
	}
	empty := writeCal(t, goodProv, rowJSON("", day, false, false, day+"T00:00:00Z", day+"T00:00:00Z"))
	if err := runCalendar(e.a, []string{"load", empty}); err == nil || e.opens != 0 {
		t.Fatalf("empty calendar id: err %v opens %d", err, e.opens)
	}
}

func TestCalendarLoadMalformed(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"provenance":`, `[]`, `{"provenance":` + goodProv + `,"rows":[{"calendar_id":"SECRETROW"}]}`} {
		e := realCal(t)
		p := writeRaw(t, body)
		err := runCalendar(e.a, []string{"load", p})
		if err == nil || !strings.Contains(err.Error(), p) || strings.Contains(err.Error(), "SECRETROW") || e.opens != 0 {
			t.Errorf("%q: err %v opens %d", body, err, e.opens)
		}
	}
}

func TestCalendarLoadEmptyFile(t *testing.T) {
	t.Parallel()
	for _, body := range []string{``, `{"provenance":` + goodProv + `,"rows":[]}`} {
		e := realCal(t)
		p := writeRaw(t, body)
		err := runCalendar(e.a, []string{"load", p})
		if err == nil || !strings.Contains(err.Error(), p) || e.opens != 0 {
			t.Errorf("%q: err %v opens %d", body, err, e.opens)
		}
	}
}

func TestCalendarLoadMissingFile(t *testing.T) {
	t.Parallel()
	e := realCal(t)
	p := filepath.Join(t.TempDir(), "nope.json")
	err := runCalendar(e.a, []string{"load", syntheticFile, p})
	if err == nil || !strings.Contains(err.Error(), p) || e.opens != 0 || len(e.store.batches) != 0 {
		t.Fatalf("err %v opens %d", err, e.opens)
	}
}

func TestCalendarLoadIdempotent(t *testing.T) {
	t.Parallel()
	e := realCal(t)
	if err := runCalendar(e.a, []string{"load", syntheticFile}); err != nil {
		t.Fatal(err)
	}
	first := map[string]contracts.SessionDay{}
	for k, v := range e.store.rows {
		first[k] = v
	}
	if err := runCalendar(e.a, []string{"load", syntheticFile}); err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 || len(e.store.rows) != 3 {
		t.Fatalf("rows %d then %d", len(first), len(e.store.rows))
	}
	for k, v := range first {
		got := e.store.rows[k]
		if !got.OpenAt.Equal(v.OpenAt) || !got.CloseAt.Equal(v.CloseAt) || got.IsSession != v.IsSession || got.EarlyClose != v.EarlyClose {
			t.Errorf("%s changed: %+v vs %+v", k, v, got)
		}
	}
}

func TestCalendarLoadClosesStore(t *testing.T) {
	t.Parallel()
	e := realCal(t)
	if err := runCalendar(e.a, []string{"load", syntheticFile, syntheticFile}); err != nil {
		t.Fatal(err)
	}
	if e.opens != 1 || e.store.closed != 1 || len(e.store.batches) != 2 {
		t.Fatalf("opens %d closed %d batches %d", e.opens, e.store.closed, len(e.store.batches))
	}
	days := []string{}
	for _, r := range e.store.batches[0] {
		days = append(days, r.SessionDay.Format(dayLayout))
	}
	if strings.Join(days, ",") != "2026-03-02,2026-03-03,2026-03-04" {
		t.Fatalf("rows not in file order: %v", days)
	}
	e = realCal(t)
	e.store.fail = errors.New("upsert-secret-text")
	err := runCalendar(e.a, []string{"load", syntheticFile})
	if err == nil || strings.Contains(err.Error(), "upsert-secret-text") || e.store.closed != 1 {
		t.Fatalf("err %v closed %d", err, e.store.closed)
	}
}

func TestCalendarLoadFeedsCalendar(t *testing.T) {
	t.Parallel()
	e := realCal(t)
	if err := runCalendar(e.a, []string{"load", syntheticFile}); err != nil {
		t.Fatal(err)
	}
	cal := newDeps().newCalendar(e.store)
	ctx := context.Background()
	s, err := cal.Session(ctx, "TEST", "2026-03-02")
	if err != nil || !s.IsSession || !s.OpenAt.Equal(ts("2026-03-02T14:30:00Z")) || !s.CloseAt.Equal(ts("2026-03-02T21:00:00Z")) {
		t.Fatalf("session %+v err %v", s, err)
	}
	s, err = cal.Session(ctx, "TEST", "2026-03-03")
	if err != nil || s.IsSession {
		t.Fatalf("closed day %+v err %v", s, err)
	}
	n, err := cal.NextSession(ctx, "TEST", "2026-03-02")
	if err != nil || n.Day != "2026-03-04" || !n.EarlyClose {
		t.Fatalf("next %+v err %v", n, err)
	}
	_, err = cal.Session(ctx, "NOPE", "2026-03-02")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.UnknownCalendar {
		t.Fatalf("unknown calendar: %v", err)
	}
}

func calSentinel(t *testing.T, dsn string) {
	t.Helper()
	e := realCal(t)
	e.a.deps.openSessionDay = func(context.Context, string) (store.SessionDayStore, error) {
		return nil, errors.New("dial " + dsn + " pw sentinel-pw")
	}
	err := runCalendar(e.a, []string{"load", syntheticFile})
	if err == nil {
		t.Fatal("want an error")
	}
	e.a.log.Error("command failed", "component", "ot", "err", err.Error())
	for _, s := range []string{dsn, "sentinel-pw"} {
		if strings.Contains(err.Error(), s) || strings.Contains(e.log.String(), s) {
			t.Fatalf("secret %q leaked: %v / %q", s, err, e.log.String())
		}
	}
}

func TestCalendarLoadMalformedDSN(t *testing.T) {
	t.Parallel()
	calSentinel(t, "postgres://sentinel-user:sentinel-pw@%zz/db")
}

func TestCalendarLoadWrongPasswordDSN(t *testing.T) {
	t.Parallel()
	calSentinel(t, "postgres://sentinel-user:sentinel-pw@127.0.0.1:0/db")
}
