package shared

import (
	"context"
	"errors"
	"testing"
)

func TestErrOrderedStopIsTheCauseOfAnOrderedCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(ErrOrderedStop)
	if !errors.Is(context.Cause(ctx), ErrOrderedStop) {
		t.Fatalf("cause = %v", context.Cause(ctx))
	}
	plain, cancelPlain := context.WithCancel(context.Background())
	cancelPlain()
	if errors.Is(context.Cause(plain), ErrOrderedStop) {
		t.Fatal("a plain cancel carries the ordered stop")
	}
}
