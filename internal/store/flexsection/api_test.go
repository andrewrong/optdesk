package flexsection_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/flexsection"
)

type key struct{ alias, section, rowKey string }

// fakeStore keeps the promises of the interface in memory.
type fakeStore struct {
	rows     map[key][]flexsection.Row
	statuses []flexsection.Status
	calls    int
}

var _ flexsection.Writer = (*fakeStore)(nil)
var _ flexsection.Reader = (*fakeStore)(nil)
var _ flexsection.Store = (*fakeStore)(nil)

func (f *fakeStore) RecordSections(_ context.Context, fetchID string, rows []flexsection.Row, statuses []flexsection.Status) (flexsection.Insert, error) {
	var ins flexsection.Insert
	if len(rows) == 0 && len(statuses) == 0 {
		return ins, nil
	}
	for _, r := range rows {
		if strings.Contains(r.Attributes, "accountId") || strings.Contains(r.Attributes, "acctAlias") {
			return flexsection.Insert{}, errors.New("account attribute")
		}
	}
	f.calls++
	if f.rows == nil {
		f.rows = map[key][]flexsection.Row{}
	}
	for _, r := range rows {
		k := key{r.Alias, r.Section, r.RowKey}
		revs := f.rows[k]
		switch {
		case len(revs) == 0:
			r.Revision = 1
			ins.New++
		case revs[len(revs)-1].AttributesSHA256 == r.AttributesSHA256:
			ins.Unchanged++
			continue
		default:
			r.Revision = revs[len(revs)-1].Revision + 1
			ins.Revised++
		}
		r.FetchID = fetchID
		f.rows[k] = append(revs, r)
	}
	f.statuses = append(f.statuses, statuses...)
	return ins, nil
}

func (f *fakeStore) Rows(_ context.Context, alias, section string) ([]flexsection.Row, error) {
	out := []flexsection.Row{}
	for k, revs := range f.rows {
		if k.alias == alias && k.section == section {
			out = append(out, revs[len(revs)-1])
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RowKey < out[j].RowKey })
	return out, nil
}

func (f *fakeStore) Statuses(_ context.Context, alias string) ([]flexsection.Status, error) {
	newest := map[string]flexsection.Status{}
	for _, s := range f.statuses {
		if s.Alias == alias && !s.RecordedAt.Before(newest[s.Section].RecordedAt) {
			newest[s.Section] = s
		}
	}
	out := []flexsection.Status{}
	for _, s := range newest {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Section < out[j].Section })
	return out, nil
}

func (f *fakeStore) Close() error { return nil }

func TestSectionStoreContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var open flexsection.OpenFunc = func(context.Context, string) (flexsection.Store, error) { return &fakeStore{}, nil }
	st, err := open(ctx, "dsn")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	t0 := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	row := func(k, sha string) flexsection.Row {
		return flexsection.Row{Alias: "a", Section: "CashReport", RowKey: k, Revision: 99, Attributes: `{"v":"` + sha + `"}`, AttributesSHA256: sha, RecordedAt: t0}
	}
	ins, err := st.RecordSections(ctx, "f1", []flexsection.Row{row("b", "1"), row("a", "1")},
		[]flexsection.Status{{FetchID: "f1", Alias: "a", Section: "CashReport", State: "present", RowCount: 2, RecordedAt: t0}})
	if err != nil || ins != (flexsection.Insert{New: 2}) {
		t.Fatalf("first record: %+v %v", ins, err)
	}
	ins, err = st.RecordSections(ctx, "f2", []flexsection.Row{row("a", "2"), row("b", "1")},
		[]flexsection.Status{{FetchID: "f2", Alias: "a", Section: "CashReport", State: "absent", RecordedAt: t0.Add(time.Hour)}})
	if err != nil || ins != (flexsection.Insert{Revised: 1, Unchanged: 1}) {
		t.Fatalf("second record: %+v %v", ins, err)
	}
	rows, _ := st.Rows(ctx, "a", "CashReport")
	if len(rows) != 2 || rows[0].RowKey != "a" || rows[0].Revision != 2 || rows[1].RowKey != "b" || rows[1].Revision != 1 {
		t.Fatalf("rows: %+v", rows)
	}
	sts, _ := st.Statuses(ctx, "a")
	if len(sts) != 1 || sts[0].State != "absent" {
		t.Fatalf("statuses: %+v", sts)
	}
}

func TestRecordSectionsRefusesAccountAttributes(t *testing.T) {
	t.Parallel()
	f := &fakeStore{}
	for _, attr := range []string{`{"accountId":"x"}`, `{"acctAlias":"x"}`} {
		_, err := f.RecordSections(context.Background(), "f", []flexsection.Row{{Alias: "a", Section: "S", RowKey: "k", Attributes: attr}}, nil)
		if err == nil {
			t.Fatalf("%s accepted", attr)
		}
	}
	rows, _ := f.Rows(context.Background(), "a", "S")
	if len(rows) != 0 {
		t.Fatalf("written: %+v", rows)
	}
}

func TestRecordSectionsEmptyMakesNoCall(t *testing.T) {
	t.Parallel()
	f := &fakeStore{}
	ins, err := f.RecordSections(context.Background(), "f", nil, nil)
	if err != nil || ins != (flexsection.Insert{}) || f.calls != 0 {
		t.Fatalf("%+v %v %d", ins, err, f.calls)
	}
}
