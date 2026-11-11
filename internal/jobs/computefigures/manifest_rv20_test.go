package computefigures

import (
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestManifestParsesRv20Keys(t *testing.T) {
	t.Parallel()
	var m manifest
	if err := toml.Unmarshal(manifestBytes, &m); err != nil {
		t.Fatal(err)
	}
	if m.Rv20 != "market.rv20" {
		t.Fatalf("rv20 = %q, want market.rv20", m.Rv20)
	}
	if want := []string{"SPX", "NDX", "RUT"}; !reflect.DeepEqual(m.Rv20Indices, want) {
		t.Fatalf("rv20_indices = %v, want %v", m.Rv20Indices, want)
	}
}
