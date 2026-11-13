package main

import (
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/accounts"
)

// plainBook hides the Sourced method of the book it wraps.
type plainBook struct{ accounts.Book }

// loadStatementBook writes accounts.toml with main m, the aliases m, a, b, c and the statement line given, and loads it.
func loadStatementBook(t *testing.T, statement string) accounts.Book {
	t.Helper()
	a, _ := testApp(t, "")
	writeBookText(t, a, bookText("m", "m", "a", "b", "c")+statement)
	b, err := a.loadBook()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStatementAliasesOfPlainBookIsNil(t *testing.T) {
	t.Parallel()
	b := loadStatementBook(t, "statement = [\"a\"]\n")
	if got := statementAliases(plainBook{b}); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestStatementAliasesOfSourcedBook(t *testing.T) {
	t.Parallel()
	b := loadStatementBook(t, "statement = [\"a\", \"b\"]\n")
	got := statementAliases(b)
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("got %v", got)
	}
	got[0] = "changed"
	if again := statementAliases(b); again[0] != "a" {
		t.Fatalf("statementAliases returned the slice of the book, not a copy: %v", again)
	}
}

func TestWithoutStatementKeepsOrder(t *testing.T) {
	t.Parallel()
	list := []string{"m", "a", "b", "c"}
	b := loadStatementBook(t, "statement = [\"a\", \"c\"]\n")
	if got := withoutStatement(b, list); !slices.Equal(got, []string{"m", "b"}) {
		t.Fatalf("got %v", got)
	}
	if got := withoutStatement(plainBook{b}, list); !slices.Equal(got, list) {
		t.Fatalf("plain book: got %v", got)
	}
	if !slices.Equal(list, []string{"m", "a", "b", "c"}) {
		t.Fatalf("input changed: %v", list)
	}
}
