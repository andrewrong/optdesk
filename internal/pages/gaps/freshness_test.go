package gaps

import (
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestFreshnessClosedExpectationIsClosed(t *testing.T) {
	t.Parallel()
	e := store.Expectation{DatasetID: "d", ExpectedDay: "2026-01-05", Closed: true}
	if got := Freshness(e, time.Unix(0, 0)); got != Closed {
		t.Fatalf("got %s, want %s", got, Closed)
	}
}

func TestFreshnessHolidayWithoutWindowIsUnknown(t *testing.T) {
	t.Parallel()
	// A holiday has no window row in stage 1, so the caller has no expectation to pass.
	if got := Freshness(store.Expectation{DatasetID: "d"}, time.Unix(0, 0)); got != Unknown {
		t.Fatalf("got %s, want %s", got, Unknown)
	}
}

func TestFreshnessNoExpectationIsUnknown(t *testing.T) {
	t.Parallel()
	if got := Freshness(store.Expectation{}, time.Unix(0, 0)); got != Unknown {
		t.Fatalf("got %s, want %s", got, Unknown)
	}
}

func TestFreshnessLateThenStale(t *testing.T) {
	t.Parallel()
	due := time.Date(2026, 1, 5, 22, 0, 0, 0, time.UTC)
	e := store.Expectation{DatasetID: "d", ExpectedDay: "2026-01-05", PresentDay: "2026-01-02", DueBy: due}
	if got := Freshness(e, due.Add(-time.Minute)); got != Fresh {
		t.Fatalf("before DueBy: got %s, want %s", got, Fresh)
	}
	if got := Freshness(e, due.Add(time.Minute)); got != Late {
		t.Fatalf("after DueBy: got %s, want %s", got, Late)
	}
	e.Gap = "expired"
	if got := Freshness(e, due.Add(time.Hour)); got != Stale {
		t.Fatalf("after DueBy with a gap: got %s, want %s", got, Stale)
	}
	e.PresentDay = "2026-01-05"
	if got := Freshness(e, due.Add(time.Hour)); got != Fresh {
		t.Fatalf("present on the expected day: got %s, want %s", got, Fresh)
	}
}

func TestFreshnessPresentAfterExpectedIsInvalid(t *testing.T) {
	t.Parallel()
	e := store.Expectation{DatasetID: "d", ExpectedDay: "2026-01-05", PresentDay: "2026-01-06"}
	if got := Freshness(e, time.Unix(0, 0)); got != Invalid {
		t.Fatalf("present after expected: got %s, want %s", got, Invalid)
	}
	e = store.Expectation{ExpectedDay: "2026-01-05"}
	if got := Freshness(e, time.Unix(0, 0)); got != Invalid {
		t.Fatalf("no dataset id: got %s, want %s", got, Invalid)
	}
}
