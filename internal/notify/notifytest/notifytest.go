// Package notifytest holds the fakes for tests of the notify packages: a Sender and a GapSource that
// record their calls. It is a test kit and is never imported by cmd/ot.
package notifytest

import (
	"context"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

// Sender is a fake notify.Sender. It records every text it is asked to send, also a failed one.
type Sender struct {
	mu    sync.Mutex
	err   error
	texts []string
	// OnSend, when set, runs inside each Send before it returns (to observe state at send time).
	OnSend func()
}

// SetErr makes the next Sends return err; nil makes them succeed.
func (s *Sender) SetErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// Send records text and returns the configured error.
func (s *Sender) Send(_ context.Context, text string) error {
	s.mu.Lock()
	hook := s.OnSend
	s.texts = append(s.texts, text)
	err := s.err
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
	return err
}

// Texts returns a copy of the texts sent so far.
func (s *Sender) Texts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.texts...)
}

// Source is a fake notify.GapSource over a slice of gaps.
type Source struct {
	mu      sync.Mutex
	gaps    []store.Gap
	openErr error
	markErr error
	marked  []store.Gap
	markAt  []time.Time
}

// SetGaps replaces the open gaps the source returns.
func (s *Source) SetGaps(gaps ...store.Gap) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gaps = append([]store.Gap(nil), gaps...)
}

// SetOpenErr makes OpenGaps fail with err; nil clears it.
func (s *Source) SetOpenErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.openErr = err
}

// SetMarkErr makes MarkGapReported fail with err; nil clears it.
func (s *Source) SetMarkErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markErr = err
}

// OpenGaps returns a copy of the gaps as set, with ReportedAt as marked so far.
func (s *Source) OpenGaps(context.Context) ([]store.Gap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.openErr != nil {
		return nil, s.openErr
	}
	return append([]store.Gap(nil), s.gaps...), nil
}

// MarkGapReported records the call; on success it sets ReportedAt of the gap with the same key.
func (s *Source) MarkGapReported(_ context.Context, g store.Gap, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.markErr != nil {
		return s.markErr
	}
	s.marked = append(s.marked, g)
	s.markAt = append(s.markAt, at)
	for i := range s.gaps {
		o := s.gaps[i]
		if o.JobID == g.JobID && o.DatasetID == g.DatasetID && o.Day == g.Day && o.Reason == g.Reason {
			s.gaps[i].ReportedAt = at
		}
	}
	return nil
}

// Marked returns the gaps marked reported so far, in call order.
func (s *Source) Marked() []store.Gap {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.Gap(nil), s.marked...)
}

// MarkTimes returns the times passed to the successful marks, in call order.
func (s *Source) MarkTimes() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Time(nil), s.markAt...)
}
