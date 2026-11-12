package optionledger

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"
)

// recentDB answers recentFetchesSQL from rows laid out in the column order of fetchColumns.
type recentDB struct {
	fakeDB
	rows [][]any
}

func (d *recentDB) Select(_ context.Context, sql string, args []any, scan func(scanner) error) error {
	if sql != recentFetchesSQL {
		panic("unexpected query: " + sql)
	}
	var got [][]any
	for _, r := range d.rows {
		if r[0] == args[0] && strings.HasPrefix(r[1].(string), "file:") {
			got = append(got, r)
		}
	}
	sort.Slice(got, func(i, j int) bool { return got[i][5].(time.Time).After(got[j][5].(time.Time)) })
	if n := args[1].(int); len(got) > n {
		got = got[:n]
	}
	return replay(got, scan)
}

func importRow(login, query string, finished time.Time) []any {
	day := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	return []any{login, query, &day, (*time.Time)(nil), finished, finished, "ok", "", "", "h", "1", 1, 0, 3, 3, 0, 0, "id"}
}

func TestRecentFetchesLimitCapsTheList(t *testing.T) {
	t.Parallel()
	db := &recentDB{}
	for i := 0; i < 5; i++ {
		db.rows = append(db.rows, importRow("import", "file:r:1:a", fixedNow.Add(time.Duration(i)*time.Hour)))
	}
	got, err := (&Store{db: db}).RecentFetches(context.Background(), "import", 2)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d records, %v", len(got), err)
	}
	if !got[0].FinishedAt.After(got[1].FinishedAt) || got[0].RowsNew != 3 || got[0].FirstDay != "2026-11-01" || got[0].LastDay != "" {
		t.Errorf("records = %+v", got)
	}
	if got, err := (&Store{db: db}).RecentFetches(context.Background(), "import", 0); err != nil || got == nil || len(got) != 0 {
		t.Errorf("limit 0 = %v, %v", got, err)
	}
}

func TestRecentFetchesNoRecordGivesEmptySlice(t *testing.T) {
	t.Parallel()
	db := &recentDB{rows: [][]any{importRow("import", "file:r:1:a", fixedNow)}}
	got, err := (&Store{db: db}).RecentFetches(context.Background(), "nobody", 10)
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("got %#v, %v", got, err)
	}
}
