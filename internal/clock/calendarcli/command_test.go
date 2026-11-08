package calendarcli_test

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock/calendarcli"
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
	case "exit3":
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

func TestExecRunsCommand(t *testing.T) {
	t.Parallel()
	code, err := calendarcli.Exec(context.Background(), helper("exit3"))
	if err != nil || code != 3 {
		t.Fatalf("code, err = %d, %v", code, err)
	}
	if _, err := calendarcli.Exec(context.Background(), []string{filepath.Join(t.TempDir(), "absent")}); err == nil {
		t.Fatal("a command that cannot start returned no error")
	}
}

func TestExecKilledByContextIsRetryable(t *testing.T) {
	t.Parallel()
	fifo := filepath.Join(t.TempDir(), "ready")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// opening for read returns once the child has opened the pipe: it is running
		if r, err := os.Open(fifo); err == nil {
			_ = r.Close()
		}
		cancel()
	}()
	code, err := calendarcli.Exec(ctx, helper("block", fifo))
	if err == nil {
		t.Fatalf("a killed child returned exit code %d and no error", code)
	}
}
