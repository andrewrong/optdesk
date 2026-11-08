package dryrun_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/dryrun"
)

var _ notify.NewDryRunSenderFunc = dryrun.New

type failWriter struct{ err error }

func (f failWriter) Write([]byte) (int, error) { return 0, f.err }

func TestDryRunWritesText(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	s := dryrun.New(&buf)
	if err := s.Send(context.Background(), "two gaps\nline 2"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := s.Send(context.Background(), "again"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got, want := buf.String(), "two gaps\nline 2\nagain\n"; got != want {
		t.Fatalf("written %q, want %q", got, want)
	}
}

func TestDryRunWriterErrorIsErrSend(t *testing.T) {
	t.Parallel()
	s := dryrun.New(failWriter{err: errors.New("disk full")})
	err := s.Send(context.Background(), "x")
	if !errors.Is(err, notify.ErrSend) {
		t.Fatalf("err = %v, want notify.ErrSend", err)
	}
}
