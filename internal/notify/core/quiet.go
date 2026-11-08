package core

import (
	"fmt"
	"time"
)

// window is the quiet window as minutes after midnight; it may cross midnight.
type window struct{ from, to int }

func parseClock(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("notify: bad quiet time %q, want HH:MM", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}

func parseWindow(from, to string) (window, error) {
	f, err := parseClock(from)
	if err != nil {
		return window{}, err
	}
	t, err := parseClock(to)
	if err != nil {
		return window{}, err
	}
	return window{from: f, to: t}, nil
}

// quiet reports whether now, read in zone, falls in [from, to). Equal ends make an empty window.
func quiet(now time.Time, zone *time.Location, w window) bool {
	l := now.In(zone)
	m := l.Hour()*60 + l.Minute()
	switch {
	case w.from == w.to:
		return false
	case w.from < w.to:
		return m >= w.from && m < w.to
	default:
		return m >= w.from || m < w.to
	}
}
