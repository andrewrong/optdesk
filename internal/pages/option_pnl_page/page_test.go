package option_pnl_page

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestNewPageMetaIsOptionPnlPage(t *testing.T) {
	t.Parallel()
	p := NewPage(Config{RunID: "r", BuildID: "b", Scope: "s", Zone: time.UTC})
	if p.Meta.Name != "option_pnl_page" {
		t.Fatalf("Meta.Name = %q", p.Meta.Name)
	}
	if p.Meta.Schema != "page.option_pnl_page.v1" {
		t.Fatalf("Meta.Schema = %q", p.Meta.Schema)
	}
}

func TestBuildNilZoneIsError(t *testing.T) {
	t.Parallel()
	p := NewPage(Config{})
	_, err := p.Build(context.Background(), nil, contracts.Day("2026-11-13"), time.Unix(0, 0))
	if err == nil || !strings.Contains(err.Error(), "nil zone") {
		t.Fatalf("err = %v, want nil zone", err)
	}
}
