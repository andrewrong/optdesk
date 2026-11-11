package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

type uiInvalid struct{}

func (uiInvalid) Error() string { return "invalid" }
func (uiInvalid) Field() string { return "symbol" }
func (uiInvalid) Why() string   { return "does not match" }

// uiStore is a fake underlying.Store that applies the rule of the real one: active with a ConID, else pending.
type uiStore struct {
	mu      sync.Mutex
	rows    []underlying.Row
	applies int
	refuse  bool
	opened  int
	closed  int
	openErr error
}

func (s *uiStore) open(context.Context, string) (underlying.Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened++
	if s.openErr != nil {
		return nil, s.openErr
	}
	return s, nil
}

func (s *uiStore) Current(context.Context) ([]underlying.Row, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]underlying.Row(nil), s.rows...), nil
}
func (s *uiStore) History(context.Context, int) ([]underlying.Row, error) { return nil, nil }
func (s *uiStore) Apply(_ context.Context, c underlying.Change) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applies++
	if s.refuse {
		return 0, uiInvalid{}
	}
	for _, a := range c.Adds {
		r := underlying.Row{
			Key: a.Market + ":" + a.Symbol, Symbol: a.Symbol, Market: a.Market, Kind: a.Kind, ConID: a.ConID,
			Options: a.Options, Fundamentals: a.Fundamentals, Status: underlying.StatusPending,
		}
		if a.ConID > 0 {
			r.Status = underlying.StatusActive
		}
		s.rows = append(s.rows, r)
	}
	return len(c.Adds), nil
}
func (s *uiStore) Close() error { s.closed++; return nil }
func (s *uiStore) Pending(context.Context, time.Time) ([]underlying.Row, error) {
	return nil, nil
}
func (s *uiStore) Settle(context.Context, underlying.Outcome) (bool, error) { return false, nil }

const uiFile = `
[[underlying]]
issuer = "cik:0000320193"
market = "US"
symbol = "aapl"
sec_type = "STK"
con_id = 265598

[[underlying]]
issuer = "cik:0000789019"
market = "US"
symbol = "MSFT"
sec_type = "STK"
`

func uiWrite(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "watch.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// uiRun runs the command function with a fake opener through the inner function.
func uiRun(t *testing.T, s *uiStore, dsn, body string, check bool) (string, error) {
	t.Helper()
	var sb strings.Builder
	err := underlyingImport(context.Background(), s.open, dsn, []byte(body), check, &sb)
	return sb.String(), err
}

func TestUnderlyingImportAddsActiveAndPending(t *testing.T) {
	t.Parallel()
	s := &uiStore{}
	out, err := uiRun(t, s, "dsn", uiFile, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.rows) != 2 {
		t.Fatalf("rows = %+v", s.rows)
	}
	want := map[string]string{"US:AAPL": underlying.StatusActive, "US:MSFT": underlying.StatusPending}
	for _, r := range s.rows {
		if r.Status != want[r.Key] || r.Kind != underlying.KindStock || !r.Options || !r.Fundamentals {
			t.Errorf("row = %+v", r)
		}
	}
	if !strings.Contains(out, "added=2 existing=0 pending=1") || !strings.Contains(out, "pending US:MSFT") || strings.Contains(out, "pending US:AAPL") {
		t.Errorf("out = %q", out)
	}
	if s.closed != 1 {
		t.Errorf("closed = %d", s.closed)
	}
}

func TestUnderlyingImportIsIdempotent(t *testing.T) {
	t.Parallel()
	s := &uiStore{}
	if _, err := uiRun(t, s, "dsn", uiFile, false); err != nil {
		t.Fatal(err)
	}
	out, err := uiRun(t, s, "dsn", uiFile, false)
	if err != nil {
		t.Fatal(err)
	}
	if s.applies != 1 || len(s.rows) != 2 || !strings.Contains(out, "added=0 existing=2 pending=0") {
		t.Errorf("applies=%d rows=%d out=%q", s.applies, len(s.rows), out)
	}
}

func TestUnderlyingImportBadIssuerWritesNothing(t *testing.T) {
	t.Parallel()
	s := &uiStore{}
	body := strings.Replace(uiFile, "cik:0000789019", "bogus-issuer", 1)
	_, err := uiRun(t, s, "dsn", body, false)
	if err == nil || !strings.Contains(err.Error(), "bogus-issuer") || strings.Contains(err.Error(), "/") {
		t.Fatalf("err = %v", err)
	}
	if s.applies != 0 {
		t.Error("applied")
	}
}

func TestUnderlyingImportRefusedRowNamesField(t *testing.T) {
	t.Parallel()
	s := &uiStore{refuse: true}
	_, err := uiRun(t, s, "dsn", uiFile, false)
	if err == nil || !strings.Contains(err.Error(), "symbol") || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("err = %v", err)
	}
	if len(s.rows) != 0 {
		t.Error("wrote")
	}
}

func TestUnderlyingImportEmptyDSN(t *testing.T) {
	t.Parallel()
	s := &uiStore{}
	_, err := uiRun(t, s, "", uiFile, false)
	if err == nil || !strings.Contains(err.Error(), "OT_PG_DSN_SETTINGS") || s.opened != 0 {
		t.Fatalf("err = %v opened = %d", err, s.opened)
	}
}

func TestUnderlyingImportOpenErrorWithheld(t *testing.T) {
	t.Parallel()
	s := &uiStore{openErr: errors.New("dial postgres://u:secretpw@h/db failed")}
	_, err := uiRun(t, s, "dsn", uiFile, false)
	if err == nil || strings.Contains(err.Error(), "secretpw") || !strings.Contains(err.Error(), "OT_PG_DSN_SETTINGS") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnderlyingCheck(t *testing.T) {
	t.Parallel()
	row := func(k string) underlying.Row { return underlying.Row{Key: k} }
	cases := []struct {
		name    string
		rows    []underlying.Row
		wantErr bool
		out     string
	}{
		{"equal", []underlying.Row{row("US:AAPL"), row("US:MSFT")}, false, "file=2 store=2"},
		{"unequal", []underlying.Row{row("US:AAPL")}, true, "file=2 store=1"},
		{"equal count, name missing", []underlying.Row{row("US:AAPL"), row("US:TSLA")}, true, "file=2 store=1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := &uiStore{rows: c.rows}
			out, err := uiRun(t, s, "dsn", uiFile, true)
			var ee *exitError
			if c.wantErr != errors.As(err, &ee) || (ee != nil && ee.code == 0) {
				t.Fatalf("err = %v", err)
			}
			if !strings.Contains(out, c.out) || s.applies != 0 {
				t.Errorf("out=%q applies=%d", out, s.applies)
			}
		})
	}
}

func TestUnderlyingCmdSubcommand(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_X=1\n")
	for _, args := range [][]string{nil, {"export"}} {
		if err := underlyingCmd(a, args); err == nil || !strings.Contains(err.Error(), "import") {
			t.Errorf("args %v: err = %v", args, err)
		}
	}
	p := uiWrite(t, uiFile)
	if err := underlyingCmd(a, []string{"import", p}); err == nil || !strings.Contains(err.Error(), "OT_PG_DSN_SETTINGS") || strings.Contains(err.Error(), p) {
		t.Errorf("err = %v", err)
	}
}
