package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	fiRefused  = "upload refused: the form is not valid or has expired, load the page again"
	fiTooLarge = "upload too large: at most 64 MiB"
)

// fiNothingKept checks that a post left no record and never reached the store.
func fiNothingKept(t *testing.T, s *fiSite) {
	t.Helper()
	if len(s.st.dsns) != 0 {
		t.Fatalf("the store was reached with %v", s.st.dsns)
	}
	if _, _, f, h := s.st.snapshot(); f != 0 || h != 0 {
		t.Fatalf("%d records, %d rows", f, h)
	}
}

func TestFIDeclaredLengthOver413(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	r := fiRequest(t, append(s.tokenParts(), fiFile("a.xml", fiXML(fiAccount, "1"))))
	r.ContentLength = 64<<20 + 1
	rec := serve(s.h, r)
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), fiTooLarge) {
		t.Fatalf("%d %.200q", rec.Code, rec.Body)
	}
	fiNothingKept(t, s)
}

func TestFIUnknownLength413(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	r := fiRequest(t, append(s.tokenParts(), fiFile("a.xml", fiXML(fiAccount, "1"))))
	r.ContentLength = -1
	rec := serve(s.h, r)
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), fiTooLarge) {
		t.Fatalf("%d %.200q", rec.Code, rec.Body)
	}
	fiNothingKept(t, s)
}

func TestFIStreamOver413(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	// the declared length is the cap, the stream is longer: a file of zeros whose end is never reached
	var head bytes.Buffer
	mw := multipart.NewWriter(&head)
	for _, p := range s.tokenParts() {
		if err := mw.WriteField(p.field, string(p.body)); err != nil {
			t.Fatal(err)
		}
	}
	fw, err := mw.CreateFormFile("files", "big.xml")
	if err != nil {
		t.Fatal(err)
	}
	_ = fw
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/"+flexImportPostRoute,
		io.MultiReader(&head, &hugeReader{n: 66 << 20}))
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.RemoteAddr, r.Host, r.ContentLength = "127.0.0.1:5555", postListen, 64<<20
	rec := serve(s.h, r)
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), fiTooLarge) {
		t.Fatalf("%d %.200q", rec.Code, rec.Body)
	}
	if _, _, f, _ := s.st.snapshot(); f != 0 {
		t.Fatalf("%d records", f)
	}
}

func TestFITooLarge(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	// 51 MiB of zeros gzip to a few dozen KiB: over the 50 MiB limit after decompression
	big := fiGzip(t, make([]byte, 51<<20))
	vm := decodeFI(t, s.upload(t, fiFile("big.xml.gz", big), fiFile("ok.xml", fiXML(fiAccount, "1"))))
	if len(vm.Files) != 2 || vm.Files[0].Status != "failed" || vm.Files[0].Reason != "too_large" || vm.Files[1].Status != "stored" {
		t.Fatalf("%+v", vm.Files)
	}
	if s.st.fetches[0].ErrorCode != "too_large" {
		t.Fatalf("%+v", s.st.fetches[0])
	}
}

func TestFIBadGzip(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	vm := decodeFI(t, s.upload(t, fiFile("a.gz", []byte{0x1f, 0x8b, 0x00})))
	if len(vm.Files) != 1 || vm.Files[0].Reason != "bad_gzip" || s.st.fetches[0].ErrorCode != "bad_gzip" {
		t.Fatalf("%+v", vm.Files)
	}
}

func TestFINineFilesTooMany(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	var files []fiPart
	for i := 1; i <= 10; i++ {
		files = append(files, fiFile(fmt.Sprintf("f%d.xml", i), fiXML(fiAccount, fmt.Sprint(i))))
	}
	vm := decodeFI(t, s.upload(t, files...))
	if len(vm.Files) != 9 {
		t.Fatalf("%d results", len(vm.Files))
	}
	for i, f := range vm.Files[:8] {
		if f.Status != "stored" {
			t.Fatalf("file %d: %+v", i, f)
		}
	}
	if f := vm.Files[8]; f.Name != "(more files)" || f.Status != "failed" || f.Reason != "too_many_files" {
		t.Fatalf("%+v", f)
	}
	if len(s.st.rows) != 8 || len(s.st.fetches) != 9 || s.st.fetches[8].ErrorCode != "too_many_files" {
		t.Fatalf("%d rows, %d records", len(s.st.rows), len(s.st.fetches))
	}
}

func TestFIFileBeforeFieldsRefused(t *testing.T) {
	t.Parallel()
	for name, order := range map[string]func(*fiSite) []fiPart{
		"file first": func(s *fiSite) []fiPart {
			return append([]fiPart{fiFile("a.xml", fiXML(fiAccount, "1"))}, s.tokenParts()...)
		},
		"between fields": func(s *fiSite) []fiPart {
			tp := s.tokenParts()
			return []fiPart{tp[0], fiFile("a.xml", fiXML(fiAccount, "1")), tp[1]}
		},
		"no fields": func(*fiSite) []fiPart { return []fiPart{fiFile("a.xml", fiXML(fiAccount, "1"))} },
		"no file":   func(s *fiSite) []fiPart { return s.tokenParts()[:1] },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newFISite(t, fiOpts{})
			rec := serve(s.h, fiRequest(t, order(s)))
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), fiRefused) {
				t.Fatalf("%d %.200q", rec.Code, rec.Body)
			}
			fiNothingKept(t, s)
		})
	}
}

func TestFILedgerUnsetPostRefused(t *testing.T) {
	t.Parallel()
	for name, o := range map[string]fiOpts{"line unset": {noLedgerLine: true}, "file missing": {noFlexFile: true}, "file bad": {badFlexFile: true}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newFISite(t, o)
			rec := s.upload(t, fiFile("a.xml", fiXML(fiAccount, "1")))
			vm := decodeFI(t, rec)
			if rec.Code != 200 || vm.Form != nil || len(vm.Gaps) != 1 || len(vm.Files) != 0 {
				t.Fatalf("%d %+v", rec.Code, vm)
			}
			fiNothingKept(t, s)
		})
	}
}

func TestFIWriterOpenFails(t *testing.T) {
	t.Parallel()
	st := newFIStore()
	st.openErr = errors.New("dial tcp 10.0.0.9:5432: password=hunter2")
	s := newFISite(t, fiOpts{st: st})
	rec := s.upload(t, fiFile("a.xml", fiXML(fiAccount, "1")), fiFile("b.xml", []byte("junk")))
	vm := decodeFI(t, rec)
	if rec.Code != 200 || len(vm.Files) != 2 {
		t.Fatalf("%d %+v", rec.Code, vm)
	}
	for _, f := range vm.Files {
		if f.Status != "failed" || f.Reason != "store_error" {
			t.Fatalf("%+v", f)
		}
	}
	if len(vm.Gaps) != 1 || !strings.Contains(vm.Gaps[0].Text, "nothing was stored") {
		t.Fatalf("%+v", vm.Gaps)
	}
	if _, _, f, _ := st.snapshot(); f != 0 || len(st.dsns) != 1 {
		t.Fatalf("%d records, %d open attempts: one try per post", f, len(st.dsns))
	}
	for _, leak := range []string{"hunter2", "10.0.0.9", "dial", fiLedgerDSN} {
		if strings.Contains(rec.Body.String(), leak) || strings.Contains(s.logs.String(), leak) {
			t.Fatalf("%q leaked", leak)
		}
	}
}

func TestFIWriteFailsLeavesStoreErrorFile(t *testing.T) {
	t.Parallel()
	st := newFIStore()
	st.writeErr = errors.New("insert failed")
	s := newFISite(t, fiOpts{st: st})
	vm := decodeFI(t, s.upload(t, fiFile("a.xml", fiXML(fiAccount, "1"))))
	if len(vm.Files) != 1 || vm.Files[0].Reason != "store_error" || len(vm.Gaps) != 1 {
		t.Fatalf("%+v", vm)
	}
	if o, c, _, _ := st.snapshot(); o != 1 || c != 1 {
		t.Fatalf("opened %d closed %d", o, c)
	}
}

func TestFIFileNameDisplayOnly(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	long := strings.Repeat("a", 300) + ".xml"
	vm := decodeFI(t, s.upload(t,
		fiFile("../../etc/"+long, fiXML(fiAccount, "1")),
		fiFile(`C:\Users\me\evil.xml`, fiXML(fiAccount, "2")),
		fiFile("", fiXML(fiAccount, "3"))))
	if len(vm.Files) != 3 {
		t.Fatalf("%+v", vm.Files)
	}
	if got := vm.Files[0].Name; got != strings.Repeat("a", 64) {
		t.Fatalf("name %q", got)
	}
	if got := vm.Files[1].Name; got != "evil.xml" {
		t.Fatalf("name %q", got)
	}
	if got := vm.Files[2].Name; got != "" {
		t.Fatalf("name %q", got)
	}
	for _, f := range s.st.fetches {
		if strings.Contains(fmt.Sprintf("%+v", f), "evil") || strings.Contains(fmt.Sprintf("%+v", f), "etc") {
			t.Fatalf("a name was stored: %+v", f)
		}
	}
}

func TestFIUnknownPartsIgnored(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	var parts []fiPart
	parts = append(parts, fiPart{field: "note", body: []byte("hello")})
	parts = append(parts, s.tokenParts()...)
	for i := 1; i <= 8; i++ {
		parts = append(parts, fiPart{field: "extra", file: "x.xml", body: []byte("<big/>")},
			fiFile(fmt.Sprintf("f%d.xml", i), fiXML(fiAccount, fmt.Sprint(i))))
	}
	rec := serve(s.h, fiRequest(t, parts))
	vm := decodeFI(t, rec)
	if rec.Code != 200 || len(vm.Files) != 8 {
		t.Fatalf("%d, %d results", rec.Code, len(vm.Files))
	}
	for _, f := range vm.Files {
		if f.Status != "stored" {
			t.Fatalf("%+v", f)
		}
	}
	if len(s.st.fetches) != 8 {
		t.Fatalf("%d records", len(s.st.fetches))
	}
}

func TestFINoSecretInAnyBody(t *testing.T) {
	t.Parallel()
	sentinels := []string{fiAccount, "ACCT_OTHER", "file:", fiFlexToken, "sentinel", "hunter2", "10.0.0.9", "postgres://"}
	check := func(what string, s *fiSite, rec *httptest.ResponseRecorder) {
		t.Helper()
		all := rec.Body.String() + "\n" + s.logs.String()
		for _, bad := range sentinels {
			if strings.Contains(all, bad) {
				t.Errorf("%s: %q reached the body or the log:\n%.600s", what, bad, all)
			}
		}
	}
	s := newFISite(t, fiOpts{})
	good, other := fiXML(fiAccount, "1"), fiXML("ACCT_OTHER", "1")
	check("get", s, s.get(t))
	check("stored", s, s.upload(t, fiFile("a.xml", good)))
	check("present", s, s.upload(t, fiFile("a.xml", good)))
	check("unconfigured", s, s.upload(t, fiFile("o.xml", other)))
	check("refused file", s, s.upload(t, fiFile("n.xml", []byte("junk"))))
	check("get after", s, s.get(t))
	check("forged", s, serve(s.h, fiRequest(t, []fiPart{{field: "expires", body: []byte("1")}, {field: "mac", body: []byte("x")}, fiFile("a.xml", good)})))
	over := fiRequest(t, append(s.tokenParts(), fiFile("a.xml", good)))
	over.ContentLength = 65 << 20
	check("over", s, serve(s.h, over))
	check("405", s, serve(s.h, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+flexImportPostRoute, nil)))
	check("off", s, newFISite(t, fiOpts{noEdit: true}).get(t))
	for name, o := range map[string]fiOpts{"unset": {noLedgerLine: true}, "nofile": {noFlexFile: true}, "badfile": {badFlexFile: true}} {
		s2 := newFISite(t, o)
		check(name+" get", s2, s2.get(t))
		check(name+" post", s2, s2.upload(t, fiFile("a.xml", good)))
	}
	broken := newFIStore()
	broken.openErr = errors.New("dial tcp 10.0.0.9:5432 password=hunter2")
	s3 := newFISite(t, fiOpts{st: broken})
	check("open fails", s3, s3.upload(t, fiFile("a.xml", good)))
	check("recent fails", s3, s3.get(t))
}
