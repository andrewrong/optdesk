package datacheck

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

const (
	datasetID = "datacheck.run"
	subjectID = "check"
	dayLayout = "2006-01-02"
)

// Announcer sends one text; cmd/ot adapts notify.Sender.
type Announcer interface {
	Announce(ctx context.Context, text string) error
}

// Stores is what the job reads and writes. Announce may be nil: findings are stored, nothing is sent. Every
// other field except Expected and RuleVersions is required. RuleVersions (dataset id to expected rule version) may
// be nil: candidate_rule_version then returns the error no_expected_version.
type Stores struct {
	Cov      coverage.Reader
	Probe    finding.Probe
	Out      finding.Writer
	Expected []ExpectedDataset
	// RuleVersions is the version each dataset's writer has now; cmd/ot fills it.
	RuleVersions map[string]string
	Announce     Announcer
}

type typedError struct {
	kind contracts.ErrorKind
	code string
	err  error
}

func (e *typedError) Error() string {
	if e.err == nil {
		return "datacheck: " + e.code
	}
	return "datacheck: " + e.code + ": " + e.err.Error()
}
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func fail(kind contracts.ErrorKind, code string, err error) contracts.JobResult {
	return contracts.JobResult{Err: &typedError{kind: kind, code: code, err: err}}
}

type job struct {
	m     Manifest
	st    Stores
	clk   clock.Clock
	rules []Rule
}

// New builds the data_check job from the embedded job.toml. An error names a key, never a value.
func New(st Stores, clk clock.Clock) (contracts.Job, error) {
	return newWithRules(st, clk, defaultRules())
}

func newWithRules(st Stores, clk clock.Clock, rules []Rule) (contracts.Job, error) {
	switch {
	case st.Cov == nil:
		return nil, errors.New("datacheck: Cov is required")
	case st.Probe == nil:
		return nil, errors.New("datacheck: Probe is required")
	case st.Out == nil:
		return nil, errors.New("datacheck: Out is required")
	case clk == nil:
		return nil, errors.New("datacheck: clock is required")
	}
	m, err := DefaultManifest()
	if err != nil {
		return nil, err
	}
	return &job{m: m, st: st, clk: clk, rules: rules}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	s := j.m.Schedule
	return s.Calendar, s.Anchor, s.Zone, s.Offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.m.Schedule.RunBound }

// RestartSensitive is false: a rerun is a new run and adds rows.
func (j *job) RestartSensitive() bool { return j.m.Schedule.RestartSensitive }

// Plan returns the one key of the day, {datacheck.run, check}, and a hash of the manifest version and the day.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s", j.m.ID, j.m.Version, day)))
	return []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: subjectID}}, hex.EncodeToString(sum[:]), nil
}

// Run runs every rule over the windows, stores one run and sends one message for the new findings. A rule that
// fails becomes one unknown finding. Returned is the number of rules, Usable the rules that did not fail.
func (j *job) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	if len(j.rules) == 0 {
		return fail(contracts.Permanent, "no_rules", nil)
	}
	checkDay, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return fail(contracts.Permanent, "bad_day", nil)
	}
	started := j.clk.Now()
	windows, err := j.windows(ctx, checkDay)
	if err != nil {
		return fail(contracts.Retryable, "read_sessions", err)
	}
	in := Input{
		CheckDay: day, Now: started, Windows: windows, Cov: j.st.Cov, Probe: j.st.Probe, Expected: j.st.Expected,
		RuleVersions: j.st.RuleVersions,
	}
	findings := []finding.Finding{}
	unknown := 0
	for _, r := range j.rules {
		fs, err := r.Run(ctx, in)
		if ctx.Err() != nil {
			return fail(contracts.Retryable, "cut", ctx.Err())
		}
		if err != nil {
			unknown++
			findings = append(findings, finding.Finding{
				Rule: r.ID, Dataset: r.ID, Day: day, Severity: finding.SeverityUnknown,
				Evidence: []finding.Count{{Name: "read_failed", N: 1}},
			})
			continue
		}
		findings = append(findings, fs...)
	}
	run, err := j.st.Out.InsertRun(ctx, finding.Run{
		CheckDay: day, StartedAt: started, FinishedAt: j.clk.Now(),
		RulesTotal: len(j.rules), RulesUnknown: unknown, Findings: dedupe(findings),
	})
	if err != nil {
		return fail(contracts.Retryable, "insert_run", err)
	}
	res := contracts.JobResult{Returned: len(j.rules), Usable: len(j.rules) - unknown}
	if j.st.Announce == nil {
		slog.Info("datacheck: no announcer, findings are stored only", "run_id", run.RunID)
		return res
	}
	if text, ok := announceText(run.Findings); ok {
		if err := j.st.Announce.Announce(ctx, text); err != nil {
			return fail(contracts.Retryable, "announce", err)
		}
	}
	if err := j.st.Out.MarkNoticed(ctx, run.RunID); err != nil {
		return fail(contracts.Retryable, "mark_noticed", err)
	}
	return res
}

// windows returns, per calendar of Expected and XNYS and XTKS, the last CheckWindowSessions session days before
// checkDay, ascending.
func (j *job) windows(ctx context.Context, checkDay time.Time) (map[string][]contracts.Day, error) {
	cals := map[string]bool{"XNYS": true, "XTKS": true}
	for _, e := range j.st.Expected {
		cals[e.Calendar] = true
	}
	ids := make([]string, 0, len(cals))
	for c := range cals {
		ids = append(ids, c)
	}
	sort.Strings(ids)
	from := contracts.Day(checkDay.AddDate(0, 0, -coverage.LookbackDays).Format(dayLayout))
	to := contracts.Day(checkDay.AddDate(0, 0, -1).Format(dayLayout))
	out := make(map[string][]contracts.Day, len(ids))
	for _, c := range ids {
		ss, err := j.st.Cov.Sessions(ctx, c, from, to)
		if err != nil {
			return nil, err
		}
		var days []contracts.Day
		for _, s := range ss {
			if s.IsSession && s.Day >= from && s.Day <= to {
				days = append(days, s.Day)
			}
		}
		slices.Sort(days)
		if len(days) > CheckWindowSessions {
			days = days[len(days)-CheckWindowSessions:]
		}
		out[c] = days
	}
	return out, nil
}
