package report_page

import (
	"context"
	"regexp"
	"testing"
)

var cjk = regexp.MustCompile(`[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]`)

func TestBuildTextsAreEnglish(t *testing.T) {
	t.Parallel()
	for _, market := range []string{"US", "JP"} {
		b, err := Build(context.Background(), cfg(market, "close", riskFile(t, "gap")), day, at)
		if err != nil {
			t.Fatal(err)
		}
		if loc := cjk.Find(b); loc != nil {
			t.Errorf("%s: output holds CJK text %q", market, loc)
		}
	}
}

func TestBuildGapSentences(t *testing.T) {
	t.Parallel()
	us := blocksOf(build(t, cfg("US", "close", riskFile(t, "gap"))))
	g := us["actions"].(map[string]any)["gaps"].([]any)[0].(map[string]any)
	if g["text"] != "Actions not built yet" {
		t.Errorf("not_built text = %v", g["text"])
	}
	jp := blocksOf(build(t, cfg("JP", "open", riskFile(t, "gap"))))
	g = jp["actions"].(map[string]any)["gaps"].([]any)[0].(map[string]any)
	if g["reason"] != "jp_slice_missing" || g["text"] != "Actions Japan session data not connected yet" {
		t.Errorf("jp gap = %v", g)
	}
}
