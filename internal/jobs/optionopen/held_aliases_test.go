package optionopen

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/store"
)

// recordingAccounts returns one stock position for alias "mine" and records the aliases asked for.
type recordingAccounts struct {
	mu    sync.Mutex
	asked map[string]bool
}

func (r *recordingAccounts) Positions(_ context.Context, alias, _ string) ([]store.Position, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.asked == nil {
		r.asked = map[string]bool{}
	}
	r.asked[alias] = true
	if alias != "mine" {
		return nil, nil
	}
	return []store.Position{{Symbol: "TEST", SecType: "stock", Currency: "USD", Quantity: "10", ConID: 7}}, nil
}
func (r *recordingAccounts) Values(context.Context, string, string) ([]store.AccountValue, error) {
	return nil, nil
}
func (r *recordingAccounts) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return nil, nil
}

func TestStoredHeldAliasesReadsTheGivenAlias(t *testing.T) {
	t.Parallel()
	acc := &recordingAccounts{}
	s := Stored{Accounts: acc, HeldAliases: []string{"mine"}}
	got, err := s.positions(context.Background(), time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "TEST", got[0].Symbol)
	require.Equal(t, map[string]bool{"mine": true}, acc.asked)
}

func TestStoredHeldAliasesUnsetReadsNothing(t *testing.T) {
	t.Parallel()
	acc := &recordingAccounts{}
	s := Stored{Accounts: acc}
	got, err := s.positions(context.Background(), time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Empty(t, got)
	require.Empty(t, acc.asked)
}

// ownerAliases are the aliases the rigs of the other tests of the package give to Stored.
var ownerAliases = []string{"A", "B"}
