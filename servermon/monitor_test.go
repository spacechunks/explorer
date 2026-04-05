package servermon_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/sourcegraph/jsonrpc2"
	workloadv1alpha2 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/servermon"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeManagementAPI struct {
	result func() []player
}

type player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (f fakeManagementAPI) serveWs(w http.ResponseWriter, r *http.Request) {
	upgrader := gorilla.Upgrader{}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	data, err := json.Marshal(f.result())
	if err != nil {
		log.Printf("marshal: %v\n", err)
		conn.Close()
		return
	}

	for {
		var req jsonrpc2.Request
		if err := conn.ReadJSON(&req); err != nil {
			log.Printf("read: %v\n", err)
			conn.Close()
			break
		}

		resp := jsonrpc2.Response{
			ID:     req.ID,
			Result: new(json.RawMessage(data)),
		}

		if err := conn.WriteJSON(resp); err != nil {
			log.Printf("write: %v\n", err)
			conn.Close()
			break
		}
	}
}

func (f fakeManagementAPI) Run(t *testing.T, port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.serveWs(w, r)
	})

	s := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	if err := s.ListenAndServe(); err != nil {
		require.NoError(t, err)
	}
}

func TestServerMonStopsWorkload(t *testing.T) {
	var (
		wlID        = "blabla"
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		fake        = fakeManagementAPI{
			result: func() []player {
				return []player{}
			},
		}
		wlMock = mock.NewMockV1alpha2WorkloadServiceClient(t)
		mon    = servermon.New(
			slog.New(slog.NewTextHandler(os.Stdout, nil)),
			servermon.Config{
				PlayerCountCheckInterval:      2 * time.Second,
				MCServerManagementAPIEndpoint: "ws://localhost:30749",
			},
			wlMock,
		)
	)

	_ = os.Setenv("PLATFORMD_WORKLOAD_ID", wlID)

	defer cancel()

	wlMock.
		EXPECT().
		StopWorkload(mocky.Anything, &workloadv1alpha2.WorkloadStopRequest{
			Id: wlID,
		}).
		Return(&workloadv1alpha2.WorkloadStopResponse{}, nil)

	go fake.Run(t, 30749)
	go func() {
		err := mon.Run(ctx)
		require.NoError(t, err)
	}()

	<-ctx.Done()
}

func TestServerMonKeepsWorkloadWhenPlayersArePresent(t *testing.T) {
	var (
		wlID        = "blabla"
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		fake        = fakeManagementAPI{
			result: func() []player {
				return []player{
					{ID: "1", Name: "A"},
				}
			},
		}
		wlMock = mock.NewMockV1alpha2WorkloadServiceClient(t)
		mon    = servermon.New(
			slog.New(slog.NewTextHandler(os.Stdout, nil)),
			servermon.Config{
				PlayerCountCheckInterval:      2 * time.Second,
				MCServerManagementAPIEndpoint: "ws://localhost:30748",
			},
			wlMock,
		)
	)

	_ = os.Setenv("PLATFORMD_WORKLOAD_ID", wlID)

	defer cancel()

	wlMock.
		EXPECT().
		StopWorkload(mocky.Anything, mocky.Anything).
		Return(&workloadv1alpha2.WorkloadStopResponse{}, nil)

	go fake.Run(t, 30748)
	go func() {
		err := mon.Run(ctx)
		require.NoError(t, err)
	}()

	<-ctx.Done()

	wlMock.AssertNotCalled(t, "StopWorkload", mocky.Anything, &workloadv1alpha2.WorkloadStopRequest{
		Id: wlID,
	})
}
