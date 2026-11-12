package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
)

func TestRedactErrText(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, in, want string }{
		{"dsn", "dial postgres://u:pw@h/db failed", "dial [redacted] failed"},
		{"userinfo url", "get https://user:pw@host/x", "get [redacted]"},
		{"token pair", "bad token=abc123 here", "bad token=[redacted] here"},
		{"authorization", "Authorization: Bearer abc.def", "Authorization: [redacted]"},
		{"control", "a\x00\n\tb", "a b"},
		{"plain", "boom", "boom"},
	} {
		if got := redactErrText(tc.in); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	long := strings.Repeat("é", 200)
	got := redactErrText(long)
	if len(got) != 300 || !strings.HasPrefix(long, got) {
		t.Fatalf("len %d", len(got))
	}
	if got := redactErrText(strings.Repeat("x", 301)); len(got) != 300 {
		t.Fatalf("len %d", len(got))
	}
}

func TestOutcomeOf(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   runner.KeyRun
		want keyOutcome
	}{
		{"already present", runner.KeyRun{Group: runner.GroupAlreadyPresent, Usable: 9}, keyOutcome{outcome: "present"}},
		{"ran present", runner.KeyRun{Group: runner.GroupRan, Mark: "present", Usable: 4}, keyOutcome{outcome: "written", rows: 4}},
		{"ran no_value", runner.KeyRun{Group: runner.GroupRan, Mark: "no_value"}, keyOutcome{outcome: "no_value", reason: "no_value"}},
		{"ran no_value reason", runner.KeyRun{Group: runner.GroupRan, Mark: "no_value", ReasonCode: "r"}, keyOutcome{outcome: "no_value", reason: "r"}},
		{"ran failed", runner.KeyRun{Group: runner.GroupRan, ErrorCode: "e"}, keyOutcome{outcome: "failed", reason: "e"}},
		{"ran before close", runner.KeyRun{Group: runner.GroupRan, ReasonCode: "empty_before_close"}, keyOutcome{outcome: "skipped", reason: "before_close"}},
		{"rerun present", runner.KeyRun{Group: runner.GroupRerun, Mark: "present", Usable: 2}, keyOutcome{outcome: "written", rows: 2}},
		{"not due", runner.KeyRun{Group: runner.GroupNotDue}, keyOutcome{outcome: "skipped", reason: "not_due"}},
		{"lease held", runner.KeyRun{Group: runner.GroupLeaseHeld}, keyOutcome{outcome: "skipped", reason: "lease_held"}},
		{"not reached", runner.KeyRun{Group: runner.GroupNotReached}, keyOutcome{outcome: "skipped", reason: "not_reached"}},
	} {
		if got := outcomeOf(tc.in); got != tc.want {
			t.Errorf("%s: %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func finished(l *slog.Logger, dataset, subject string, errText string) {
	attrs := []any{"dataset", dataset, "subject", subject}
	if errText != "" {
		attrs = append(attrs, "error", errText)
	}
	l.Info("job run finished", attrs...)
}

func TestKeyReportRecordsLastAttemptAndPassesOn(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	h := newKeyReport(slog.NewTextHandler(&buf, nil))
	l := slog.New(h).With("component", "x")
	finished(l, "d", "s", "first")
	finished(l, "d", "s", "second")
	finished(l, "d", "other", "o")
	l.Info("something else", "dataset", "d", "subject", "s", "error", "ignored")
	got := h.recorded()
	if got[keyID{"d", "s"}] != "second" || got[keyID{"d", "other"}] != "o" || len(got) != 2 {
		t.Fatalf("recorded %v", got)
	}
	if n := strings.Count(buf.String(), "\n"); n != 4 || !strings.Contains(buf.String(), "error=first") {
		t.Fatalf("passed on %d lines: %q", n, buf.String())
	}
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("not enabled")
	}
}

func TestPrintKeyLines(t *testing.T) {
	t.Parallel()
	k := func(d, s string) contracts.ExpectedKey { return contracts.ExpectedKey{DatasetID: d, SubjectKey: s} }
	keys := []runner.KeyRun{
		{Key: k("a", "1"), Group: runner.GroupRan, Mark: "no_value"},
		{Key: k("a", "2"), Group: runner.GroupRan, ErrorCode: "e"},
		{Key: k("a", "3"), Group: runner.GroupRan, Mark: "present", Usable: 7},
	}
	texts := map[keyID]string{{"a", "1"}: "after stop_after", {"a", "2"}: "dial postgres://u:pw@h/db", {"a", "3"}: "old"}
	var buf bytes.Buffer
	c := printKeyLines(&buf, "2026-11-04", keys, []contracts.ExpectedKey{k("a", "1"), k("b", "9")}, texts)
	want := `key day=2026-11-04 dataset=a subject=1 outcome=no_value rows=0 reason=no_value
key day=2026-11-04 dataset=a subject=2 outcome=failed rows=0 reason=e error="dial [redacted]"
key day=2026-11-04 dataset=a subject=3 outcome=written rows=7
key day=2026-11-04 dataset=b subject=9 outcome=skipped rows=0 reason=in_window
`
	if buf.String() != want {
		t.Fatalf("got\n%s\nwant\n%s", buf.String(), want)
	}
	if line := statusLine("2026-11-04", c); line != "backfill 2026-11-04 status=ok written=1 present=0 no_value=1 failed=1 skipped=1" {
		t.Fatal(line)
	}
}

func TestStatusLineNothingWritten(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	keys := []runner.KeyRun{
		{Group: runner.GroupRan, Mark: "no_value"},
		{Group: runner.GroupRan, Mark: "no_value"},
	}
	c := printKeyLines(&buf, "2026-11-04", keys, nil, nil)
	if line := statusLine("2026-11-04", c); line != "backfill 2026-11-04 status=nothing_written written=0 present=0 no_value=2 failed=0 skipped=0" {
		t.Fatal(line)
	}
}

func TestNoValueKeyPrintsNoErrorText(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	keys := []runner.KeyRun{{Key: contracts.ExpectedKey{DatasetID: "a", SubjectKey: "1"}, Group: runner.GroupRan, Mark: "no_value", ReasonCode: "r"}}
	printKeyLines(&buf, "2026-11-04", keys, nil, map[keyID]string{{"a", "1"}: "after stop_after"})
	if strings.Contains(buf.String(), "error=") {
		t.Fatalf("no_value line carries error text: %q", buf.String())
	}
}
