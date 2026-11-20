package grpcbroker

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestReconnectIsRetryable(t *testing.T) {
	t.Parallel()
	for _, msg := range []string{"session_reconnecting", "history_unavailable"} {
		err := wrapErr(context.Background(), "bars", status.Error(codes.Unavailable, msg))
		if err == nil || kindErr(t, err) != contracts.Retryable {
			t.Fatalf("%s: got %v", msg, err)
		}
	}
}
