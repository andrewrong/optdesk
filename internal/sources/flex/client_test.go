package flex

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/sources"
)

const testToken = "faketokenvalue"

type fakeTransport struct {
	mu    sync.Mutex
	reqs  []*http.Request
	reply func(n int, r *http.Request) *http.Response
}

func (f *fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r)
	n := len(f.reqs)
	f.mu.Unlock()
	resp := f.reply(n, r)
	if resp == nil {
		return nil, errors.New("dial " + r.URL.String())
	}
	resp.Request = r
	return resp, nil
}

func (f *fakeTransport) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

func respond(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

type waits struct {
	mu sync.Mutex
	ds []time.Duration
}

func (w *waits) wait(_ context.Context, d time.Duration) error {
	w.mu.Lock()
	w.ds = append(w.ds, d)
	w.mu.Unlock()
	return nil
}

func newTest(t *testing.T, tr *fakeTransport) (*client, *waits) {
	t.Helper()
	w := &waits{}
	c, err := newClient(sources.FlexConfig{Token: testToken, Wait: w.wait, Clock: clockwork.NewFakeClock()},
		withTransport(tr), withPacer(&pacer{}))
	if err != nil {
		t.Fatal(err)
	}
	return c, w
}

var req = sources.FlexRequest{QueryID: "7654321", First: "2026-10-01", Last: "2026-10-31"}

func TestFetchTradesFromFixture(t *testing.T) {
	t.Parallel()
	send, stmt := string(fixture(t, "send_ok.xml")), string(fixture(t, "statement_trades.xml"))
	tr := &fakeTransport{reply: func(n int, _ *http.Request) *http.Response {
		if n == 1 {
			return respond(200, send)
		}
		return respond(200, stmt)
	}}
	c, w := newTest(t, tr)
	rep, err := c.Fetch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Statements) != 1 || len(rep.Statements[0].Trades) != 2 || len(rep.RawSHA256) != 64 || rep.FetchedAt.IsZero() {
		t.Fatalf("report %+v", rep)
	}
	if tr.count() != 2 {
		t.Fatalf("requests %d", tr.count())
	}
	r0, r1 := tr.reqs[0].URL, tr.reqs[1].URL
	if r0.Scheme+"://"+r0.Host != flexHost || r0.Path != sendRequest || r1.Path != getStatement {
		t.Fatalf("urls %v %v", r0, r1)
	}
	q := r0.Query()
	if q.Get("t") != testToken || q.Get("q") != "7654321" || q.Get("v") != "3" || q.Get("fd") != "20261001" || q.Get("td") != "20261031" {
		t.Fatalf("send query %v", q)
	}
	if q1 := r1.Query(); q1.Get("q") != "1234567890" || q1.Get("v") != "3" {
		t.Fatalf("get query %v", q1)
	}
	// Second request waits the 7 seconds of the pacer.
	if len(w.ds) != 1 || w.ds[0] != 7*time.Second {
		t.Fatalf("waits %v", w.ds)
	}
}

func TestPollsOnlyOnNotReady(t *testing.T) {
	t.Parallel()
	send, notReady, stmt := string(fixture(t, "send_ok.xml")), string(fixture(t, "statement_not_ready.xml")), string(fixture(t, "statement_trades.xml"))
	tr := &fakeTransport{reply: func(n int, _ *http.Request) *http.Response {
		switch {
		case n == 1:
			return respond(200, send)
		case n < 5:
			return respond(200, notReady)
		}
		return respond(200, stmt)
	}}
	c, _ := newTest(t, tr)
	if _, err := c.Fetch(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if tr.count() != 5 {
		t.Fatalf("requests %d, want 5", tr.count())
	}
}

func TestPollLimitTen(t *testing.T) {
	t.Parallel()
	send, notReady := string(fixture(t, "send_ok.xml")), string(fixture(t, "statement_not_ready.xml"))
	tr := &fakeTransport{reply: func(n int, _ *http.Request) *http.Response {
		if n == 1 {
			return respond(200, send)
		}
		return respond(200, notReady)
	}}
	c, _ := newTest(t, tr)
	_, err := c.Fetch(context.Background(), req)
	if k, code := flexKindOf(t, err); k != kindInProgress || code != "1019" {
		t.Fatalf("err %v", err)
	}
	if tr.count() != 11 {
		t.Fatalf("requests %d, want 1 send + 10 polls", tr.count())
	}
}

func TestOtherErrorNotRepeated(t *testing.T) {
	t.Parallel()
	send, bad := string(fixture(t, "send_ok.xml")), string(fixture(t, "statement_error.xml"))
	tr := &fakeTransport{reply: func(n int, _ *http.Request) *http.Response {
		if n == 1 {
			return respond(200, send)
		}
		return respond(200, bad)
	}}
	c, _ := newTest(t, tr)
	_, err := c.Fetch(context.Background(), req)
	if k, code := flexKindOf(t, err); k != kindFlexError || code != "1012" {
		t.Fatalf("err %v", err)
	}
	if tr.count() != 2 {
		t.Fatalf("requests %d, want 2", tr.count())
	}
}

func TestNoRedirectFollowed(t *testing.T) {
	t.Parallel()
	tr := &fakeTransport{reply: func(int, *http.Request) *http.Response {
		r := respond(302, "")
		r.Header.Set("Location", "https://elsewhere.invalid/x")
		return r
	}}
	c, _ := newTest(t, tr)
	_, err := c.Fetch(context.Background(), req)
	if k, code := flexKindOf(t, err); k != kindHTTPStatus || code != "302" {
		t.Fatalf("err %v", err)
	}
	if tr.count() != 1 {
		t.Fatalf("requests %d, want 1", tr.count())
	}
}

func TestBodyOverLimitRefused(t *testing.T) {
	t.Parallel()
	tr := &fakeTransport{reply: func(int, *http.Request) *http.Response {
		return respond(200, strings.Repeat("a", maxBody+1))
	}}
	c, _ := newTest(t, tr)
	_, err := c.Fetch(context.Background(), req)
	if k, _ := flexKindOf(t, err); k != kindTooLarge {
		t.Fatalf("err %v", err)
	}
}

func TestErrorsHoldNoToken(t *testing.T) {
	t.Parallel()
	tr := &fakeTransport{reply: func(int, *http.Request) *http.Response { return nil }}
	c, _ := newTest(t, tr)
	_, err := c.Fetch(context.Background(), req)
	if k, _ := flexKindOf(t, err); k != kindNetwork {
		t.Fatalf("err %v", err)
	}
	for _, e := range []error{err, failCode(kindHTTPStatus, "500"), failCode(kindFlexError, "1012")} {
		for _, secret := range []string{testToken, req.QueryID, "interactivebrokers", "https:"} {
			if strings.Contains(e.Error(), secret) {
				t.Fatalf("%q in %q", secret, e)
			}
		}
		if errors.Unwrap(e) != nil {
			t.Fatalf("cause kept: %v", e)
		}
	}
}
