package main

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/sources"
)

type fakeBarSource struct{}

func (fakeBarSource) DailyBars(context.Context, sources.BarRequest) (sources.BarResult, error) {
	return sources.BarResult{}, nil
}

func TestRegistryHasYfinance(t *testing.T) {
	t.Parallel()
	if _, ok := barSources["yfinance"]; !ok || len(barSources) != 1 {
		t.Fatalf("registry %v", barSources)
	}
	src, err := jobFactory{}.barSource("yfinance")
	if err != nil || src == nil {
		t.Fatalf("yfinance: %v %v", src, err)
	}
}

func TestUnknownSourceIdIsError(t *testing.T) {
	t.Parallel()
	_, err := jobFactory{}.barSource("tiingo")
	if err == nil || !strings.Contains(err.Error(), `"tiingo"`) || !strings.Contains(err.Error(), "yfinance") {
		t.Fatalf("error %v", err)
	}
}

func TestSwitchIsRegistryEntry(t *testing.T) {
	t.Parallel()
	var got sources.BarConfig
	reg := map[string]sources.NewBarSourceFunc{
		"yfinance": barSources["yfinance"],
		"tiingo": func(c sources.BarConfig) (sources.BarSource, error) {
			got = c
			return fakeBarSource{}, nil
		},
	}
	src, err := jobFactory{uv: "/opt/tools/uv"}.barSourceIn(reg, "tiingo")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := src.(fakeBarSource); !ok {
		t.Fatalf("source %T", src)
	}
	abs, err := filepath.Abs("pyside")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/opt/tools/uv", "run", "--locked", "--project", abs, "python", "-m", "optdesk.refbars"}
	if !slices.Equal(got.Command, want) {
		t.Fatalf("command %v, want %v", got.Command, want)
	}
}
