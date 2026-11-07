package fredseries

import (
	"strings"
	"testing"
	"time"
)

const windowHead = "version = 1\nid = \"fred_series\"\nseries = [\"DGS10\"]\n"

func TestWindowOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		table string
		cal   string
		close time.Duration
	}{
		{"full", "[window]\nrule = \"after_close\"\nlateness_hours = 24\ncalendar = \"XNYS\"\n", "XNYS", 24 * time.Hour},
		{"no calendar", "[window]\nrule = \"after_close\"\nlateness_hours = 24\n", "", 24 * time.Hour},
		{"no lateness", "[window]\nrule = \"after_close\"\ncalendar = \"XNYS\"\n", "XNYS", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m, err := parseManifest([]byte(windowHead + tc.table))
			if err != nil {
				t.Fatal(err)
			}
			cal, open, closeOff, err := windowOf(m)
			if err != nil {
				t.Fatal(err)
			}
			if cal != tc.cal || open != 0 || closeOff != tc.close {
				t.Fatalf("got (%q, %v, %v)", cal, open, closeOff)
			}
		})
	}
}

func TestWindowErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		table string
		key   string
	}{
		{"missing table", "", "rule"},
		{"before_open", "[window]\nrule = \"before_open\"\n", "rule"},
		{"empty rule", "[window]\nrule = \"\"\n", "rule"},
		{"negative lateness", "[window]\nrule = \"after_close\"\nlateness_hours = -1\n", "lateness_hours"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := []byte(windowHead + tc.table)
			m, err := parseManifest(data)
			if err != nil {
				t.Fatal(err)
			}
			_, _, _, err = windowOf(m)
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("windowOf err = %v", err)
			}
			for _, v := range []string{"before_open", "-1"} {
				if strings.Contains(err.Error(), v) {
					t.Fatalf("error carries value %q: %v", v, err)
				}
			}
			if _, err := build(data, &fakeSource{}, &fakeWriter{}, fakeClock{}); err == nil {
				t.Fatal("New accepted the manifest")
			}
		})
	}
}

func TestEmbeddedWindow(t *testing.T) {
	t.Parallel()
	j, err := New(&fakeSource{}, &fakeWriter{}, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	w, ok := j.(interface {
		Window() (string, time.Duration, time.Duration)
	})
	if !ok {
		t.Fatal("job has no Window method")
	}
	cal, open, closeOff := w.Window()
	if cal != "XNYS" || open != 0 || closeOff != 96*time.Hour {
		t.Fatalf("got (%q, %v, %v)", cal, open, closeOff)
	}
}

func TestWindowCoversNextSession(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	j, err := New(&fakeSource{}, &fakeWriter{}, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	w, ok := j.(interface {
		Window() (string, time.Duration, time.Duration)
	})
	if !ok {
		t.Fatal("job has no Window method")
	}
	_, _, off := w.Window()
	at := func(m time.Month, d, h int) time.Time { return time.Date(2026, m, d, h, 0, 0, 0, loc) }
	cases := []struct {
		name       string
		sess, next time.Time
	}{
		{"friday to monday", at(time.November, 6, 16), at(time.November, 9, 16)},
		{"mlk weekend", at(time.January, 16, 16), at(time.January, 20, 16)},
		{"labor day weekend", at(time.September, 4, 16), at(time.September, 8, 16)},
		{"good friday", at(time.April, 2, 16), at(time.April, 6, 16)},
		{"thanksgiving early close", at(time.November, 25, 16), at(time.November, 27, 13)},
	}
	var longest time.Duration
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.sess.Add(off).Before(tc.next) {
				t.Fatalf("window closes at %v, before next session close %v", tc.sess.Add(off), tc.next)
			}
		})
		if g := tc.next.Sub(tc.sess); g > longest {
			longest = g
		}
	}
	if off < longest {
		t.Fatalf("offset %v is shorter than the longest gap %v", off, longest)
	}
	jan := cases[1]
	if !jan.sess.Add(off - time.Hour).Before(jan.next) {
		t.Fatalf("offset %v is not the smallest whole hours that holds", off)
	}
}
