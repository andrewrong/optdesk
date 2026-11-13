package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// The shared fakes and builders of the tests of the Flex import route. The values are synthetic: the account
// number, the token and the DSNs only let a test look for them in a body.
const (
	fiAccount   = "ACCT_A"
	fiAlias     = "main"
	fiLedgerDSN = "postgres://ledger-sentinel-dsn"
	fiPagesDSN  = "postgres://pages-sentinel-dsn"
	fiFlexToken = "flex-token-sentinel"
	fiEnv       = "OT_PG_DSN_SETTINGS=postgres://settings-sentinel-dsn\n" +
		"OT_PG_DSN_PAGES=" + fiPagesDSN + "\nOT_FLEX_TOKEN=" + fiFlexToken + "\nOT_FLEX_ACCT_MAIN=" + fiAccount + "\n"
	fiLedgerLine = "OT_PG_DSN_OPTION_LEDGER=" + fiLedgerDSN + "\n"
)

// fiStore is an in-memory option ledger store: the writer and the reader of one ledger, and the recent read.
type fiStore struct {
	mu sync.Mutex
	// rows holds the hash of the newest revision of each key (alias, level, transaction).
	rows map[[3]string]string
	// handed is every row a writer call was given, in order.
	handed  []store.FlexTradeRow
	fetches []store.FlexFetchRecord
	dsns    []string
	opened  int
	closed  int

	openErr    error
	writeErr   error
	recentErr  error
	noRecentRd bool
}

func newFIStore() *fiStore { return &fiStore{rows: map[[3]string]string{}} }

func (s *fiStore) open(_ context.Context, dsn string) (store.OptionLedgerStore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dsns = append(s.dsns, dsn)
	if s.openErr != nil {
		return nil, s.openErr
	}
	s.opened++
	if s.noRecentRd {
		return fiNoRecent{s}, nil
	}
	return s, nil
}

// fiNoRecent is the same store without the read of past imports.
type fiNoRecent struct{ s *fiStore }

func (f fiNoRecent) RecordFlexFetch(ctx context.Context, rec store.FlexFetchRecord, rows []store.FlexTradeRow) (store.FlexInsert, error) {
	return f.s.RecordFlexFetch(ctx, rec, rows)
}

func (f fiNoRecent) ImportFlexTradeRows(ctx context.Context, rows []store.FlexTradeRow) (store.FlexImportResult, error) {
	return f.s.ImportFlexTradeRows(ctx, rows)
}

func (f fiNoRecent) FlexTradeRows(ctx context.Context, alias string) ([]store.FlexTradeRow, error) {
	return f.s.FlexTradeRows(ctx, alias)
}

func (f fiNoRecent) LatestOkFetch(ctx context.Context, login, q string) (store.FlexFetchRecord, bool, error) {
	return f.s.LatestOkFetch(ctx, login, q)
}

func (f fiNoRecent) StoredAliases(ctx context.Context) ([]string, error) {
	return f.s.StoredAliases(ctx)
}
func (f fiNoRecent) Close() error { return f.s.Close() }

// RecordFlexFetch applies the rule of the store: a new key is New, a stored key with the same hash Unchanged, with
// another hash Revised. It counts RowsNew into the record, as the store does.
func (s *fiStore) RecordFlexFetch(_ context.Context, rec store.FlexFetchRecord, rows []store.FlexTradeRow) (store.FlexInsert, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeErr != nil {
		return store.FlexInsert{}, s.writeErr
	}
	var ins store.FlexInsert
	for _, r := range rows {
		s.handed = append(s.handed, r)
		k := [3]string{r.Alias, r.LevelOfDetail, r.TransactionID}
		switch h, ok := s.rows[k]; {
		case !ok:
			ins.New++
		case h == r.AttributesSHA256:
			ins.Unchanged++
		default:
			ins.Revised++
		}
		s.rows[k] = r.AttributesSHA256
	}
	rec.RowsNew, rec.RowsRevised, rec.RowsUnchanged = ins.New, ins.Revised, ins.Unchanged
	s.fetches = append(s.fetches, rec)
	return ins, nil
}

func (s *fiStore) ImportFlexTradeRows(context.Context, []store.FlexTradeRow) (store.FlexImportResult, error) {
	return store.FlexImportResult{}, errors.New("not used")
}

func (s *fiStore) FlexTradeRows(_ context.Context, alias string) ([]store.FlexTradeRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []store.FlexTradeRow{}
	for k, h := range s.rows {
		if k[0] == alias {
			out = append(out, store.FlexTradeRow{Alias: alias, LevelOfDetail: k[1], TransactionID: k[2], AttributesSHA256: h})
		}
	}
	return out, nil
}

func (s *fiStore) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}
func (s *fiStore) StoredAliases(context.Context) ([]string, error) { return []string{}, nil }

func (s *fiStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed++
	return nil
}

// RecentFetches answers the records of the login newest first, at most limit of them.
func (s *fiStore) RecentFetches(_ context.Context, login string, limit int) ([]store.FlexFetchRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recentErr != nil {
		return nil, s.recentErr
	}
	out := []store.FlexFetchRecord{}
	for _, f := range slices.Backward(s.fetches) {
		if f.Login == login && strings.HasPrefix(f.QueryID, "file:") && len(out) < limit {
			out = append(out, f)
		}
	}
	return out, nil
}

// seed stores a row of the alias under key as an earlier import did.
func (s *fiStore) seed(alias, txn, hash string) { s.rows[[3]string{alias, "EXECUTION", txn}] = hash }

func (s *fiStore) snapshot() (opened, closed, fetches, handed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.opened, s.closed, len(s.fetches), len(s.handed)
}

// fiOpts are the states of the site a test asks for.
type fiOpts struct {
	noEdit, noLedgerLine, noFlexFile, badFlexFile bool
	st                                            *fiStore
}

// fiSite is the site of the route over a fake ledger.
type fiSite struct {
	h    http.Handler
	st   *fiStore
	form settingsapi.SettingsForm
	logs *bytes.Buffer
}

func newFISite(t *testing.T, o fiOpts) *fiSite {
	t.Helper()
	env := fiEnv
	if !o.noLedgerLine {
		env += fiLedgerLine
	}
	a, logs := testApp(t, env)
	d := newDeps()
	a.deps.newRuleParams, a.deps.newSettingsForm = d.newRuleParams, d.newSettingsForm
	if o.st == nil {
		o.st = newFIStore()
	}
	a.deps.openOptionLedgerStore, a.deps.openOptionLedgerRO = o.st.open, o.st.open
	a.deps.loadFlexFile = func(string) (sources.FlexFile, error) {
		switch {
		case o.noFlexFile:
			return sources.FlexFile{}, fmt.Errorf("%w: no file", sources.ErrFlexNotConfigured)
		case o.badFlexFile:
			return sources.FlexFile{}, fmt.Errorf("%w: entry 1", sources.ErrFlexConfig)
		}
		return sources.FlexFile{Accounts: []sources.FlexAccount{{Alias: fiAlias, AccountEnv: "OT_FLEX_ACCT_MAIN"}}}, nil
	}
	form := d.newSettingsForm(nil, ownerTokenTTL, a.deps.newRuleParams())
	var rd siteReaders
	if !o.noEdit {
		rd.edit = &ownerEdit{form: form, token: postKey}
	}
	return &fiSite{h: siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler()), st: o.st, form: form, logs: logs}
}

// fiPart is one part of a multipart body: a field, or a file when file is set.
type fiPart struct {
	field, file string
	body        []byte
}

func fiFile(name string, body []byte) fiPart { return fiPart{field: "files", file: name, body: body} }

// fiBody writes the parts in order and returns the content type and the body.
func fiBody(t *testing.T, parts []fiPart) (string, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		params := map[string]string{"name": p.field}
		if p.file != "" {
			params["filename"] = p.file
		}
		h.Set("Content-Disposition", mime.FormatMediaType("form-data", params))
		pw, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pw.Write(p.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return mw.FormDataContentType(), buf.Bytes()
}

// fiRequest is a well-formed upload: tailnet peer and Host, the owner cookie, the content type of the body, its length declared.
func fiRequest(t *testing.T, parts []fiPart) *http.Request {
	t.Helper()
	ct, body := fiBody(t, parts)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/"+flexImportPostRoute, bytes.NewReader(body))
	r.Header.Set("Content-Type", ct)
	r.RemoteAddr = postPeer
	r.Host = postHost
	withOwnerCookie(r, postKey)
	return r
}

// upload posts the files.
func (s *fiSite) upload(t *testing.T, files ...fiPart) *httptest.ResponseRecorder {
	t.Helper()
	return serve(s.h, fiRequest(t, files))
}

func (s *fiSite) get(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	return settingsGet(t, s.h, http.MethodGet, "/"+siteFlexImportRoute)
}

// fiXML is a report of one statement of account with one EXECUTION trade per transaction id.
func fiXML(account string, ids ...string) []byte {
	return fiXMLWith(account, "", ids...)
}

// fiXMLWith adds the attribute text extra to every trade, so a row can differ from an earlier one.
func fiXMLWith(account, extra string, ids ...string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><FlexQueryResponse queryName="T" type="AF"><FlexStatements count="1">`)
	fmt.Fprintf(&b, `<FlexStatement accountId=%q fromDate="2025-01-01" toDate="2025-12-31"><Trades>`, account)
	for _, id := range ids {
		fmt.Fprintf(&b, `<Trade accountId=%q transactionID=%q levelOfDetail="EXECUTION" assetCategory="OPT" symbol="XYZ" `+
			`buySell="SELL" quantity="-1" tradeDate="2025-03-04" %s/>`, account, id, extra)
	}
	b.WriteString(`</Trades></FlexStatement></FlexStatements></FlexQueryResponse>`)
	return []byte(b.String())
}

func fiHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// fiVM is the part of the page the tests read.
type fiVM struct {
	Sentence string `json:"sentence"`
	Form     *struct {
		PostPath string `json:"post_path"`
		Expires  string `json:"expires"`
		MAC      string `json:"mac"`
	} `json:"form"`
	Gaps  []struct{ Reason, Text string } `json:"gaps"`
	Files []struct {
		Name, Status, Reason string
		Aliases              []struct {
			Alias       string
			Statements  int
			RowsParsed  int `json:"rows_parsed"`
			RowsNew     int `json:"rows_new"`
			RowsPresent int `json:"rows_present"`
		}
		Unmapped int `json:"unmapped_statements"`
	} `json:"files"`
	Recent []struct {
		Alias, Status, Reason string
		RowsParsed            int `json:"rows_parsed"`
		RowsNew               int `json:"rows_new"`
		RowsPresent           int `json:"rows_present"`
	} `json:"recent"`
}

func decodeFI(t *testing.T, rec *httptest.ResponseRecorder) fiVM {
	t.Helper()
	var vm fiVM
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil {
		t.Fatalf("%d %.300q: %v", rec.Code, rec.Body, err)
	}
	return vm
}

// hugeReader reads n zero bytes.
type hugeReader struct{ n int64 }

func (h *hugeReader) Read(b []byte) (int, error) {
	if h.n <= 0 {
		return 0, io.EOF
	}
	k := min(int64(len(b)), h.n)
	clear(b[:k])
	h.n -= k
	return int(k), nil
}
