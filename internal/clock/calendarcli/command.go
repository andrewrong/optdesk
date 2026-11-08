package calendarcli

import (
	"context"
	"errors"
	"os/exec"
)

// Exec is the real clock.CommandFunc: it runs argv to its end with its output discarded and
// returns the exit code. When the context is done after the child ended, it returns an error and
// no exit code, so a child killed at its deadline is a failure to wait, not an exit code.
func Exec(ctx context.Context, argv []string) (int, error) {
	if len(argv) == 0 {
		return -1, errors.New("calendarcli: empty command")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return -1, ctxErr
	}
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return -1, err
}
