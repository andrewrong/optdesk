package flex

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/andrewrong/optdesk/internal/sources"
)

var _ sources.ParseFlexFunc = Parse

func gz(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParsePlainAndGzipShareOneHash(t *testing.T) {
	t.Parallel()
	body := fixture(t, "statement_trades.xml")
	plain, err := Parse(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	zipped, err := Parse(bytes.NewReader(gz(t, body)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if want := hex.EncodeToString(sum[:]); plain.RawSHA256 != want || zipped.RawSHA256 != want {
		t.Fatalf("hashes %s %s want %s", plain.RawSHA256, zipped.RawSHA256, want)
	}
	if len(plain.Statements) == 0 || len(plain.Statements) != len(zipped.Statements) {
		t.Fatalf("statements %d vs %d", len(plain.Statements), len(zipped.Statements))
	}
	if !plain.FetchedAt.IsZero() || !zipped.FetchedAt.IsZero() {
		t.Fatal("FetchedAt must stay zero")
	}
}

func TestParseBadGzip(t *testing.T) {
	t.Parallel()
	good := gz(t, fixture(t, "statement_trades.xml"))
	for name, body := range map[string][]byte{
		"header":    {0x1f, 0x8b, 0x00},
		"truncated": good[:len(good)/2],
	} {
		_, err := Parse(bytes.NewReader(body))
		if err == nil {
			t.Fatalf("%s: no error", name)
		}
		if k, c := flexKindOf(t, err); k != kindNotReport || c != "gzip" {
			t.Fatalf("%s: kind %s code %s", name, k, c)
		}
	}
}

func TestParseOverLimit(t *testing.T) {
	t.Parallel()
	big := bytes.Repeat([]byte{' '}, maxBody+1)
	for name, body := range map[string][]byte{"plain": big, "gzip": gz(t, big)} {
		_, err := Parse(bytes.NewReader(body))
		if err == nil {
			t.Fatalf("%s: no error", name)
		}
		if k, _ := flexKindOf(t, err); k != kindTooLarge {
			t.Fatalf("%s: kind %s", name, k)
		}
	}
}

func TestParseDoctypeRefused(t *testing.T) {
	t.Parallel()
	_, err := Parse(bytes.NewReader(fixture(t, "statement_doctype.xml")))
	if err == nil {
		t.Fatal("no error")
	}
	if k, c := flexKindOf(t, err); k != kindNotReport || c != "doctype" {
		t.Fatalf("kind %s code %s", k, c)
	}
}

func TestParseNotAReport(t *testing.T) {
	t.Parallel()
	for name, body := range map[string][]byte{
		"other root": []byte(`<Other/>`),
		"text":       []byte(`hello`),
		"empty":      nil,
	} {
		_, err := Parse(bytes.NewReader(body))
		if err == nil {
			t.Fatalf("%s: no error", name)
		}
		if k, _ := flexKindOf(t, err); k != kindNotReport {
			t.Fatalf("%s: kind %s", name, k)
		}
	}
}
