package option_result_page_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/option_result_page"
)

func TestNewPageMeta(t *testing.T) {
	t.Parallel()
	p := option_result_page.NewPage(option_result_page.Config{}, "main")
	if p.Meta.Name != "option_result_page" || p.Meta.Schema != "page.option_result_page.v1" {
		t.Fatalf("Meta = %+v", p.Meta)
	}
	if p.Build == nil {
		t.Fatal("Build is nil")
	}
}

func TestConfigContract(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_empty.json")
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, c.Zone)
	b, err := option_result_page.NewPage(c, "main").Build(context.Background(), noSeries{}, contracts.Day("2026-01-02"), at)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
}

func TestNotesCarryCountGloss(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	c.Aliases = []string{"main"}
	notes := strs(buildMap(t, c, "main")["notes"])
	if len(notes) != 2 {
		t.Fatalf("notes = %v", notes)
	}
	if notes[0] != "本页只含账户 main" {
		t.Errorf("notes[0] = %q, want the scope note", notes[0])
	}
	want := "计数说明：本页的一笔指账户的一个已平仓头寸，认购与认沽合计，按方向和币种分列；期权盈亏页只计已结束的卖出认沽批次，两者口径不同，笔数不会相等。"
	if notes[1] != want {
		t.Errorf("notes[1] = %q, want %q", notes[1], want)
	}
}
