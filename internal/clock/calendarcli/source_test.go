package calendarcli_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock/calendarcli"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const from, to = contracts.Day("2026-11-02"), contracts.Day("2026-11-30")

func testdata(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "calendar_xnys.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func outOf(argv []string) string {
	i := slices.Index(argv, "--out")
	return argv[i+1]
}

// writing returns a fake runner that writes body to --out and exits with code.
func writing(body string, code int, seen *[]string) func(context.Context, []string) (int, error) {
	return func(_ context.Context, argv []string) (int, error) {
		if seen != nil {
			*seen = argv
		}
		if err := os.WriteFile(outOf(argv), []byte(body), 0o600); err != nil {
			return -1, err
		}
		return code, nil
	}
}

func kindOf(t *testing.T, err error) contracts.ErrorKind {
	t.Helper()
	var te contracts.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("not a typed error: %v", err)
	}
	return te.Kind()
}

func TestSessionDaysParsesGeneratedFile(t *testing.T) {
	t.Parallel()
	src := calendarcli.New([]string{"gen"}, t.TempDir(), writing(testdata(t), 0, nil))
	rows, err := src.SessionDays(context.Background(), "XNYS", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 29 {
		t.Fatalf("rows = %d, want 29", len(rows))
	}
	if got := rows[0].SessionDay.Format("2006-01-02"); got != "2026-11-02" || !rows[0].IsSession {
		t.Fatalf("first row = %s %v", got, rows[0].IsSession)
	}
	if rows[5].IsSession { // 2026-11-07 is a Saturday
		t.Fatal("Saturday is a session")
	}
}

func TestArgvHoldsCalendarFromToOut(t *testing.T) {
	t.Parallel()
	var seen []string
	work := t.TempDir()
	src := calendarcli.New([]string{"uv", "run"}, work, writing(testdata(t), 0, &seen))
	if _, err := src.SessionDays(context.Background(), "XNYS", from, to); err != nil {
		t.Fatal(err)
	}
	want := []string{"uv", "run", "--calendar", "XNYS", "--from", "2026-11-02", "--to", "2026-11-30", "--out"}
	if len(seen) != len(want)+1 || !slices.Equal(seen[:len(want)], want) {
		t.Fatalf("argv = %v", seen)
	}
	if !strings.HasPrefix(seen[len(seen)-1], work) {
		t.Fatalf("out %q is not under workDir", seen[len(seen)-1])
	}
}

func exitCase(t *testing.T, code int) {
	t.Helper()
	src := calendarcli.New(nil, t.TempDir(), func(context.Context, []string) (int, error) { return code, nil })
	_, err := src.SessionDays(context.Background(), "XNYS", from, to)
	if err == nil || kindOf(t, err) != contracts.Permanent {
		t.Fatalf("exit %d: err = %v", code, err)
	}
}

func TestExitCode1IsPermanent(t *testing.T) { t.Parallel(); exitCase(t, 1) }

func TestExitCode2IsPermanent(t *testing.T) { t.Parallel(); exitCase(t, 2) }

func TestOtherExitIsPermanent(t *testing.T) { t.Parallel(); exitCase(t, 7); exitCase(t, -1) }

func TestStartFailureIsRetryable(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	secret := errors.New("fork /secret/path/uv: no such file")
	src := calendarcli.New(nil, work, func(context.Context, []string) (int, error) { return -1, secret })
	_, err := src.SessionDays(context.Background(), "XNYS", from, to)
	if err == nil || kindOf(t, err) != contracts.Retryable {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), work) {
		t.Fatalf("error text leaks: %v", err)
	}
}

func permanentFor(t *testing.T, body string) {
	t.Helper()
	src := calendarcli.New(nil, t.TempDir(), writing(body, 0, nil))
	_, err := src.SessionDays(context.Background(), "XNYS", from, to)
	if err == nil || kindOf(t, err) != contracts.Permanent {
		t.Fatalf("err = %v", err)
	}
}

func TestMissingProvenancePermanent(t *testing.T) {
	t.Parallel()
	body := testdata(t)
	permanentFor(t, strings.Replace(body, `"generator_version": "1"`, `"generator_version": ""`, 1))
	permanentFor(t, strings.Replace(body, `"provenance"`, `"provenanc"`, 1))
}

func TestDuplicateDayPermanent(t *testing.T) {
	t.Parallel()
	body := strings.Replace(testdata(t), `"session_day": "2026-11-03"`, `"session_day": "2026-11-02"`, 1)
	permanentFor(t, body)
	// a gap is not one row per calendar day
	permanentFor(t, strings.Replace(testdata(t), `"session_day": "2026-11-03"`, `"session_day": "2026-11-04"`, 1))
	// a day outside the range
	permanentFor(t, strings.Replace(testdata(t), `"session_day": "2026-11-30"`, `"session_day": "2026-12-01"`, 1))
}

func TestWrongCalendarIdPermanent(t *testing.T) {
	t.Parallel()
	permanentFor(t, strings.Replace(testdata(t), `"calendar_id": "XNYS"`, `"calendar_id": "XTKS"`, 1))
}

func TestTempDirRemoved(t *testing.T) {
	t.Parallel()
	for _, code := range []int{0, 1} {
		work := t.TempDir()
		var seen []string
		src := calendarcli.New(nil, work, writing(testdata(t), code, &seen))
		_, _ = src.SessionDays(context.Background(), "XNYS", from, to)
		if _, err := os.Stat(filepath.Dir(outOf(seen))); !os.IsNotExist(err) {
			t.Fatalf("exit %d: temporary directory remains: %v", code, err)
		}
		left, err := os.ReadDir(work)
		if err != nil || len(left) != 0 {
			t.Fatalf("exit %d: workDir holds %v, %v", code, left, err)
		}
	}
}

func TestBadRequestedDayIsPermanentAndRunsNothing(t *testing.T) {
	t.Parallel()
	ran := false
	src := calendarcli.New(nil, t.TempDir(), func(context.Context, []string) (int, error) { ran = true; return 0, nil })
	_, err := src.SessionDays(context.Background(), "XNYS", "2026-13-01", to)
	if err == nil || kindOf(t, err) != contracts.Permanent || ran {
		t.Fatalf("err = %v, ran = %v", err, ran)
	}
}
