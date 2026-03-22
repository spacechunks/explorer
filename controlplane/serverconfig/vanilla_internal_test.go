package serverconfig

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestMutateProperties(t *testing.T) {
	input := `#Minecraft server properties
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
	expected := `log-ips = false
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
	actual, err := sanatizeServerProperties([]byte(input))
	require.NoError(t, err)

	if d := cmp.Diff(expected, string(actual)); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}
