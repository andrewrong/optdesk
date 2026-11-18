package option_candidates_page

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

// fakeOwner answers Params with its list; the other methods are not used by the page.
type fakeOwner struct {
	ownerset.Reader
	params []ownerset.Param
	err    error
}

func (f fakeOwner) Params(context.Context, time.Time) ([]ownerset.Param, error) {
	return f.params, f.err
}

// fakeRules resolves the stored map over the default 0.3 of put_delta_max.
type fakeRules struct{ settingsapi.RuleParams }

func (fakeRules) Resolve(stored map[string]string) map[string]string {
	if v, ok := stored["put_delta_max"]; ok {
		return map[string]string{"put_delta_max": v}
	}
	return map[string]string{"put_delta_max": "0.3"}
}

func factsOf(t *testing.T, c Config) (tierFacts, error) {
	t.Helper()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	return tierInputOf(context.Background(), c, buildDay, rows, testNow)
}

func TestDeltaBoundFromOwner(t *testing.T) {
	t.Parallel()
	c := Config{Owner: fakeOwner{params: []ownerset.Param{{Key: "put_delta_max", Value: "0.25"}}}, RuleParams: fakeRules{}}
	f, err := factsOf(t, c)
	if err != nil {
		t.Fatal(err)
	}
	if f.in.Params.DeltaMax != "0.25" || !f.deltaSet {
		t.Fatalf("params %+v set %v", f.in.Params, f.deltaSet)
	}
}

func TestDeltaBoundDefault(t *testing.T) {
	t.Parallel()
	f, err := factsOf(t, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if f.in.Params.DeltaMax != "0.3" || f.deltaSet || f.in.Params.ThinDelta != "0.15" ||
		f.in.Params.MaxCashAgeDays != 3 || f.in.Params.DaysInYear != 365 {
		t.Fatalf("params %+v set %v", f.in.Params, f.deltaSet)
	}
}

func TestOwnerErrorReturned(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := factsOf(t, Config{Owner: fakeOwner{err: boom}, RuleParams: fakeRules{}})
	if !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
}

func TestAccountInputIsText(t *testing.T) {
	t.Parallel()
	acc := &fakeAccount{values: cashAt("A", "2026-11-09", "120000.00"), pos: map[string][]store.Position{
		"A/2026-11-09": {{SecType: "OPT", Right: "P", Strike: "80", Multiplier: "100", Quantity: "-1", Currency: "USD"}}}}
	f, err := factsOf(t, Config{Account: acc, Aliases: []string{"A"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.in.Accounts) != 1 {
		t.Fatalf("accounts %+v", f.in.Accounts)
	}
	a := f.in.Accounts[0]
	if a.Alias != "A" || a.Cash != "120000.00" || a.Currency != "USD" || a.AgeDays != 0 || a.CaptureDay != buildDay ||
		len(a.Positions) != 1 || a.Positions[0].Strike != "80" || a.Positions[0].Quantity != "-1" {
		t.Fatalf("account %+v", a)
	}
}

func TestAccountLookback3Days(t *testing.T) {
	t.Parallel()
	acc := &fakeAccount{values: cashAt("A", "2026-11-06", "5000.00")}
	f, err := factsOf(t, Config{Account: acc, Aliases: []string{"A", "B"}})
	if err != nil {
		t.Fatal(err)
	}
	a, b := f.in.Accounts[0], f.in.Accounts[1]
	if a.Cash != "5000.00" || a.AgeDays != 3 || a.CaptureDay != "2026-11-06" {
		t.Fatalf("A %+v", a)
	}
	if b.Cash != "" || b.AgeDays != 0 {
		t.Fatalf("B %+v", b)
	}
	for _, r := range acc.reads {
		if r == "values B/2026-11-05" {
			t.Fatalf("probed beyond 3 days: %v", acc.reads)
		}
	}
}

func TestAccountReaderErrorReturned(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := factsOf(t, Config{Account: &fakeAccount{err: boom}, Aliases: []string{"A"}})
	if !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
}

func TestNoAccountReaderGivesNoAccounts(t *testing.T) {
	t.Parallel()
	f, err := factsOf(t, Config{Aliases: []string{"A"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.in.Accounts) != 0 {
		t.Fatalf("accounts %+v", f.in.Accounts)
	}
}

func TestRowWithBadGatesIsMarkedUnreadable(t *testing.T) {
	t.Parallel()
	r := tierPut(1, "ok", "85.00", "0.42")
	r.Gates = "not json"
	row := tierRowOf(r, "k", buildDay)
	if len(row.Marks) != 1 || row.Marks[0] != unreadableGates || row.QuoteReason != "" {
		t.Fatalf("row %+v", row)
	}
}

// TestReaderErrorReturned: a reader error of the cash or the owner settings is an error of Build, not a
// page without tiers.
func TestReaderErrorReturned(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	for name, mod := range map[string]func(*Config){
		"account": func(c *Config) { c.Account, c.Aliases = &fakeAccount{err: boom}, []string{"A"} },
		"owner":   func(c *Config) { c.Owner, c.RuleParams = fakeOwner{err: boom}, fakeRules{} },
	} {
		rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
		c := tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)))})
		mod(&c)
		if _, err := Build(context.Background(), c, nil, buildDay, testNow); !errors.Is(err, boom) {
			t.Errorf("%s: Build err = %v", name, err)
		}
	}
}
