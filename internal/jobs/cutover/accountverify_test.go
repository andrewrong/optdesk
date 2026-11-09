package cutover_test

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs/cutover"
)

func TestVerifyAccountMissing(t *testing.T) {
	t.Parallel()
	f := newFake()
	in := line("main", "2026-11-05", "1", "1")
	res, err := cutover.VerifyAccount(context.Background(), strings.NewReader(in), f, nil, through)
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines != 1 || res.Missing != 1 || res.Different != 0 || f.writes != 0 {
		t.Fatalf("res = %+v, writes %d", res, f.writes)
	}
}

func TestVerifyAccountDifferent(t *testing.T) {
	t.Parallel()
	f := newFake()
	if _, err := cutover.ImportAccount(context.Background(), strings.NewReader(line("main", "2026-11-05", "1", "1")), f, nil, through); err != nil {
		t.Fatal(err)
	}
	f.writes = 0
	same, err := cutover.VerifyAccount(context.Background(), strings.NewReader(line("main", "2026-11-05", "1", "1")), f, nil, through)
	if err != nil || same.Missing != 0 || same.Different != 0 {
		t.Fatalf("same = %+v, %v", same, err)
	}
	res, err := cutover.VerifyAccount(context.Background(), strings.NewReader(line("main", "2026-11-05", "2", "1")), f, nil, through)
	if err != nil {
		t.Fatal(err)
	}
	if res.Different != 1 || res.Missing != 0 || f.writes != 0 {
		t.Fatalf("res = %+v, writes %d", res, f.writes)
	}
}
