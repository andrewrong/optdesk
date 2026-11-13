package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/accountcapture"
	"github.com/andrewrong/optdesk/internal/store"
)

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

// writeBook writes body as accounts.toml under the OT_HOME of a.
func writeBook(t *testing.T, a *app, body string) {
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
	writeBook(t, a, book)
	b, err := a.accountBook()
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

func TestAccountBookRefusesMissingFile(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	if _, err := a.accountBook(); err == nil {
		t.Fatal("a missing accounts.toml was accepted")
	}
}

func TestBookAliasesMainFirst(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	writeBook(t, a, bookText("b", "a", "b", "c"))
	b, err := a.accountBook()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(bookAliases(b), ","); got != "b,a,c" {
		t.Fatalf("aliases %s", got)
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
	writeBook(t, a, bookText("a", "a", "b"))
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
