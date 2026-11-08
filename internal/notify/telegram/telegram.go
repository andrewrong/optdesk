package telegram

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/go-telegram/bot"
)

// sendDeadline bounds one call; it is taken from the context given to Send.
const sendDeadline = 10 * time.Second

// Status classes of a failed delivery; they are the only detail an error carries.
const (
	class4xx     = "4xx"
	class5xx     = "5xx"
	classNetwork = "network"
	classTimeout = "timeout"
)

// sendError is notify.ErrSend with a status class. The library's error is dropped on purpose: it
// can carry the request URL, which holds the token.
type sendError struct{ class string }

func (e *sendError) Error() string        { return notify.ErrSend.Error() + ": " + e.class }
func (e *sendError) Is(target error) bool { return target == notify.ErrSend }

// statusClient records the status of the last response; the library reports only the body's
// error code, so the HTTP class is read here.
type statusClient struct {
	inner  bot.HttpClient
	status int
}

func (c *statusClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.inner.Do(req)
	if resp != nil {
		c.status = resp.StatusCode
	}
	return resp, err
}

type sender struct {
	token, chatID string
	o             notify.TelegramOptions
}

// New is a notify.NewTelegramSenderFunc. An empty token or chat id is refused.
func New(token, chatID string, o notify.TelegramOptions) (notify.Sender, error) {
	if strings.TrimSpace(token) == "" || strings.TrimSpace(chatID) == "" {
		return nil, errors.New("telegram: token and chat id are required")
	}
	return &sender{token: token, chatID: chatID, o: o}, nil
}

// Send posts text as one plain message (no parse mode) within sendDeadline.
func (s *sender) Send(ctx context.Context, text string) error {
	ctx, cancel := context.WithTimeout(ctx, sendDeadline)
	defer cancel()

	var inner bot.HttpClient = http.DefaultClient
	if s.o.HTTPClient != nil {
		inner = s.o.HTTPClient
	}
	sc := &statusClient{inner: inner}
	opts := []bot.Option{bot.WithSkipGetMe(), bot.WithHTTPClient(sendDeadline, sc)}
	if s.o.ServerURL != "" {
		opts = append(opts, bot.WithServerURL(s.o.ServerURL))
	}
	b, err := bot.New(s.token, opts...)
	if err != nil {
		return &sendError{class: classNetwork}
	}
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: s.chatID, Text: text}); err != nil {
		return &sendError{class: classify(ctx, sc.status)}
	}
	return nil
}

func classify(ctx context.Context, status int) string {
	switch {
	case status >= 500:
		return class5xx
	case status >= 400:
		return class4xx
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return classTimeout
	default:
		return classNetwork
	}
}
