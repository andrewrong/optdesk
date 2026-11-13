package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestFIRouteOffWithoutOwnerEdit(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{noEdit: true})
	rec := serve(s.h, fiRequest(t, []fiPart{fiFile("a.xml", fiXML(fiAccount, "1"))}))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("%d %.200q", rec.Code, rec.Body)
	}
	if o, _, f, _ := s.st.snapshot(); o != 0 || f != 0 {
		t.Fatalf("store used: opened %d, %d records", o, f)
	}
	// the page is there and read-only
	vm := decodeFI(t, s.get(t))
	if vm.Form != nil || len(vm.Gaps) != 1 {
		t.Fatalf("%+v", vm)
	}
}

func TestFIPostMethodsAre405(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodHead} {
		rec := serve(s.h, httptest.NewRequestWithContext(t.Context(), m, "/"+flexImportPostRoute, nil))
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodPost {
			t.Fatalf("%s: %d allow %q", m, rec.Code, rec.Header().Get("Allow"))
		}
	}
}

func TestFIReadOnlyWhenLineUnset(t *testing.T) {
	t.Parallel()
	for name, o := range map[string]fiOpts{
		"ledger line unset": {noLedgerLine: true},
		"file missing":      {noFlexFile: true},
		"file not valid":    {badFlexFile: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newFISite(t, o)
			rec := s.get(t)
			vm := decodeFI(t, rec)
			if rec.Code != 200 || vm.Form != nil || len(vm.Gaps) != 1 || len(vm.Recent) != 0 {
				t.Fatalf("%d %+v", rec.Code, vm)
			}
			if !strings.Contains(rec.Body.String(), `"form":null`) {
				t.Fatalf("%.300q", rec.Body)
			}
			if o, _, _, _ := s.st.snapshot(); o != 0 {
				t.Fatalf("a store was opened: %d", o)
			}
		})
	}
}

func TestFIGetCarriesFormAndReadsWithPagesLine(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	vm := decodeFI(t, s.get(t))
	if vm.Form == nil || vm.Form.PostPath != "/owner-flex-import" || vm.Form.Expires != "0" || vm.Form.MAC != strings.Repeat("0", 64) {
		t.Fatalf("%+v", vm.Form)
	}
	if len(s.st.dsns) != 1 || s.st.dsns[0] != fiPagesDSN {
		t.Fatalf("the recent list was read with %v", s.st.dsns)
	}
	if o, c, _, _ := s.st.snapshot(); o != 1 || c != 1 {
		t.Fatalf("opened %d closed %d", o, c)
	}
}

func TestFIRecentListsFailedRecord(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	// one post: a good file and a refused one
	if rec := s.upload(t, fiFile("a.xml", fiXML(fiAccount, "1", "2")), fiFile("b.xml", []byte("not xml"))); rec.Code != 200 {
		t.Fatalf("%d %.200q", rec.Code, rec.Body)
	}
	vm := decodeFI(t, s.get(t))
	if len(vm.Recent) != 2 {
		t.Fatalf("%+v", vm.Recent)
	}
	// newest first: the failed record of the second file, then the stored one
	if r := vm.Recent[0]; r.Status != "failed" || r.Reason != "not_report" || r.Alias != "" {
		t.Fatalf("%+v", r)
	}
	if r := vm.Recent[1]; r.Status != "stored" || r.Alias != fiAlias || r.RowsParsed != 2 || r.RowsNew != 2 || r.RowsPresent != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestFIRecentReadFails(t *testing.T) {
	t.Parallel()
	for name, o := range map[string]func(*fiStore){
		"read error": func(s *fiStore) { s.recentErr = errors.New("driver: password=hunter2") },
		"open error": func(s *fiStore) { s.openErr = errors.New("dial tcp 10.0.0.9:5432") },
		"no reader":  func(s *fiStore) { s.noRecentRd = true },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			st := newFIStore()
			o(st)
			s := newFISite(t, fiOpts{st: st})
			rec := s.get(t)
			vm := decodeFI(t, rec)
			if rec.Code != 200 || vm.Form == nil || len(vm.Recent) != 0 || len(vm.Gaps) != 1 || vm.Gaps[0].Text != "past imports cannot be read" {
				t.Fatalf("%d %+v", rec.Code, vm)
			}
			for _, leak := range []string{"hunter2", "10.0.0.9", "driver", fiPagesDSN} {
				if strings.Contains(rec.Body.String(), leak) || strings.Contains(s.logs.String(), leak) {
					t.Fatalf("%q leaked", leak)
				}
			}
		})
	}
}

func TestFIRecentRowsPresentIsDerived(t *testing.T) {
	t.Parallel()
	st := newFIStore()
	st.fetches = []store.FlexFetchRecord{{Login: "import", QueryID: "file:0a0b0c0d:1:main", FinishedAt: postNow.Add(time.Hour),
		RowsParsed: 5, RowsNew: 2, RowsRevised: 1}}
	vm := decodeFI(t, newFISite(t, fiOpts{st: st}).get(t))
	if len(vm.Recent) != 1 || vm.Recent[0].RowsPresent != 2 || vm.Recent[0].Status != "stored" {
		t.Fatalf("%+v", vm.Recent)
	}
}
