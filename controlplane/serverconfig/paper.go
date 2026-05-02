package serverconfig

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/spacechunks/explorer/controlplane/blob"
)

func defaultPaperGlobalStr() string {
	return `
proxies:
  proxy-protocol: false
  bungee-cord:
    online-mode: true
  velocity:
    enabled: false
`
}

type paperGlobal struct {
	Proxies proxiesConfig `json:"proxies"`
}

type proxiesConfig struct {
	ProxyProtocol bool `json:"proxy-protocol"`
	BungeeCord    struct {
		OnlineMode bool `json:"online-mode"`
	} `json:"bungee-cord"`
	Velocity struct {
		Enabled bool `json:"enabled"`
	} `json:"velocity"`
}

func sanatizePaperGlobal(data []byte) ([]byte, error) {
	var global paperGlobal
	if err := yaml.Unmarshal(data, &blob.Object{}); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	global.Proxies.ProxyProtocol = false
	global.Proxies.Velocity.Enabled = false
	global.Proxies.BungeeCord.OnlineMode = true

	ret, err := yaml.Marshal(global)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	return ret, nil
}
