package optionopen

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPullSendsSixtyPerCall(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	var ts []Target
	for i := range 130 {
		ts = append(ts, target(1, 10+i, ClassPuts, 95-float64(i)*0.01, Put))
	}
	c := newCore(b)
	c.Chunk = 60
	c.Pull(context.Background(), ts)
	var sizes []int
	for _, cs := range b.optCalls {
		sizes = append(sizes, len(cs))
	}
	if len(sizes) != 3 || sizes[0] != 60 || sizes[1] != 60 || sizes[2] != 10 {
		t.Errorf("calls %v, want [60 60 10]", sizes)
	}
}

func TestDeferredItemsCarryTheirCodeInTheSummary(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.step = 5 * time.Minute
	c := newCore(b)
	c.Budget = 8 * time.Minute
	c.Chunk = 2
	var ts []Target
	for i := range 10 {
		ts = append(ts, target(1, 10+i, ClassPuts, 95-float64(i)*0.1, Put))
	}
	r := c.Pull(context.Background(), ts)
	deferred := 0
	for _, row := range r.Rows {
		if row.Status == stDeferred {
			deferred++
			if code(row) != "BUDGET" {
				t.Errorf("deferred item holds code %q, want BUDGET", code(row))
			}
		}
	}
	if deferred == 0 || deferred == len(ts) {
		t.Fatalf("deferred %d of %d, want some but not all", deferred, len(ts))
	}
	if r.Summary.Deferred["BUDGET"] != deferred {
		t.Errorf("summary deferred %v, want BUDGET=%d", r.Summary.Deferred, deferred)
	}
	if line := r.Summary.String(); !strings.Contains(line, "deferred_BUDGET="+strconv.Itoa(deferred)) {
		t.Errorf("line %q does not name deferred_BUDGET=%d", line, deferred)
	}
}
