package yfinance

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

// CommandFunc runs argv with stdin as its standard input to its end and returns its standard output
// and exit code. err is non-nil only when the command could not be run or waited for, or the context
// ended; then no exit code is meant.
type CommandFunc func(ctx context.Context, argv []string, stdin []byte) (stdout []byte, exitCode int, err error)

// Exec is the real CommandFunc. When the context is done after the child ended, it returns the context
// error and no exit code, so a child killed at its deadline is a failure to wait, not an exit code.
func Exec(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
	if len(argv) == 0 {
		return nil, -1, errors.New("yfinance: empty command")
	}
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = &out
	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, -1, ctxErr
	}
	if err == nil {
		return out.Bytes(), 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out.Bytes(), exit.ExitCode(), nil
	}
	return nil, -1, err
}
