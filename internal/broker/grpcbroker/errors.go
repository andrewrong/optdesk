package grpcbroker

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// brokerError is the contracts.TypedError of this package. Its text never holds the socket path.
type brokerError struct {
	kind contracts.ErrorKind
	text string
	err  error
}

func (e *brokerError) Error() string             { return e.text }
func (e *brokerError) Kind() contracts.ErrorKind { return e.kind }
func (e *brokerError) Unwrap() error             { return e.err }

// permanent is a Permanent error made by this package, before or after a call.
func permanent(text string) error {
	return &brokerError{kind: contracts.Permanent, text: "broker: " + text}
}

// kindOf maps a gRPC code: UNAVAILABLE and DEADLINE_EXCEEDED are Retryable, every other code Permanent.
func kindOf(c codes.Code) contracts.ErrorKind {
	if c == codes.Unavailable || c == codes.DeadlineExceeded {
		return contracts.Retryable
	}
	return contracts.Permanent
}

// wrapErr turns the error of a call into a TypedError. A transport error carries the code and
// no message, because the message of a failed dial names the socket path.
func wrapErr(ctx context.Context, op string, err error) error {
	if err == nil {
		return nil
	}
	var te contracts.TypedError
	if errors.As(err, &te) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &brokerError{kind: contracts.Retryable, text: "broker: " + op + ": deadline exceeded", err: err}
	}
	if errors.Is(err, context.Canceled) {
		return &brokerError{kind: contracts.Permanent, text: "broker: " + op + ": canceled", err: err}
	}
	st := status.Convert(err)
	text := "broker: " + op + ": " + st.Code().String()
	if st.Code() != codes.Unavailable && st.Message() != "" {
		text += ": " + st.Message()
	}
	return &brokerError{kind: kindOf(st.Code()), text: text, err: ctx.Err()}
}
