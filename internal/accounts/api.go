// Package accounts is the owner's account book: each account is named once, by an alias, and keyed
// to its IBKR account id.
package accounts

import "errors"

// Book answers for the accounts the owner named. An id is the IBKR account id and stays inside the
// book's callers; an alias is the name the owner gave (mine, wife, daughter).
type Book interface {
	// Alias returns the alias of the account id, and false when the book does not know the id.
	Alias(id string) (alias string, ok bool)
	// Main returns the alias of the owner's main account.
	Main() string
	// Aliases returns every alias of the book in the order the owner wrote them.
	Aliases() []string
}

// ErrUnknownAccount is the typed reason for an id the book does not know. Test it with errors.Is.
var ErrUnknownAccount = errors.New("accounts: unknown account")
