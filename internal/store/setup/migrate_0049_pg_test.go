//go:build pg

package setup

import (
	"context"
	"testing"
)

// TestMig49GrantsPG: after 0049 both capture writers hold table-level SELECT and INSERT on obs.option_day and nothing that removes rows.
func TestMig49GrantsPG(t *testing.T) {
	t.Parallel()
	s := newScratch(t, true)
	if err := migrateLocked(context.Background(), t, s.openAs(t, testPassword), migrationsUpTo(t, 49), quietLog()); err != nil {
		t.Fatalf("apply 1 to 49: %v", err)
	}
	for _, role := range []string{"ot_w_option_eod", "ot_w_option_open"} {
		for priv, want := range map[string]bool{"SELECT": true, "INSERT": true, "DELETE": false, "TRUNCATE": false} {
			n := s.count(t, `SELECT count(*) FROM (SELECT 1) x WHERE has_table_privilege('`+role+`', 'obs.option_day', '`+priv+`')`)
			if got := n == 1; got != want {
				t.Errorf("%s %s on obs.option_day = %v, want %v", role, priv, got, want)
			}
		}
	}
}
