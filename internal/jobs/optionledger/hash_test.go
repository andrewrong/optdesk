package optionledger_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optionledger"
)

var _ jobs.FlexAttributesSHA256Func = optionledger.AttributesSHA256

func TestAttributesSHA256Fixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/attrs_hash.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		Attrs     map[string]string `json:"attrs"`
		Canonical string            `json:"canonical"`
		SHA256    string            `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) < 3 {
		t.Fatalf("fixture has %d entries", len(entries))
	}
	for _, e := range entries {
		got, err := optionledger.AttributesSHA256(e.Attrs)
		if err != nil {
			t.Fatal(err)
		}
		if got != e.SHA256 {
			t.Errorf("attrs %v: got %s, want %s", e.Attrs, got, e.SHA256)
		}
	}
}

func TestAttributesSHA256Ignores(t *testing.T) {
	t.Parallel()
	base := map[string]string{"a": "1", "b": "2"}
	with := map[string]string{"a": "1", "b": "2", "accountId": "X", "acctAlias": "a"}
	h1, err1 := optionledger.AttributesSHA256(base)
	h2, err2 := optionledger.AttributesSHA256(with)
	if err1 != nil || err2 != nil || h1 != h2 {
		t.Fatalf("hashes differ: %s %s (%v %v)", h1, h2, err1, err2)
	}
	if len(with) != 4 {
		t.Fatal("input map was modified")
	}
}

func TestAttributesSHA256KeyOrder(t *testing.T) {
	t.Parallel()
	// Known literal: {"a":"<&>","b":"x"} with no HTML escaping.
	got, err := optionledger.AttributesSHA256(map[string]string{"b": "x", "a": "<&>"})
	if err != nil {
		t.Fatal(err)
	}
	// sha256 of {"a":"<&>","b":"x"}, computed by the shell.
	const want = "b8659152d686651fca8ffb48927f1525c163aa6a0f97529d02a319483051605c"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
