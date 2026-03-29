package serverconfig

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestPaperConfigAdjustments(t *testing.T) {
	SetVelocitySecret("secret")

	input := `
proxies:
  bungee-cord:
    online-mode: true
  proxy-protocol: true
  velocity:
    enabled: false
    online-mode: false
    secret: "blalala"
`
	expectedCfg := paperGlobal{
		Proxies: proxiesConfig{
			ProxyProtocol: false,
			Velocity: struct {
				Enabled    bool   `json:"enabled"`
				OnlineMode bool   `json:"online-mode"`
				Secret     string `json:"secret"`
			}{
				Enabled:    true,
				OnlineMode: true,
				Secret:     "secret",
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
