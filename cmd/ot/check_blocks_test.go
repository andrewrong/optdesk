package main

import (
	"encoding/json"
	"testing"
)

func TestCheckAcceptsShortPutTiersKey(t *testing.T) {
	t.Parallel()
	body := map[string]json.RawMessage{
		"schema": json.RawMessage(`"option_candidates_page"`), "schema_version": json.RawMessage(`1`),
		"groups": json.RawMessage(`[]`), "short_put_tiers": json.RawMessage(`null`),
	}
	if got := compareBlocks("option_candidates_page", body); len(got) != 0 {
		t.Fatalf("want no difference, got %v", got)
	}
}
