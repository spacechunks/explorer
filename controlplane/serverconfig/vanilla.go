package serverconfig

import (
	"fmt"

	"github.com/magiconair/properties"
)

var defaultServerPropertiesStr = `
server-ip = 0.0.0.0
server-port = 25565
management-server-allowed-origins = *
management-server-enabled = true
management-server-host = localhost
management-server-port = 26656
management-server-secret = Q7fL9x2aVbW4nK8tYp3ZcR6mH1sJd5uE0qTgA8wB
management-server-tls-enabled = false
online-mode = false
log-ips = false
use-native-transport = true
`

func sanatizeServerProperties(data []byte) ([]byte, error) {
	props, err := properties.LoadString(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse properties: %w", err)
	}

	vals := map[string]any{
		"server-ip":                         "0.0.0.0",
		"server-port":                       25565,
		"management-server-allowed-origins": "*",
		"management-server-enabled":         true,
		"management-server-host":            "localhost",
		"management-server-port":            26656,
		// FIXME: management server is only accessible using localhost, but at some point don't hardcode it
		"management-server-secret":      "Q7fL9x2aVbW4nK8tYp3ZcR6mH1sJd5uE0qTgA8wB",
		"management-server-tls-enabled": false,
		"online-mode":                   false,
		"log-ips":                       false,
		"use-native-transport":          true,
	}

	for k, v := range vals {
		if err := props.SetValue(k, v); err != nil {
			return nil, fmt.Errorf("set %s: %w", k, err)
		}
	}

	return []byte(props.String()), nil
}
