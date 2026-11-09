package alphavantage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

const testKey = "SECRETKEY123"

// fakeDoer counts calls and answers each with the same status and body, or a transport error.
type fakeDoer struct {
	calls  atomic.Int32
	status int
	body   string
	err    error
	last   *http.Request
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.calls.Add(1)
	f.last = req
	if f.err != nil {
		return nil, f.err
	}
	return &http.Response{StatusCode: f.status, Body: io.NopCloser(strings.NewReader(f.body))}, nil
}

func newTestSource(t *testing.T, d sources.HTTPDoer) sources.EarningsSource {
	t.Helper()
	s, err := New(sources.EarningsConfig{BaseURL: "http://av.invalid", APIKey: testKey, Doer: d})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func readTestdata(t *testing.T, name string) string {
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

func TestParseByHeaderName(t *testing.T) {
	t.Parallel()
	got, err := newTestSource(t, &fakeDoer{status: 200, body: readTestdata(t, "calendar.csv")}).Fetch(context.Background())
	if err != nil || len(got) != 520 {
		t.Fatalf("len %d err %v", len(got), err)
	}
	want := sources.EarningsListing{Symbol: "T001", ReportDate: "2026-11-11", FiscalPeriodEnd: "2026-09-30", Estimate: "0.11"}
	if got[1] != want {
		t.Fatalf("got %+v want %+v", got[1], want)
	}
	if got[0].Estimate != "" {
		t.Fatalf("empty estimate became %q", got[0].Estimate)
	}
	// Same rows, columns in another order with an extra column: found by name.
	var b strings.Builder
	b.WriteString("currency,timeOfTheDay,estimate,fiscalDateEnding,reportDate,name,symbol\n")
	for i := 0; i < 500; i++ {
		b.WriteString("USD,pre-market,1.50,2026-09-30,2026-12-01,Name,S" + string(rune('A'+i%26)) + "\n")
	}
	got, err = newTestSource(t, &fakeDoer{status: 200, body: b.String()}).Fetch(context.Background())
	if err != nil || len(got) != 500 {
		t.Fatalf("reordered: len %d err %v", len(got), err)
	}
	want = sources.EarningsListing{Symbol: "SA", ReportDate: "2026-12-01", FiscalPeriodEnd: "2026-09-30", Estimate: "1.50", TimeOfDay: "pre-market"}
	if got[0] != want {
		t.Fatalf("got %+v want %+v", got[0], want)
	}
	// A header without the required names is not a calendar.
	_, err = newTestSource(t, &fakeDoer{status: 200, body: "a,b,c\n1,2,3\n"}).Fetch(context.Background())
	if err == nil || codeOf(t, err) != "not_csv" {
		t.Fatalf("err %v", err)
	}
}

func TestNoticeBodyIsError(t *testing.T) {
	t.Parallel()
	_, err := newTestSource(t, &fakeDoer{status: 200, body: readTestdata(t, "notice.json")}).Fetch(context.Background())
	if err == nil || codeOf(t, err) != "rate_limited" {
		t.Fatalf("err %v", err)
	}
	var te contracts.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("not typed: %v", err)
	}
	// A body that is neither csv nor a notice.
	_, err = newTestSource(t, &fakeDoer{status: 200, body: "<html>nope</html>"}).Fetch(context.Background())
	if err == nil || codeOf(t, err) != "not_csv" {
		t.Fatalf("err %v", err)
	}
}

func TestShortCalendarIsError(t *testing.T) {
	t.Parallel()
	body := "symbol,name,reportDate,fiscalDateEnding,estimate,currency\n"
	for i := 0; i < 499; i++ {
		body += "AAA,N,2026-11-10,2026-09-30,1,USD\n"
	}
	_, err := newTestSource(t, &fakeDoer{status: 200, body: body}).Fetch(context.Background())
	if err == nil || codeOf(t, err) != "short_calendar" {
		t.Fatalf("499 rows: %v", err)
	}
	_, err = newTestSource(t, &fakeDoer{status: 200, body: body + "AAA,N,2026-11-10,2026-09-30,1,USD\n"}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("500 rows: %v", err)
	}
}

func TestKeyNeverInError(t *testing.T) {
	t.Parallel()
	cases := map[string]*fakeDoer{
		"transport": {err: errors.New(`Get "http://av.invalid/query?apikey=` + testKey + `": dial failed`)},
		"status":    {status: 500, body: "error " + testKey},
		"bad body":  {status: 200, body: "<" + testKey},
		"bad csv":   {status: 200, body: "symbol,reportDate\n\"" + testKey + "\n"},
	}
	for name, d := range cases {
		_, err := newTestSource(t, d).Fetch(context.Background())
		if err == nil || strings.Contains(err.Error(), testKey) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	d := &fakeDoer{status: 200, body: readTestdata(t, "calendar.csv")}
	if _, err := newTestSource(t, d).Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	q := d.last.URL.Query()
	if q.Get("apikey") != testKey || q.Get("function") != "EARNINGS_CALENDAR" || q.Get("horizon") != "3month" ||
		d.last.Header.Get("Authorization") != "" {
		t.Fatalf("request %v", d.last.URL.Redacted())
	}
}

func TestOneCallPerFetch(t *testing.T) {
	t.Parallel()
	for name, d := range map[string]*fakeDoer{
		"ok":        {status: 200, body: readTestdata(t, "calendar.csv")},
		"5xx":       {status: 503},
		"redirect":  {status: 302},
		"transport": {err: errors.New("boom")},
	} {
		_, _ = newTestSource(t, d).Fetch(context.Background())
		if n := d.calls.Load(); n != 1 {
			t.Fatalf("%s: %d calls", name, n)
		}
	}
}

func TestNoticeIsRateLimitedNoRetry(t *testing.T) {
	t.Parallel()
	d := &fakeDoer{status: 200, body: readTestdata(t, "notice.json")}
	_, err := newTestSource(t, d).Fetch(context.Background())
	if err == nil || codeOf(t, err) != "rate_limited" {
		t.Fatalf("err %v", err)
	}
	if n := d.calls.Load(); n != 1 {
		t.Fatalf("%d calls", n)
	}
	d429 := &fakeDoer{status: 429}
	_, err = newTestSource(t, d429).Fetch(context.Background())
	if err == nil || codeOf(t, err) != "rate_limited" || d429.calls.Load() != 1 {
		t.Fatalf("429: %v, %d calls", err, d429.calls.Load())
	}
}

func TestNewValidates(t *testing.T) {
	t.Parallel()
	for _, c := range []sources.EarningsConfig{{APIKey: "k"}, {BaseURL: "http://x"}} {
		if _, err := New(c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
}
