package serverconfig

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestSpigotConfigAdjustments(t *testing.T) {
	input := `
settings:
  bungeecord: false
`
	expectedCfg := spigotCfg{
		Settings: struct {
			BungeeCord bool `yaml:"bungeecord"`
		}{
			BungeeCord: true,
		},
	}

	expectedYaml, err := yaml.Marshal(expectedCfg)
	require.NoError(t, err)

	actual, err := sanatizeSpigot([]byte(input))
	require.NoError(t, err)

	if d := cmp.Diff(string(expectedYaml), string(actual)); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}
