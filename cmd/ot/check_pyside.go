package main

import (
	"context"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	pysideSwitch     = "OT_LIVE_PYSIDE"
	pysideDirDefault = "pyside"
)

func init() {
	// check.go registers "check" first (files init in name order); this wrapper takes the --pyside-dir flag
	// of `ot check pyside` out of the arguments, because runCheck parses only --net.
	base := commands["check"]
	commands["check"] = func(a *app, args []string) error {
		if len(args) == 0 || args[0] != "pyside" {
			return base(a, args)
		}
		dir, rest, err := splitPysideDir(args[1:])
		if err != nil {
			return errors.New(checkUsage + " [--pyside-dir dir]")
		}
		services := map[string]checkService{"pyside": checkPyside(dir)}
		return runCheck(a, append([]string{"pyside"}, rest...), services)
	}
}

// splitPysideDir takes --pyside-dir out of args and returns it (default "pyside") with the other arguments.
func splitPysideDir(args []string) (string, []string, error) {
	fs := flag.NewFlagSet("check pyside", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("pyside-dir", pysideDirDefault, "directory of the pyside project")
	net := fs.Bool("net", false, "run the checks that call the public network")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return "", nil, errors.New("bad arguments")
	}
	var rest []string
	if *net {
		rest = append(rest, "--net")
	}
	return *dir, rest, nil
}

// junitSuites is the part of a pytest junit file this check reads.
type junitSuites struct {
	Suites []junitSuite `xml:"testsuite"`
	Cases  []junitCase  `xml:"testcase"`
}

type junitSuite struct {
	Cases []junitCase `xml:"testcase"`
}

type junitCase struct {
	Class      string       `xml:"classname,attr"`
	Name       string       `xml:"name,attr"`
	Failure    *junitDetail `xml:"failure"`
	Error      *junitDetail `xml:"error"`
	Skipped    *junitDetail `xml:"skipped"`
	Properties []struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	} `xml:"properties>property"`
}

type junitDetail struct {
	Message string `xml:"message,attr"`
}

// parseJunit returns every testcase of a junit file, whether the root is <testsuites> or <testsuite>.
func parseJunit(data []byte) ([]junitCase, error) {
	var root junitSuites
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	cases := root.Cases
	for _, s := range root.Suites {
		cases = append(cases, s.Cases...)
	}
	return cases, nil
}

// oneLine keeps a message on one line of output.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

// checkPyside runs the live pytest set of dir through a.runPytest (via a.pytest) with OT_LIVE_PYSIDE=1 and
// (a.pytest passes --junitxml=<temp file>) prints one line per test from the junit file: passed is PASS, failed or error is FAIL, skipped is FAIL.
func checkPyside(dir string) checkService {
	return func(_ context.Context, e *checkEnv) {
		tmp, err := os.MkdirTemp("", "ot-check-pyside-")
		if err != nil {
			e.c.emit(Result{Status: statusFail, Check: "pyside", Reason: "no temporary directory for the junit file"})
			return
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		junit := filepath.Join(tmp, "junit.xml")
		env := append(os.Environ(), pysideSwitch+"=1")
		for _, k := range []string{"OT_HOME", "OT_IBKR_HEALTH_PORT"} {
			if v := e.a.getenv(k); v != "" {
				env = append(env, k+"="+v)
			}
		}
		runErr := e.a.pytest(dir, env, junit) //nolint:contextcheck // the hook of check.go takes no context
		data, err := os.ReadFile(junit)
		if err != nil {
			reason := "no junit file"
			if runErr != nil {
				reason = "pytest did not run: " + oneLine(runErr.Error())
			}
			e.c.emit(Result{Status: statusFail, Check: "pyside", Reason: reason})
			return
		}
		cases, err := parseJunit(data)
		if err != nil || len(cases) == 0 {
			e.c.emit(Result{Status: statusFail, Check: "pyside", Reason: "junit file holds no test"})
			return
		}
		bad := false
		for _, c := range cases {
			r := pysideLine(c)
			bad = bad || r.Status == statusFail
			e.c.emit(r)
		}
		if runErr != nil && !bad {
			e.c.emit(Result{Status: statusFail, Check: "pyside", Reason: "pytest exited with an error: " + oneLine(runErr.Error())})
		}
	}
}

// pysideLine is the Result of one testcase.
func pysideLine(c junitCase) Result {
	day := ""
	for _, p := range c.Properties {
		if p.Name == "as_of" {
			day = p.Value
		}
	}
	r := Result{Status: statusPass, Check: "pyside." + c.Name, Day: day, Reason: "passed"}
	switch {
	case c.Failure != nil:
		r.Status, r.Reason = statusFail, "failed: "+oneLine(c.Failure.Message)
	case c.Error != nil:
		r.Status, r.Reason = statusFail, "error: "+oneLine(c.Error.Message)
	case c.Skipped != nil:
		r.Status = statusFail
		r.Reason = fmt.Sprintf("skipped (a live test must run; check %s, OT_HOME and OT_IBKR_HEALTH_PORT): %s",
			pysideSwitch, oneLine(c.Skipped.Message))
	}
	return r
}
