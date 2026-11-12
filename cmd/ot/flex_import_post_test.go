package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func fiGzip(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFIForgedTokenIs403(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	tok := s.form.Token(postNow)
	forged := []fiPart{{field: "expires", body: []byte(tok.Expires)}, {field: "mac", body: []byte(strings.Repeat("0", len(tok.MAC)))},
		fiFile("a.xml", fiXML(fiAccount, "1"))}
	rec := serve(s.h, fiRequest(t, forged))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "upload refused: the form is not valid or has expired, load the page again") {
		t.Fatalf("%d %.200q", rec.Code, rec.Body)
	}
	if len(s.st.dsns) != 0 {
		t.Fatalf("the store was reached: %v", s.st.dsns)
	}
	// a request from another host is refused the same way
	r := fiRequest(t, append(s.tokenParts(), fiFile("a.xml", fiXML(fiAccount, "1"))))
	r.Host = "evil.example"
	if rec := serve(s.h, r); rec.Code != http.StatusForbidden || len(s.st.dsns) != 0 {
		t.Fatalf("%d, dsns %v", rec.Code, s.st.dsns)
	}
}

func TestFIPlainAndGzipSameHash(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	xml := fiXML(fiAccount, "1", "2")
	vm := decodeFI(t, s.upload(t, fiFile("a.xml", xml), fiFile("a.xml.gz", fiGzip(t, xml))))
	if len(vm.Files) != 2 || vm.Files[0].Status != "stored" || vm.Files[1].Status != "present" {
		t.Fatalf("%+v", vm.Files)
	}
	if len(s.st.fetches) != 2 {
		t.Fatalf("%d records", len(s.st.fetches))
	}
	want := fiHash(xml)
	for _, f := range s.st.fetches {
		if f.RawSHA256 != want {
			t.Fatalf("hash %s, want %s", f.RawSHA256, want)
		}
	}
}

func TestFIOneRefusedOfTwo(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	vm := decodeFI(t, s.upload(t, fiFile("bad.xml", []byte("<html/>")), fiFile("good.xml", fiXML(fiAccount, "1"))))
	if len(vm.Files) != 2 || vm.Files[0].Status != "failed" || vm.Files[0].Reason != "not_report" || vm.Files[1].Status != "stored" {
		t.Fatalf("%+v", vm.Files)
	}
	if len(s.st.fetches) != 2 || s.st.fetches[0].Status != "failed" || s.st.fetches[0].ErrorCode != "not_report" || s.st.fetches[1].Status != "ok" {
		t.Fatalf("%+v", s.st.fetches)
	}
	if o, c, _, _ := s.st.snapshot(); o != 1 || c != 1 {
		t.Fatalf("opened %d closed %d: once per post", o, c)
	}
}

func TestFITwoRefusedDistinctQueryIDs(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	s.upload(t, fiFile("a.xml", []byte("x")), fiFile("b.xml", []byte("x")))
	if len(s.st.fetches) != 2 || s.st.fetches[0].QueryID == s.st.fetches[1].QueryID {
		t.Fatalf("%+v", s.st.fetches)
	}
	for i, f := range s.st.fetches {
		if !strings.HasPrefix(f.QueryID, "file:") || !strings.HasSuffix(f.QueryID, ":"+string(rune('1'+i))+":-") {
			t.Fatalf("query id %q", f.QueryID)
		}
	}
	// another post has another request id
	s.upload(t, fiFile("a.xml", []byte("x")))
	if a, b := strings.Split(s.st.fetches[0].QueryID, ":")[1], strings.Split(s.st.fetches[2].QueryID, ":")[1]; a == b || len(a) != 8 {
		t.Fatalf("request ids %q %q", a, b)
	}
}

func TestFIAllRefusedWritesFailedRecords(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	vm := decodeFI(t, s.upload(t, fiFile("a.xml", []byte("1")), fiFile("b.xml", []byte("2")), fiFile("c.xml", nil)))
	o, c, f, _ := s.st.snapshot()
	if o != 1 || c != 1 || f != 3 || len(vm.Files) != 3 {
		t.Fatalf("opened %d closed %d records %d files %d", o, c, f, len(vm.Files))
	}
	for _, r := range s.st.fetches {
		if r.Status != "failed" || r.ErrorKind != "import" || r.Login != "import" {
			t.Fatalf("%+v", r)
		}
	}
}

func TestFISameBytesTwiceInOnePost(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	xml := fiXML(fiAccount, "7")
	vm := decodeFI(t, s.upload(t, fiFile("a.xml", xml), fiFile("a.xml", xml)))
	if len(s.st.fetches) != 2 || s.st.fetches[0].QueryID == s.st.fetches[1].QueryID {
		t.Fatalf("%+v", s.st.fetches)
	}
	if vm.Files[0].Status != "stored" || vm.Files[1].Status != "present" || vm.Files[1].Aliases[0].RowsNew != 0 {
		t.Fatalf("%+v", vm.Files)
	}
}

func TestFIUnmappedAccountUnconfigured(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	vm := decodeFI(t, s.upload(t, fiFile("other.xml", fiXML("ACCT_OTHER", "1", "2"))))
	if len(vm.Files) != 1 || vm.Files[0].Status != "unconfigured" || vm.Files[0].Unmapped != 1 || len(vm.Files[0].Aliases) != 0 {
		t.Fatalf("%+v", vm.Files)
	}
	if _, _, f, h := s.st.snapshot(); f != 1 || h != 0 || len(s.st.rows) != 0 {
		t.Fatalf("%d records, %d rows handed, %d stored", f, h, len(s.st.rows))
	}
	if strings.Contains(s.logs.String(), "ACCT_OTHER") {
		t.Fatal("an account number reached the log")
	}
}

func TestFINoTradesNothingFound(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	vm := decodeFI(t, s.upload(t, fiFile("empty.xml", fiXML(fiAccount))))
	if len(vm.Files) != 1 || vm.Files[0].Status != "nothing_found" || len(vm.Files[0].Aliases) != 0 {
		t.Fatalf("%+v", vm.Files)
	}
	if _, _, f, h := s.st.snapshot(); f != 1 || h != 0 {
		t.Fatalf("%d records, %d rows", f, h)
	}
}

func TestFISameFileTwicePresent(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	xml := fiXML(fiAccount, "1", "2", "3")
	if vm := decodeFI(t, s.upload(t, fiFile("a.xml", xml))); vm.Files[0].Status != "stored" || vm.Files[0].Aliases[0].RowsNew != 3 {
		t.Fatalf("%+v", vm.Files)
	}
	vm := decodeFI(t, s.upload(t, fiFile("a.xml", xml)))
	if a := vm.Files[0].Aliases[0]; vm.Files[0].Status != "present" || a.RowsNew != 0 || a.RowsPresent != 3 || a.RowsParsed != 3 {
		t.Fatalf("%+v", vm.Files)
	}
	if vm.Files[0].Aliases[0].Alias != fiAlias {
		t.Fatalf("alias %q", vm.Files[0].Aliases[0].Alias)
	}
	got := decodeFI(t, s.get(t))
	if len(got.Recent) != 2 || got.Recent[0].Status != "present" || got.Recent[1].Status != "stored" {
		t.Fatalf("%+v", got.Recent)
	}
}

func TestFIOverStoredRowsNoRevision(t *testing.T) {
	t.Parallel()
	st := newFIStore()
	st.seed(fiAlias, "1", "hash-of-another-field-set")
	s := newFISite(t, fiOpts{st: st})
	vm := decodeFI(t, s.upload(t, fiFile("a.xml", fiXMLWith(fiAccount, `commentary="added"`, "1"))))
	if vm.Files[0].Status != "present" || vm.Files[0].Aliases[0].RowsNew != 0 {
		t.Fatalf("%+v", vm.Files)
	}
	if _, _, _, handed := st.snapshot(); handed != 0 || st.rows[[3]string{fiAlias, "EXECUTION", "1"}] != "hash-of-another-field-set" {
		t.Fatalf("%d rows handed to the writer", handed)
	}
	if st.fetches[0].RowsRevised != 0 {
		t.Fatalf("%+v", st.fetches[0])
	}
}

func TestFIPageValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	sch := newSchemaSet("../..")
	check := func(what string, rec interface{ Bytes() []byte }) {
		if err := sch.validate("flex_import", json.RawMessage("1"), rec.Bytes()); err != nil {
			t.Errorf("%s: %v\n%.400s", what, err, rec.Bytes())
		}
	}
	check("get", s.get(t).Body)
	check("post", s.upload(t, fiFile("a.xml", fiXML(fiAccount, "1")), fiFile("b.xml", []byte("no")), fiFile("c.xml", fiXML("ACCT_X", "1"))).Body)
	check("off", newFISite(t, fiOpts{noEdit: true}).get(t).Body)
}
