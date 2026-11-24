package barsrefresh

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

type atClock struct{ t time.Time }

func (c atClock) Now() time.Time { return c.t }

// fakeCal makes every weekday a session that closes at closeHour UTC (US) or 06:00 UTC (JP); early maps a
// day to its early close.
type fakeCal struct{ early map[contracts.Day]time.Time }

func (f fakeCal) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return clock.Session{}, err
	}
	s := clock.Session{CalendarID: id, Day: day, IsSession: d.Weekday() != time.Saturday && d.Weekday() != time.Sunday}
	s.CloseAt = d.Add(21 * time.Hour)
	if id == "XTKS" {
		s.CloseAt = d.Add(6 * time.Hour)
	}
	if e, ok := f.early[day]; ok {
		s.CloseAt, s.EarlyClose = e, true
	}
	return s, nil
}

func (fakeCal) NextSession(context.Context, string, contracts.Day) (clock.Session, error) {
	return clock.Session{}, errors.New("unused")
}

type fakeBarSource struct {
	mu    sync.Mutex
	calls []sources.BarRequest
	fn    func(sources.BarRequest) (sources.BarResult, error)
}

func (f *fakeBarSource) DailyBars(_ context.Context, req sources.BarRequest) (sources.BarResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	f.mu.Unlock()
	return f.fn(req)
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

var errDeadline = newErr(contracts.Retryable, "history deadline exceeded")

func yb(day, close string) sources.RefBar {
	return sources.RefBar{Day: contracts.Day(day), Open: close, High: close, Low: close, Close: close, Volume: "1000"}
}

// buildFB builds a job with the fallback; the weekly step is off unless a test turns it on.
func buildFB(
	t *testing.T, now time.Time, cal clock.Calendar, h *fakeHistory, st bar.PriceBarWriter, src sources.BarSource, entries ...jobs.WatchEntry,
) (*job, *syncBuf) {
	t.Helper()
	rd, _ := st.(bar.PriceBarReader)
	j, err := newJob(&fakeBroker{listed: true}, h, fakeResolver{}, st, rd, entries, atClock{now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WithFallback(j, src, cal); err != nil {
		t.Fatal(err)
	}
	jb := j.(*job)
	jb.m.AdjustedCadenceDays = 0
	buf := &syncBuf{}
	jb.log = slog.New(slog.NewTextHandler(buf, nil))
	return jb, buf
}

var closed = time.Date(2026, 11, 9, 21, 40, 0, 0, time.UTC)

func countQueries(h *fakeHistory, conID int64) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, q := range h.queries {
		if q.Contract.ConID == conID {
			n++
		}
	}
	return n
}

func yfRow(id int, day, close string) contracts.PriceBar {
	r := stored(id, day, close, 1)
	r.Source = "yfinance"
	return r
}

func TestWithFallbackRefusals(t *testing.T) {
	t.Parallel()
	j, _ := buildFB(t, closed, fakeCal{}, &fakeHistory{}, &memBars{}, &fakeBarSource{}, entry("cik:1", "AAA", 101))
	if _, err := WithFallback(j, nil, fakeCal{}); err == nil {
		t.Fatal("nil source")
	}
	if _, err := WithFallback(j, &fakeBarSource{}, nil); err == nil {
		t.Fatal("nil calendar")
	}
	if _, err := WithFallback(otherJob{}, &fakeBarSource{}, fakeCal{}); err == nil {
		t.Fatal("job of another package")
	}
}

type otherJob struct{ contracts.Job }
