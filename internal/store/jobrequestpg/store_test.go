package jobrequestpg

import (
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// TestOpenersHaveTheInterfaceTypes: the three openers are assignable to the opener types of the interface
// package, and the opened store keeps the three roles' methods.
func TestOpenersHaveTheInterfaceTypes(t *testing.T) {
	t.Parallel()
	var (
		r jobrequest.OpenReaderFunc   = OpenReader
		p jobrequest.OpenPosterFunc   = OpenPoster
		c jobrequest.OpenConsumerFunc = OpenConsumer
	)
	if r == nil || p == nil || c == nil {
		t.Fatal("an opener is nil")
	}
}

// TestKeysRoundTrip: the lines stored by Finish read back the same, a zero DueAt stays zero, and a document of
// another version is refused.
func TestKeysRoundTrip(t *testing.T) {
	t.Parallel()
	in := []jobrequest.KeyLine{
		{Dataset: "calendar", Subject: "XNYS", Group: "rerun", Status: "done", Returned: 3, Usable: 2, Attempt: 1, Mark: "m"},
		{Dataset: "bars", Subject: "AAPL", Group: "not_due", Status: "skipped", ReasonCode: "not_due",
			DueAt: time.Date(2026, 11, 12, 21, 0, 0, 0, time.UTC)},
	}
	raw, err := encodeKeys(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decodeKeys(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
	if _, err := decodeKeys([]byte(`{"v":2,"lines":[]}`)); err == nil {
		t.Error("version 2 accepted")
	}
	if out, err := decodeKeys(nil); err != nil || out != nil {
		t.Errorf("decode nil = %v, %v", out, err)
	}
}
