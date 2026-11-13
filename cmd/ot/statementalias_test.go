package main

import (
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/accounts"
)

// srcBook is a Book that also declares its statement aliases.
type srcBook struct {
	accounts.Book
	stmt []string
}

func (b srcBook) Statement() []string { return b.stmt }

func TestStatementAliasesOfPlainBookIsNil(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	writeBookText(t, a, bookText("m", "m", "a"))
	b, err := a.loadBook()
	if err != nil {
		t.Fatal(err)
	}
	if got := statementAliases(plainBook{b}); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

// plainBook hides the Sourced method of the book it wraps.
type plainBook struct{ accounts.Book }

func TestStatementAliasesOfSourcedBook(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	writeBookText(t, a, bookText("m", "m", "a", "b"))
	b, err := a.loadBook()
	if err != nil {
		t.Fatal(err)
	}
	src := srcBook{Book: b, stmt: []string{"a", "b"}}
	got := statementAliases(src)
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("got %v", got)
	}
	got[0] = "changed"
	if src.stmt[0] != "a" {
		t.Fatal("statementAliases returned the slice of the book, not a copy")
	}
}

func TestWithoutStatementKeepsOrder(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	writeBookText(t, a, bookText("m", "m", "a", "b", "c"))
	b, err := a.loadBook()
	if err != nil {
		t.Fatal(err)
	}
	list := []string{"m", "a", "b", "c"}
	got := withoutStatement(srcBook{Book: b, stmt: []string{"a", "c"}}, list)
	if !slices.Equal(got, []string{"m", "b"}) {
		t.Fatalf("got %v", got)
	}
	if got := withoutStatement(plainBook{b}, list); !slices.Equal(got, list) {
		t.Fatalf("plain book: got %v", got)
	}
	if !slices.Equal(list, []string{"m", "a", "b", "c"}) {
		t.Fatalf("input changed: %v", list)
	}
}
