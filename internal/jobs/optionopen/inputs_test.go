package optionopen

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRv20FromStoredCloses(t *testing.T) {
	t.Parallel()
	through := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	closes := map[string]string{}
	for i := 0; i < 21; i++ {
		c := "100"
		if i%2 == 1 {
			c = "101.005016708417" // 100 * e^0.01
		}
		closes[through.AddDate(0, 0, i-20).Format(time.DateOnly)] = c
	}
	s := Stored{Bars: fakeBars{closes: closes}}
	got, ok, err := s.rv20(context.Background(), 1, through)
	require.NoError(t, err)
	require.True(t, ok)
	require.InDelta(t, math.Sqrt(20.0/19.0*1e-4)*math.Sqrt(252), got, 1e-6)

	delete(closes, through.Format(time.DateOnly))
	_, ok, err = s.rv20(context.Background(), 1, through)
	require.NoError(t, err)
	require.False(t, ok, "twenty closes are not enough")
}
