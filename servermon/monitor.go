package servermon

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/sourcegraph/jsonrpc2/websocket"
	workloadv1alpha2 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
)

type Config struct {
	PlayerCountCheckInterval      time.Duration
	MCServerManagementAPIEndpoint string
	MCServerManagementAPIToken    string
}

type Monitor struct {
	logger *slog.Logger
	conf   Config
	client workloadv1alpha2.WorkloadServiceClient
}

func New(logger *slog.Logger, cfg Config, client workloadv1alpha2.WorkloadServiceClient) Monitor {
	return Monitor{
		logger: logger,
		conf:   cfg,
		client: client,
	}
}

type player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (m Monitor) Run(ctx context.Context) error {
	if err := waitEndpointReady(m.conf.MCServerManagementAPIEndpoint, 20*time.Second); err != nil {
		return fmt.Errorf("wait endpoint ready: %w", err)
	}

	wsConn, resp, err := gorilla.DefaultDialer.Dial(m.conf.MCServerManagementAPIEndpoint, map[string][]string{
		"Authorization": {"Bearer " + m.conf.MCServerManagementAPIToken},
	})
	if err != nil {
		// connecting in general is broken, otherwise we have a response
		if resp == nil {
			return fmt.Errorf("dial: %w", err)
		}

		data, _ := io.ReadAll(resp.Body)
		m.logger.ErrorContext(
			ctx,
			"failed to connect to management api",
			"err", err,
			"status_code", resp.StatusCode,
			"body", string(data),
		)
		return fmt.Errorf("dial: %w", err)
	}

	rpcConn := jsonrpc2.NewConn(ctx, websocket.NewObjectStream(wsConn), &m)
	defer rpcConn.Close()

	listTicker := time.NewTicker(1 * time.Second)
	defer listTicker.Stop()

	checkTicker := time.NewTicker(m.conf.PlayerCountCheckInterval)
	defer checkTicker.Stop()

	joined := atomic.Bool{}

	workloadID := os.Getenv("PLATFORMD_WORKLOAD_ID")
	if workloadID == "" {
		return fmt.Errorf("PLATFORMD_WORKLOAD_ID not set")
	}

	logger := m.logger.With("workload_id", workloadID)

	go func() {
		for range listTicker.C {
			players := make([]player, 0)

			if err := rpcConn.Call(ctx, "minecraft:players", nil, &players); err != nil {
				logger.ErrorContext(ctx, "failed to call players", "err", err)
				continue
			}

			logger.Debug("got players", "player_count", len(players))

			if len(players) > 0 {
				joined.Store(true)
			}
		}
	}()

	go func() {
		for range checkTicker.C {
			if joined.Load() {
				joined.Store(false)
				break
			}

			logger.Info(
				"player count has been 0 for too long, cleaning up",
				"check_interval", m.conf.PlayerCountCheckInterval,
			)

			if _, err := m.client.StopWorkload(ctx, &workloadv1alpha2.WorkloadStopRequest{
				Id: workloadID,
			}); err != nil {
				logger.Error(
					"failed to stop workload, retry happens next player count check",
					"workload_id", workloadID,
					"err", err,
				)
			}
		}
	}()

	<-ctx.Done()
	return nil
}

// Handle is present, because jsonrpc2 crashes if we pass a nil handler to jsonrpc2.NewConn
// and receive a message afterward.
func (m Monitor) Handle(_ context.Context, _ *jsonrpc2.Conn, _ *jsonrpc2.Request) {}

func waitEndpointReady(endpoint string, timeout time.Duration) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse endpoint: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		conn, err := net.DialTimeout("tcp", u.Host, 1*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s did not respond within %v", u.Host, timeout)
		case <-time.After(2 * time.Second):
			continue
		}
	}
}
