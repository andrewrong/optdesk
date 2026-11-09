package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/andrewrong/optdesk/internal/compute"
)

func init() { register("alerts", alertsCmd) }

// alertsFile is the name of the rule file under $OT_HOME that replaces the embedded rules.
const alertsFile = "alerts.toml"

// alertsCmd is `ot alerts check [--file path]`: it loads a rule file, prints the rule count and each rule id,
// and opens no database. Without --file it checks what ot scheduler would load: $OT_HOME/alerts.toml when it
// exists, else the embedded rules.
func alertsCmd(a *app, args []string) error {
	const usage = "usage: ot alerts check [--file path]"
	if len(args) == 0 || args[0] != "check" {
		return errors.New(usage)
	}
	fs := flag.NewFlagSet("alerts check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("file", "", "rule file to check")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		return errors.New(usage)
	}
	rules, err := loadAlertRules(a, *file)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(a.stdout, "rules: %d\n", len(rules.Rules))
	for _, r := range rules.Rules {
		_, _ = fmt.Fprintln(a.stdout, r.ID)
	}
	return nil
}

// loadAlertRules loads the rule file at path; an empty path means $OT_HOME/alerts.toml, which may be absent
// (the embedded rules then apply). A named file that is absent is an error. A bad file's error names the key
// and the rule id and holds no value.
func loadAlertRules(a *app, path string) (compute.AlertRules, error) {
	explicit := path != ""
	if !explicit {
		home := a.getenv("OT_HOME")
		if home == "" {
			return a.deps.alertRules()
		}
		path = filepath.Join(home, alertsFile)
	}
	f, err := os.Open(path)
	switch {
	case err == nil:
	case !explicit && errors.Is(err, os.ErrNotExist):
		return a.deps.alertRules()
	default:
		return compute.AlertRules{}, fmt.Errorf("the alert rule file %s cannot be opened", alertsFile)
	}
	defer func() { _ = f.Close() }()
	rules, err := a.deps.loadAlertRules(f)
	if err != nil {
		return compute.AlertRules{}, fmt.Errorf("%s: %w", alertsFile, err)
	}
	return rules, nil
}
