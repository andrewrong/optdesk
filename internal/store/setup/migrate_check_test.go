package setup

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRow struct {
	v  int64
	ok bool
}

type fakeRows struct {
	rows []fakeRow
	i    int
}

func (f *fakeRows) Next() bool { f.i++; return f.i <= len(f.rows) }
func (f *fakeRows) Scan(dest ...any) error {
	r := f.rows[f.i-1]
	*dest[0].(*int64), *dest[1].(*bool) = r.v, r.ok
	return nil
}
func (f *fakeRows) Err() error { return nil }
func (f *fakeRows) Close()     {}

func TestMigrateMissingRows(t *testing.T) {
	t.Parallel()
	// rows are in id order: 0 is the goose bootstrap row; 5 is applied out of order after 7;
	// 3 was rolled back (latest row false); 4 was rolled back then re-applied; 9 is not in the list.
	rows := []fakeRow{{0, true}, {1, true}, {2, true}, {3, true}, {3, false}, {4, true}, {4, false}, {4, true}, {7, true}, {5, true}, {9, true}}
	q := func(context.Context, string) (versionRows, error) { return &fakeRows{rows: rows}, nil }
	missing, extra, err := missingFrom(context.Background(), q, []int64{1, 2, 3, 4, 5, 6, 7, 8})
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{3, 6, 8}; !slices.Equal(missing, want) {
		t.Errorf("missing = %v, want %v", missing, want)
	}
	if want := []int64{0, 9}; !slices.Equal(extra, want) {
		t.Errorf("extra = %v, want %v", extra, want)
	}
}

func TestMigrateMissingErrorsHoldNoDSN(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		code string
		want string
	}{
		"permission": {"42501", "permission denied"},
		"absent":     {"42P01", "absent"},
		"other":      {"XX000", "failed"},
	} {
		q := func(context.Context, string) (versionRows, error) {
			return nil, &pgconn.PgError{Code: tc.code, Message: "boom " + sentinel}
		}
		_, _, err := missingFrom(context.Background(), q, []int64{1})
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), sentinel) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	dsn := "postgres://u:" + sentinel + "@127.0.0.1:1/optdesk?connect_timeout=1"
	_, _, err := MigrateMissing(context.Background(), dsn, nil)
	if !errors.Is(err, errCheckConnect) || strings.Contains(err.Error(), sentinel) {
		t.Errorf("connect: err = %v", err)
	}
	_, _, err = MigrateMissing(context.Background(), "postgres://u:"+sentinel+"@[bad", nil)
	if !errors.Is(err, errCheckConnect) {
		t.Errorf("parse: err = %v", err)
	}
}
