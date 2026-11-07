package quotesnapshot

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestStatusMapping(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		q      broker.Quote
		status int
		code   string // "" is nil
	}{
		{"ok", broker.Quote{Status: broker.StatusOK}, 1, ""},
		{"partial", broker.Quote{Status: broker.StatusPartial}, 1, ""},
		{"no data", broker.Quote{Status: broker.StatusNoData}, 2, ""},
		{"no subscription code", broker.Quote{Status: broker.StatusNoSubscription, ErrorCode: 10089}, 5, "10089"},
		{"no subscription name", broker.Quote{Status: broker.StatusNoSubscription}, 5, "NO_SUBSCRIPTION"},
		{"timeout", broker.Quote{Status: broker.StatusTimeout}, 5, "TIMEOUT"},
		{"error", broker.Quote{Status: broker.StatusError}, 5, "ERROR"},
		{"error code", broker.Quote{Status: broker.StatusError, ErrorCode: 200}, 5, "200"},
		{"disconnected", broker.Quote{Status: broker.StatusDisconnected}, 5, "DISCONNECTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := mapQuote(tc.q, day, "0945", "cap", "USD", 7)
			if r.Status != tc.status {
				t.Fatalf("status = %d, want %d", r.Status, tc.status)
			}
			if tc.code == "" && r.ErrorCode != nil || tc.code != "" && (r.ErrorCode == nil || *r.ErrorCode != tc.code) {
				t.Fatalf("error_code = %v, want %q", r.ErrorCode, tc.code)
			}
			if r.InstrumentID != 7 || r.Slot != "0945" || r.Currency != "USD" || r.CaptureID != "cap" {
				t.Fatalf("row = %+v", r)
			}
		})
	}
}

func TestWriterError(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{err: errors.New("db down")}
	j := newJob(t, "", &fakeBroker{}, &fakeResolver{}, w, "2026-11-05 09:50")
	res := j.Run(context.Background(), "2026-11-05", []contracts.ExpectedKey{slotKey("0945")})
	if res.Usable != 0 || res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("res = %+v", res)
	}
}

func TestResolverError(t *testing.T) {
	t.Parallel()
	for _, kind := range []contracts.ErrorKind{contracts.Retryable, contracts.Permanent} {
		w := &fakeWriter{}
		b := &fakeBroker{}
		r := &fakeResolver{err: &typedError{kind: kind, err: errors.New("resolver")}}
		j := newJob(t, "", b, r, w, "2026-11-05 09:50")
		res := j.Run(context.Background(), "2026-11-05", []contracts.ExpectedKey{slotKey("0945")})
		if res.Err == nil || res.Err.Kind() != kind {
			t.Fatalf("%s: res = %+v", kind, res)
		}
		if len(w.calls) != 0 {
			t.Fatalf("%s: chunk written despite the resolver error", kind)
		}
	}
}
