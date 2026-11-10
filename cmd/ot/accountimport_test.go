package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

// impStore is an AccountStore with the insert-only rule; failWrite makes any write fail the test.
type impStore struct {
	mu        sync.Mutex
	t         *testing.T
	failWrite bool
	caps      map[string]store.Capture
	writes    int
	closed    int
}

func (s *impStore) WriteCapture(_ context.Context, c store.Capture) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failWrite {
		s.t.Error("WriteCapture called")
	}
	s.writes++
	if _, ok := s.caps[c.ID]; !ok {
		s.caps[c.ID] = c
	}
	return nil
}

func (s *impStore) WriteExecutions(context.Context, string, []store.Execution) error { return nil }

func (s *impStore) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []store.Position{}
	for _, c := range s.caps {
		if c.Alias == alias && c.SessionDay == day {
			out = append(out, c.Positions...)
		}
	}
	return out, nil
}

func (s *impStore) Values(_ context.Context, alias, day string) ([]store.AccountValue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []store.AccountValue{}
	for _, c := range s.caps {
		if c.Alias == alias && c.SessionDay == day {
			out = append(out, c.Values...)
		}
	}
	return out, nil
}

func (s *impStore) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return []store.Execution{}, nil
}

func (s *impStore) Close() error { s.mu.Lock(); s.closed++; s.mu.Unlock(); return nil }

const impDSN = "OT_PG_DSN_ACCOUNT=postgres://u:secretpw@h/db\n"

func impLine(alias, day, netliq, qty string) string {
	return `{"alias":"` + alias + `","session_day":"` + day + `","at":"` + day + `T21:00:00Z","status":"ok",` +
		`"values":[{"tag":"NetLiquidation","value":"` + netliq + `","currency":"USD"}],` +
		`"positions":[{"symbol":"SPY","sec_type":"STK","exchange":"","currency":"USD","expiry":"","strike":"",` +
		`"multiplier":"1","right":"","con_id":7,"quantity":"` + qty + `","avg_cost":"500.25"}]}` + "\n"
}

// impRig returns an app wired to a fake store, the directory holding account.jsonl and the standard output.
func impRig(t *testing.T, input string, st *impStore) (*app, string) {
	t.Helper()
	a, _ := testApp(t, impDSN)
	st.t = t
	if st.caps == nil {
		st.caps = map[string]store.Capture{}
	}
	a.deps.openAccount = func(context.Context, string) (store.AccountStore, error) { return st, nil }
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "account.jsonl"), []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	return a, dir
}

func outOf(a *app) string { return a.stdout.(interface{ String() string }).String() }

func TestAccountImportRequiresThrough(t *testing.T) {
	t.Parallel()
	st := &impStore{failWrite: true}
	a, dir := impRig(t, impLine("main", "2026-11-05", "1", "1"), st)
	err := accountImport(a, []string{dir})
	if err == nil || !strings.Contains(err.Error(), "--through is required") {
		t.Fatalf("err = %v", err)
	}
	if st.writes != 0 {
		t.Fatal("wrote without --through")
	}
}

func TestAccountImportVerifyWritesNothing(t *testing.T) {
	t.Parallel()
	st := &impStore{failWrite: true}
	a, dir := impRig(t, impLine("main", "2026-11-05", "1", "1"), st)
	err := accountImport(a, []string{dir, "--through", "2026-11-06", "--verify"})
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 1 {
		t.Fatalf("err = %v, want exit 1 for a missing day", err)
	}
	if st.writes != 0 || len(st.caps) != 0 || st.closed != 1 {
		t.Fatalf("writes %d, caps %d, closed %d", st.writes, len(st.caps), st.closed)
	}
	if !strings.Contains(outOf(a), "missing=1") {
		t.Fatalf("stdout %q", outOf(a))
	}
}

func TestAccountImportExitCodes(t *testing.T) {
	t.Parallel()
	in := impLine("main", "2026-11-05", "100.5", "10")
	other := impLine("main", "2026-11-05", "100.5", "11")
	for _, tc := range []struct {
		name    string
		seed    string
		input   string
		verify  bool
		wantErr bool
	}{
		{"import inserts", "", in, false, false},
		{"import same", in, in, false, false},
		{"import conflict", in, other, false, true},
		{"verify equal", in, in, true, false},
		{"verify missing", "", in, true, true},
		{"verify different", in, other, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			st := &impStore{}
			a, dir := impRig(t, tc.input, st)
			if tc.seed != "" {
				seed, seedDir := impRig(t, tc.seed, st)
				if err := accountImport(seed, []string{seedDir, "--through", "2026-11-06"}); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{dir, "--through", "2026-11-06"}
			if tc.verify {
				args = append(args, "--verify")
			}
			err := accountImport(a, args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			var ee *exitError
			if tc.wantErr && (!errors.As(err, &ee) || ee.code != 1) {
				t.Fatalf("err = %v, want exit code 1", err)
			}
		})
	}
}

func TestAccountImportPrintsCountsOnly(t *testing.T) {
	t.Parallel()
	st := &impStore{}
	a, dir := impRig(t, impLine("main", "2026-11-05", "123456.78", "10"), st)
	if err := accountImport(a, []string{dir, "--through", "2026-11-06"}); err != nil {
		t.Fatal(err)
	}
	out := outOf(a)
	if out != "account-import lines=1 inserted=1 same=0 conflict=0\n" {
		t.Fatalf("stdout %q", out)
	}
	for _, bad := range []string{"123456.78", "SPY", "main", "secretpw", "NetLiquidation"} {
		if strings.Contains(out, bad) {
			t.Fatalf("stdout holds %q", bad)
		}
	}
}

func TestAccountImportReport(t *testing.T) {
	t.Parallel()
	in := impLine("main", "2026-11-05", "100.5", "10")
	st := &impStore{}
	a, dir := impRig(t, in, st)
	rep := filepath.Join(t.TempDir(), "report.json")
	if err := accountImport(a, []string{dir, "--through", "2026-11-06", "--report", rep}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(rep)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("keys %v", got)
	}
	sum := sha256.Sum256([]byte(in))
	var dataset, sha string
	var ok bool
	var counts map[string]int
	for k, p := range map[string]any{"dataset": &dataset, "file_sha256": &sha, "ok": &ok, "counts": &counts} {
		if err := json.Unmarshal(got[k], p); err != nil {
			t.Fatalf("key %s: %v", k, err)
		}
	}
	if dataset != "account" || sha != hex.EncodeToString(sum[:]) || !ok || counts["inserted"] != 1 || counts["lines"] != 1 {
		t.Fatalf("report %s", raw)
	}
	if strings.Contains(string(raw), "100.5") || strings.Contains(string(raw), "SPY") {
		t.Fatal("report holds a row")
	}
}

func TestAccountImportReportNotOkOnConflict(t *testing.T) {
	t.Parallel()
	st := &impStore{}
	a, dir := impRig(t, impLine("main", "2026-11-05", "1", "1"), st)
	if err := accountImport(a, []string{dir, "--through", "2026-11-06"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "account.jsonl"), []byte(impLine("main", "2026-11-05", "1", "2")), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := filepath.Join(t.TempDir(), "r.json")
	if err := accountImport(a, []string{dir, "--through", "2026-11-06", "--report", rep}); err == nil {
		t.Fatal("conflict did not fail")
	}
	raw, err := os.ReadFile(rep)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(raw, &got); err != nil || got.OK {
		t.Fatalf("report %s, err %v", raw, err)
	}
}

func TestAccountImportMapRefusesUnnamedAlias(t *testing.T) {
	t.Parallel()
	st := &impStore{failWrite: true}
	a, dir := impRig(t, impLine("old", "2026-11-05", "1", "1")+impLine("other", "2026-11-05", "1", "1"), st)
	err := accountImport(a, []string{dir, "--through", "2026-11-06", "--map", "old=main"})
	if err == nil {
		t.Fatal("unmapped alias accepted")
	}
	if st.writes != 0 {
		t.Fatal("wrote before refusing")
	}
}

func TestAccountImportMapRenames(t *testing.T) {
	t.Parallel()
	st := &impStore{}
	a, dir := impRig(t, impLine("old", "2026-11-05", "1", "1"), st)
	if err := accountImport(a, []string{dir, "--through", "2026-11-06", "--map", "old=main"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.caps["main:2026-11-05:import"]; !ok {
		t.Fatalf("caps %v", st.caps)
	}
}

func TestAccountImportMissingDSN(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_LOADER=x\n")
	a.deps.openAccount = func(context.Context, string) (store.AccountStore, error) {
		t.Error("store opened")
		return nil, errors.New("no")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "account.jsonl"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := accountImport(a, []string{dir, "--through", "2026-11-06"})
	if err == nil || !strings.Contains(err.Error(), store.DSNEnvAccount) {
		t.Fatalf("err = %v", err)
	}
}

func TestAccountImportOpenErrorWithheld(t *testing.T) {
	t.Parallel()
	a, dir := impRig(t, impLine("main", "2026-11-05", "1", "1"), &impStore{})
	a.deps.openAccount = func(context.Context, string) (store.AccountStore, error) {
		return nil, errors.New("dial postgres://u:secretpw@h/db failed")
	}
	err := accountImport(a, []string{dir, "--through", "2026-11-06"})
	if err == nil || strings.Contains(err.Error(), "secretpw") || strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("err = %v", err)
	}
}
