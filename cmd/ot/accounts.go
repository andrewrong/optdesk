package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/andrewrong/optdesk/internal/accounts"
	accountstoml "github.com/andrewrong/optdesk/internal/accounts/toml"
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

// heldAliases returns the aliases of the book of the factory: none for a factory built without one.
func (f jobFactory) heldAliases() []string {
	if f.book == nil {
		return nil
	}
	return f.book.Aliases()
}
