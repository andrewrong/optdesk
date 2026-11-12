package optionledger_test

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/optionledger"
)

// fakeLog implements optionledger.ImportLog over a slice, with the rule of the contract.
type fakeLog struct{ all []store.FlexFetchRecord }

func (f *fakeLog) RecentFetches(_ context.Context, login string, limit int) ([]store.FlexFetchRecord, error) {
	var out []store.FlexFetchRecord
	for _, r := range f.all {
		if r.Login == login && strings.HasPrefix(r.QueryID, "file:") {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].FinishedAt.Equal(out[j].FinishedAt) {
			return out[i].FinishedAt.After(out[j].FinishedAt)
		}
		return out[i].QueryID > out[j].QueryID
	})
	if limit >= 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// latestIDs (a record is named by its RawSHA256 here) is a caller that reads only through the interface.
func latestIDs(ctx context.Context, l optionledger.ImportLog, login string, n int) ([]string, error) {
	recs, err := l.RecentFetches(ctx, login, n)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(recs))
	for _, r := range recs {
		ids = append(ids, r.RawSHA256)
	}
	return ids, nil
}

func TestImportLogContract(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	f := &fakeLog{all: []store.FlexFetchRecord{
		{RawSHA256: "a", Login: "import", QueryID: "file:1", FinishedAt: t0},
		{RawSHA256: "b", Login: "import", QueryID: "file:2", FinishedAt: t0},
		{RawSHA256: "c", Login: "import", QueryID: "123456", FinishedAt: t0.Add(time.Hour)},
		{RawSHA256: "d", Login: "other", QueryID: "file:3", FinishedAt: t0},
		{RawSHA256: "e", Login: "import", QueryID: "file:4", FinishedAt: t0.Add(-time.Hour)},
	}}
	var l optionledger.ImportLog = f
	got, err := latestIDs(context.Background(), l, "import", 2)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "b,a" {
		t.Fatalf("got %v, want [b a]", got)
	}
}
