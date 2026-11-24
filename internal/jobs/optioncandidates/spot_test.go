package optioncandidates

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// fakeQuotes is a store.QuoteSnapshotReader over fixed rows; it keeps the arguments of the last read.
type fakeQuotes struct {
	rows     []contracts.QuoteSnapshot
	err      error
	id       int64
	from, to contracts.Day
}

func (q *fakeQuotes) QuoteSnapshots(_ context.Context, id int64, from, to contracts.Day) ([]contracts.QuoteSnapshot, error) {
	q.id, q.from, q.to = id, from, to
	return q.rows, q.err
}

func snap(d, at string, last *float64, status int) contracts.QuoteSnapshot {
	t, _ := time.Parse(time.RFC3339, at)
	return contracts.QuoteSnapshot{SessionDay: day(d), ReceivedAt: t, Last: last, Status: status}
}

func TestSpotFromQuote(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)
	rows := []contracts.QuoteSnapshot{
		snap("2026-11-05", "2026-11-05T20:00:00Z", f64(90), 1),
		snap("2026-11-06", "2026-11-06T15:00:00Z", f64(95.5), 1),
		snap("2026-11-06", "2026-11-06T20:30:00Z", f64(96.25), 1),
		snap("2026-11-06", "2026-11-06T21:00:00Z", nil, 1),
	}
	q := &fakeQuotes{rows: rows}
	price, d, _, ok, err := spotFromQuote(context.Background(), q, 7, "2026-11-09", now)
	if err != nil || !ok || price != "96.25" || d != "2026-11-06" {
		t.Fatalf("got %q %s %v %v", price, d, ok, err)
	}
	if q.id != 7 || q.from != "2026-11-02" || q.to != "2026-11-09" {
		t.Fatalf("read %d %s %s", q.id, q.from, q.to)
	}

	// a row received after the clock is not taken; a row whose status is not ok is not taken
	q.rows = append(rows,
		snap("2026-11-09", "2026-11-09T14:00:01Z", f64(99), 1),
		snap("2026-11-09", "2026-11-09T13:00:00Z", f64(98), 4))
	price, d, _, ok, _ = spotFromQuote(context.Background(), q, 7, "2026-11-09", now)
	if !ok || price != "96.25" || d != "2026-11-06" {
		t.Fatalf("after the clock: %q %s %v", price, d, ok)
	}

	q.rows = nil
	if _, _, _, ok, err := spotFromQuote(context.Background(), q, 7, "2026-11-09", now); ok || err != nil {
		t.Fatalf("no rows: %v %v", ok, err)
	}
	q.err = errors.New("down")
	if _, _, _, ok, err := spotFromQuote(context.Background(), q, 7, "2026-11-09", now); ok || err == nil {
		t.Fatalf("error: %v %v", ok, err)
	}
}

func openAt(d, at string, price *float64) contracts.OptionOpen {
	t, _ := time.Parse(time.RFC3339, at)
	return contracts.OptionOpen{SessionDay: day(d), ReceivedAt: t, UnderlyingPrice: price}
}

func TestOpenSpot(t *testing.T) {
	t.Parallel()
	rows := []contracts.OptionOpen{
		openAt("2026-11-09", "2026-11-09T14:31:00Z", f64(101)),
		openAt("2026-11-09", "2026-11-09T14:35:00Z", f64(102.5)),
		openAt("2026-11-09", "2026-11-09T14:40:00Z", nil),
		openAt("2026-11-06", "2026-11-06T20:59:00Z", f64(90)),
	}
	price, at, ok := openSpot(rows, "2026-11-09")
	if !ok || price != "102.5" || !at.Equal(time.Date(2026, 11, 9, 14, 35, 0, 0, time.UTC)) {
		t.Fatalf("got %q %v %v", price, at, ok)
	}
	if _, _, ok := openSpot(rows[3:], "2026-11-09"); ok {
		t.Fatal("row of the day before taken")
	}
	if _, _, ok := openSpot(rows[2:3], "2026-11-09"); ok {
		t.Fatal("row without price taken")
	}
	if _, _, ok := openSpot(nil, "2026-11-09"); ok {
		t.Fatal("no rows gave a price")
	}
}
