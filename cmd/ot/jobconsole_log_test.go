package main

import (
	"log/slog"
	"strings"
	"testing"

	"errors"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/profile"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// A failed console request keeps its cause when the host logs through the
// whitelisting handler of the profile, as the live scheduler does.
func TestConsoleFailedLineKeepsErrThroughWhitelist(t *testing.T) {
	t.Parallel()
	p, err := profile.Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	r := newJCRig(t)
	r.host.log = slog.New(profile.NewLogHandler(p, slog.NewTextHandler(r.out, nil)))
	r.host.set([]contracts.Job{jcJob{id: "fred_series", keys: jcKey, planFn: func() error { return errors.New("plan: boom") }}}, r.bf)
	r.add(1, "fred_series", jcDay, false)
	r.turn(jcNever)
	r.wantEnd(jobrequest.StateFailed, "plan_failed")
	got := jcErrorLines(r)
	if len(got) != 1 {
		t.Fatalf("ERROR lines %q, want one", got)
	}
	if !strings.Contains(got[0], `err="plan: boom"`) {
		t.Fatalf("line %q lacks err=\"plan: boom\"", got[0])
	}
}
