package yfinance

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

// TestHelperProcess is the child the Exec tests re-execute: the test binary with "--" and a mode.
func TestHelperProcess(t *testing.T) {
	t.Parallel()
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		return // an ordinary run of the test binary
	}
	switch args[1] {
	case "echo":
		in, _ := io.ReadAll(os.Stdin)
		_, _ = os.Stdout.WriteString(strings.ToUpper(string(in)))
		os.Exit(3)
	case "block":
		w, err := os.OpenFile(args[2], os.O_WRONLY, 0)
		if err != nil {
			os.Exit(4)
		}
		_ = w.Close()
		<-time.After(time.Hour)
	}
}

func helper(mode ...string) []string {
	return append([]string{os.Args[0], "-test.run=^TestHelperProcess$", "--"}, mode...)
}

func TestExecStdinStdout(t *testing.T) {
	t.Parallel()
	out, code, err := Exec(context.Background(), helper("echo"), []byte("abc"))
	if err != nil || code != 3 || string(out) != "ABC" {
		t.Fatalf("out, code, err = %q, %d, %v", out, code, err)
	}
	if _, _, err := Exec(context.Background(), []string{filepath.Join(t.TempDir(), "absent")}, nil); err == nil {
		t.Fatal("a command that cannot start returned no error")
	}
}

func TestExecDeadlineKillsChild(t *testing.T) {
	t.Parallel()
	fifo := filepath.Join(t.TempDir(), "ready")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	fake := clockwork.NewFakeClock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-fake.After(time.Minute)
		cancel()
	}()
	go func() {
		// opening for read returns once the child has opened the pipe: it is running
		if r, err := os.Open(fifo); err == nil {
			_ = r.Close()
		}
		_ = fake.BlockUntilContext(context.Background(), 1)
		fake.Advance(time.Minute)
	}()
	_, code, err := Exec(ctx, helper("block", fifo), nil)
	if err == nil {
		t.Fatalf("a killed child returned exit code %d and no error", code)
	}
}
