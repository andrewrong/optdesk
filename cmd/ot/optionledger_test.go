package main

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	optionledgerjob "github.com/andrewrong/optdesk/internal/jobs/optionledger"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	ledgerDSNLine = "OT_PG_DSN_OPTION_LEDGER=led-dsn-secret\n"
	tokenA        = "token-a-secret"
	acctA         = "U1234567"
)

// flexTestFile is a file of two logins and one account: login main is required.
var flexTestFile = sources.FlexFile{
	Logins: []sources.FlexLogin{
		{Name: "main", TokenEnv: "TOK_MAIN", TradeQueries: []string{"11"}, Required: true},
		{Name: "second", TokenEnv: "TOK_SECOND", TradeQueries: []string{"22"}},
	},
	Accounts: []sources.FlexAccount{{Alias: "a", AccountEnv: "ACCT_A"}},
}

// ledgerFakeStore is the option ledger store with no rows.
type ledgerFakeStore struct {
	mu      sync.Mutex
	closed  int
	aliases []string
	rows    map[string][]store.FlexTradeRow
	records []store.FlexFetchRecord
}

func (s *ledgerFakeStore) RecordFlexFetch(_ context.Context, rec store.FlexFetchRecord, rows []store.FlexTradeRow) (store.FlexInsert, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, rec)
	return store.FlexInsert{New: len(rows)}, nil
}

func (s *ledgerFakeStore) ImportFlexTradeRows(context.Context, []store.FlexTradeRow) (store.FlexImportResult, error) {
	return store.FlexImportResult{}, nil
}

func (s *ledgerFakeStore) FlexTradeRows(_ context.Context, alias string) ([]store.FlexTradeRow, error) {
	return append([]store.FlexTradeRow{}, s.rows[alias]...), nil
}

func (s *ledgerFakeStore) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}

func (s *ledgerFakeStore) StoredAliases(context.Context) ([]string, error) {
	return append([]string{}, s.aliases...), nil
}

func (s *ledgerFakeStore) Close() error { s.mu.Lock(); s.closed++; s.mu.Unlock(); return nil }

// ledgerRig is a run-once rig with the option_ledger constructors faked.
type ledgerRig struct {
	*rig
	st      *ledgerFakeStore
	opened  []string
	cfg     jobs.OptionLedgerConfig
	built   bool
	tokens  []string
	flexErr error
	file    sources.FlexFile
	path    string
}

type tokenSource struct{ token string }

// Fetch answers one statement of the account of ACCT_A with one trade row.
func (tokenSource) Fetch(context.Context, sources.FlexRequest) (sources.FlexReport, error) {
	st := sources.FlexStatement{AccountID: acctA, Trades: []map[string]string{
		{"accountId": acctA, "levelOfDetail": "EXECUTION", "transactionID": "t1", "symbol": "ABC"},
	}}
	return sources.FlexReport{FetchedAt: time.Date(2026, 11, 5, 3, 0, 0, 0, time.UTC), RawSHA256: "00", Statements: []sources.FlexStatement{st}}, nil
}

// newLedgerRig fills the rig; real makes the job the real one (the rule of the job is then under test).
func newLedgerRig(t *testing.T, env string, real bool) *ledgerRig {
	t.Helper()
	l := &ledgerRig{rig: newRig(t, env), st: &ledgerFakeStore{}, file: flexTestFile}
	d := &l.a.deps
	d.openOptionLedgerStore = func(_ context.Context, dsn string) (store.OptionLedgerStore, error) {
		l.opened = append(l.opened, dsn)
		return l.st, nil
	}
	d.loadFlexFile = func(path string) (sources.FlexFile, error) {
		l.path = path
		return l.file, l.flexErr
	}
	d.newFlexSource = func(c sources.FlexConfig) (sources.FlexSource, error) {
		l.tokens = append(l.tokens, c.Token)
		return tokenSource{c.Token}, nil
	}
	if real {
		d.newOptionLedgerJob = optionledgerjob.New
	} else {
		d.newOptionLedgerJob = func(_ jobs.OptionLedgerStores, c jobs.OptionLedgerConfig, _ clock.Clock) (contracts.Job, error) {
			l.cfg, l.built = c, true
			return namedJob{id: "option_ledger"}, nil
		}
	}
	return l
}

func (l *ledgerRig) jobIDs() []string {
	var ids []string
	for _, j := range l.jobs {
		ids = append(ids, j.ID())
	}
	return ids
}

func (l *ledgerRig) run(t *testing.T) {
	t.Helper()
	if err := runOnce(l.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
}

func TestOptionLedgerInRegistry(t *testing.T) {
	t.Parallel()
	env := fullEnv + ledgerDSNLine + "TOK_MAIN=" + tokenA + "\nTOK_SECOND=token-b-secret\nACCT_A=" + acctA + "\n"
	l := newLedgerRig(t, env, false)
	l.run(t)
	if !slices.Contains(l.jobIDs(), "option_ledger") {
		t.Fatalf("jobs %v hold no option_ledger", l.jobIDs())
	}
	ids := l.jobIDs()
	if slices.Index(ids, "option_ledger") != slices.Index(ids, "option_eod")+1 {
		t.Errorf("option_ledger does not follow option_eod: %v", ids)
	}
	if !slices.Equal(l.opened, []string{"led-dsn-secret"}) {
		t.Errorf("store opened with %v", l.opened)
	}
	if want := filepath.Join(l.a.getenv("OT_HOME"), "flex.toml"); l.path != want {
		t.Errorf("flex file %q, want %q", l.path, want)
	}
	if !slices.Equal(l.tokens, []string{tokenA, "token-b-secret"}) {
		t.Errorf("tokens %v: one source per login from the variable the file names", l.tokens)
	}
	if got := l.cfg.Accounts; len(got) != 1 || got[acctA] != "a" {
		t.Errorf("accounts %v, want the number of ACCT_A mapped to alias a", got)
	}
	if len(l.cfg.Logins) != 2 || !l.cfg.Logins[0].Required || l.cfg.Logins[0].Source == nil || l.cfg.Logins[1].Source == nil {
		t.Errorf("logins %+v", l.cfg.Logins)
	}
	if l.st.closed != 1 {
		t.Errorf("ledger store closed %d times, want 1", l.st.closed)
	}
}

func TestLoginWithoutTokenLeftOut(t *testing.T) {
	t.Parallel()
	// TOK_SECOND is empty: that login reaches the job with a nil Source and a logged reason.
	env := fullEnv + ledgerDSNLine + "TOK_MAIN=" + tokenA + "\nTOK_SECOND=\nACCT_A=" + acctA + "\n"
	l := newLedgerRig(t, env, false)
	l.run(t)
	if len(l.cfg.Logins) != 2 || l.cfg.Logins[1].Name != "second" || l.cfg.Logins[1].Source != nil || l.cfg.Logins[0].Source == nil {
		t.Fatalf("logins %+v: want main with a source and second with none", l.cfg.Logins)
	}
	log := l.log.String()
	if !strings.Contains(log, "login second: TOK_SECOND is not set") {
		t.Errorf("log lacks the login and the variable name:\n%s", log)
	}
	if !slices.Contains(l.jobIDs(), "option_ledger") {
		t.Errorf("a login without a token took the job out: %v", l.jobIDs())
	}
}

func TestRequiredLoginWithoutTokenFails(t *testing.T) {
	t.Parallel()
	// the required login has no token: the wiring builds the job, the run returns a failed result.
	env := fullEnv + ledgerDSNLine + "TOK_MAIN=\nTOK_SECOND=token-b-secret\nACCT_A=" + acctA + "\n"
	l := newLedgerRig(t, env, true)
	l.run(t)
	var job contracts.Job
	for _, j := range l.jobs {
		if j.ID() == "option_ledger" {
			job = j
		}
	}
	if job == nil {
		t.Fatalf("the wiring did not build the job: %v", l.jobIDs())
	}
	keys, _, err := job.Plan(context.Background(), "2026-11-04")
	if err != nil {
		t.Fatal(err)
	}
	res := job.Run(context.Background(), "2026-11-04", keys)
	if res.Err == nil {
		t.Fatal("the run of a required login without a token returned an ok result")
	}
}

func TestMissingFlexFileKeepsOtherJobs(t *testing.T) {
	t.Parallel()
	env := fullEnv + ledgerDSNLine
	l := newLedgerRig(t, env, false)
	l.flexErr = sources.ErrFlexNotConfigured
	l.run(t)
	if l.built || len(l.opened) != 0 {
		t.Errorf("a missing file still built the job (%v) or opened the store (%v)", l.built, l.opened)
	}
	want := []string{"fred_series", "option_eod", "quote_snapshot", "compute_figures", "calendar_refresh", "earnings_refresh"}
	if !slices.Equal(l.jobIDs(), want) {
		t.Errorf("jobs %v, want %v", l.jobIDs(), want)
	}
	if !strings.Contains(l.log.String(), "flex.toml is not configured") {
		t.Errorf("no logged reason:\n%s", l.log.String())
	}
}

func TestMissingLedgerDSNKeepsOtherJobs(t *testing.T) {
	t.Parallel()
	l := newLedgerRig(t, fullEnv, false)
	l.run(t)
	if l.built || len(l.opened) != 0 || l.path != "" {
		t.Errorf("an empty line still built the job (%v), opened the store (%v) or read the file (%q)", l.built, l.opened, l.path)
	}
	want := []string{"fred_series", "option_eod", "quote_snapshot", "compute_figures", "calendar_refresh", "earnings_refresh"}
	if !slices.Equal(l.jobIDs(), want) {
		t.Errorf("jobs %v, want %v", l.jobIDs(), want)
	}
	if !strings.Contains(l.log.String(), "OT_PG_DSN_OPTION_LEDGER is not set") {
		t.Errorf("no logged reason:\n%s", l.log.String())
	}
}

func TestFlexValuesNeverPrinted(t *testing.T) {
	t.Parallel()
	env := fullEnv + ledgerDSNLine + "TOK_MAIN=" + tokenA + "\nTOK_SECOND=\nACCT_A=" + acctA + "\n"
	l := newLedgerRig(t, env, true)
	l.run(t)
	out := l.log.String() + l.a.stdout.(interface{ String() string }).String()
	for _, secret := range []string{tokenA, acctA, "led-dsn-secret"} {
		if strings.Contains(out, secret) {
			t.Errorf("a value of the file or the line is printed: %q", secret)
		}
	}
}

func TestOptionLedgerResultShape(t *testing.T) {
	t.Parallel()
	env := fullEnv + ledgerDSNLine + "TOK_MAIN=" + tokenA + "\nTOK_SECOND=token-b-secret\nACCT_A=" + acctA + "\n"
	l := newLedgerRig(t, env, true)
	l.run(t)
	var job contracts.Job
	for _, j := range l.jobs {
		if j.ID() == "option_ledger" {
			job = j
		}
	}
	if job == nil {
		t.Fatal("no option_ledger job")
	}
	keys, _, err := job.Plan(context.Background(), "2026-11-04")
	if err != nil {
		t.Fatal(err)
	}
	// a key names the job's dataset and an alias; the account number is nowhere in it
	if len(keys) != 1 || keys[0].SubjectKey != "a" {
		t.Fatalf("keys %+v, want one key of alias a", keys)
	}
	res := job.Run(context.Background(), "2026-11-04", keys)
	if res.Err != nil {
		t.Fatalf("status of the run: %v", res.Err)
	}
	if strings.Contains(keys[0].DatasetID+keys[0].SubjectKey, acctA) {
		t.Error("the account number is in a key")
	}
	if len(l.st.records) == 0 || l.st.records[0].Statements != 1 {
		t.Errorf("fetch records %+v: the statement of the allow-listed account is not recorded", l.st.records)
	}
}
