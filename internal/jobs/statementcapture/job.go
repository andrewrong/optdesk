package statementcapture

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"
	// The zone database is embedded so that the New York zone loads on every machine.
	_ "time/tzdata"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/flexsection"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

const (
	datasetID  = "account.capture"
	subjectKey = "flex"
	calendarID = "XNYS"
	marketZone = "America/New_York"

	sectionPosition = "OpenPosition"
	sectionEquity   = "EquitySummaryByReportDateInBase"
	sectionNAV      = "ChangeInNAV"
	statePresent    = "present"
)

// Config is what the job needs. Aliases are the statement-sourced aliases (none is the main alias).
type Config struct {
	Aliases  []string
	Sections flexsection.Reader
	Account  store.AccountWriter
	Marks    mark.Writer
	Calendar clock.Calendar
	Clock    clock.Clock
}

type job struct {
	m       manifest
	aliases []string
	cfg     Config
	loc     *time.Location
	log     *slog.Logger
}

// New builds statement_capture from the embedded job.toml. Without Aliases, Sections, Account, Marks, Calendar or
// Clock it returns an error.
func New(cfg Config) (contracts.Job, error) {
	return newJob(cfg, slog.Default())
}

func newJob(cfg Config, log *slog.Logger) (*job, error) {
	if len(cfg.Aliases) == 0 || cfg.Sections == nil || cfg.Account == nil || cfg.Marks == nil || cfg.Calendar == nil || cfg.Clock == nil {
		return nil, errors.New("statementcapture: aliases, sections, account, marks, calendar and clock are required")
	}
	m, err := parseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(marketZone)
	if err != nil {
		return nil, err
	}
	aliases := append([]string(nil), cfg.Aliases...)
	sort.Strings(aliases)
	return &job{m: m, aliases: aliases, cfg: cfg, loc: loc, log: log}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendar, anchor, zone string, offset time.Duration) {
	return j.m.Schedule.Calendar, j.m.Schedule.Anchor, "", j.m.offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.m.runBound }

// RestartSensitive is true when a restart inside the window must be avoided.
func (j *job) RestartSensitive() bool { return j.m.Schedule.RestartSensitive }

// Plan expects one key per day; the hash covers the job, the day and the aliases.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: subjectKey}}
	sum := sha256.Sum256([]byte(strings.Join([]string{j.m.ID, string(day), strings.Join(j.aliases, ",")}, "\n")))
	return keys, hex.EncodeToString(sum[:]), nil
}

type manifest struct {
	Version  int    `toml:"version"`
	ID       string `toml:"id"`
	Schedule struct {
		Calendar         string `toml:"calendar"`
		Anchor           string `toml:"anchor"`
		Offset           string `toml:"offset"`
		RunBound         string `toml:"run_bound"`
		RestartSensitive bool   `toml:"restart_sensitive"`
	} `toml:"schedule"`

	offset, runBound time.Duration
}

//go:embed job.toml
var manifestBytes []byte

func parseManifest(data []byte) (manifest, error) {
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return manifest{}, errors.New("statementcapture: manifest: not valid toml")
	}
	if m.ID == "" || m.Schedule.Calendar == "" || m.Schedule.Anchor != "session_close" {
		return manifest{}, errors.New("statementcapture: manifest: id, schedule.calendar and a session_close schedule.anchor are required")
	}
	var err error
	if m.offset, err = time.ParseDuration(m.Schedule.Offset); err != nil {
		return manifest{}, errors.New("statementcapture: manifest: schedule.offset is not a duration")
	}
	if m.runBound, err = time.ParseDuration(m.Schedule.RunBound); err != nil {
		return manifest{}, errors.New("statementcapture: manifest: schedule.run_bound is not a duration")
	}
	return m, nil
}

// retryable wraps a store error; the text is a fixed word, never the store's message.
func retryable(step string) error {
	return &typedError{kind: contracts.Retryable, text: "statementcapture: " + step + " failed"}
}

// firstError keeps the first error of a run; a Retryable one replaces a Permanent one.
func firstError(cur contracts.TypedError, err error) contracts.TypedError {
	te, ok := err.(contracts.TypedError) //nolint:errorlint // the job builds only typedError values
	if !ok {
		return cur
	}
	if cur == nil || (cur.Kind() == contracts.Permanent && te.Kind() == contracts.Retryable) {
		return te
	}
	return cur
}

// Run captures every alias. Returned counts the session report days tried and Usable those written.
func (j *job) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	if day == "" {
		day = contracts.Day(j.cfg.Clock.Now().In(j.loc).Format(time.DateOnly))
	}
	var res contracts.JobResult
	for _, alias := range j.aliases {
		r := j.runAlias(ctx, alias, string(day))
		res.Returned += r.Returned
		res.Usable += r.Usable
		if r.Err != nil {
			res.Err = firstError(res.Err, r.Err)
		}
	}
	return res
}

// source is what an alias has stored, decoded.
type source struct {
	positions map[string][]row
	equity    map[string]row
	nav       []row
}

// decodeRows reads the rows of a section; refusing is the error of a row whose report date has another form.
func (j *job) decodeRows(ctx context.Context, alias, section string, byDate bool) (map[string][]row, []row, error) {
	rows, err := j.cfg.Sections.Rows(ctx, alias, section)
	if err != nil {
		return nil, nil, retryable("read " + section)
	}
	byDay := map[string][]row{}
	var all []row
	for _, r := range rows {
		a, derr := decodeAttrs(r.Attributes)
		if derr != nil {
			return nil, nil, refuse("-", section, "row is not a JSON object")
		}
		x := row{key: r.RowKey, at: r.RecordedAt, a: a}
		if !byDate {
			all = append(all, x)
			continue
		}
		d, err := x.date("-", "reportDate")
		if err != nil {
			return nil, nil, err
		}
		byDay[d] = append(byDay[d], x)
	}
	return byDay, all, nil
}

func (j *job) load(ctx context.Context, alias string) (source, error) {
	var s source
	var err error
	if s.positions, _, err = j.decodeRows(ctx, alias, sectionPosition, true); err != nil {
		return s, err
	}
	eq, _, err := j.decodeRows(ctx, alias, sectionEquity, true)
	if err != nil {
		return s, err
	}
	s.equity = map[string]row{}
	for d, rs := range eq {
		sort.Slice(rs, func(a, b int) bool { return rs[a].key < rs[b].key })
		s.equity[d] = rs[0]
	}
	_, s.nav, err = j.decodeRows(ctx, alias, sectionNAV, false)
	return s, err
}

// present is the reason a day cannot be written from the statuses, or "" when both sections are present.
func (j *job) absentReason(ctx context.Context, alias string) (string, error) {
	sts, err := j.cfg.Sections.Statuses(ctx, alias)
	if err != nil {
		return "", retryable("read statuses")
	}
	state := map[string]string{}
	for _, s := range sts {
		state[s.Section] = s.State
	}
	for _, name := range []string{sectionPosition, sectionEquity} {
		if state[name] != statePresent {
			return name + " is not present", nil
		}
	}
	return "", nil
}

// tally is what one alias run counted.
type tally struct {
	written, notSession, noEquity, noPositions, refused int
	skipped                                             skipped
}

// days lists the report days up to runDay, newest first.
func (s source) days(runDay string) []string {
	set := map[string]bool{}
	for d := range s.positions {
		set[d] = true
	}
	for d := range s.equity {
		set[d] = true
	}
	var out []string
	for d := range set {
		if d <= runDay {
			out = append(out, d)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

func (j *job) runAlias(ctx context.Context, alias, runDay string) contracts.JobResult {
	var res contracts.JobResult
	reason, err := j.absentReason(ctx, alias)
	if err == nil && reason != "" {
		j.log.Info("statement capture left out", "alias", alias, "reason", reason)
		return res
	}
	var src source
	if err == nil {
		src, err = j.load(ctx, alias)
	}
	if err != nil {
		j.logError(alias, err)
		res.Err = firstError(nil, err)
		return res
	}
	var t tally
	for _, d := range src.days(runDay) {
		ok, derr := j.runDay(ctx, alias, d, src, &t, &res)
		if derr != nil {
			j.logError(alias, derr)
			res.Err = firstError(res.Err, derr)
		}
		if ok {
			res.Usable++
		}
	}
	j.log.Info("statement capture", "alias", alias, "written", t.written, "not_session_day", t.notSession,
		"no_equity_row", t.noEquity, "no_position_row", t.noPositions, "refused", t.refused,
		"skipped_category", t.skipped.category, "skipped_level", t.skipped.level, "skipped_zero_position", t.skipped.zero)
	return res
}

func (j *job) logError(alias string, err error) {
	j.log.Warn("statement capture failed", "alias", alias, "error", err.Error())
}

// runDay writes one report day; the bool is true when its capture and marks were written.
func (j *job) runDay(ctx context.Context, alias, d string, src source, t *tally, res *contracts.JobResult) (bool, error) {
	sess, err := j.cfg.Calendar.Session(ctx, calendarID, contracts.Day(d))
	if err != nil {
		return false, retryable("calendar")
	}
	if !sess.IsSession {
		t.notSession++
		return false, nil
	}
	pos, eq := src.positions[d], src.equity[d]
	switch {
	case eq.a == nil:
		t.noEquity++
		return false, nil
	case len(pos) == 0:
		t.noPositions++
		return false, nil
	}
	res.Returned++
	base, err := baseCurrency(d, src.nav)
	var md mappedDay
	if err == nil {
		md, err = mapDay(d, pos, eq, base)
	}
	t.skipped.category += md.skipped.category
	t.skipped.level += md.skipped.level
	t.skipped.zero += md.skipped.zero
	if err != nil {
		t.refused++
		return false, err
	}
	if err := j.write(ctx, alias, d, newest(pos, eq), md); err != nil {
		return false, err
	}
	t.written++
	return true, nil
}

// newest is the newest RecordedAt of the source rows of a day.
func newest(pos []row, eq row) time.Time {
	at := eq.at
	for _, r := range pos {
		if r.at.After(at) {
			at = r.at
		}
	}
	return at
}

// write stores the capture and then its marks.
func (j *job) write(ctx context.Context, alias, d string, at time.Time, md mappedDay) error {
	id := alias + ":" + d + ":" + subjectKey
	c := store.Capture{ID: id, Alias: alias, At: at, SessionDay: d, Status: "ok", Values: md.values,
		Positions: make([]store.Position, 0, len(md.positions))}
	marks := make([]mark.PositionMark, 0, len(md.positions))
	for _, p := range md.positions {
		c.Positions = append(c.Positions, p.pos)
		marks = append(marks, mark.PositionMark{
			CaptureID: id, Alias: alias, MarkedAt: at, SessionDay: d, ConID: p.pos.ConID, Attempt: 1,
			Currency: p.pos.Currency, Price: p.mark, Basis: mark.BasisClose, Close: p.mark, Status: "ok",
		})
	}
	if err := j.cfg.Account.WriteCapture(ctx, c); err != nil {
		return retryable("write capture")
	}
	if len(marks) == 0 {
		return nil
	}
	if err := j.cfg.Marks.WriteMarks(ctx, marks); err != nil {
		return retryable("write marks")
	}
	return nil
}
