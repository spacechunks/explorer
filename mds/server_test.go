package mds_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	workloadv1alpha2 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/internal/resource"
	"github.com/spacechunks/explorer/internal/resource/codec"
	"github.com/spacechunks/explorer/mds"
	"github.com/spacechunks/explorer/platformd/workload"
	"github.com/spacechunks/explorer/test/fixture"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type metadata struct {
	InstanceID string `json:"instanceId"`

	Chunk struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		Tags        []string  `json:"tags"`
		CreatedAt   time.Time `json:"createdAt"`
		UpdatedAt   time.Time `json:"updatedAt"`
	} `json:"chunk"`

	FlavorVersion struct {
		ID               string    `json:"id"`
		Version          string    `json:"version"`
		MinecraftVersion string    `json:"minecraftVersion"`
		CreatedAt        time.Time `json:"createdAt"`
	} `json:"flavorVersion"`

	OrderedBy string `json:"orderedBy"`
}

func TestMDSWorks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ins := fixture.Instance()
	meta := workload.Metadata{
		ID: ins.ID,
		Chunk: resource.Chunk{
			ID:          ins.Chunk.ID,
			Name:        ins.Chunk.Name,
			Description: ins.Chunk.Description,
			Tags:        ins.Chunk.Tags,
			CreatedAt:   ins.Chunk.CreatedAt,
			UpdatedAt:   ins.Chunk.CreatedAt,
		},
		FlavorVersion: codec.FlavorVersionToDomain(codec.FlavorVersionToTransport(ins.FlavorVersion)),
		OrderedBy:     ins.OrderedBy,
	}

	expected := metadata{
		InstanceID: meta.ID,
		OrderedBy:  meta.OrderedBy,
	}

	expected.Chunk.ID = meta.Chunk.ID
	expected.Chunk.Name = meta.Chunk.Name
	expected.Chunk.Description = meta.Chunk.Description
	expected.Chunk.Tags = meta.Chunk.Tags
	expected.Chunk.CreatedAt = meta.Chunk.CreatedAt
	expected.Chunk.UpdatedAt = meta.Chunk.UpdatedAt

	expected.FlavorVersion.ID = meta.FlavorVersion.ID
	expected.FlavorVersion.Version = meta.FlavorVersion.Version
	expected.FlavorVersion.MinecraftVersion = meta.FlavorVersion.MinecraftVersion
	expected.FlavorVersion.CreatedAt = meta.FlavorVersion.CreatedAt

	os.Setenv("PLATFORMD_WORKLOAD_ID", ins.ID)

	mockSvc := mock.NewMockV1alpha2WorkloadServiceClient(t)
	mockSvc.EXPECT().WorkloadMetadata(mocky.Anything, &workloadv1alpha2.WorkloadMetadataRequest{
		WorkloadId: ins.ID,
	}).Return(&workloadv1alpha2.WorkloadMetadataResponse{
		Metadata: workload.MetadataToTransport(meta),
	}, nil)

	mdsService := mds.New(
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
		":4245",
		mockSvc,
	)

	go func() {
		if err := mdsService.Run(ctx); err != nil {
			t.Log(err)
			fmt.Println(err)
		}
	}()

	// http server needs some time to serve
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://127.0.0.1:4245")
	require.NoError(t, err)

	cancel()

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var actual metadata
	require.NoError(t, json.Unmarshal(data, &actual))

	if d := cmp.Diff(expected, actual); d != "" {
		t.Fatalf("diff (-want, +got): %s", d)
	}
}

func TestMDSReturnsNotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	os.Setenv("PLATFORMD_WORKLOAD_ID", "abc")

	mockSvc := mock.NewMockV1alpha2WorkloadServiceClient(t)
	mockSvc.EXPECT().WorkloadMetadata(mocky.Anything, &workloadv1alpha2.WorkloadMetadataRequest{
		WorkloadId: "abc",
	}).Return(nil, status.Error(codes.NotFound, "workload not found"))

	mdsService := mds.New(
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
		":4245",
		mockSvc,
	)

	go func() {
		if err := mdsService.Run(ctx); err != nil {
			t.Log(err)
			fmt.Println(err)
		}
	}()

	// http server needs some time to serve
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://127.0.0.1:4245")
	require.NoError(t, err)

	cancel()

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	if d := cmp.Diff("{\"msg\":\"workload not found\"}\n", string(data)); d != "" {
		t.Fatalf("diff (-want, +got): %s", d)
	}
}
