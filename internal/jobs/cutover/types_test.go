package cutover_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/cutover"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestImportAccountFuncContract(t *testing.T) {
	t.Parallel()
	var imp cutover.ImportAccountFunc = func(
		_ context.Context, r io.Reader, _ store.AccountStore, m map[string]string, through contracts.Day,
	) (jobs.ImportResult, error) {
		b, err := io.ReadAll(r)
		if err != nil {
			return jobs.ImportResult{}, err
		}
		if m["a"] != "b" || through != "2026-01-02" {
			return jobs.ImportResult{}, errors.New("arguments lost")
		}
		return jobs.ImportResult{Lines: len(b), Inserted: 1, Same: 2, Conflict: 3}, nil
	}
	got, err := imp(context.Background(), strings.NewReader("xy"), nil, map[string]string{"a": "b"}, "2026-01-02")
	if err != nil || got != (jobs.ImportResult{Lines: 2, Inserted: 1, Same: 2, Conflict: 3}) {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestVerifyAccountFuncContract(t *testing.T) {
	t.Parallel()
	var ver cutover.VerifyAccountFunc = func(
		_ context.Context, r io.Reader, _ store.AccountReader, _ map[string]string, _ contracts.Day,
	) (cutover.VerifyResult, error) {
		b, _ := io.ReadAll(r)
		return cutover.VerifyResult{Lines: len(b), Missing: 1, Different: 2}, nil
	}
	got, err := ver(context.Background(), strings.NewReader("abc"), nil, nil, "2026-01-02")
	if err != nil || got != (cutover.VerifyResult{Lines: 3, Missing: 1, Different: 2}) {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestCompareFiguresFuncContract(t *testing.T) {
	t.Parallel()
	var cmp cutover.CompareFiguresFunc = func(_ context.Context, _, _, _, _ io.Reader, _ store.ReadingReader) (cutover.FigureReport, error) {
		return cutover.FigureReport{
			Results: []cutover.FigureResult{
				{ID: "a", Version: 1, Status: cutover.FigureChecked},
				{ID: "b", Version: 2, Status: cutover.FigureUnchecked, Reason: "no_old_value"},
			},
			OK: true,
		}, nil
	}
	rep, err := cmp(context.Background(), nil, nil, nil, nil, nil)
	if err != nil || !rep.OK || len(rep.Results) != 2 || rep.Results[1].Status != "unchecked" || rep.Results[1].Reason != "no_old_value" {
		t.Fatalf("got %+v, %v", rep, err)
	}
	for _, s := range []cutover.FigureStatus{
		cutover.FigureChecked, cutover.FigureExplained, cutover.FigureUnexplained, cutover.FigureNew, cutover.FigureUnchecked,
	} {
		if s == "" {
			t.Fatal("empty status")
		}
	}
}
