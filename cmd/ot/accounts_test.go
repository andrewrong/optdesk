package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/accounts"
)

// removeBook deletes the accounts.toml of the app's home.
func removeBook(t *testing.T, a *app) {
	t.Helper()
	if err := os.Remove(filepath.Join(a.getenv("OT_HOME"), accountsFileName)); err != nil {
		t.Fatal(err)
	}
}

func TestLoadBookDemoReadsNoFile(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	removeBook(t, a)
	a.profile.Name = "demo"
	b, err := a.loadBook()
	if err != nil || !slices.Equal(b.Aliases(), []string{"main"}) || b.Main() != "main" {
		t.Fatalf("%v %v", b, err)
	}
}

func TestLoadBookRefusesMissingFileWithoutPath(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
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
	a, _ := testApp(t, "")
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
	a, _ := testApp(t, "")
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

func TestOpenStoredHoldsAliasesOfBook(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	writeBook(t, a, "p", "q")
	b, err := a.loadBook()
	if err != nil {
		t.Fatal(err)
	}
	jc := &jobCtx{f: &jobFactory{book: b}}
	if got := openStored(jc, nil, nil).HeldAliases; !slices.Equal(got, []string{"p", "q"}) {
		t.Errorf("HeldAliases %v", got)
	}
}

func TestStartRefusedWithoutBook(t *testing.T) {
	t.Parallel()
	t.Run("run-once opens no store", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, fullEnv)
		removeBook(t, r.a)
		err := runOnce(r.a, []string{"--day", "2026-11-04"})
		if !errors.Is(err, errAccountsBook) {
			t.Fatalf("run-once without accounts.toml gave %v", err)
		}
		if ev := r.ev.list(); len(ev) != 0 {
			t.Errorf("stores opened before the refusal: %v", ev)
		}
	})
	t.Run("serve readers", func(t *testing.T) {
		t.Parallel()
		a, _ := testApp(t, pagesDSNName+"=x\n")
		removeBook(t, a)
		_, _, err := openSiteReaders(context.Background(), a)
		if !errors.Is(err, errAccountsBook) {
			t.Fatalf("serve without accounts.toml gave %v", err)
		}
	})
	t.Run("demo serves without the file", func(t *testing.T) {
		t.Parallel()
		a, _ := testApp(t, "")
		removeBook(t, a)
		a.profile.Name = "demo"
		if _, closeFn, err := openSiteReaders(context.Background(), a); err != nil {
			t.Fatal(err)
		} else {
			closeFn()
		}
	})
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
		a, errb := testApp(t, "OT_UNUSED=1\n")
		aliases, err := siteAliasList(a)
		if err != nil {
			t.Fatal(err)
		}
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
