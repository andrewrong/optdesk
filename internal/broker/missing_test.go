package broker_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
)

func TestMissingReasonString(t *testing.T) {
	t.Parallel()
	cases := map[broker.MissingReason]string{
		broker.MissingNotReceivedRelease: "not_received_release",
		broker.MissingNotReceivedGrace:   "not_received_grace",
		broker.MissingInvalid:            "invalid",
		broker.MissingError:              "error",
		broker.MissingReceivedZero:       "received_zero",
		broker.MissingMarketDataType:     "market_data_type",
	}
	for r, want := range cases {
		if got := r.String(); got != want {
			t.Fatalf("%d: got %q, want %q", int(r), got, want)
		}
	}
}
