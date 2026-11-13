package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/andrewrong/optdesk/internal/accounts"
	accountstoml "github.com/andrewrong/optdesk/internal/accounts/toml"
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// accountsFileName is the file of $OT_HOME that names the owner's accounts (docs/contracts/accounts.md).
const accountsFileName = "accounts.toml"

// errAccountsBook is the start refusal when accounts.toml cannot be loaded. The text of the load error is
// kept (it names a line, never an id); the path is not.
var errAccountsBook = errors.New("accounts.toml cannot be loaded")

// demoBook is the book of the demo profile: one alias, no id. The demo profile reads no file.
type demoBook struct{}

func (demoBook) Alias(string) (string, bool) { return "", false }
func (demoBook) Main() string                { return "main" }
func (demoBook) Aliases() []string           { return []string{"main"} }
func (demoBook) IDs() []string               { return nil }

// loadBook constructs the book from $OT_HOME/accounts.toml. The demo profile gets demoBook and reads no file;
// any other profile refuses a missing or invalid file, so nothing collects under an alias before the owner
// has written it (account-identity Decision 9).
func (a *app) loadBook() (accounts.Book, error) {
	if a.profile.Name == "demo" {
		return demoBook{}, nil
	}
	home := a.getenv("OT_HOME")
	if home == "" {
		return nil, fmt.Errorf("%w: OT_HOME is not set", errAccountsBook)
	}
	path := filepath.Join(home, accountsFileName)
	b, err := accountstoml.Load(path)
	if err != nil {
		// the text of a load error names a line; the path of a read error is replaced by the file name
		return nil, fmt.Errorf("%w: %s", errAccountsBook, strings.ReplaceAll(err.Error(), path, accountsFileName))
	}
	return b, nil
}

// flexAccounts is the account map of the Flex job and of the web import: the account number of each book
// entry to its alias, the one rule that maps both.
func flexAccounts(b accounts.Book) map[string]string {
	ids, aliases := b.IDs(), b.Aliases()
	m := make(map[string]string, len(ids))
	for i, id := range ids {
		m[id] = aliases[i]
	}
	return m
}

// bookAliasList is the alias list of the account pages: main first, then the other aliases in the order of
// the book.
func bookAliasList(b accounts.Book) []string {
	list := []string{b.Main()}
	for _, al := range b.Aliases() {
		if al != b.Main() {
			list = append(list, al)
		}
	}
	return list
}

// checkImportMap loads the book and refuses an importer `--map` whose target is not one of its aliases.
func (a *app) checkImportMap(m map[string]string) error {
	book, err := a.loadBook()
	if err != nil {
		return err
	}
	return checkAliasMap(book, m)
}

// checkAliasMap refuses a `--map from=to` whose `to` is not an alias of the book.
func checkAliasMap(b accounts.Book, m map[string]string) error {
	known := map[string]bool{}
	for _, al := range b.Aliases() {
		known[al] = true
	}
	for _, to := range m {
		if !known[to] {
			return errors.New("--map target is not an alias of accounts.toml")
		}
	}
	return nil
}

// bookAliases is the alias list of the held lists: every alias of the book in the order of the file. A factory
// without a book (a test) gives none, which reads no held names.
func (f *jobFactory) bookAliases() []string {
	if f.book == nil {
		return nil
	}
	return f.book.Aliases()
}

// parseAccountSockets parses the comma list of socket paths of OT_ACCOUNT_SOCKETS. The old label=socket form is
// refused, and so is an empty or repeated entry.
func parseAccountSockets(v string) ([]string, error) {
	bad := fmt.Errorf("%s is not a comma list of socket paths", envAccountSockets)
	var out []string
	for _, part := range strings.Split(v, ",") {
		p := strings.TrimSpace(part)
		if p == "" || strings.Contains(p, "=") || len(p) >= maxSocketPath || slices.Contains(out, p) {
			return nil, bad
		}
		out = append(out, p)
	}
	return out, nil
}

// kindUnknownAccount is the named state of an answer whose id is not in the book: the kind a run row stores as its
// error code and a log record names. It is not Retryable, so the runner treats it as permanent.
const kindUnknownAccount contracts.ErrorKind = "unknown_account"

// unknownAccountError is the permanent typed error of an answer whose id is not in the book, of the named state
// unknown_account. Its text carries the index of the socket and never an id.
type unknownAccountError struct{ socket int }

func (e unknownAccountError) Error() string {
	return fmt.Sprintf("unknown account (socket %d)", e.socket)
}
func (unknownAccountError) Kind() contracts.ErrorKind { return kindUnknownAccount }
func (unknownAccountError) Unwrap() error             { return accounts.ErrUnknownAccount }

// noSocketError is the permanent typed error of an alias that no socket answers for.
type noSocketError struct{ alias string }

func (e noSocketError) Error() string {
	return "no socket reports the account " + e.alias
}
func (noSocketError) Kind() contracts.ErrorKind { return contracts.Permanent }

// gatewayResolver gives one reader per alias of the book, found by the id each socket's answer reports
// (Decision 5 of docs/design/account-identity.md). The socket of an alias is cached once an answer has named it.
type gatewayResolver struct {
	book    accounts.Book
	sockets []broker.AccountReader

	mu    sync.Mutex
	owner map[int]string // socket index -> alias, set from an answer
}

// newGatewayResolver is the resolver over the readers of the sockets, in the order of OT_ACCOUNT_SOCKETS.
func newGatewayResolver(book accounts.Book, sockets []broker.AccountReader) *gatewayResolver {
	return &gatewayResolver{book: book, sockets: sockets, owner: map[int]string{}}
}

// Readers returns one reader per alias of the book. Closing them closes nothing: the sockets are closed by the
// caller, once each.
func (g *gatewayResolver) Readers() map[string]broker.AccountReader {
	out := map[string]broker.AccountReader{}
	for _, al := range g.book.Aliases() {
		out[al] = aliasReader{g: g, alias: al}
	}
	return out
}

// socketOf returns the index of the socket named for alias, or -1.
func (g *gatewayResolver) socketOf(alias string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, al := range g.owner {
		if al == alias {
			return i
		}
	}
	return -1
}

// isNamed reports whether an answer has named the socket i.
func (g *gatewayResolver) isNamed(i int) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.owner[i]
	return ok
}

// answer is what one call on a socket returned: the id it reports and the call's own result.
type answer[T any] struct {
	id  string
	val T
}

// ask returns the answer of the socket that reports the account of alias. A socket already known to be the
// alias's is asked alone; otherwise the sockets not yet named are asked in order, each answer naming its socket.
// A failing socket is a failed read; an id not in the book is unknownAccountError.
func ask[T any](g *gatewayResolver, alias string, call func(broker.AccountReader) (answer[T], error)) (T, error) {
	var zero T
	known := g.socketOf(alias)
	try := func(i int) (T, bool, error) {
		ans, err := call(g.sockets[i])
		if err != nil {
			return zero, false, err
		}
		got, ok := g.book.Alias(ans.id)
		if !ok {
			return zero, false, unknownAccountError{socket: i}
		}
		g.mu.Lock()
		g.owner[i] = got
		g.mu.Unlock()
		return ans.val, got == alias, nil
	}
	if known >= 0 {
		v, match, err := try(known)
		if err != nil {
			return zero, err
		}
		if !match {
			return zero, noSocketError{alias: alias}
		}
		return v, nil
	}
	var firstErr error
	for i := range g.sockets {
		if g.isNamed(i) {
			continue
		}
		v, match, err := try(i)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if match {
			return v, nil
		}
	}
	if firstErr != nil {
		return zero, firstErr
	}
	return zero, noSocketError{alias: alias}
}

// aliasReader reads the account of one alias through the resolver; the answers carry the alias.
type aliasReader struct {
	g     *gatewayResolver
	alias string
}

func (r aliasReader) Account(ctx context.Context) (broker.Account, error) {
	acc, err := ask(r.g, r.alias, func(s broker.AccountReader) (answer[broker.Account], error) {
		a, err := s.Account(ctx)
		return answer[broker.Account]{id: a.Alias, val: a}, err
	})
	if err != nil {
		return broker.Account{}, err
	}
	acc.Alias = r.alias
	return acc, nil
}

func (r aliasReader) Executions(ctx context.Context) (broker.Executions, error) {
	ex, err := ask(r.g, r.alias, func(s broker.AccountReader) (answer[broker.Executions], error) {
		e, err := s.Executions(ctx)
		return answer[broker.Executions]{id: e.Alias, val: e}, err
	})
	if err != nil {
		return broker.Executions{}, err
	}
	ex.Alias = r.alias
	return ex, nil
}

func (aliasReader) Close() error { return nil }
