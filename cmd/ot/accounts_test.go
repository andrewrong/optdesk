package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/andrewrong/optdesk/internal/jobs/accountcapture"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	err := runWired(l.a, []string{"--day", "2026-11-04"})
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
	err := runWired(r.a, []string{"--day", "2026-11-04"})
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

// inventedID builds an account id at run time from two pieces, so that no id-shaped string is committed.
func inventedID(n int) string { return "U" + "000" + fmt.Sprintf("%04d", n) }

// testAliasIDs gives each alias the tests use its own invented id.
var testAliasIDs = map[string]int{"a": 1, "b": 2, "c": 3, "mine": 4, "wife": 5, "m": 6}

func aliasID(alias string) string {
	n, ok := testAliasIDs[alias]
	if !ok {
		panic("no test id for alias " + alias)
	}
	return inventedID(n)
}

// bookText is the body of an accounts.toml with main and the aliases, in the order given.
func bookText(main string, aliases ...string) string {
	var b strings.Builder
	for _, al := range aliases {
		fmt.Fprintf(&b, "%s = %q\n", al, aliasID(al))
	}
	fmt.Fprintf(&b, "main = %q\n", main)
	return b.String()
}

// writeBookText writes body as accounts.toml under the OT_HOME of a.
func writeBookText(t *testing.T, a *app, body string) {
	t.Helper()
	if err := writeFile(filepath.Join(a.getenv("OT_HOME"), accountsFileName), body); err != nil {
		t.Fatal(err)
	}
}

// idReader answers every call with the account id it was given and counts nothing else.
type idReader struct {
	id  string
	err error
}

func (r idReader) Account(context.Context) (broker.Account, error) {
	return broker.Account{Alias: r.id, ValuesCollected: true}, r.err
}
func (r idReader) Executions(context.Context) (broker.Executions, error) {
	return broker.Executions{Alias: r.id, Collected: true}, r.err
}
func (idReader) Close() error { return nil }

func resolverOf(t *testing.T, book string, socks ...broker.AccountReader) map[string]broker.AccountReader {
	t.Helper()
	a, _ := testApp(t, "")
	writeBookText(t, a, book)
	b, err := a.loadBook()
	if err != nil {
		t.Fatal(err)
	}
	return newGatewayResolver(b, socks).Readers()
}

func TestParseAccountSockets(t *testing.T) {
	t.Parallel()
	got, err := parseAccountSockets("/s/a.sock, /s/b.sock")
	if err != nil || len(got) != 2 || got[0] != "/s/a.sock" || got[1] != "/s/b.sock" {
		t.Fatalf("got %v err %v", got, err)
	}
	for _, bad := range []string{"a=/s/a.sock", "/s/a.sock,", "", "/s/a.sock,/s/a.sock", "/" + strings.Repeat("x", 200)} {
		if _, err := parseAccountSockets(bad); err == nil || !strings.Contains(err.Error(), envAccountSockets) {
			t.Errorf("%.20q: err %v, want a refusal naming %s", bad, err, envAccountSockets)
		}
	}
}

// recordWriter records the aliases a capture was written under.
type recordWriter struct {
	mu      sync.Mutex
	written []string
}

func (w *recordWriter) WriteCapture(_ context.Context, c store.Capture) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.written = append(w.written, c.Alias+"|"+c.ID)
	return nil
}
func (w *recordWriter) WriteExecutions(_ context.Context, alias string, _ []store.Execution) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.written = append(w.written, alias)
	return nil
}

// TestAccountWiringHoldsNoID is the no-id test of Decision 8 for the account wiring: the sockets report the id of
// an account in the book, the id of one outside it and a failed dial; the real job runs over the resolver, and no
// log record, job key, error text or result holds an id. Nothing is stored for the id outside the book.
func TestAccountWiringHoldsNoID(t *testing.T) {
	t.Parallel()
	known, stranger := aliasID("a"), inventedID(9)
	var logs bytes.Buffer
	a, errb := testApp(t, fullEnv+"OT_PG_DSN_ACCOUNT=acct-dsn\nOT_ACCOUNT_SOCKETS=/s/a.sock,/s/x.sock,/s/down.sock\n")
	a.log = slog.New(slog.NewTextHandler(&logs, nil))
	writeBookText(t, a, bookText("a", "a", "b"))
	a.deps.dialAccount = func(_ context.Context, sock string, _ broker.Options) (broker.AccountReader, error) {
		switch sock {
		case "/s/a.sock":
			return idReader{id: known}, nil
		case "/s/x.sock":
			return idReader{id: stranger}, nil
		}
		return nil, errors.New("dial " + known + stranger)
	}
	a.deps.openAccount = func(context.Context, string) (store.AccountStore, error) {
		return fakeAcctStore{n: new(atomic.Int32)}, nil
	}
	w := &recordWriter{}
	a.deps.newAccountCapture = func(c accountcapture.Config) (contracts.Job, error) {
		c.Writer = w
		return accountcapture.New(c)
	}
	var extra closerSet
	j, err := a.accountCapture(context.Background(), jobFactory{d: a.deps, clk: fakeClock{time.Date(2026, 3, 4, 20, 0, 0, 0, time.UTC)}}, &extra)
	if err != nil {
		t.Fatal(err)
	}
	keys, hash, err := j.Plan(context.Background(), "2026-03-04")
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), "2026-03-04", keys)
	out := fmt.Sprintf("%v %v %v %+v %s %s", keys, hash, w.written, res, logs.String(), errb.String())
	if res.Err != nil {
		out += res.Err.Error()
	}
	for _, id := range []string{known, stranger} {
		if strings.Contains(out, id) {
			t.Fatalf("an id appears in the output: %s", out)
		}
	}
	for _, row := range w.written {
		if !strings.HasPrefix(row, "a|") && row != "a" {
			t.Fatalf("a row was stored under %q", row)
		}
	}
	if len(w.written) == 0 {
		t.Fatal("nothing stored for the account of the book")
	}
}
