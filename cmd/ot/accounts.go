package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/andrewrong/optdesk/internal/accounts"
	accountstoml "github.com/andrewrong/optdesk/internal/accounts/toml"
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// accountsFileName is the owner's account book under $OT_HOME (docs/design/account-identity.md, Decision 1).
const accountsFileName = "accounts.toml"

// accountBook loads $OT_HOME/accounts.toml. The error of the loader names a line, never an account id.
func (a *app) accountBook() (accounts.Book, error) {
	home := a.getenv("OT_HOME")
	if home == "" {
		return nil, fmt.Errorf("OT_HOME is not set")
	}
	path := filepath.Join(home, accountsFileName)
	b, err := accountstoml.Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s does not exist", accountsFileName)
	}
	if err != nil {
		// The loader names a line and never an id; a path in its text is replaced by the file name.
		return nil, fmt.Errorf("%s: %s", accountsFileName, strings.ReplaceAll(err.Error(), path, accountsFileName))
	}
	return b, nil
}

// bookAliases returns the aliases of the book, main first, the others in the order the owner wrote them.
func bookAliases(b accounts.Book) []string {
	main := b.Main()
	out := []string{main}
	for _, al := range b.Aliases() {
		if al != main {
			out = append(out, al)
		}
	}
	return out
}

// heldAliasList is the alias list of the book for a builder that works without it: a book that cannot be loaded
// logs one WARN line (no path, no id) and gives nil, which means no held names and no account read.
func (a *app) heldAliasList(job string) []string {
	b, err := a.accountBook()
	if err != nil {
		a.log.Warn("account book not usable, no account is read", "component", "ot", "job", job)
		return nil
	}
	return bookAliases(b)
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

// unknownAccountError is the permanent typed error of an answer whose id is not in the book. Its text carries the
// index of the socket and never an id.
type unknownAccountError struct{ socket int }

func (e unknownAccountError) Error() string {
	return fmt.Sprintf("unknown account (socket %d)", e.socket)
}
func (unknownAccountError) Kind() contracts.ErrorKind { return contracts.Permanent }
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
