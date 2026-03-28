package servermon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/sourcegraph/jsonrpc2/websocket"
)

type Config struct {
	PlayerCountCheckInterval      time.Duration
	MCServerManagementAPIEndpoint string
	MCServerManagementAPIToken    string
}

type Monitor struct {
	logger *slog.Logger
	conf   Config
}

func New(logger *slog.Logger, cfg Config) Monitor {
	return Monitor{
		logger: logger,
		conf:   cfg,
	}
}

type player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (m Monitor) Run(ctx context.Context) error {
	wsConn, _, err := gorilla.DefaultDialer.Dial(m.conf.MCServerManagementAPIEndpoint, map[string][]string{
		"Authorization": {"Bearer " + m.conf.MCServerManagementAPIToken},
	})
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	rpcConn := jsonrpc2.NewConn(ctx, websocket.NewObjectStream(wsConn), &m)
	defer rpcConn.Close()

	listTicker := time.NewTicker(1 * time.Second)
	defer listTicker.Stop()

	checkTicker := time.NewTicker(m.conf.PlayerCountCheckInterval)
	defer checkTicker.Stop()

	joined := false

	for {
		select {
		case <-listTicker.C:
			players := make([]player, 0)

			if err := rpcConn.Call(ctx, "minecraft:players", nil, &players); err != nil {
				return fmt.Errorf("call players: %w", err)
			}

			if len(players) > 0 {
				joined = true
			}
		case <-checkTicker.C:
			if joined {
				m.logger.Info("JOINED")
				joined = false
			} else {
				m.logger.Info("KILL")
			}

		case <-ctx.Done():
			return nil
		}
	}
}

// Handle is present, because jsonrpc2 crashes if we pass a nil handler to jsonrpc2.NewConn
// and receive a message afterward.
func (m Monitor) Handle(_ context.Context, _ *jsonrpc2.Conn, _ *jsonrpc2.Request) {}
