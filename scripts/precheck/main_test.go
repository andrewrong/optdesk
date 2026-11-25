package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeRun(failIdx map[int]bool, ran *[]string) runFunc {
	steps := buildSteps()
	idx := map[string]int{}
	for i, s := range steps {
		idx[s.name] = i
	}
	return func(s step, _ string, out io.Writer) error {
		*ran = append(*ran, s.name)
		if failIdx[idx[s.name]] {
			_, _ = io.WriteString(out, "boom of "+s.name+"\n")
			return errors.New("failed")
		}
		return nil
	}
}

func TestStepsMatchWorkflows(t *testing.T) {
	t.Parallel()
	var lines []string
	for _, f := range []string{"ci-go.yml", "checks.yml"} {
		b, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", f))
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(string(b), "\n") {
			l = strings.TrimSpace(l)
			// a command is a one-line run: or a line of a run: | block
			lines = append(lines, strings.TrimPrefix(l, "run: "))
		}
	}
	checked := 0
	for _, s := range buildSteps() {
		if len(s.argv) < 2 {
			continue
		}
		cmd := strings.Join(s.argv, " ")
		if !strings.HasPrefix(cmd, "go tool ") && !strings.HasPrefix(cmd, "go run ./scripts/") && !strings.HasPrefix(cmd, "sh contracts/") {
			continue
		}
		checked++
		found := false
		for _, l := range lines {
			if l == cmd {
				found = true
			}
		}
		if !found {
			t.Errorf("step %q: %q is not a run: line of a workflow", s.name, cmd)
		}
	}
	if checked < 5 {
		t.Fatalf("only %d commands checked", checked)
	}
}

func TestRunAllReportsEveryFailure(t *testing.T) {
	t.Parallel()
	var ran []string
	var out bytes.Buffer
	steps := buildSteps()
	status := runAll(steps, false, fakeRun(map[int]bool{1: true, 3: true}, &ran), "", &out)
	if status == 0 {
		t.Fatal("status zero with failures")
	}
	if len(ran) != len(steps) {
		t.Fatalf("ran %d of %d steps", len(ran), len(steps))
	}
	for _, i := range []int{1, 3} {
		if !strings.Contains(out.String(), steps[i].name+": FAIL\n") {
			t.Errorf("no FAIL line for %q:\n%s", steps[i].name, out.String())
		}
	}
	if !strings.Contains(out.String(), "boom of "+steps[1].name) {
		t.Error("output of the failed step not printed")
	}
}

func TestRunAllPass(t *testing.T) {
	t.Parallel()
	var ran []string
	var out bytes.Buffer
	steps := buildSteps()
	if status := runAll(steps, false, fakeRun(nil, &ran), "", &out); status != 0 {
		t.Fatalf("status %d", status)
	}
	if n := strings.Count(out.String(), ": PASS\n"); n != len(steps) {
		t.Fatalf("%d PASS lines, want %d", n, len(steps))
	}
}

func TestFastSkipsSlowSteps(t *testing.T) {
	t.Parallel()
	var ran []string
	var out bytes.Buffer
	if status := runAll(buildSteps(), true, fakeRun(nil, &ran), "", &out); status != 0 {
		t.Fatalf("status %d", status)
	}
	for _, name := range []string{"go-arch-lint", "generated code is current", "go test"} {
		if !strings.Contains(out.String(), name+": SKIP\n") {
			t.Errorf("%q not SKIP:\n%s", name, out.String())
		}
		for _, r := range ran {
			if r == name {
				t.Errorf("%q was run", name)
			}
		}
	}
}

func TestCRLFCheck(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write(t, dir, "bad.txt", []byte("a\r\nb\n"))
	write(t, dir, "good.txt", []byte("a\nb\n"))
	var out bytes.Buffer
	if err := checkCRLF(dir, []string{"bad.txt", "good.txt"}, &out); err == nil {
		t.Fatal("no error")
	}
	if !strings.Contains(out.String(), "bad.txt") || strings.Contains(out.String(), "good.txt") {
		t.Fatalf("output: %q", out.String())
	}
}

func TestSizeCheck(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write(t, dir, "big.bin", make([]byte, maxFileSize+1))
	write(t, dir, "ok.bin", make([]byte, maxFileSize))
	var out bytes.Buffer
	if err := checkSize(dir, []string{"big.bin", "ok.bin"}, &out); err == nil {
		t.Fatal("no error")
	}
	if !strings.Contains(out.String(), "big.bin") || strings.Contains(out.String(), "ok.bin") {
		t.Fatalf("output: %q", out.String())
	}
}

func TestAffectedPackages(t *testing.T) {
	t.Parallel()
	deps := "m/a m/b\nm/b \nm/c m/b\nm/d \n"
	got := affectedPackages("m", []string{"b/x.go", "README.md"}, deps)
	if strings.Join(got, " ") != "./a ./b ./c" {
		t.Fatalf("got %v", got)
	}
	if got := affectedPackages("m", []string{"contracts/x"}, deps); len(got) != 1 || got[0] != "./..." {
		t.Fatalf("got %v", got)
	}
}

func write(t *testing.T, dir, name string, b []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
		t.Fatal(err)
	}
}
