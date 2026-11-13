package accounts_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/accounts"
)

// fakeBook is a Book over a map of id to alias; order lists the aliases as written.
type fakeBook struct {
	byID  map[string]string
	order []string
	main  string
}

func (f fakeBook) Alias(id string) (string, bool) { a, ok := f.byID[id]; return a, ok }
func (f fakeBook) Main() string                   { return f.main }
func (f fakeBook) Aliases() []string              { return slices.Clone(f.order) }

var _ accounts.Book = fakeBook{}

// invented builds an invented id at run time from pieces, so no id-shaped literal exists.
func invented(n int) string { return "U" + "000" + fmt.Sprintf("%04d", n) }

// resolve is the user side: a caller turns an id into an alias or the typed reason.
func resolve(b accounts.Book, id string) (string, error) {
	if a, ok := b.Alias(id); ok {
		return a, nil
	}
	return "", fmt.Errorf("resolve: %w", accounts.ErrUnknownAccount)
}

func newBook() (fakeBook, string, string) {
	mine, wife := invented(1), invented(2)
	return fakeBook{
		byID:  map[string]string{mine: "mine", wife: "wife"},
		order: []string{"mine", "wife"},
		main:  "mine",
	}, mine, wife
}

func TestBookUserSide(t *testing.T) {
	t.Parallel()
	var b accounts.Book
	b, mine, wife := newBook()

	for id, want := range map[string]string{mine: "mine", wife: "wife"} {
		got, err := resolve(b, id)
		if err != nil || got != want {
			t.Fatalf("resolve(%q) = %q, %v; want %q", id, got, err, want)
		}
	}
	if _, err := resolve(b, invented(9)); !errors.Is(err, accounts.ErrUnknownAccount) {
		t.Fatalf("unknown id: err = %v, want ErrUnknownAccount", err)
	}
	if b.Main() != "mine" {
		t.Fatalf("Main = %q", b.Main())
	}
	if got := b.Aliases(); !slices.Equal(got, []string{"mine", "wife"}) {
		t.Fatalf("Aliases = %v", got)
	}
	if _, ok := b.Alias(b.Main()); ok {
		t.Fatal("an alias is not an id")
	}
}

func TestErrUnknownAccountSentinel(t *testing.T) {
	t.Parallel()
	if accounts.ErrUnknownAccount == nil || accounts.ErrUnknownAccount.Error() == "" {
		t.Fatal("ErrUnknownAccount must be a non-nil error with text")
	}
	wrapped := fmt.Errorf("a: %w", fmt.Errorf("b: %w", accounts.ErrUnknownAccount))
	if !errors.Is(wrapped, accounts.ErrUnknownAccount) {
		t.Fatal("wrapped sentinel must match with errors.Is")
	}
	if errors.Is(errors.New(accounts.ErrUnknownAccount.Error()), accounts.ErrUnknownAccount) {
		t.Fatal("sentinel is identity, not text")
	}
}
