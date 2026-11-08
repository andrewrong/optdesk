package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/notify"
)

const (
	testToken = "test-token-SENTINEL"
	testChat  = "1000001"
)

type captured struct {
	path string
	form map[string]string
}

func server(t *testing.T, status int, body string, got chan<- captured) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		c := captured{path: r.URL.Path, form: map[string]string{}}
		for k, v := range r.MultipartForm.Value {
			c.form[k] = v[0]
		}
		if got != nil {
			got <- c
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const okBody = `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1000001,"type":"private"}}}`

func newSender(t *testing.T, o notify.TelegramOptions) notify.Sender {
	t.Helper()
	var f notify.NewTelegramSenderFunc = New
	s, err := f(testToken, testChat, o)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTelegramSendsText(t *testing.T) {
	t.Parallel()
	got := make(chan captured, 1)
	srv := server(t, 200, okBody, got)
	s := newSender(t, notify.TelegramOptions{ServerURL: srv.URL})
	if err := s.Send(context.Background(), "job missed"); err != nil {
		t.Fatal(err)
	}
	c := <-got
	if c.path != "/bot"+testToken+"/sendMessage" {
		t.Fatalf("path %q", c.path)
	}
	if c.form["text"] != "job missed" || c.form["chat_id"] != testChat {
		t.Fatalf("form %v", c.form)
	}
}

func TestTelegramNoParseMode(t *testing.T) {
	t.Parallel()
	got := make(chan captured, 1)
	srv := server(t, 200, okBody, got)
	s := newSender(t, notify.TelegramOptions{ServerURL: srv.URL})
	if err := s.Send(context.Background(), "a_b *c*"); err != nil {
		t.Fatal(err)
	}
	if _, ok := (<-got).form["parse_mode"]; ok {
		t.Fatal("parse_mode sent")
	}
}

func TestTelegramErrorHoldsNoToken(t *testing.T) {
	t.Parallel()
	srv := server(t, 400, `{"ok":false,"error_code":400,"description":"bad `+testToken+`"}`, nil)
	for name, url := range map[string]string{"status": srv.URL, "network": "http://127.0.0.1:1"} {
		s := newSender(t, notify.TelegramOptions{ServerURL: url})
		err := s.Send(context.Background(), "x")
		if !errors.Is(err, notify.ErrSend) {
			t.Fatalf("%s: %v", name, err)
		}
		for _, bad := range []string{testToken, testChat, "127.0.0.1", "http"} {
			if strings.Contains(err.Error(), bad) {
				t.Fatalf("%s: error %q holds %q", name, err, bad)
			}
		}
	}
}

func TestTelegramStatusClasses(t *testing.T) {
	t.Parallel()
	expired, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	cases := []struct {
		name, want string
		status     int
		url        string
		ctx        context.Context
	}{
		{name: "4xx", want: "4xx", status: 403},
		{name: "429", want: "4xx", status: 429},
		{name: "5xx", want: "5xx", status: 502},
		{name: "network", want: "network", url: "http://127.0.0.1:1"},
		{name: "timeout", want: "timeout", url: "http://127.0.0.1:1", ctx: expired},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			url := c.url
			if url == "" {
				url = server(t, c.status, fmt.Sprintf(`{"ok":false,"error_code":%d,"description":"d"}`, c.status), nil).URL
			}
			ctx := c.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			err := newSender(t, notify.TelegramOptions{ServerURL: url}).Send(ctx, "x")
			if !errors.Is(err, notify.ErrSend) || !strings.HasSuffix(err.Error(), ": "+c.want) {
				t.Fatalf("got %v, want class %s", err, c.want)
			}
		})
	}
}

type deadlineClient struct{ deadline chan time.Time }

func (d deadlineClient) Do(r *http.Request) (*http.Response, error) {
	dl, _ := r.Context().Deadline()
	d.deadline <- dl
	return nil, errors.New("stop")
}

func TestTelegramDeadline(t *testing.T) {
	t.Parallel()
	dc := deadlineClient{deadline: make(chan time.Time, 1)}
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return dc.Do(r) })}
	s := newSender(t, notify.TelegramOptions{ServerURL: "http://example.invalid", HTTPClient: hc})
	_ = s.Send(context.Background(), "x")
	dl := <-dc.deadline
	if left := time.Until(dl); dl.IsZero() || left > 10*time.Second || left < 5*time.Second {
		t.Fatalf("deadline %v is %v away, want about 10s", dl, time.Until(dl))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTelegramEmptyConfigRefused(t *testing.T) {
	t.Parallel()
	for _, c := range [][2]string{{"", testChat}, {testToken, ""}, {" ", " "}} {
		if s, err := New(c[0], c[1], notify.TelegramOptions{}); err == nil || s != nil {
			t.Fatalf("New(%q,%q) = %v, %v", c[0], c[1], s, err)
		}
	}
}
