// Command ot is the optdesk binary: the composition root. It selects the profile and
// builds the log handler through internal/profile, and dispatches to the subcommands that
// register themselves in an init().
package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/profile"
)

// app is what a subcommand receives.
type app struct {
	profile profile.Profile
	log     *slog.Logger
	stdout  io.Writer
	getenv  func(string) string
	deps    deps
}

// command runs one subcommand with the arguments after its name.
type command func(a *app, args []string) error

var commands = map[string]command{}

// register adds a subcommand; a later file of this package calls it from its init().
func register(name string, run command) {
	if _, dup := commands[name]; dup {
		panic("ot: subcommand registered twice: " + name)
	}
	commands[name] = run
}

// newApp selects the profile and builds the log handler, only through internal/profile.
func newApp(getenv func(string) string, stdout, stderr io.Writer) (*app, error) {
	var load profile.LoadFunc = profile.Load
	var wrap profile.NewLogHandlerFunc = profile.NewLogHandler
	p, err := load(getenv)
	if err != nil {
		return nil, err
	}
	h := wrap(p, slog.NewTextHandler(stderr, nil))
	return &app{profile: p, log: slog.New(h), stdout: stdout, getenv: getenv}, nil
}

func usage(stderr io.Writer) {
	names := make([]string, 0, len(commands))
	for n := range commands {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Fprintf(stderr, "usage: ot <command> [args]\ncommands: %s\n", strings.Join(names, " "))
}

// exitError is an error of a command that carries the exit code of the process; run returns that code
// instead of 1, so a command never calls os.Exit and its deferred closes run.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	a, err := newApp(getenv, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "ot:", err)
		return 2
	}
	a.deps = newDeps()
	if len(args) == 0 || commands[args[0]] == nil {
		usage(stderr)
		return 2
	}
	if err := commands[args[0]](a, args[1:]); err != nil {
		a.log.Error("command failed", "component", "ot", "err", err.Error())
		var ee *exitError
		if errors.As(err, &ee) {
			return ee.code
		}
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}
