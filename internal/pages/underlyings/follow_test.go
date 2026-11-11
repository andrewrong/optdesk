package underlyings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFollowTable(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "fixtures", "underlyings.follow.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Status       string   `json:"status"`
		Options      bool     `json:"options"`
		Fundamentals bool     `json:"fundamentals"`
		IssuerSet    bool     `json:"issuer_set"`
		Held         bool     `json:"held"`
		FollowedBy   []string `json:"followed_by"`
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 24 {
		t.Fatalf("%d cases", len(cases))
	}
	for _, c := range cases {
		got := Follows(c.Status, c.Options, c.Fundamentals, c.IssuerSet, c.Held)
		if len(got) == 0 && len(c.FollowedBy) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.FollowedBy) {
			t.Errorf("%+v: got %v", c, got)
		}
	}
}

func TestFollowsNeverNil(t *testing.T) {
	t.Parallel()
	if got := Follows("pending", true, true, true, true); got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want empty non-nil", got)
	}
}
