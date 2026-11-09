package cutover_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs/cutover"
)

func TestCanonicalHashSortsKeys(t *testing.T) {
	t.Parallel()
	a, err := cutover.CanonicalHash([]byte(`{"b": 1, "a": {"y": "x<&>", "x": [1.50, true]}}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := cutover.CanonicalHash([]byte(`{"a":{"x":[1.50,true],"y":"x<&>"},"b":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("key order or spaces changed the hash: %s %s", a, b)
	}
	c, _ := cutover.CanonicalHash([]byte(`{"a":{"x":[1.5,true],"y":"x<&>"},"b":1}`))
	if a == c {
		t.Fatal("1.50 and 1.5 must hash differently: numbers keep their text")
	}
}

func TestCanonicalHashKnownValue(t *testing.T) {
	t.Parallel()
	got, err := cutover.CanonicalHash([]byte(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	// sha256 of the text {"a":1,"b":2}
	const want = "43258cff783fe7036d8a43033f830adfc60ec037382473548ac742b888292777"
	if got != want {
		t.Fatalf("hash = %s", got)
	}
}

func TestCanonicalHashRefusesNonObject(t *testing.T) {
	t.Parallel()
	for _, in := range []string{`[1]`, `3`, `{"a":1} x`, `{`} {
		if _, err := cutover.CanonicalHash([]byte(in)); err == nil {
			t.Fatalf("%q accepted", in)
		}
	}
}

func TestReadLinesNumbersAndErrors(t *testing.T) {
	t.Parallel()
	var got []int
	err := cutover.ReadLines(strings.NewReader("{\"a\":1}\n\n{\"a\":2}"), func(n int, raw []byte) error {
		got = append(got, n)
		return nil
	})
	if err != nil || len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("lines %v err %v", got, err)
	}

	sentinel := errors.New("secret-content-xyz")
	err = cutover.ReadLines(strings.NewReader("{\"a\":1}\n{\"a\":2}\n"), func(n int, _ []byte) error {
		if n == 2 {
			return sentinel
		}
		return nil
	})
	var le *cutover.LineError
	if !errors.As(err, &le) || le.Line != 2 || !errors.Is(err, sentinel) {
		t.Fatalf("err = %v", err)
	}
}

func TestLineErrorHasNoContent(t *testing.T) {
	t.Parallel()
	_, err := cutover.CanonicalHash([]byte(`{"acct":"SECRETVALUE" oops}`))
	if err == nil {
		t.Fatal("accepted")
	}
	err = cutover.ReadLines(strings.NewReader("{\"acct\":\"SECRETVALUE\" oops}\n"), func(_ int, raw []byte) error {
		_, e := cutover.CanonicalHash(raw)
		return e
	})
	var le *cutover.LineError
	if !errors.As(err, &le) || le.Line != 1 {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "SECRET") || !strings.Contains(err.Error(), "1") {
		t.Fatalf("message = %q", err.Error())
	}
}
