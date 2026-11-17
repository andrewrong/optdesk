package valuationpg

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// recorder is a database/sql driver that records what the store sends and answers each query from a function of the
// test. It is defined here and used by read_test.go too; every test makes its own, so no state is shared.
type recorder struct {
	mu     sync.Mutex
	events []string // begin, commit, rollback, exec, query in the order they happen
	execs  []string // the SQL of every exec
	// answer returns the columns and rows for a query; nil rows give an empty result.
	answer func(query string, args []driver.NamedValue) [][]driver.Value
}

func (r *recorder) log(ev string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *recorder) count(ev string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e == ev {
			n++
		}
	}
	return n
}

func (r *recorder) store() *Store { return &Store{db: sql.OpenDB(recConnector{r})} }

type recConnector struct{ r *recorder }

func (c recConnector) Connect(context.Context) (driver.Conn, error) { return recConn(c), nil }
func (c recConnector) Driver() driver.Driver                        { return recDriver{} }

type recDriver struct{}

func (recDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("recorder: use the connector")
}

type recConn struct{ r *recorder }

func (recConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("recorder: no prepare") }
func (recConn) Close() error                        { return nil }
func (c recConn) Begin() (driver.Tx, error)         { c.r.log("begin"); return recTx(c), nil }
func (c recConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c.Begin()
}

func (c recConn) ExecContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	c.r.log("exec")
	c.r.mu.Lock()
	c.r.execs = append(c.r.execs, q)
	c.r.mu.Unlock()
	return driver.RowsAffected(1), nil
}

func (c recConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.r.log("query")
	var rows [][]driver.Value
	if c.r.answer != nil {
		rows = c.r.answer(q, args)
	}
	return &recRows{rows: rows}, nil
}

type recTx struct{ r *recorder }

func (t recTx) Commit() error   { t.r.log("commit"); return nil }
func (t recTx) Rollback() error { t.r.log("rollback"); return nil }

type recRows struct {
	rows [][]driver.Value
	i    int
}

func (r *recRows) Columns() []string {
	n := 0
	if len(r.rows) > 0 {
		n = len(r.rows[0])
	}
	cols := make([]string, n)
	for i := range cols {
		cols[i] = "c"
	}
	return cols
}
func (r *recRows) Close() error { return nil }
func (r *recRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// statusAnswer answers the status read of PutReview with the two flags.
func statusAnswer(effective, void bool) func(string, []driver.NamedValue) [][]driver.Value {
	return func(q string, _ []driver.NamedValue) [][]driver.Value {
		if q == reviewStatusSQL {
			return [][]driver.Value{{effective, void}}
		}
		return nil
	}
}

func TestPutRTableOneTransaction(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	s := rec.store()
	err := s.PutRTable(context.Background(), valuation.RTableVersion{
		EffectiveDate: "2026-01-02",
		Rows: []valuation.RTableRow{
			{YieldUpTo: "3.0", US: "1.1", JP: "1.2", HKCN: "1.3"},
			{YieldUpTo: "4.0", US: "1.4", JP: "1.5", HKCN: "1.6"},
			{YieldUpTo: "", US: "1.7", JP: "1.8", HKCN: "1.9"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if b, c, x, rb := rec.count("begin"), rec.count("commit"), rec.count("exec"), rec.count("rollback"); b != 1 || c != 1 || x != 3 || rb != 0 {
		t.Errorf("begin=%d commit=%d exec=%d rollback=%d, want 1, 1, 3, 0", b, c, x, rb)
	}
}

func TestPutReviewAfterEffectiveIsClosed(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"draft", "effective"} {
		rec := &recorder{answer: statusAnswer(true, false)}
		err := rec.store().PutReview(context.Background(), valuation.Review{ReviewID: "r1", CompanyKey: "US:AAA", Status: status})
		if !errors.Is(err, valuation.ErrClosed) {
			t.Errorf("%s row after effective: err = %v, want ErrClosed", status, err)
		}
		if len(rec.execs) != 0 || rec.count("commit") != 0 {
			t.Errorf("%s row after effective: inserted %v, committed %d", status, rec.execs, rec.count("commit"))
		}
	}
	// Its void row is the one row still accepted.
	rec := &recorder{answer: statusAnswer(true, false)}
	err := rec.store().PutReview(context.Background(), valuation.Review{
		ReviewID: "r1", CompanyKey: "US:AAA", Status: "void", Type: "first", VersionDate: "2026-01-02", V: "10", P: "3", VoidReason: "typo",
	})
	if err != nil || len(rec.execs) != 1 || rec.count("commit") != 1 {
		t.Errorf("void row of an effective review: err = %v, inserts = %d, commits = %d", err, len(rec.execs), rec.count("commit"))
	}
	// A second void row is closed.
	rec = &recorder{answer: statusAnswer(true, true)}
	err = rec.store().PutReview(context.Background(), valuation.Review{ReviewID: "r1", CompanyKey: "US:AAA", Status: "void"})
	if !errors.Is(err, valuation.ErrClosed) {
		t.Errorf("second void row: err = %v, want ErrClosed", err)
	}
}

func TestVoidNeedsEffective(t *testing.T) {
	t.Parallel()
	for _, hasVoid := range []bool{false, true} {
		rec := &recorder{answer: statusAnswer(false, hasVoid)}
		err := rec.store().PutReview(context.Background(), valuation.Review{ReviewID: "r2", CompanyKey: "US:AAA", Status: "void"})
		if !errors.Is(err, valuation.ErrNotEffective) {
			t.Errorf("void row, no effective row (void present %v): err = %v, want ErrNotEffective", hasVoid, err)
		}
		if len(rec.execs) != 0 {
			t.Errorf("a refused void row was inserted: %v", rec.execs)
		}
	}
	// A draft of a review with no row is inserted.
	rec := &recorder{answer: statusAnswer(false, false)}
	if err := rec.store().PutReview(context.Background(), valuation.Review{ReviewID: "r3", CompanyKey: "US:AAA", Status: "draft"}); err != nil {
		t.Errorf("draft of a new review: %v", err)
	}
	if len(rec.execs) != 1 {
		t.Errorf("draft inserts = %d, want 1", len(rec.execs))
	}
}

var writeVerb = regexp.MustCompile(`(?i)\b(update|delete)\b`)

func TestNoUpdateOrDeleteStatements(t *testing.T) {
	t.Parallel()
	if len(statements) == 0 {
		t.Fatal("write.go lists no statement")
	}
	for _, q := range statements {
		if writeVerb.MatchString(q) {
			t.Errorf("statement sends UPDATE or DELETE: %s", q)
		}
	}
	// The statements the store really sends are all in the list.
	rec := &recorder{answer: statusAnswer(false, false)}
	s := rec.store()
	ctx := context.Background()
	_ = s.PutProfile(ctx, valuation.Profile{CompanyKey: "US:AAA"})
	_ = s.PutReview(ctx, valuation.Review{ReviewID: "r", Status: "draft"})
	_ = s.PutRVersion(ctx, valuation.RVersion{})
	_ = s.PutRTable(ctx, valuation.RTableVersion{Rows: []valuation.RTableRow{{}}})
	_ = s.PutMarginTable(ctx, valuation.MarginTableVersion{Rows: []valuation.MarginTableRow{{}}})
	if len(rec.execs) != 5 {
		t.Fatalf("execs = %d, want 5", len(rec.execs))
	}
	listed := strings.Join(statements, "\n")
	for _, q := range rec.execs {
		if !strings.Contains(listed, q) {
			t.Errorf("sent statement is not in the list: %s", q)
		}
	}
}
