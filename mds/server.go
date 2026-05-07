package mds

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	workloadv1alpha2 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	logger *slog.Logger
	server *http.Server
	client workloadv1alpha2.WorkloadServiceClient
}

type chunk struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type flavorVersion struct {
	ID               string    `json:"id"`
	Version          string    `json:"version"`
	MinecraftVersion string    `json:"minecraftVersion"`
	CreatedAt        time.Time `json:"createdAt"`
}

type metadata struct {
	InstanceID    string        `json:"instanceId"`
	Chunk         chunk         `json:"chunk"`
	FlavorVersion flavorVersion `json:"flavorVersion"`
	OrderedBy     string        `json:"orderedBy"`
}

type httpErr struct {
	Msg string `json:"msg"`
}

func New(logger *slog.Logger, addr string, service workloadv1alpha2.WorkloadServiceClient) Server {
	return Server{
		logger: logger,
		server: &http.Server{
			Addr: addr,
		},
		client: service,
	}
}

func (s Server) Run(ctx context.Context) error {
	s.logger.InfoContext(ctx, "started mds", "addr", s.server.Addr)
	var (
		mux        = http.NewServeMux()
		workloadID = os.Getenv("PLATFORMD_WORKLOAD_ID")
	)

	if workloadID == "" {
		return errors.New("PLATFORMD_WORKLOAD_ID not set")
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		enc := json.NewEncoder(w)
		w.Header().Set("Content-Type", "application/json")

		resp, err := s.client.WorkloadMetadata(ctx, &workloadv1alpha2.WorkloadMetadataRequest{
			WorkloadId: workloadID,
		})

		if err != nil {
			s.logger.InfoContext(ctx, "failed to fetch workload metadata", "err", err)
			statusCode, httpErr := toHTTPErr(err)

			w.WriteHeader(statusCode)
			if err := enc.Encode(httpErr); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				s.logger.InfoContext(ctx, "failed to encode response", "err", err)
			}
			return
		}

		meta := metadata{
			InstanceID: workloadID,
			Chunk: chunk{
				ID:          resp.Metadata.Chunk.Id,
				Name:        resp.Metadata.Chunk.Name,
				Description: resp.Metadata.Chunk.Description,
				Tags:        resp.Metadata.Chunk.Tags,
				CreatedAt:   resp.Metadata.Chunk.CreatedAt.AsTime(),
				UpdatedAt:   resp.Metadata.Chunk.UpdatedAt.AsTime(),
			},
			FlavorVersion: flavorVersion{
				ID:               resp.Metadata.FlavorVersion.Id,
				Version:          resp.Metadata.FlavorVersion.Version,
				MinecraftVersion: resp.Metadata.FlavorVersion.MinecraftVersion,
				CreatedAt:        resp.Metadata.FlavorVersion.CreatedAt.AsTime(),
			},
			OrderedBy: resp.Metadata.OrderedBy,
		}

		w.WriteHeader(http.StatusOK)
		if err := enc.Encode(meta); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.InfoContext(ctx, "failed to encode response", "err", err)
			return
		}
	})

	go func() {
		<-ctx.Done()
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.ErrorContext(ctx, "failed to shutdown mds", "err", err)
		}
	}()

	s.server.Handler = mux

	if err := s.server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}

	return nil
}

func toHTTPErr(err error) (int, httpErr) {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError, httpErr{
			Msg: err.Error(),
		}
	}

	var statusCode int
	switch st.Code() {
	case codes.NotFound:
		statusCode = http.StatusNotFound
	default:
		statusCode = http.StatusInternalServerError
	}

	return statusCode, httpErr{
		Msg: st.Message(),
	}
}
