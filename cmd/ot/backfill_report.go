package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
)

var (
	reDSN          = regexp.MustCompile(`(?i)\bpostgres(?:ql)?://\S*`)
	reUserinfoURL  = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s/@]*@\S*`)
	reSecretPair   = regexp.MustCompile(`(?i)\b(password|passwd|pwd|token|secret|apikey|api_key|key)=[^\s&;]*`)
	reAuthzHeader  = regexp.MustCompile(`(?i)(authorization:\s*)(?:bearer|basic)\s+[^\s;]+`)
	reControlChars = regexp.MustCompile(`[\p{Cc}]+`)
)

const (
	redactedText = "[redacted]"
	maxErrBytes  = 300
)

// redactErrText makes the text of a job error fit for the output: a postgres DSN and a URL with userinfo,
// the value of password|passwd|pwd|token|secret|apikey|api_key|key pairs and the credential of an
// Authorization header become [redacted], control characters become one space, and the text is cut at
// 300 bytes on a rune boundary. It is the five rules of safeErrText in internal/runner/reconcile, which
// is unexported there (own implementation; the follow-up exports that function and replaces this body).
func redactErrText(s string) string {
	s = reDSN.ReplaceAllString(s, redactedText)
	s = reUserinfoURL.ReplaceAllString(s, redactedText)
	s = reSecretPair.ReplaceAllString(s, "${1}="+redactedText)
	s = reAuthzHeader.ReplaceAllString(s, "${1}"+redactedText)
	s = reControlChars.ReplaceAllString(s, " ")
	if len(s) > maxErrBytes {
		n := maxErrBytes
		for n > 0 && !utf8.RuneStart(s[n]) {
			n--
		}
		s = s[:n]
	}
	return s
}

// keyID names a key by dataset and subject, the two attributes of the runner's log line.
type keyID struct{ dataset, subject string }

// reportState is shared by a keyReport and the handlers derived from it.
type reportState struct {
	mu    sync.Mutex
	texts map[keyID]string
}

// keyReport is a slog.Handler that records the `error` attribute of each `job run finished` record by
// dataset and subject (the last attempt wins) and passes every record on to the handler it wraps.
type keyReport struct {
	next slog.Handler
	st   *reportState
	// pre holds the attributes of WithAttrs, which may name the dataset or the subject.
	pre []slog.Attr
}

func newKeyReport(next slog.Handler) *keyReport {
	return &keyReport{next: next, st: &reportState{texts: map[keyID]string{}}}
}

func (h *keyReport) Enabled(ctx context.Context, l slog.Level) bool { return h.next.Enabled(ctx, l) }

func (h *keyReport) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "job run finished" {
		var id keyID
		var text string
		visit := func(a slog.Attr) bool {
			switch a.Key {
			case "dataset":
				id.dataset = a.Value.String()
			case "subject":
				id.subject = a.Value.String()
			case "error":
				text = a.Value.String()
			}
			return true
		}
		for _, a := range h.pre {
			visit(a)
		}
		r.Attrs(visit)
		h.st.mu.Lock()
		// An attempt that logs no error clears the text of an earlier one.
		if text == "" {
			delete(h.st.texts, id)
		} else {
			h.st.texts[id] = text
		}
		h.st.mu.Unlock()
	}
	return h.next.Handle(ctx, r)
}

func (h *keyReport) WithAttrs(attrs []slog.Attr) slog.Handler {
	pre := append(append([]slog.Attr{}, h.pre...), attrs...)
	return &keyReport{next: h.next.WithAttrs(attrs), st: h.st, pre: pre}
}

func (h *keyReport) WithGroup(name string) slog.Handler {
	return &keyReport{next: h.next.WithGroup(name), st: h.st, pre: h.pre}
}

// recorded returns a copy of the recorded texts.
func (h *keyReport) recorded() map[keyID]string {
	h.st.mu.Lock()
	defer h.st.mu.Unlock()
	out := make(map[keyID]string, len(h.st.texts))
	for k, v := range h.st.texts {
		out[k] = v
	}
	return out
}

// keyOutcome is the outcome of one key of a backfill and its counters.
type keyOutcome struct {
	outcome string
	rows    int
	reason  string
}

const (
	outWritten  = "written"
	outPresent  = "present"
	outNoValue  = "no_value"
	outFailed   = "failed"
	outSkipped  = "skipped"
	markPresent = "present"
	markNoValue = "no_value"
)

// outcomeOf classifies one KeyRun.
func outcomeOf(k runner.KeyRun) keyOutcome {
	switch k.Group {
	case runner.GroupAlreadyPresent:
		return keyOutcome{outcome: outPresent}
	case runner.GroupNotDue:
		return keyOutcome{outcome: outSkipped, reason: "not_due"}
	case runner.GroupLeaseHeld:
		return keyOutcome{outcome: outSkipped, reason: "lease_held"}
	case runner.GroupNotReached:
		return keyOutcome{outcome: outSkipped, reason: "not_reached"}
	case runner.GroupRan, runner.GroupRerun:
		switch {
		case k.ErrorCode != "":
			return keyOutcome{outcome: outFailed, reason: k.ErrorCode}
		case k.Mark == markPresent:
			return keyOutcome{outcome: outWritten, rows: k.Usable}
		case k.Mark == markNoValue:
			reason := k.ReasonCode
			if reason == "" {
				reason = markNoValue
			}
			return keyOutcome{outcome: outNoValue, reason: reason}
		case k.ReasonCode != "":
			reason := k.ReasonCode
			if reason == "empty_before_close" {
				reason = "before_close"
			}
			return keyOutcome{outcome: outSkipped, reason: reason}
		}
	}
	return keyOutcome{outcome: outSkipped, reason: "not_reached"}
}

// keyCounts are the counters of the status line.
type keyCounts struct{ written, present, noValue, failed, skipped int }

func (c *keyCounts) add(outcome string) {
	switch outcome {
	case outWritten:
		c.written++
	case outPresent:
		c.present++
	case outNoValue:
		c.noValue++
	case outFailed:
		c.failed++
	default:
		c.skipped++
	}
}

func (c keyCounts) status() string {
	if c.written == 0 && c.present == 0 {
		return "nothing_written"
	}
	return "ok"
}

// printKeyLines writes one line per key of keys, then one line per key of inWindow that keys lacks, and
// returns the counts. A text of texts is printed as error="<text>" after redaction.
func printKeyLines(w io.Writer, day contracts.Day, keys []runner.KeyRun, inWindow []contracts.ExpectedKey, texts map[keyID]string) keyCounts {
	var c keyCounts
	seen := map[keyID]bool{}
	line := func(key contracts.ExpectedKey, o keyOutcome) {
		id := keyID{key.DatasetID, key.SubjectKey}
		var b strings.Builder
		fmt.Fprintf(&b, "key day=%s dataset=%s subject=%s outcome=%s rows=%d", day, id.dataset, id.subject, o.outcome, o.rows)
		if o.reason != "" {
			fmt.Fprintf(&b, " reason=%s", o.reason)
		}
		if t, ok := texts[id]; ok && t != "" && o.outcome == outFailed {
			fmt.Fprintf(&b, " error=%q", redactErrText(t))
		}
		_, _ = fmt.Fprintln(w, b.String())
		c.add(o.outcome)
	}
	for _, k := range keys {
		seen[keyID{k.Key.DatasetID, k.Key.SubjectKey}] = true
		line(k.Key, outcomeOf(k))
	}
	for _, k := range inWindow {
		if !seen[keyID{k.DatasetID, k.SubjectKey}] {
			line(k, keyOutcome{outcome: outSkipped, reason: "in_window"})
		}
	}
	return c
}

// statusLine is the last output line of ot backfill.
func statusLine(day contracts.Day, c keyCounts) string {
	return fmt.Sprintf("backfill %s status=%s written=%d present=%d no_value=%d failed=%d skipped=%d",
		day, c.status(), c.written, c.present, c.noValue, c.failed, c.skipped)
}

// installKeyReport makes a keyReport over the current default handler the slog default and returns it with
// the function that puts the previous default back.
//
// The slog default and the log writer are process-wide, so the install holds keyReportMu until the returned
// function runs: two installs at once would read each other's handler as the previous one and make the log
// package and slog call each other forever.
func installKeyReport() (*keyReport, func()) {
	keyReportMu.Lock()
	prev := slog.Default()
	report := newKeyReport(prev.Handler())
	// slog.SetDefault also points the log package at the new handler; the default handler of slog writes
	// through the log package, so the log writer is put back or the two would call each other.
	logOut, logFlags := log.Writer(), log.Flags()
	slog.SetDefault(slog.New(report))
	log.SetOutput(logOut)
	log.SetFlags(logFlags)
	return report, func() {
		slog.SetDefault(prev)
		log.SetOutput(logOut)
		log.SetFlags(logFlags)
		keyReportMu.Unlock()
	}
}

var keyReportMu sync.Mutex
