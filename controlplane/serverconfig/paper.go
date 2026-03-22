package serverconfig

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/spacechunks/explorer/controlplane/blob"
)

/*
proxies:
	bungee-cord:
	  online-mode: true
	proxy-protocol: false
	velocity:
	  enabled: false
	  online-mode: true
	  secret: ""
*/

type paperGlobal struct {
	Proxies proxiesConfig `json:"proxies"`
}

type proxiesConfig struct {
	ProxyProtocol bool `json:"proxy-protocol"`
	Velocity struct {
		Enabled    bool   `json:"enabled"`
		OnlineMode bool   `json:"online-mode"`
		Secret     string `json:"secret"`
	}
}

func sanatizePaperGlobal(data []byte) ([]byte, error) {
	var global paperGlobal
	if err := yaml.Unmarshal(data, &blob.Object{}); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	global.Proxies.ProxyProtocol = false

	global.Proxies.Velocity.Enabled = true
	global.Proxies.Velocity.OnlineMode = true
	global.Proxies.Velocity.Secret = ""

	ret, err := yaml.Marshal(global)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	return ret, nil
}
