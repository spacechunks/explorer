package serverconfig_test

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spacechunks/explorer/controlplane/serverconfig"
	"github.com/stretchr/testify/require"
)

func TestSanatizeConfigs(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)

	paperGlobal := `
proxies:
  bungee-cord:
    online-mode: true
  proxy-protocol: true
  velocity:
    enabled: false
    online-mode: false
    secret: "blalala"
`

	err = root.Mkdir("config", os.ModePerm)
	require.NoError(t, err)

	err = root.WriteFile("config/paper-global.yml", []byte(paperGlobal), os.ModePerm)
	require.NoError(t, err)

	properties := `
#Minecraft server properties
#Mon Mar 09 17:21:18 CET 2026
log-ips=true
management-server-allowed-origins=
management-server-enabled=false
management-server-host=1.1.1.1
management-server-port=1337
management-server-secret=wrong
management-server-tls-enabled=false
online-mode=true
server-ip=
server-port=1337
use-native-transport=true
`

	err = root.WriteFile("server.properties", []byte(properties), os.ModePerm)
	require.NoError(t, err)

	serverconfig.SetVelocitySecret("secret2")

	err = serverconfig.SanitizeConfigs(root)
	require.NoError(t, err)

	expectedPaperGlobal := `proxies:
  proxy-protocol: false
  velocity:
    enabled: true
    online-mode: true
    secret: secret2
`

	expectedProperties := `log-ips = false
management-server-allowed-origins = *
management-server-enabled = true
management-server-host = localhost
management-server-port = 26656
management-server-secret = change-me-later
management-server-tls-enabled = false
online-mode = false
server-ip = 0.0.0.0
server-port = 25565
use-native-transport = true
`

	actualPaperGlobal, err := root.ReadFile("config/paper-global.yml")
	require.NoError(t, err)

	actualProperties, err := root.ReadFile("server.properties")
	require.NoError(t, err)

	if d := cmp.Diff(expectedPaperGlobal, string(actualPaperGlobal)); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}

	if d := cmp.Diff(expectedProperties, string(actualProperties)); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}

func TestSanatizeConfigWritesDefaultConfigs(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)

	serverconfig.SetVelocitySecret("secret1")

	err = serverconfig.SanitizeConfigs(root)
	require.NoError(t, err)

	expectedPaperGlobal := `
proxies:
  proxy-protocol: false
  velocity:
    enabled: true
    online-mode: true
    secret: secret1
`

	expectedProperties := `
server-ip = 0.0.0.0
server-port = 25565
management-server-allowed-origins = *
management-server-enabled = true
management-server-host = localhost
management-server-port = 26656
management-server-secret = change-me-later
management-server-tls-enabled = false
online-mode = false
log-ips = false
use-native-transport = true
`

	actualPaperGlobal, err := root.ReadFile("config/paper-global.yml")
	require.NoError(t, err)

	actualProperties, err := root.ReadFile("server.properties")
	require.NoError(t, err)

	if d := cmp.Diff(expectedPaperGlobal, string(actualPaperGlobal)); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}

	if d := cmp.Diff(expectedProperties, string(actualProperties)); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}
