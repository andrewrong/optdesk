package profile_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/profile"
)

func TestProfileContract(t *testing.T) {
	t.Parallel()
	var load profile.LoadFunc = func(getenv func(string) string) (profile.Profile, error) {
		if getenv("OT_HOME") == "" {
			return profile.Profile{Name: "demo", LogFields: []string{"msg"}}, nil
		}
		return profile.Profile{Name: "real"}, nil
	}
	var wrap profile.NewLogHandlerFunc = func(p profile.Profile, next slog.Handler) slog.Handler {
		return next.WithAttrs(nil) // a pass-through is enough to hold the shape
	}
	p, err := load(func(string) string { return "" })
	if err != nil || p.Name != "demo" {
		t.Fatalf("profile %+v err %v", p, err)
	}
	var buf bytes.Buffer
	h := wrap(p, slog.NewTextHandler(&buf, nil))
	_ = h.Handle(context.Background(), slog.NewRecord(time.Time{}, slog.LevelInfo, "hello", 0))
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("log = %q", buf.String())
	}
}
