package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
)

// checkErrMax is the number of characters of an error text of the check: crewd records 200 characters of stderr.
const checkErrMax = 200

var leadingNumber = regexp.MustCompile(`^[0-9]+`)

// migrateCheckCmd is `ot migrate check --repo <clone> [--commit <sha>]`. It exits 0 only when every migration of the
// commit is applied and the commit changes nothing under internal/store/setup since the deployed build. It writes
// nothing: git is read through argv, the database through deps.migrateMissing alone.
func migrateCheckCmd(a *app, args []string) error {
	fs := flag.NewFlagSet("migrate check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repo := fs.String("repo", "", "path of the git clone that holds the commit")
	commit := fs.String("commit", "", "the held commit (default: CREW_COMMIT)")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || *repo == "" {
		return errors.New("usage: ot migrate check --repo <clone> [--commit <sha>]")
	}
	if *commit == "" {
		*commit = a.getenv("CREW_COMMIT")
	}
	bi, _ := debug.ReadBuildInfo()
	return shortCheckErr(migrateCheck(a, *repo, *commit, bi))
}

// shortCheckErr cuts the text of err to checkErrMax characters.
func shortCheckErr(err error) error {
	if err == nil {
		return nil
	}
	r := []rune(err.Error())
	if len(r) <= checkErrMax {
		return err
	}
	return errors.New(string(r[:checkErrMax]))
}

// migrateCheck returns the first failing reason, or nil. bi is the build info of the running binary.
func migrateCheck(a *app, repo, commit string, bi *debug.BuildInfo) error {
	if commit == "" {
		return errors.New("no commit: pass --commit or set CREW_COMMIT")
	}
	if strings.HasPrefix(commit, "-") {
		return errors.New("the commit is not a revision")
	}
	if err := ensureCommit(repo, commit); err != nil {
		return err
	}
	deployed, err := deployedRevision(bi)
	if err != nil {
		return err
	}
	out, err := gitOut(repo, "diff", "--name-only", deployed, commit, "--", "internal/store/setup/")
	if err != nil {
		return fmt.Errorf("git diff of internal/store/setup failed: %w", err)
	}
	if files := strings.Fields(out); len(files) > 0 {
		return fmt.Errorf("the commit changes internal/store/setup (%s): held for the hand setup step", files[0])
	}
	want, err := treeVersions(repo, commit)
	if err != nil {
		return err
	}
	if len(want) == 0 {
		return errors.New("the commit has no migration file under migrations/")
	}
	env, err := a.envFile()
	if err != nil {
		return err
	}
	dsn := env[store.DSNEnvRunner]
	if dsn == "" {
		return fmt.Errorf("%s is not set", store.DSNEnvRunner)
	}
	missing, extra, err := a.deps.migrateMissing(context.Background(), dsn, want)
	if err != nil {
		return err
	}
	if len(extra) > 0 {
		_, _ = fmt.Fprintln(a.stdout, "extra "+joinVersions(extra))
	}
	if len(missing) > 0 {
		_, _ = fmt.Fprintln(a.stdout, "missing "+joinVersions(missing))
		return fmt.Errorf("migration %s of the commit is not applied", joinVersions(missing))
	}
	return nil
}

// ensureCommit makes sure the clone holds commit: when it does not, it runs `git fetch --quiet origin` once and tests again.
func ensureCommit(repo, commit string) error {
	if _, err := gitOut(repo, "cat-file", "-e", commit+"^{commit}"); err == nil {
		return nil
	}
	_, _ = gitOut(repo, "fetch", "--quiet", "origin")
	if _, err := gitOut(repo, "cat-file", "-e", commit+"^{commit}"); err != nil {
		return fmt.Errorf("the commit %s is not in the clone, also not after git fetch origin", commit)
	}
	return nil
}

func joinVersions(v []int64) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = strconv.FormatInt(n, 10)
	}
	return strings.Join(s, ",")
}

// deployedRevision is the vcs.revision of bi; a missing or modified build has no known commit.
func deployedRevision(bi *debug.BuildInfo) (string, error) {
	if bi == nil {
		return "", errors.New("no build info: the deployed commit is unknown")
	}
	var rev, modified string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if rev == "" {
		return "", errors.New("the binary carries no vcs.revision: the deployed commit is unknown")
	}
	if modified == "true" {
		return "", errors.New("the binary was built from a modified tree (vcs.modified): the deployed commit is unknown")
	}
	return rev, nil
}

// treeVersions lists the versions of the files of migrations/ in the tree of commit.
func treeVersions(repo, commit string) ([]int64, error) {
	out, err := gitOut(repo, "ls-tree", "--name-only", commit, "migrations/")
	if err != nil {
		return nil, fmt.Errorf("git ls-tree of the commit failed: %w", err)
	}
	var vs []int64
	for line := range strings.SplitSeq(out, "\n") {
		num := leadingNumber.FindString(path.Base(strings.TrimSpace(line)))
		if num == "" {
			continue
		}
		v, err := strconv.ParseInt(num, 10, 64)
		if err != nil {
			continue
		}
		vs = append(vs, v)
	}
	slices.Sort(vs)
	return slices.Compact(vs), nil
}

// gitOut runs `git -C repo args...` as an argv, without a shell, and returns its standard output.
func gitOut(repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", fmt.Errorf("exit %d", ee.ExitCode())
		}
		return "", errors.New("git did not run")
	}
	return string(out), nil
}
