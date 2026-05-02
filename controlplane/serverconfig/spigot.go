package serverconfig

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/spacechunks/explorer/controlplane/blob"
)

func defaultSpigotStr() string {
	return `
settings:
  bungeecord: true
`
}

type spigotCfg struct {
	Settings struct {
		BungeeCord bool `yaml:"bungeecord"`
	} `json:"settings"`
}

func sanatizeSpigot(data []byte) ([]byte, error) {
	var spigot spigotCfg
	if err := yaml.Unmarshal(data, &blob.Object{}); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	spigot.Settings.BungeeCord = true

	ret, err := yaml.Marshal(spigot)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	return ret, nil
}
