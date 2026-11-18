package option_notices_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
	buildDay   = contracts.Day("2026-11-09")
)

// fakeAccount holds rows by "alias/day"; errs holds a reader error by alias.
type fakeAccount struct {
	positions map[string][]store.Position
	values    map[string][]store.AccountValue
	errs      map[string]error
}

func (f fakeAccount) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	if err := f.errs[alias]; err != nil {
		return nil, err
	}
	rows := f.positions[alias+"/"+day]
	if rows == nil {
		rows = []store.Position{}
	}
	return rows, nil
}

func (f fakeAccount) Values(_ context.Context, alias, day string) ([]store.AccountValue, error) {
	rows := f.values[alias+"/"+day]
	if rows == nil {
		rows = []store.AccountValue{}
	}
	return rows, nil
}

func (fakeAccount) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return []store.Execution{}, nil
}

// fakeCandidates holds guard rows by day and records every call.
type fakeCandidates struct {
	mu    sync.Mutex
	rows  map[string][]store.Candidate
	calls []string
	err   error
}

func (f *fakeCandidates) Candidates(_ context.Context, day contracts.Day, kind string) ([]store.Candidate, error) {
	f.mu.Lock()
	f.calls = append(f.calls, string(day)+"/"+kind)
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	rows := f.rows[string(day)+"/"+kind]
	if rows == nil {
		rows = []store.Candidate{}
	}
	return rows, nil
}

type fakeEarnings struct {
	rows map[string][]contracts.EarningsDate
}

func (f fakeEarnings) EarningsDates(_ context.Context, issuer string, _, _ contracts.Day) ([]contracts.EarningsDate, error) {
	rows := f.rows[issuer]
	if rows == nil {
		rows = []contracts.EarningsDate{}
	}
	return rows, nil
}

// fakeRule records its inputs and answers a fixed result.
type fakeRule struct {
	mu     sync.Mutex
	inputs []compute.NoticeInput
	params []compute.NoticeParams
	result compute.NoticeResult
}

func (f *fakeRule) Evaluate(in compute.NoticeInput, p compute.NoticeParams) compute.NoticeResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inputs = append(f.inputs, in)
	f.params = append(f.params, p)
	return f.result
}

func stock(symbol string) store.Position {
	return store.Position{Symbol: symbol, SecType: "stock", Multiplier: "1", Quantity: "100", Currency: "USD"}
}

func shortPut(symbol, expiry string) store.Position {
	return store.Position{Symbol: symbol, SecType: "option", Expiry: expiry, Strike: "95", Right: "P",
		Multiplier: "100", Quantity: "-1", Currency: "USD"}
}

func testConfig(a fakeAccount, rule compute.OptionNotices) Config {
	return Config{RunID: "r1", BuildID: "b1", Scope: "s1", Zone: time.UTC, Main: "main", Aliases: []string{"main", "other"},
		Account: a, Candidates: &fakeCandidates{}, Earnings: fakeEarnings{}, Rule: rule,
		Params: compute.NoticeParams{ExpiryDays: 5, CushionWarn: "0.15", CushionAct: "0.05"}}
}

type outPage struct {
	Gaps     []gap        `json:"gaps"`
	Day      string       `json:"day"`
	Status   string       `json:"run_status"`
	Stale    *staleLabel  `json:"stale"`
	Accounts []aliasBlock `json:"accounts"`
}

func buildPage(t *testing.T, c Config) outPage {
	t.Helper()
	b, err := Build(context.Background(), c, nil, buildDay, time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var p outPage
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAliasBlockPerAlias(t *testing.T) {
	t.Parallel()
	a := fakeAccount{positions: map[string][]store.Position{
		"main/2026-11-09":  {stock("ACME")},
		"other/2026-11-09": {stock("BETA")},
	}}
	p := buildPage(t, testConfig(a, &fakeRule{}))
	if len(p.Accounts) != 2 || p.Accounts[0].Alias != "main" || !p.Accounts[0].Main || p.Accounts[1].Alias != "other" || p.Accounts[1].Main {
		t.Fatalf("accounts = %+v", p.Accounts)
	}
	for _, b := range p.Accounts {
		if b.Status != "ok" || b.DataDay == nil || *b.DataDay != "2026-11-09" {
			t.Errorf("block = %+v", b)
		}
	}
	if len(p.Gaps) != 0 || p.Day != "2026-11-09" {
		t.Errorf("gaps %v day %s", p.Gaps, p.Day)
	}
}

func TestMainMissingIsPageGap(t *testing.T) {
	t.Parallel()
	a := fakeAccount{positions: map[string][]store.Position{"other/2026-11-09": {stock("BETA")}}}
	c := testConfig(a, &fakeRule{})
	p := buildPage(t, c)
	if len(p.Gaps) != 1 || p.Gaps[0].Reason != "no_capture" || p.Gaps[0].Kind != "gap" {
		t.Fatalf("gaps = %+v", p.Gaps)
	}
	if p.Accounts[0].Status != "no_capture" {
		t.Errorf("main block = %+v", p.Accounts[0])
	}
	validate(t, "main_missing", buildBytes(t, c))
}

func TestSecondMissingIsInfoBlock(t *testing.T) {
	t.Parallel()
	a := fakeAccount{positions: map[string][]store.Position{"main/2026-11-09": {stock("ACME")}}}
	p := buildPage(t, testConfig(a, &fakeRule{}))
	b := p.Accounts[1]
	if b.Status != "no_capture" || b.DataDay != nil || len(b.Notices) != 0 || b.Note == nil || *b.Note != noteNoCapture {
		t.Fatalf("block = %+v", b)
	}
}

func TestSecondMissingNeverFailsBuild(t *testing.T) {
	t.Parallel()
	a := fakeAccount{
		positions: map[string][]store.Position{"main/2026-11-09": {stock("ACME")}},
		errs:      map[string]error{"other": errors.New("no rows")},
	}
	p := buildPage(t, testConfig(a, &fakeRule{}))
	if p.Accounts[1].Status != "no_capture" {
		t.Errorf("other = %+v", p.Accounts[1])
	}
	if len(p.Gaps) != 0 {
		t.Errorf("gaps = %+v", p.Gaps)
	}
}

func TestMainReaderErrorFailsBuild(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	c := testConfig(fakeAccount{errs: map[string]error{"main": boom}}, &fakeRule{})
	_, err := Build(context.Background(), c, nil, buildDay, time.Time{})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestNilReaderIsGap(t *testing.T) {
	t.Parallel()
	c := testConfig(fakeAccount{}, &fakeRule{})
	c.Account = nil
	p := buildPage(t, c)
	if len(p.Gaps) != 1 || p.Gaps[0].Reason != "no_reader" || len(p.Accounts) != 0 {
		t.Fatalf("page = %+v", p)
	}
	c = testConfig(fakeAccount{positions: map[string][]store.Position{"main/2026-11-09": {stock("ACME")}}}, &fakeRule{})
	c.Candidates = nil
	p = buildPage(t, c)
	if len(p.Gaps) != 1 || p.Gaps[0].Reason != "no_reader" || p.Accounts[0].Status != "ok" {
		t.Fatalf("page = %+v", p)
	}
}

func TestNilRuleIsError(t *testing.T) {
	t.Parallel()
	c := testConfig(fakeAccount{}, nil)
	if _, err := Build(context.Background(), c, nil, buildDay, time.Time{}); err == nil || !strings.Contains(err.Error(), "rule") {
		t.Fatalf("err = %v", err)
	}
	c = testConfig(fakeAccount{}, &fakeRule{})
	c.Zone = nil
	if _, err := Build(context.Background(), c, nil, buildDay, time.Time{}); err == nil {
		t.Error("nil zone gives no error")
	}
}

func TestStaleLabelWhenOlderDay(t *testing.T) {
	t.Parallel()
	a := fakeAccount{positions: map[string][]store.Position{
		"main/2026-11-06":  {stock("ACME")},
		"other/2026-11-09": {stock("BETA")},
	}}
	p := buildPage(t, testConfig(a, &fakeRule{}))
	if p.Accounts[0].Status != "stale" || p.Accounts[1].Status != "ok" {
		t.Errorf("statuses = %s %s", p.Accounts[0].Status, p.Accounts[1].Status)
	}
	if p.Stale == nil || !p.Stale.IsStale || !strings.Contains(p.Stale.Text, "2026-11-06") || p.Stale.DataTime != "2026-11-06T00:00:00+00:00" {
		t.Errorf("stale = %+v", p.Stale)
	}
	a.positions["main/2026-11-09"] = []store.Position{stock("ACME")}
	if p = buildPage(t, testConfig(a, &fakeRule{})); p.Stale != nil {
		t.Errorf("stale = %+v", p.Stale)
	}
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(schemaDir + file)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func buildBytes(t *testing.T, c Config) []byte {
	t.Helper()
	b, err := Build(context.Background(), c, nil, buildDay, time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func validate(t *testing.T, name string, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	schema, err := c.Compile(schemaHost + "page.option_notices_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(v); err != nil {
		t.Errorf("%s: %v\n%s", name, err, b)
	}
}

// fixture is a testdata file: stored rows and the rule's answer.
type fixture struct {
	Positions map[string][]store.Position     `json:"positions"`
	Values    map[string][]store.AccountValue `json:"values"`
	Notices   []compute.Notice                `json:"notices"`
	Cash      []compute.CashDay               `json:"cash"`
	Cushion   string                          `json:"cushion"`
}

func TestOutputValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"severities", "second_missing", "no_rows"} {
		raw, err := os.ReadFile("testdata/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var f fixture
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		rule := &fakeRule{result: compute.NoticeResult{RuleVersion: 1, Notices: f.Notices, CashByDate: f.Cash, Cushion: f.Cushion}}
		c := testConfig(fakeAccount{positions: f.Positions, values: f.Values}, rule)
		b := buildBytes(t, c)
		validate(t, name, b)
		if name == "severities" {
			var p outPage
			if err := json.Unmarshal(b, &p); err != nil {
				t.Fatal(err)
			}
			if got := len(p.Accounts[0].Notices); got != 3 {
				t.Errorf("notices = %d", got)
			}
		}
	}
}

func TestBuiltPageHasPutsAndHints(t *testing.T) {
	t.Parallel()
	a := fakeAccount{positions: map[string][]store.Position{"main/2026-11-09": {shortPut("ACME", "2026-11-13")}}}
	rule := &fakeRule{result: compute.NoticeResult{Cushion: "0.1", CashByDate: []compute.CashDay{
		{Day: "2026-11-13", Currency: "USD", Amount: "9500"}, {Day: "2026-11-20", Currency: "USD", Amount: "1"}}}}
	b := buildBytes(t, testConfig(a, rule))
	validate(t, "puts_hints", b)
	var raw struct {
		Accounts []map[string]json.RawMessage `json:"accounts"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	acc := raw.Accounts[0]
	var cash []map[string]json.RawMessage
	if err := json.Unmarshal(acc["cash_by_date"], &cash); err != nil || len(cash) != 2 {
		t.Fatalf("cash: %v %s", err, acc["cash_by_date"])
	}
	for _, r := range cash {
		if !strings.HasPrefix(string(r["puts"]), "[") {
			t.Errorf("puts = %s", r["puts"])
		}
	}
	for _, k := range []string{"cushion_hint", "note_hint"} {
		if !strings.HasPrefix(string(acc[k]), `"`) {
			t.Errorf("%s = %s", k, acc[k])
		}
	}
}

// fakeSessions answers the given days as session days and records the calendar asked for.
type fakeSessions struct {
	days []string
	err  error
	mu   sync.Mutex
	cals []string
}

func (f *fakeSessions) SessionDays(_ context.Context, cal string, _, _ contracts.Day) ([]contracts.SessionDay, error) {
	f.mu.Lock()
	f.cals = append(f.cals, cal)
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	out := []contracts.SessionDay{}
	for _, d := range f.days {
		tm, _ := time.Parse("2006-01-02", d)
		out = append(out, contracts.SessionDay{SessionDay: types.SerializableDate{Time: tm}, IsSession: true})
	}
	// a non-session day that must be ignored
	tm, _ := time.Parse("2006-01-02", "2026-11-17")
	return append(out, contracts.SessionDay{SessionDay: types.SerializableDate{Time: tm}, IsSession: false}), nil
}

// buildOn builds the page of day with main captured on captureDay.
func buildOn(t *testing.T, s store.SessionDayReader, day, captureDay string) outPage {
	t.Helper()
	a := fakeAccount{positions: map[string][]store.Position{"main/" + captureDay: {stock("ACME")}}}
	c := testConfig(a, &fakeRule{})
	c.Sessions = s
	b, err := Build(context.Background(), c, nil, contracts.Day(day), time.Date(2026, 11, 18, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var p outPage
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

var wedSessions = []string{"2026-11-12", "2026-11-13", "2026-11-16", "2026-11-17", "2026-11-18"}

func TestPreviousSessionCaptureIsFresh(t *testing.T) {
	t.Parallel()
	fs := &fakeSessions{days: wedSessions}
	p := buildOn(t, fs, "2026-11-18", "2026-11-17")
	if p.Accounts[0].Status != "ok" || p.Stale != nil {
		t.Errorf("status = %s stale = %+v", p.Accounts[0].Status, p.Stale)
	}
	if len(fs.cals) != 1 || fs.cals[0] != "XNYS" {
		t.Errorf("calendars = %v", fs.cals)
	}
}

func TestTwoSessionsOldCaptureIsStale(t *testing.T) {
	t.Parallel()
	p := buildOn(t, &fakeSessions{days: wedSessions}, "2026-11-18", "2026-11-16")
	if p.Accounts[0].Status != "stale" || p.Stale == nil || !strings.Contains(p.Stale.Text, "2026-11-16") {
		t.Errorf("status = %s stale = %+v", p.Accounts[0].Status, p.Stale)
	}
}

func TestMondayAcceptsFridayCapture(t *testing.T) {
	t.Parallel()
	s := &fakeSessions{days: []string{"2026-11-05", "2026-11-06", "2026-11-09"}}
	p := buildOn(t, s, "2026-11-09", "2026-11-06")
	if p.Accounts[0].Status != "ok" || p.Stale != nil {
		t.Errorf("status = %s stale = %+v", p.Accounts[0].Status, p.Stale)
	}
	if p = buildOn(t, s, "2026-11-09", "2026-11-05"); p.Accounts[0].Status != "stale" {
		t.Errorf("Thursday status = %s", p.Accounts[0].Status)
	}
}

func TestNilSessionsKeepsOldComparison(t *testing.T) {
	t.Parallel()
	p := buildOn(t, nil, "2026-11-18", "2026-11-17")
	if p.Accounts[0].Status != "stale" || p.Stale == nil {
		t.Errorf("status = %s stale = %+v", p.Accounts[0].Status, p.Stale)
	}
	p = buildOn(t, &fakeSessions{}, "2026-11-18", "2026-11-17")
	if p.Accounts[0].Status != "stale" {
		t.Errorf("empty sessions: status = %s", p.Accounts[0].Status)
	}
}

func TestSessionReaderErrorFailsBuild(t *testing.T) {
	t.Parallel()
	c := testConfig(fakeAccount{}, &fakeRule{})
	c.Sessions = &fakeSessions{err: errors.New("boom")}
	_, err := Build(context.Background(), c, nil, buildDay, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "option_notices_page") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v", err)
	}
}
