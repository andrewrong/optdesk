package jquants

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

const testKey = "SECRETKEY123"

// fakeDoer counts calls and answers each with the same status and body, or a transport error.
type fakeDoer struct {
	mu     sync.Mutex
	calls  int
	status int
	body   string
	err    error
	reqs   []*http.Request
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	return &http.Response{StatusCode: f.status, Body: io.NopCloser(strings.NewReader(f.body))}, nil
}

// sleepRec records the durations asked of Sleep and never waits; err, when set, is returned by every call.
type sleepRec struct {
	mu  sync.Mutex
	got []time.Duration
	err error
}

func (r *sleepRec) sleep(_ context.Context, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, d)
	return r.err
}

func newSourceRec(t *testing.T, d sources.HTTPDoer, symbols ...string) (sources.EarningsSource, *sleepRec) {
	t.Helper()
	rec := &sleepRec{}
	s, err := New(sources.EarningsConfig{BaseURL: "http://jq.invalid", APIKey: testKey, Symbols: symbols, Doer: d, Sleep: rec.sleep})
	if err != nil {
		t.Fatal(err)
	}
	return s, rec
}

func newSource(t *testing.T, d sources.HTTPDoer, symbols ...string) sources.EarningsSource {
	t.Helper()
	s, _ := newSourceRec(t, d, symbols...)
	return s
}

// seqDoer answers the statuses in order, the last one for every call after; it records each call's code.
type seqDoer struct {
	mu       sync.Mutex
	statuses []int
	codes    []string
	body     string
}

func (f *seqDoer) Do(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := f.statuses[min(len(f.codes), len(f.statuses)-1)]
	f.codes = append(f.codes, req.URL.Query().Get("code"))
	return &http.Response{StatusCode: st, Body: io.NopCloser(strings.NewReader(f.body))}, nil
}

func testdata(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var ce sources.CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("not a CodedError: %v", err)
	}
	return ce.ErrorCode()
}

func TestJQuantsParse(t *testing.T) {
	t.Parallel()
	d := &fakeDoer{status: 200, body: testdata(t, "earnings_date.json")}
	got, err := newSource(t, d, "9999").Fetch(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("len %d err %v", len(got), err)
	}
	want := sources.EarningsListing{Symbol: "9999", ReportDate: "2026-08-04", FiscalPeriodEnd: "2027-03-31"}
	if got[0] != want {
		t.Fatalf("got %+v want %+v", got[0], want)
	}
	if k := d.reqs[0].Header.Get("x-api-key"); k != testKey {
		t.Fatalf("header key %q", k)
	}
	if strings.Contains(d.reqs[0].URL.String(), testKey) || d.reqs[0].Header.Get("Authorization") != "" {
		t.Fatal("key in URL or Authorization header set")
	}
}

func TestJQuantsReturnsEveryDatedRow(t *testing.T) {
	t.Parallel()
	got, err := newSource(t, &fakeDoer{status: 200, body: testdata(t, "earnings_date.json")}, "9999").Fetch(context.Background())
	if err != nil || len(got) != 2 || got[0].ReportDate != "2026-08-04" || got[1].ReportDate != "2026-05-12" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestJQuantsFiveDigitCodeSent(t *testing.T) {
	t.Parallel()
	d := &fakeDoer{status: 200, body: testdata(t, "earnings_date_empty.json")}
	if _, err := newSource(t, d, "9999").Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c := d.reqs[0].URL.Query().Get("code"); c != "99990" {
		t.Fatalf("code %q", c)
	}
	if d.reqs[0].URL.Path != "/v2/fins/earnings-date" {
		t.Fatalf("path %q", d.reqs[0].URL.Path)
	}
}

func TestJQuantsEmptyBodyIsAbsent(t *testing.T) {
	t.Parallel()
	got, err := newSource(t, &fakeDoer{status: 200, body: testdata(t, "earnings_date_empty.json")}, "9999").Fetch(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestJQuantsEmptyDateIsAbsent(t *testing.T) {
	t.Parallel()
	b := `{"data":[{"PubDate":"2026-02-01","SchDate":"","FYE":"2026-03-31","Code":"99990"}]}`
	got, err := newSource(t, &fakeDoer{status: 200, body: b}, "9999").Fetch(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestJQuantsKeyNeverInError(t *testing.T) {
	t.Parallel()
	cases := []*fakeDoer{
		{status: 500, body: "boom " + testKey},
		{status: 200, body: "not json " + testKey},
		{err: errors.New("dial " + testKey)},
		{status: 200, body: `{"data":[],"pagination_key":"` + testKey + `"}`},
	}
	for _, d := range cases {
		_, err := newSource(t, d, "9999").Fetch(context.Background())
		if err == nil || strings.Contains(err.Error(), testKey) {
			t.Fatalf("err %v", err)
		}
	}
}

func TestJQuantsAuthErrorIsTyped(t *testing.T) {
	t.Parallel()
	for _, st := range []int{401, 403} {
		d := &fakeDoer{status: st}
		_, err := newSource(t, d, "9999").Fetch(context.Background())
		if err == nil || codeOf(t, err) != "http_status" || !strings.Contains(err.Error(), "status") {
			t.Fatalf("status %d err %v", st, err)
		}
		if d.calls != 1 {
			t.Fatalf("calls %d", d.calls)
		}
	}
}

func TestJQuantsBadRequestIsTyped(t *testing.T) {
	t.Parallel()
	_, err := newSource(t, &fakeDoer{status: 400, body: `{"message":"x"}`}, "9999").Fetch(context.Background())
	if err == nil || codeOf(t, err) != "http_status" {
		t.Fatalf("err %v", err)
	}
	_, err = newSource(t, &fakeDoer{status: 200, body: "<html>"}, "9999").Fetch(context.Background())
	if err == nil || codeOf(t, err) != "not_json" {
		t.Fatalf("err %v", err)
	}
}

func TestJQuantsPaginationKeyIsError(t *testing.T) {
	t.Parallel()
	_, err := newSource(t, &fakeDoer{status: 200, body: `{"data":[],"pagination_key":"abc"}`}, "9999").Fetch(context.Background())
	if err == nil || codeOf(t, err) != "unexpected_pagination" {
		t.Fatalf("err %v", err)
	}
	if _, err := newSource(t, &fakeDoer{status: 200, body: `{"data":[],"pagination_key":null}`}, "9999").Fetch(context.Background()); err != nil {
		t.Fatalf("null key: %v", err)
	}
}

func TestJQuantsOneCallPerSymbol(t *testing.T) {
	t.Parallel()
	d := &fakeDoer{status: 200, body: testdata(t, "earnings_date_empty.json")}
	if _, err := newSource(t, d, "1111", "2222", "3333").Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.calls != 3 {
		t.Fatalf("calls %d", d.calls)
	}
	for i, want := range []string{"11110", "22220", "33330"} {
		if c := d.reqs[i].URL.Query().Get("code"); c != want {
			t.Fatalf("call %d code %q", i, c)
		}
	}
}

func kindOf(t *testing.T, err error) contracts.ErrorKind {
	t.Helper()
	var te contracts.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("not a TypedError: %v", err)
	}
	return te.Kind()
}

func TestJQuantsRateLimitedRetriesOnce(t *testing.T) {
	t.Parallel()
	d := &seqDoer{statuses: []int{429, 200}, body: testdata(t, "earnings_date.json")}
	src, rec := newSourceRec(t, d, "9999")
	got, err := src.Fetch(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("len %d err %v", len(got), err)
	}
	if len(d.codes) != 2 || d.codes[0] != "99990" || d.codes[1] != "99990" {
		t.Fatalf("codes %v", d.codes)
	}
	if len(rec.got) != 1 || rec.got[0] != 61*time.Second {
		t.Fatalf("sleeps %v", rec.got)
	}
}

func TestJQuantsRateLimitedTwiceFails(t *testing.T) {
	t.Parallel()
	d := &seqDoer{statuses: []int{429}}
	src, rec := newSourceRec(t, d, "1111", "2222", "3333")
	_, err := src.Fetch(context.Background())
	if err == nil || codeOf(t, err) != "rate_limited" || kindOf(t, err) != contracts.Retryable {
		t.Fatalf("err %v", err)
	}
	if len(d.codes) != 2 || d.codes[0] != "11110" || d.codes[1] != "11110" {
		t.Fatalf("codes %v", d.codes)
	}
	if len(rec.got) != 1 || rec.got[0] != 61*time.Second {
		t.Fatalf("sleeps %v", rec.got)
	}
}

func TestJQuantsServerErrorNotRetried(t *testing.T) {
	t.Parallel()
	d := &fakeDoer{status: 503}
	src, rec := newSourceRec(t, d, "1111", "2222", "3333")
	if _, err := src.Fetch(context.Background()); err == nil || codeOf(t, err) != "http_status" {
		t.Fatalf("err %v", err)
	}
	if d.calls != 1 || len(rec.got) != 0 {
		t.Fatalf("calls %d sleeps %v", d.calls, rec.got)
	}
}

func TestJQuantsPacesCalls(t *testing.T) {
	t.Parallel()
	body := testdata(t, "earnings_date_empty.json")
	src, rec := newSourceRec(t, &fakeDoer{status: 200, body: body}, "1111", "2222", "3333")
	if _, err := src.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(rec.got) != 2 || rec.got[0] != 12*time.Second || rec.got[1] != 12*time.Second {
		t.Fatalf("sleeps %v", rec.got)
	}
	one, rec1 := newSourceRec(t, &fakeDoer{status: 200, body: body}, "1111")
	if _, err := one.Fetch(context.Background()); err != nil || len(rec1.got) != 0 {
		t.Fatalf("err %v sleeps %v", err, rec1.got)
	}
}

func TestJQuantsSleepCancelled(t *testing.T) {
	t.Parallel()
	d := &fakeDoer{status: 200, body: testdata(t, "earnings_date_empty.json")}
	src, rec := newSourceRec(t, d, "1111", "2222", "3333")
	rec.err = context.Canceled
	_, err := src.Fetch(context.Background())
	if err == nil || codeOf(t, err) != "network" || kindOf(t, err) != contracts.Retryable {
		t.Fatalf("err %v", err)
	}
	if d.calls != 1 {
		t.Fatalf("calls %d", d.calls)
	}
}

func TestJQuantsRealSleepCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := realSleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
}
