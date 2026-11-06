package profile_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/profile"
)

var _ profile.NewLogHandlerFunc = profile.NewLogHandler

func logTo(t *testing.T, fn func(*slog.Logger)) string {
	t.Helper()
	p, err := profile.Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	fn(slog.New(profile.NewLogHandler(p, slog.NewTextHandler(&buf, nil))))
	return buf.String()
}

func TestLoggingDropsRawID(t *testing.T) {
	t.Parallel()
	id := "U" + "1234567"
	out := logTo(t, func(l *slog.Logger) {
		l.With("account", id).Info("hello", "account", id, "job", "fred", slog.Group("g", "owner", id, "rows", 3))
	})
	if strings.Contains(out, id) || strings.Contains(out, "account") || strings.Contains(out, "owner") {
		t.Fatalf("raw value passed: %s", out)
	}
	for _, want := range []string{"hello", "job=fred", "g.rows=3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("%q missing from %s", want, out)
		}
	}
}

func TestLoggingRedactsIDInWhitelistedField(t *testing.T) {
	t.Parallel()
	id := "DU" + "123456"
	out := logTo(t, func(l *slog.Logger) { l.Info("x", "err", "failed for "+id) })
	if strings.Contains(out, id) || !strings.Contains(out, "[redacted]") {
		t.Fatalf("id not redacted: %s", out)
	}
}
