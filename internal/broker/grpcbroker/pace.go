package grpcbroker

import "context"

// gate limits how many calls run at once; a waiting call leaves when its context ends.
type gate chan struct{}

func newGate(n int) gate { return make(gate, n) }

func (g gate) acquire(ctx context.Context) error {
	select {
	case g <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g gate) release() { <-g }
