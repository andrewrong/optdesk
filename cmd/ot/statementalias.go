package main

import (
	"slices"

	"github.com/andrewrong/optdesk/internal/accounts"
)

// statementAliases returns the aliases of the book that have no gateway: nil for a book that is not
// accounts.Sourced, otherwise a copy of its Statement().
func statementAliases(book accounts.Book) []string {
	s, ok := book.(accounts.Sourced)
	if !ok {
		return nil
	}
	return slices.Clone(s.Statement())
}

// withoutStatement returns aliases in their order without the statement aliases of the book; the list
// itself when the book has none.
func withoutStatement(book accounts.Book, aliases []string) []string {
	stmt := statementAliases(book)
	if len(stmt) == 0 {
		return aliases
	}
	out := make([]string, 0, len(aliases))
	for _, al := range aliases {
		if !slices.Contains(stmt, al) {
			out = append(out, al)
		}
	}
	return out
}
