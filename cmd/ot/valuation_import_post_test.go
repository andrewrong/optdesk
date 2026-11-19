package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// fakeImporter is a PlanFunc over invented rows: AAA is on the list and is inserted; ZZZ is not on the list, needs a
// tick and a picked key, and is written under the key picked. It records what it was asked.
type fakeImporter struct {
	mu    sync.Mutex
	files [][]byte
	picks []map[string]valueimportapi.Pick
	state []valueimportapi.Stored
}

func (f *fakeImporter) plan(in valueimportapi.Input) (valueimportapi.Plan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files = append(f.files, in.File)
	f.picks = append(f.picks, in.Picks)
	f.state = append(f.state, in.Stored)
	sum := sha256.Sum256(in.File)
	zzz := valueimportapi.RowPlan{Ticker: "ZZZ", Name: "Zed Works", Symbol: "ZZZ", Currency: "USD", RMarket: "US", Pool: "in_pool",
		Unmatched: true, ProposedKey: "US:ZZZ", NeedsTick: true, NewProfile: true, Action: valueimportapi.ActionInsert,
		ReviewType: "first", V: "10", P: "3", VersionDate: "2026-11-01"}
	if p, ok := in.Picks["ZZZ"]; ok {
		zzz.Key, zzz.RMarket, zzz.ReviewID = p.Key, p.RMarket, "imp-test-zzz"
	}
	return valueimportapi.Plan{
		FileSHA256: hex.EncodeToString(sum[:]), AsOf: "2026-11-05",
		R:      valueimportapi.RPlan{Action: valueimportapi.ActionInsert, US: "0.9", JP: "0.8", HKCN: "0.7"},
		Margin: valueimportapi.MarginPlan{Action: valueimportapi.ActionInsert, EffectiveDate: "2026-11-05"},
		Rows: []valueimportapi.RowPlan{
			{Key: "US:AAA", Ticker: "AAA", Name: "AAA Inc", Symbol: "AAA", Currency: "USD", RMarket: "US", Pool: "in_pool",
				NewProfile: true, Action: valueimportapi.ActionInsert, ReviewType: "first", ReviewID: "imp-test-aaa",
				V: "100", P: "4", VersionDate: "2026-11-01"},
			zzz,
		},
	}, nil
}

// importSite is the valuation site with the importer injected.
func importSite(t *testing.T, withEdit bool) (http.Handler, *fakeValuationStore, *fakeImporter) {
	t.Helper()
	a, _ := testApp(t, underlyingsEnv)
	a.deps.newRuleParams = newDeps().newRuleParams
	f := newFakeValuation()
	imp := &fakeImporter{}
	a.deps.planValueImport = imp.plan
	a.deps.openValuation = func(string) (valuation.Reader, valuation.Writer, io.Closer, error) {
		f.mu.Lock()
		f.opened++
		f.mu.Unlock()
		return f, f, nopCloser{}, nil
	}
	rd := siteReaders{underlying: &fakeUnderlyingStore{rows: valuationUnderlyingRows()}, series: onePoint{}}
	if withEdit {
		rd.edit = &ownerEdit{form: newDeps().newSettingsForm(nil, ownerTokenTTL, a.deps.newRuleParams()), token: postKey}
	}
	return siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler()), f, imp
}

// importRequest is a multipart post of the fields, in the order given, then the file part last.
func importRequest(t *testing.T, fields [][2]string, file []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, kv := range fields {
		if err := mw.WriteField(kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}
	fw, err := mw.CreateFormFile("file", "export.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(file)
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/valuation-import-post", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.RemoteAddr = postPeer
	r.Host = postHost
	withOwnerCookie(r, postKey)
	return r
}

type importDoc struct {
	View string `json:"view"`
	File *struct {
		SHA256 string `json:"sha256"`
	} `json:"file"`
	Rows []struct {
		Key         string `json:"key"`
		Ticker      string `json:"ticker"`
		RMarket     string `json:"r_market"`
		ProposedKey string `json:"proposed_key"`
		NeedsTick   bool   `json:"needs_tick"`
	} `json:"rows"`
	Result []struct{ Key, Outcome, Text string } `json:"result"`
}

func decodeImport(t *testing.T, rec *httptest.ResponseRecorder) importDoc {
	t.Helper()
	var d importDoc
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("%v: %.300q", err, rec.Body)
	}
	return d
}

// failReader fails the test when the body is read.
type failReader struct{ t *testing.T }

func (f failReader) Read([]byte) (int, error) {
	f.t.Error("the body was read")
	return 0, io.EOF
}

func TestImportPostWithoutCookieWritesNothing(t *testing.T) {
	t.Parallel()
	h, f, imp := importSite(t, true)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/valuation-import-post", failReader{t})
	r.ContentLength = 100
	r.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	r.RemoteAddr, r.Host = postPeer, postHost
	rec := serve(h, r)
	if rec.Code != http.StatusForbidden || f.opened != 0 || len(f.written()) != 0 || len(imp.files) != 0 {
		t.Fatalf("%d, opened %d, %v", rec.Code, f.opened, f.written())
	}
}

func TestImportPostGetIs405(t *testing.T) {
	t.Parallel()
	h, f, _ := importSite(t, true)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/valuation-import-post", nil)
	r.RemoteAddr, r.Host = postPeer, postHost
	withOwnerCookie(r, postKey)
	rec := serve(h, r)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodPost || f.opened != 0 {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestImportPostTooLargeIs413(t *testing.T) {
	t.Parallel()
	h, f, imp := importSite(t, true)
	// a declared length over the cap
	r := importRequest(t, nil, []byte("{}"))
	r.ContentLength = valueimportapi.MaxFileBytes + importFormMargin + 1
	if rec := serve(h, r); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("declared: %d", rec.Code)
	}
	// a file part over MaxFileBytes inside a body within the cap
	r = importRequest(t, nil, bytes.Repeat([]byte("a"), valueimportapi.MaxFileBytes+10))
	if rec := serve(h, r); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("file part: %d", rec.Code)
	}
	if f.opened != 0 || len(f.written()) != 0 || len(imp.files) != 0 {
		t.Fatalf("opened %d, written %v", f.opened, f.written())
	}
}

func TestImportPreviewWritesNothing(t *testing.T) {
	t.Parallel()
	h, f, imp := importSite(t, true)
	file := []byte(`{"invented":true}`)
	rec := serve(h, importRequest(t, [][2]string{{"action", "preview"}}, file))
	d := decodeImport(t, rec)
	if rec.Code != http.StatusOK || d.View != "preview" || len(d.Rows) != 2 || len(f.written()) != 0 {
		t.Fatalf("%d %+v %v", rec.Code, d, f.written())
	}
	if len(imp.files) != 1 || !bytes.Equal(imp.files[0], file) || imp.picks[0] != nil {
		t.Fatalf("importer saw %d files, picks %v", len(imp.files), imp.picks)
	}
	st := imp.state[0]
	if len(st.Underlying) != 2 || st.Latest != "4.2" || st.R == nil || st.R.US != "0.9" || len(st.Margin) != 1 {
		t.Fatalf("stored state %+v", st)
	}
}

func TestImportConfirmOtherFileWritesNothing(t *testing.T) {
	t.Parallel()
	h, f, _ := importSite(t, true)
	rec := serve(h, importRequest(t, [][2]string{{"action", "preview"}}, []byte("file one")))
	sha := decodeImport(t, rec).File.SHA256
	rec = serve(h, importRequest(t, [][2]string{{"action", "confirm"}, {"file_sha256", sha}, {"tick", "ZZZ"}}, []byte("file two")))
	d := decodeImport(t, rec)
	if rec.Code != http.StatusOK || d.View != "preview" || d.File.SHA256 == sha || len(f.written()) != 0 {
		t.Fatalf("%d view %q written %v", rec.Code, d.View, f.written())
	}
}

func TestImportConfirmStepTwoFromPreviewPayload(t *testing.T) {
	t.Parallel()
	h, f, imp := importSite(t, true)
	file := []byte("the same file")
	prev := decodeImport(t, serve(h, importRequest(t, [][2]string{{"action", "preview"}}, file)))
	fields := [][2]string{{"action", "confirm"}, {"file_sha256", prev.File.SHA256}}
	for _, row := range prev.Rows {
		if row.NeedsTick {
			fields = append(fields, [2]string{"tick", row.Ticker}, [2]string{"key." + row.Ticker, row.ProposedKey},
				[2]string{"r_market." + row.Ticker, row.RMarket})
		}
	}
	rec := serve(h, importRequest(t, fields, file))
	d := decodeImport(t, rec)
	if rec.Code != http.StatusOK || d.View != "result" {
		t.Fatalf("%d %q: %.300q", rec.Code, d.View, rec.Body)
	}
	var keys []string
	for _, r := range d.Result {
		keys = append(keys, r.Key+"="+r.Outcome)
	}
	want := []string{"margin_table=inserted", "r_version=inserted", "US:AAA=inserted", "US:ZZZ=inserted"}
	if !slices.Equal(keys, want) {
		t.Fatalf("result %v, want %v", keys, want)
	}
	if p := imp.picks[len(imp.picks)-1]["ZZZ"]; p.Key != "US:ZZZ" || p.RMarket != "US" {
		t.Fatalf("pick %+v", p)
	}
	wantCalls := []string{"margin", "rversion", "profile", "review:effective", "profile", "review:effective"}
	if !slices.Equal(f.written(), wantCalls) {
		t.Fatalf("calls %v, want %v", f.written(), wantCalls)
	}
	if !strings.HasPrefix(f.reviews[0].ReviewID, "imp-") || f.reviews[0].Source != "owner" {
		t.Fatalf("review %+v", f.reviews[0])
	}
}
