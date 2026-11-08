package dryrun

import (
	"context"
	"io"

	"github.com/andrewrong/optdesk/internal/notify"
)

type sender struct{ w io.Writer }

// New returns a notify.Sender that writes each text and one newline to w. It needs no
// configuration and fails only when w fails, with notify.ErrSend.
func New(w io.Writer) notify.Sender { return sender{w: w} }

// Send writes text and a newline in one Write call.
func (s sender) Send(_ context.Context, text string) error {
	if _, err := io.WriteString(s.w, text+"\n"); err != nil {
		return notify.ErrSend
	}
	return nil
}
