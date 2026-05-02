package serverconfig

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestPaperConfigAdjustments(t *testing.T) {
	input := `
proxies:
  proxy-protocol: true
  bungee-cord:
    online-mode: false
  velocity:
    enabled: true
`
	expectedCfg := paperGlobal{
		Proxies: proxiesConfig{
			ProxyProtocol: false,
			BungeeCord: struct {
				OnlineMode bool `json:"online-mode"`
			}{
				OnlineMode: true,
			},
			Velocity: struct {
				Enabled bool `json:"enabled"`
			}{
				Enabled: false,
			},
		},
	}

	expectedYaml, err := yaml.Marshal(expectedCfg)
	require.NoError(t, err)

	actual, err := sanatizePaperGlobal([]byte(input))
	require.NoError(t, err)

	if d := cmp.Diff(string(expectedYaml), string(actual)); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}
