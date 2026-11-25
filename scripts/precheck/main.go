// Command precheck runs the fast checks of CI (ci-go.yml and checks.yml) in the
// order of the workflows, so that an author finds the failures before the push.
//
// Usage: go run ./scripts/precheck [-fast]
//
// -fast skips the slow steps: go-arch-lint, the regeneration check and go test.
// The program never changes the work tree itself: it runs no git add, commit,
// checkout or stash. The generate.sh step may rewrite generated files, as it does
// in CI; when it fails, git status --short names the stale ones.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const maxFileSize = 1048576

// step is one check. argv is the command of the workflow; fn, when set, runs
// instead of argv (Go code over the file list). follow are commands run after
// argv; cleanTree additionally requires an empty git status --porcelain.
type step struct {
	name      string
	argv      []string
	follow    [][]string
	cleanTree bool
	skipFast  bool
	fn        func(root string, out io.Writer) error
}

// runFunc runs one step with the repository root as working directory and
// writes the output of a failure to out.
type runFunc func(s step, root string, out io.Writer) error

// buildSteps returns the step list in the order of the workflows. It runs nothing.
func buildSteps() []step {
	return []step{
		{name: "golangci-lint", argv: []string{"go", "tool", "golangci-lint", "run", "./..."}},
		{name: "generate arch files from modules.md", argv: []string{"go", "run", "./scripts/arch"}},
		{name: "size ratchet", argv: []string{"go", "run", "./scripts/ratchet"}},
		{name: "repository hygiene", argv: []string{"go", "run", "./scripts/hygiene"}},
		{name: "AGENTS.md exists", argv: []string{"test", "-s", "AGENTS.md"}},
		{name: "no CRLF in text files", fn: crlfStep},
		{name: "no file over 1 MB", fn: sizeStep},
		{name: "go-arch-lint", argv: []string{"go", "tool", "go-arch-lint", "check"}, skipFast: true},
		{
			name:      "generated code is current",
			argv:      []string{"sh", "contracts/generate.sh"},
			follow:    [][]string{{"git", "diff", "--exit-code"}},
			cleanTree: true,
			skipFast:  true,
		},
		{name: "go test", fn: goTestStep, skipFast: true},
	}
}

// runAll runs every step, prints one line per step and returns the exit status.
func runAll(steps []step, fast bool, run runFunc, root string, out io.Writer) int {
	status := 0
	for _, s := range steps {
		if fast && s.skipFast {
			fmt.Fprintf(out, "%s: SKIP\n", s.name)
			continue
		}
		var buf bytes.Buffer
		if err := run(s, root, &buf); err != nil {
			status = 1
			fmt.Fprintf(out, "%s: FAIL\n", s.name)
			fmt.Fprint(out, buf.String())
			if buf.Len() > 0 && !strings.HasSuffix(buf.String(), "\n") {
				fmt.Fprintln(out)
			}
			fmt.Fprintf(out, "error: %v\n", err)
			continue
		}
		fmt.Fprintf(out, "%s: PASS\n", s.name)
	}
	return status
}

// execStep is the runFunc of the program: it runs the commands of a step.
func execStep(s step, root string, out io.Writer) error {
	if s.fn != nil {
		return s.fn(root, out)
	}
	if err := command(root, out, s.argv); err != nil {
		return err
	}
	for _, a := range s.follow {
		if err := command(root, out, a); err != nil {
			return err
		}
	}
	if s.cleanTree {
		porcelain, err := output(root, "git", "status", "--porcelain")
		if err != nil {
			return err
		}
		if strings.TrimSpace(porcelain) != "" {
			return fmt.Errorf("untracked generated output")
		}
	}
	return nil
}

// command runs argv in root, sending its output to out. For a failing
// generated-code step main prints git status --short.
func command(root string, out io.Writer, argv []string) error {
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Dir = root
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func output(root string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, stderr.String())
	}
	return string(b), nil
}

// trackedFiles returns the paths of git ls-files.
func trackedFiles(root string) ([]string, error) {
	s, err := output(root, "git", "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(s, "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

func crlfStep(root string, out io.Writer) error {
	files, err := trackedFiles(root)
	if err != nil {
		return err
	}
	return checkCRLF(root, files, out)
}

func sizeStep(root string, out io.Writer) error {
	files, err := trackedFiles(root)
	if err != nil {
		return err
	}
	return checkSize(root, files, out)
}

// checkCRLF names every text file of files (relative to root) holding a \r.
// A file with a NUL byte is binary and skipped, as grep -I does.
func checkCRLF(root string, files []string, out io.Writer) error {
	var bad []string
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			continue // a deleted or unreadable path is not a CRLF finding
		}
		if bytes.IndexByte(b, 0) >= 0 {
			continue
		}
		if bytes.IndexByte(b, '\r') >= 0 {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	fmt.Fprintln(out, "CRLF found in:")
	for _, f := range bad {
		fmt.Fprintln(out, f)
	}
	return fmt.Errorf("%d file(s) with CRLF", len(bad))
}

// checkSize names every regular file of files over 1 MB (1048576 bytes).
func checkSize(root string, files []string, out io.Writer) error {
	var bad []string
	for _, f := range files {
		fi, err := os.Lstat(filepath.Join(root, f))
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		if fi.Size() > maxFileSize {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	fmt.Fprintln(out, "files over 1 MB:")
	for _, f := range bad {
		fmt.Fprintln(out, f)
	}
	return fmt.Errorf("%d file(s) over 1 MB", len(bad))
}

// wideFiles change every package when they change, as in the test job of ci-go.yml.
var wideFiles = []string{"go.mod", "go.sum", ".golangci.yml", "docs/design/modules.md"}

// affectedPackages returns the go test arguments for the changed paths: the
// packages with a changed .go file and every package that imports one of them
// (deps is the output of go list -f '{{.ImportPath}} {{join .Deps " "}} ...'),
// or ./... when a wide file or a file under contracts/ changed.
func affectedPackages(mod string, changed []string, deps string) []string {
	dirs := map[string]bool{}
	for _, c := range changed {
		for _, w := range wideFiles {
			if c == w {
				return []string{"./..."}
			}
		}
		if strings.HasPrefix(c, "contracts/") {
			return []string{"./..."}
		}
		if strings.HasSuffix(c, ".go") {
			d := filepath.ToSlash(filepath.Dir(c))
			if d == "." {
				dirs[mod] = true
			} else {
				dirs[mod+"/"+d] = true
			}
		}
	}
	set := map[string]bool{}
	for _, line := range strings.Split(deps, "\n") {
		fields := strings.Fields(line)
		for _, f := range fields {
			if dirs[f] {
				set["."+strings.TrimPrefix(fields[0], mod)] = true
				break
			}
		}
	}
	pkgs := make([]string, 0, len(set))
	for p := range set {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	return pkgs
}

func goTestStep(root string, out io.Writer) error {
	mod, err := output(root, "go", "list", "-m")
	if err != nil {
		return err
	}
	mod = strings.TrimSpace(mod)
	diff, err := output(root, "git", "diff", "--name-only", "origin/main...HEAD")
	if err != nil {
		return err
	}
	changed := strings.Fields(diff)
	deps := ""
	if !wide(changed) {
		deps, err = output(root, "go", "list", "-f",
			`{{.ImportPath}} {{join .Deps " "}} {{join .TestImports " "}} {{join .XTestImports " "}}`, "./...")
		if err != nil {
			return err
		}
	}
	pkgs := affectedPackages(mod, changed, deps)
	if len(pkgs) == 0 {
		fmt.Fprintln(out, "no affected package")
		return nil
	}
	return command(root, out, append([]string{"go", "test"}, pkgs...))
}

func wide(changed []string) bool {
	return len(affectedPackages("", changed, "")) == 1 && affectedPackages("", changed, "")[0] == "./..."
}

func repoRoot() (string, error) {
	s, err := output(".", "git", "rev-parse", "--show-toplevel")
	return strings.TrimSpace(s), err
}

func main() {
	fast := flag.Bool("fast", false, "skip go-arch-lint, the regeneration check and go test")
	flag.Parse()
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "precheck:", err)
		os.Exit(2)
	}
	run := func(s step, root string, out io.Writer) error {
		err := execStep(s, root, out)
		if err != nil && s.cleanTree {
			// show the stale generated files; restore nothing
			_ = command(root, out, []string{"git", "status", "--short"})
		}
		return err
	}
	os.Exit(runAll(buildSteps(), *fast, run, root, os.Stdout))
}
