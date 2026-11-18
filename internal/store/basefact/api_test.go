package basefact_test

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/basefact"
)

type fakeReader struct {
	res basefact.Result
	err error
}

func (f fakeReader) Facts(context.Context) (basefact.Result, error) { return f.res, f.err }

func (fakeReader) Close() error { return nil }

var _ basefact.Reader = fakeReader{}

func TestFakeReaderServesCaller(t *testing.T) {
	t.Parallel()
	want := basefact.Fact{Dataset: "bars", Rule: "newest", Subject: "USD", N: 10, Bad: 0, Day: "2026-11-17", Value: "NVDA close 181.42"}
	open := basefact.OpenFunc(func(context.Context, string) (basefact.Reader, error) {
		return fakeReader{res: basefact.Result{
			Facts:  []basefact.Fact{want},
			Failed: []string{basefact.GroupOption},
		}}, nil
	})
	r, err := open(context.Background(), "dsn")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	res, err := r.Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 1 || res.Facts[0] != want {
		t.Fatalf("facts = %+v, want [%+v]", res.Facts, want)
	}
	if len(res.Failed) != 1 || res.Failed[0] != "option" {
		t.Fatalf("failed = %v, want [option]", res.Failed)
	}

	boom := errors.New("all four views failed")
	failing := basefact.OpenFunc(func(context.Context, string) (basefact.Reader, error) {
		return fakeReader{res: basefact.Result{Facts: []basefact.Fact{}, Failed: []string{"bars", "option", "other", "account"}}, err: boom}, nil
	})
	fr, err := failing(context.Background(), "dsn")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fr.Facts(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestFactsNonNilWithoutRows(t *testing.T) {
	t.Parallel()
	var r basefact.Reader = fakeReader{res: basefact.Result{Facts: []basefact.Fact{}, Failed: []string{}}}
	res, err := r.Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Facts == nil || res.Failed == nil {
		t.Fatalf("Facts nil = %v, Failed nil = %v, want both non-nil", res.Facts == nil, res.Failed == nil)
	}
	if len(res.Facts) != 0 || len(res.Failed) != 0 {
		t.Fatalf("got %d facts, %d failed, want none", len(res.Facts), len(res.Failed))
	}
}

func TestFailedNamesAreTheFourGroups(t *testing.T) {
	t.Parallel()
	got := map[string]string{
		"bars":    basefact.GroupBars,
		"option":  basefact.GroupOption,
		"other":   basefact.GroupOther,
		"account": basefact.GroupAccount,
	}
	for want, g := range got {
		if g != want {
			t.Errorf("group constant = %q, want %q", g, want)
		}
	}
}
