package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
)

func init() {
	register("check", func(a *app, args []string) error { return runCheck(a, args, checkServices) })
}

const checkUsage = "usage: ot check scheduler|serve|pyside [--net] [--repo-dir dir]"

// Statuses of a Result line.
const (
	statusPass      = "PASS"
	statusFail      = "FAIL"
	statusExplained = "EXPLAINED"
)

// allowedReasons are the five reasons an EXPLAINED line may name (docs/design/local-e2e.md Decision 4b).
var allowedReasons = []string{
	"window_open",     // the window of the dataset has not closed
	"closed_day",      // the view says the day has no session
	"non_session_day", // a weekend day, or a day with no row between two days with rows
	"no_view",         // no reader gives the fact
	"offline",         // a check that needs --net and --net is not passed
}

// Result is one line of the output: `PASS|FAIL|EXPLAINED <check> <as-of day> <reason>`.
type Result struct {
	Status string
	Check  string
	Day    string // the data day the line is about; empty prints "-"
	Reason string
}

// report writes r as one line. An EXPLAINED line whose reason does not start with one of allowedReasons is
// rejected and nothing is written; a status that is not PASS, FAIL or EXPLAINED is rejected too.
func report(w io.Writer, r Result) error {
	switch r.Status {
	case statusPass, statusFail:
	case statusExplained:
		if !explainedReasonAllowed(r.Reason) {
			return fmt.Errorf("check %s: EXPLAINED reason %q is not one of allowedReasons", r.Check, r.Reason)
		}
	default:
		return fmt.Errorf("check %s: unknown status %q", r.Check, r.Status)
	}
	day := r.Day
	if day == "" {
		day = "-"
	}
	_, err := fmt.Fprintf(w, "%s %s %s %s\n", r.Status, r.Check, day, r.Reason)
	return err
}

// explainedReasonAllowed holds when reason is one of allowedReasons, optionally followed by ": <detail>"
// or a space and a detail.
func explainedReasonAllowed(reason string) bool {
	for _, ok := range allowedReasons {
		if reason == ok || strings.HasPrefix(reason, ok+":") || strings.HasPrefix(reason, ok+" ") {
			return true
		}
	}
	return false
}

// checkOpts is what every service function receives.
type checkOpts struct {
	Net bool // --net: the checks that call the public network run

	// RepoDir is --repo-dir (default "."): the checkout whose contracts/schemas the serve check loads.
	RepoDir string
}

// checker collects the lines of a run and remembers whether one was FAIL.
type checker struct {
	w      io.Writer
	failed bool
}

// emit writes r; an invalid line (see report) becomes a FAIL line so that a wrong EXPLAINED never reads green.
func (c *checker) emit(r Result) {
	if err := report(c.w, r); err != nil {
		r = Result{Status: statusFail, Check: r.Check, Day: r.Day, Reason: "invalid result line"}
		_ = report(c.w, r)
	}
	if r.Status == statusFail {
		c.failed = true
	}
}

// checkEnv is the environment of one service function.
type checkEnv struct {
	a    *app
	opts checkOpts
	c    *checker
}

// checkService runs the checks of one service and emits their lines.
type checkService func(ctx context.Context, e *checkEnv)

// checkServices are the services of `ot check`; issues 2 to 5 of docs/design/local-e2e.md replace the entries.
var checkServices = map[string]checkService{
	"scheduler": notYetChecked("scheduler"),
	"serve":     notYetChecked("serve"),
	"pyside":    notYetChecked("pyside"),
}

// notYetChecked is the hook a later issue fills: until then the service is a FAIL line, never a silent PASS.
func notYetChecked(name string) checkService {
	return func(_ context.Context, e *checkEnv) {
		e.c.emit(Result{Status: statusFail, Check: name, Reason: "no checks written for this service yet"})
	}
}

// runCheck is `ot check <service> [--net]`.
func runCheck(a *app, args []string, services map[string]checkService) error {
	if len(args) == 0 {
		return errors.New(checkUsage)
	}
	svc, ok := services[args[0]]
	if !ok {
		names := make([]string, 0, len(services))
		for n := range services {
			names = append(names, n)
		}
		sort.Strings(names)
		return fmt.Errorf("unknown service %q: %s", args[0], strings.Join(names, ", "))
	}
	fs := flag.NewFlagSet("check "+args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts checkOpts
	fs.BoolVar(&opts.Net, "net", false, "run the checks that call the public network")
	fs.StringVar(&opts.RepoDir, "repo-dir", ".", "repository checkout whose contracts/schemas are loaded")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		return errors.New(checkUsage)
	}
	c := &checker{w: a.stdout}
	home := a.getenv("OT_HOME")
	fmt.Fprintf(c.w, "OT_HOME=%s\n", home)
	if home == "" {
		c.emit(Result{Status: statusFail, Check: "OT_HOME", Reason: "OT_HOME is not set"})
	}
	pg := "unknown"
	if dsn := a.getenv(store.DSNEnvPages); dsn == "" {
		c.emit(Result{Status: statusFail, Check: "pg", Reason: store.DSNEnvPages + " is not set"})
	} else if label, err := pgLabel(dsn); err != nil {
		c.emit(Result{Status: statusFail, Check: "pg", Reason: store.DSNEnvPages + " is not a URL with a host and a database"})
	} else {
		pg = label
	}
	fmt.Fprintf(c.w, "pg=%s\n", pg)
	svc(context.Background(), &checkEnv{a: a, opts: opts, c: c})
	if c.failed {
		return &exitError{code: 1, err: errors.New("check: at least one FAIL")}
	}
	return nil
}

// pgLabel is host/database of a postgres URL: no user, no password, no query, no port.
func pgLabel(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", errors.New("unparsable DSN")
	}
	db := strings.TrimPrefix(u.Path, "/")
	if u.Hostname() == "" || db == "" {
		return "", errors.New("DSN has no host or database")
	}
	return u.Hostname() + "/" + db, nil
}

// openGaps opens the gap reader as the pages role: the only DSN `ot check` reads is the line
// store.DSNEnvPages, from the process environment. The caller closes the reader. No error text holds the DSN.
func (e *checkEnv) openGaps(ctx context.Context) (store.GapReader, error) {
	dsn := e.a.getenv(store.DSNEnvPages)
	if dsn == "" {
		return nil, errors.New(store.DSNEnvPages + " is not set")
	}
	g, err := e.a.deps.openGapReader(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open gap reader with %s failed", store.DSNEnvPages)
	}
	return g, nil
}

// fredKey is the FRED key of the process environment; empty when unset. The env file is not read.
func fredKey(a *app) string { return a.getenv("FRED_API_KEY") }

// pytest runs `uv run pytest tests/live --junitxml=<junit>` in dir with env, through a.runPytest when set and
// by exec otherwise. A test replaces the field and never starts a process.
func (a *app) pytest(dir string, env []string, junit string) error {
	if a.runPytest != nil {
		return a.runPytest(dir, env, junit)
	}
	cmd := exec.CommandContext(context.Background(), "uv", "run", "pytest", "tests/live", "--junitxml="+junit)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	return cmd.Run()
}
