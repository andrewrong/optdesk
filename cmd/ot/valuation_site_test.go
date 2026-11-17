package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// fakeValuationStore is a valuation.Reader and Writer over memory that records what is written. Every number in
// it is made up.
type fakeValuationStore struct {
	mu       sync.Mutex
	profiles []valuation.Profile
	reviews  []valuation.Review // oldest first
	rvers    []valuation.RVersion
	rtables  []valuation.RTableVersion
	margins  []valuation.MarginTableVersion
	calls    []string // "profile", "review:<status>", "rversion", "rtable", "margin", in order
	opened   int

	failProfile bool // PutProfile returns an error
}

func newFakeValuation() *fakeValuationStore {
	f := &fakeValuationStore{}
	f.rvers = []valuation.RVersion{{EffectiveDate: "2026-10-01", YieldUsed: "3.5", RUS: "0.9", RJP: "0.8", RHKCN: "0.7"}}
	f.rtables = []valuation.RTableVersion{{EffectiveDate: "2026-10-01", Rows: []valuation.RTableRow{
		{YieldUpTo: "4.0", US: "0.9", JP: "0.8", HKCN: "0.7"}, {YieldUpTo: "4.5", US: "0.85", JP: "0.75", HKCN: "0.65"},
		{US: "0.7", JP: "0.6", HKCN: "0.5"}}}}
	f.margins = []valuation.MarginTableVersion{{EffectiveDate: "2026-10-01", Rows: []valuation.MarginTableRow{
		{PFrom: "1", M: "0.18", W: "0.07", H: "0.30", S: "0.22"}}}}
	return f
}

func (f *fakeValuationStore) record(c string) { f.calls = append(f.calls, c) }

func (f *fakeValuationStore) Current(context.Context) (valuation.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := valuation.State{Entries: []valuation.Entry{}}
	for _, p := range f.profiles {
		e := valuation.Entry{Profile: p}
		for i := len(f.reviews) - 1; i >= 0; i-- {
			r := f.reviews[i]
			if r.CompanyKey == p.CompanyKey && r.Status == "effective" && r.Type != "backfill" && !f.voided(r.ReviewID) {
				e.Current = &r
				break
			}
		}
		st.Entries = append(st.Entries, e)
	}
	slices.SortFunc(st.Entries, func(a, b valuation.Entry) int { return strings.Compare(a.Profile.CompanyKey, b.Profile.CompanyKey) })
	if n := len(f.rvers); n > 0 {
		st.RVersion = &f.rvers[n-1]
	}
	if n := len(f.rtables); n > 0 {
		st.RTable = &f.rtables[n-1]
	}
	if n := len(f.margins); n > 0 {
		st.Margin = &f.margins[n-1]
	}
	return st, nil
}

func (f *fakeValuationStore) voided(id string) bool {
	for _, r := range f.reviews {
		if r.ReviewID == id && r.Status == "void" {
			return true
		}
	}
	return false
}

func (f *fakeValuationStore) Reviews(_ context.Context, key string) ([]valuation.Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []valuation.Review{}
	for i := len(f.reviews) - 1; i >= 0; i-- {
		if f.reviews[i].CompanyKey == key {
			out = append(out, f.reviews[i])
		}
	}
	return out, nil
}

func (f *fakeValuationStore) RVersions(context.Context) ([]valuation.RVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := slices.Clone(f.rvers)
	slices.Reverse(out)
	return out, nil
}

func (f *fakeValuationStore) PutProfile(_ context.Context, p valuation.Profile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failProfile {
		return errors.New("profile write refused")
	}
	f.record("profile")
	for i, o := range f.profiles {
		if o.CompanyKey == p.CompanyKey {
			f.profiles[i] = p
			return nil
		}
	}
	f.profiles = append(f.profiles, p)
	return nil
}

func (f *fakeValuationStore) PutReview(_ context.Context, r valuation.Review) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	hasEff := false
	for _, o := range f.reviews {
		if o.ReviewID == r.ReviewID && o.Status == "effective" {
			hasEff = true
		}
	}
	if hasEff && r.Status != "void" {
		return valuation.ErrClosed
	}
	if !hasEff && r.Status == "void" {
		return valuation.ErrNotEffective
	}
	f.record("review:" + r.Status)
	f.reviews = append(f.reviews, r)
	return nil
}

func (f *fakeValuationStore) PutRVersion(_ context.Context, v valuation.RVersion) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("rversion")
	f.rvers = append(f.rvers, v)
	return nil
}

func (f *fakeValuationStore) PutRTable(_ context.Context, t valuation.RTableVersion) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("rtable")
	f.rtables = append(f.rtables, t)
	return nil
}

func (f *fakeValuationStore) PutMarginTable(_ context.Context, t valuation.MarginTableVersion) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("margin")
	f.margins = append(f.margins, t)
	return nil
}

func (f *fakeValuationStore) written() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// valuationSite is the site over the fake store: the reader of the pages, the opener of the writer, the list of
// settings.underlying (AAA and BBB) and the series with one DGS10 point. withEdit turns the owner posts on.
func valuationSite(t *testing.T, withEdit bool) (http.Handler, *fakeValuationStore) {
	t.Helper()
	a, _ := testApp(t, underlyingsEnv)
	a.deps.newRuleParams = newDeps().newRuleParams
	f := newFakeValuation()
	a.deps.openValuation = func(string) (valuation.Reader, valuation.Writer, io.Closer, error) {
		f.mu.Lock()
		f.opened++
		f.mu.Unlock()
		return f, f, nopCloser{}, nil
	}
	rd := siteReaders{
		underlying: &fakeUnderlyingStore{rows: valuationUnderlyingRows()}, series: onePoint{},
		ext: map[string]any{siteValuationKey: valuation.Reader(f)},
	}
	if withEdit {
		rd.edit = &ownerEdit{form: newDeps().newSettingsForm(nil, ownerTokenTTL, a.deps.newRuleParams()), token: postKey}
	}
	return siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler()), f
}

// validateValuationDoc checks b against the named page schema of contracts/schemas.
func validateValuationDoc(t *testing.T, b []byte, schema string) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(schemaFileLoader{})
	sch, err := c.Compile(dataHealthSchemaHost + schema)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("schema %s: %v\n%s", schema, err, b)
	}
}

func TestValuationRoutes(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, false)
	f.profiles = []valuation.Profile{{CompanyKey: "US:AAA", Name: "AAA Inc", Symbol: "AAA", Currency: "USD", RMarket: "US", Pool: "in_pool"}}
	f.reviews = []valuation.Review{{ReviewID: "r1", CompanyKey: "US:AAA", Type: "first", VersionDate: "2026-11-01",
		Status: "effective", Source: "owner", V: "100", P: "3"}}
	for _, c := range []struct{ target, schema string }{
		{"/valuation-params", "page.valuation_params_page.v1.json"},
		{"/valuation", "page.valuation_page.v1.json"},
		{"/valuation-company?company=US:AAA", "page.valuation_page.v1.json"},
		{"/valuation-form?company=US:BBB", "page.valuation_page.v1.json"},
	} {
		rec := settingsGet(t, h, http.MethodGet, c.target)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %.200q", c.target, rec.Code, rec.Body)
		}
		validateValuationDoc(t, rec.Body.Bytes(), c.schema)
	}
	// the table has the profile and the name on the list with no profile
	var vm struct {
		Rows []struct {
			CompanyKey string `json:"company_key"`
		} `json:"rows"`
		UnvaluedCount int `json:"unvalued_count"`
	}
	rec := settingsGet(t, h, http.MethodGet, "/valuation")
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil || len(vm.Rows) != 2 || vm.UnvaluedCount != 1 {
		t.Fatalf("%v %+v", err, vm)
	}
	// the params page reads the latest DGS10 point
	rec = settingsGet(t, h, http.MethodGet, "/valuation-params")
	if !strings.Contains(rec.Body.String(), `"latest_yield":{"value":"4.20"`) {
		t.Fatalf("no yield: %.400q", rec.Body)
	}
}

func TestValuationRoutesWithoutStoreAreGaps(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	rd := siteReaders{}
	h := siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler())
	for _, c := range []struct{ target, schema string }{
		{"/valuation-params", "page.valuation_params_page.v1.json"},
		{"/valuation", "page.valuation_page.v1.json"},
		{"/valuation-company?company=US:AAA", "page.valuation_page.v1.json"},
		{"/valuation-form", "page.valuation_page.v1.json"},
	} {
		rec := settingsGet(t, h, http.MethodGet, c.target)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"kind":"gap"`) {
			t.Fatalf("%s: %d %.300q", c.target, rec.Code, rec.Body)
		}
		validateValuationDoc(t, rec.Body.Bytes(), c.schema)
	}
}

func TestCompanyKeyFromQuery(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, false)
	f.profiles = []valuation.Profile{{CompanyKey: "US:AAA", Name: "AAA Inc", Symbol: "AAA", Currency: "USD", RMarket: "US", Pool: "in_pool"}}
	var vm struct {
		Company *struct {
			CompanyKey string `json:"company_key"`
		} `json:"company"`
		Gaps []struct{ Reason string } `json:"gaps"`
	}
	rec := settingsGet(t, h, http.MethodGet, "/valuation-company?company=US:AAA")
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil || vm.Company == nil || vm.Company.CompanyKey != "US:AAA" || len(vm.Gaps) != 0 {
		t.Fatalf("%v %+v", err, vm)
	}
	vm.Company = nil
	rec = settingsGet(t, h, http.MethodGet, "/valuation-company?company=US:NOSUCH")
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil || rec.Code != http.StatusOK || vm.Company != nil || len(vm.Gaps) == 0 {
		t.Fatalf("%d %v %+v", rec.Code, err, vm)
	}
}

func TestPageListHasValuation(t *testing.T) {
	t.Parallel()
	want := map[string]struct {
		route string
		order int
	}{
		"valuation_params": {"valuation-params", 140}, "valuation_table": {"valuation", 150},
		"valuation_company": {"valuation-company", 160}, "valuation_form": {"valuation-form", 170},
	}
	names := registeredPageNames()
	last := -1
	for _, n := range []string{"valuation_params", "valuation_table", "valuation_company", "valuation_form"} {
		f, ok := pageFamilyOfName(n)
		if !ok || f.route != want[n].route || f.order != want[n].order || f.build == nil {
			t.Fatalf("%s: %+v", n, f)
		}
		i := slices.Index(names, n)
		if i <= last {
			t.Fatalf("%s is at %d after %d in %v", n, i, last, names)
		}
		last = i
	}
	// the company pages (orders 180 and 190) come after the four
	if tail := names[len(names)-3:]; tail[0] != "valuation_form" || tail[1] != "company_quarters_page" || tail[2] != "company_data_page" {
		t.Fatalf("the four names are not followed by the company pages: %v", names)
	}
}

func TestValuationSavedLineOnlyForOwner(t *testing.T) {
	t.Parallel()
	for _, edit := range []bool{false, true} {
		h, _ := valuationSite(t, edit)
		rec := settingsGet(t, h, http.MethodGet, "/valuation?saved=1")
		var vm struct{ Saved string }
		if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil || (vm.Saved != "") != edit {
			t.Fatalf("edit %v: %q %v", edit, vm.Saved, err)
		}
	}
}

var (
	_ = contracts.Day("")
	_ = underlying.Row{}
)

// valuationUnderlyingRows is the list of AAA and BBB with a currency, as the owner form needs one.
func valuationUnderlyingRows() []underlying.Row {
	rows := underlyingRows()
	for i := range rows {
		rows[i].Currency = "USD"
	}
	return rows
}
