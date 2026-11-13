package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/accounts"
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// testAcctID is the invented account id of the default book; built from pieces so that no id-shaped string is
// committed.
const testAcctID = "U" + "000" + "0001"

// writeDefaultBook gives the home of the app a book of one alias, a, keyed to testAcctID.
func writeDefaultBook(t *testing.T, a *app) {
	t.Helper()
	body := "a = \"" + testAcctID + "\"\nmain = \"a\"\n"
	if err := os.WriteFile(filepath.Join(a.getenv("OT_HOME"), accountsFileName), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// bookApp is testApp with the default book written.
func bookApp(t *testing.T, env string) (*app, *bytes.Buffer) {
	t.Helper()
	a, log := testApp(t, env)
	writeDefaultBook(t, a)
	return a, log
}

// removeBook deletes the accounts.toml of the app's home.
func removeBook(t *testing.T, a *app) {
	t.Helper()
	if err := os.Remove(filepath.Join(a.getenv("OT_HOME"), accountsFileName)); err != nil {
		t.Fatal(err)
	}
}

func TestLoadBookDemoReadsNoFile(t *testing.T) {
	t.Parallel()
	a, _ := bookApp(t, "")
	removeBook(t, a)
	a.profile.Name = "demo"
	b, err := a.loadBook()
	if err != nil || !slices.Equal(b.Aliases(), []string{"main"}) || b.Main() != "main" {
		t.Fatalf("%v %v", b, err)
	}
}

func TestLoadBookRefusesMissingFileWithoutPath(t *testing.T) {
	t.Parallel()
	a, _ := bookApp(t, "")
	removeBook(t, a)
	_, err := a.loadBook()
	if !errors.Is(err, errAccountsBook) {
		t.Fatalf("a missing file gave %v", err)
	}
	if strings.Contains(err.Error(), a.getenv("OT_HOME")) {
		t.Errorf("the error names the home path: %v", err)
	}
}

func TestFlexAccountsAndAliasList(t *testing.T) {
	t.Parallel()
	a, _ := bookApp(t, "")
	writeBook(t, a, "x", "y")
	// main = x; rewrite so that main is the second alias written
	path := filepath.Join(a.getenv("OT_HOME"), accountsFileName)
	body := "x = \"U" + "000001\"\ny = \"U" + "000002\"\nmain = \"y\"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := a.loadBook()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"U" + "000001": "x", "U" + "000002": "y"}
	if got := flexAccounts(b); len(got) != 2 || got["U"+"000001"] != "x" || got["U"+"000002"] != "y" {
		t.Errorf("flex map %v, want %v", got, want)
	}
	if got := bookAliasList(b); !slices.Equal(got, []string{"y", "x"}) {
		t.Errorf("page aliases %v, want main first", got)
	}
}

func TestCheckAliasMap(t *testing.T) {
	t.Parallel()
	a, _ := bookApp(t, "")
	b, err := a.loadBook()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkAliasMap(b, map[string]string{"old": "a"}); err != nil {
		t.Errorf("an alias of the book refused: %v", err)
	}
	if err := checkAliasMap(b, map[string]string{"old": "a", "x": "nobody"}); err == nil {
		t.Error("a target outside the book accepted")
	}
	if err := checkAliasMap(b, nil); err != nil {
		t.Errorf("no map refused: %v", err)
	}
}

func TestOptionLedgerJobRefusedWithoutBook(t *testing.T) {
	t.Parallel()
	l := newLedgerRig(t, fullEnv+ledgerDSNLine+"TOK_MAIN=x\nTOK_SECOND=y\n", false)
	removeBook(t, l.a)
	err := runOnce(l.a, []string{"--day", "2026-11-04"})
	if !errors.Is(err, errAccountsBook) {
		t.Fatalf("option_ledger without accounts.toml gave %v", err)
	}
	if len(l.opened) != 0 || l.built {
		t.Errorf("the job opened a store (%v) or was built (%v)", l.opened, l.built)
	}
}

// TestNoAccountIDLeavesTheWiring is the proof of Decision 8: the ledger wiring and the page wiring run over a
// book whose id is invented at run time, and neither the log, the job key, an error text nor the page JSON holds it.
func TestNoAccountIDLeavesTheWiring(t *testing.T) {
	t.Parallel()
	id := testAcctID
	t.Run("ledger wiring", func(t *testing.T) {
		t.Parallel()
		env := fullEnv + ledgerDSNLine + "TOK_MAIN=" + tokenA + "\nTOK_SECOND=token-b-secret\n"
		l := newLedgerRig(t, env, true)
		l.run(t)
		var keys []string
		for _, j := range l.jobs {
			if j.ID() != "option_ledger" {
				continue
			}
			ks, _, err := j.Plan(context.Background(), "2026-11-04")
			if err != nil {
				t.Fatal(err)
			}
			for _, k := range ks {
				keys = append(keys, k.DatasetID, k.SubjectKey)
			}
			res := j.Run(context.Background(), "2026-11-04", ks)
			if res.Err != nil {
				keys = append(keys, res.Err.Error())
			}
		}
		if len(keys) == 0 {
			t.Fatal("no option_ledger job")
		}
		out := l.log.String() + l.a.stdout.(interface{ String() string }).String() + strings.Join(keys, "\n")
		if strings.Contains(out, id) {
			t.Errorf("the account id is in a log record, a key or an error:\n%s", out)
		}
	})
	t.Run("unknown id in a statement", func(t *testing.T) {
		t.Parallel()
		env := fullEnv + ledgerDSNLine + "TOK_MAIN=" + tokenA + "\nTOK_SECOND=token-b-secret\n"
		l := newLedgerRig(t, env, true)
		writeBook(t, l.a, "z")
		l.run(t)
		for _, j := range l.jobs {
			if j.ID() != "option_ledger" {
				continue
			}
			ks, _, _ := j.Plan(context.Background(), "2026-11-04")
			res := j.Run(context.Background(), "2026-11-04", ks)
			if res.Err != nil && strings.Contains(res.Err.Error(), id) {
				t.Errorf("error text holds the id: %v", res.Err)
			}
		}
		if strings.Contains(l.log.String(), id) {
			t.Errorf("log holds the id:\n%s", l.log.String())
		}
	})
	t.Run("page wiring", func(t *testing.T) {
		t.Parallel()
		a, errb := bookApp(t, "OT_UNUSED=1\n")
		aliases := siteAliasList(a)
		var asked []string
		rd := householdReaders(&asked)
		rd.aliases = aliases
		h := siteHandler(a, householdClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
		rec := companiesGet(t, h, http.MethodGet, "/"+siteHouseholdRoute)
		if rec.Code != 200 || len(asked) == 0 {
			t.Fatalf("%d %v", rec.Code, asked)
		}
		if strings.Contains(rec.Body.String(), id) || strings.Contains(errb.String(), id) || strings.Contains(strings.Join(asked, ","), id) {
			t.Error("the account id is in the page JSON, the log or a reader call")
		}
	})
}

var _ accounts.Book = demoBook{}

// fixedBook is a book of the given aliases, main first listed, for the wiring tests.
type fixedBook struct{ demoBook }

func (fixedBook) Aliases() []string { return []string{"mine", "wife"} }
func (fixedBook) Main() string      { return "mine" }

func TestOpenStoredHeldAliasesComeFromTheBook(t *testing.T) {
	t.Parallel()
	jc := &jobCtx{f: &jobFactory{book: fixedBook{}}}
	if got := openStored(jc, nil, nil).HeldAliases; !slices.Equal(got, []string{"mine", "wife"}) {
		t.Fatalf("HeldAliases = %v, want the aliases of the book", got)
	}
	if got := openStored(&jobCtx{f: &jobFactory{}}, nil, nil).HeldAliases; len(got) != 0 {
		t.Fatalf("HeldAliases = %v without a book, want none", got)
	}
}

func TestWireHeldCallsWrapsTheHeldConstructorWithTheBook(t *testing.T) {
	t.Parallel()
	var got store.AccountReader
	f := &jobFactory{book: fixedBook{}}
	f.d.openAccount = func(context.Context, string) (store.AccountStore, error) { return nil, errors.New("unused") }
	f.d.newOptionEodHeld = func(
		_ broker.Broker, _ store.InstrumentResolver, _ store.OptionEodWriter, _ []jobs.WatchEntry, acct store.AccountReader, _ clock.Clock, _ symbol.Translator,
	) (contracts.Job, error) {
		got = acct
		return nil, nil
	}
	var extra closerSet
	wireHeldCalls(runOnceConfig{accountDSN: "x"}, f, &extra, slog.New(slog.DiscardHandler))
	if _, err := f.d.newOptionEodWatch(nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("the held constructor got no reader")
	}
	if _, raw := got.(*lazyAccountReader); raw {
		t.Fatal("the held constructor got the bare reader, not one carrying the aliases of the book")
	}
}

func TestRunOnceRefusedWithoutBookBeforeAnyJobIsWired(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	removeBook(t, r.a)
	err := runOnce(r.a, []string{"--day", "2026-11-04"})
	if !errors.Is(err, errAccountsBook) {
		t.Fatalf("run-once without accounts.toml gave %v", err)
	}
}

func TestServeRefusedWithoutBook(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	removeBook(t, r.a)
	err := runServe(r.a, []string{"--addr", "127.0.0.1:0"})
	if !errors.Is(err, errAccountsBook) {
		t.Fatalf("serve started or failed for another reason without accounts.toml: %v", err)
	}
}
