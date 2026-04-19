package worker_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/controlplane/worker"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/test/fixture"
	mocky "github.com/stretchr/testify/mock"
)

func TestArchiveFullChunk(t *testing.T) {
	var (
		c = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors[0].Versions[0].BuildStatus = resource.FlavorVersionBuildStatusCompleted
			tmp.Flavors[0].Versions[1].BuildStatus = resource.FlavorVersionBuildStatusCompleted

			tmp.Flavors[1].Versions[0].BuildStatus = resource.FlavorVersionBuildStatusCompleted
			tmp.Flavors[1].Versions[1].BuildStatus = resource.FlavorVersionBuildStatusCompleted
		})
		logger          = slog.New(slog.NewTextHandler(os.Stdout, nil))
		mockArchiveRepo = mock.NewMockChunkArchiveRepository(t)
		mockChunkRepo   = mock.NewMockChunkRepository(t)
		mockInsRepo     = mock.NewMockInstanceRepository(t)
	)

	mockChunkRepo.
		EXPECT().
		AllDeletedFlavors(mocky.Anything).
		Return(map[string]string{
			c.Flavors[0].ID: c.ID,
			c.Flavors[1].ID: c.ID,
		}, nil)

	for _, f := range c.Flavors {
		tmp := f
		tmp.Versions = nil

		mockChunkRepo.
			EXPECT().
			FlavorByID(mocky.Anything, f.ID).
			Return(f, nil).
			Once()

		mockChunkRepo.
			EXPECT().
			FlavorByID(mocky.Anything, f.ID).
			Return(tmp, nil).
			Once()

		for _, v := range f.Versions {
			mockInsRepo.
				EXPECT().
				CountInstancesByFlavorVersionID(mocky.Anything, v.ID).
				Return(uint(0), nil)

			mockArchiveRepo.
				EXPECT().
				ArchiveFlavorVersion(mocky.Anything, f.ID, v).
				Return(nil)
		}

		mockArchiveRepo.
			EXPECT().
			ArchiveFlavor(
				mocky.Anything,
				c.ID,
				mocky.MatchedBy(func(flavor resource.Flavor) bool {
					return flavor.ID == f.ID
				}),
			).
			Return(nil)
	}

	withoutFlavors := fixture.Chunk(func(tmp *resource.Chunk) {
		tmp.Flavors = nil
	})

	mockChunkRepo.
		EXPECT().
		GetChunkByID(mocky.Anything, c.ID).
		Return(withoutFlavors, nil)

	mockArchiveRepo.
		EXPECT().
		ArchiveChunk(mocky.Anything, withoutFlavors).
		Return(nil)

	w := worker.NewArchiveWorker(logger, mockChunkRepo, mockInsRepo, mockArchiveRepo)

	_ = w.Work(context.Background(), nil)
}

func TestDoNotArchiveFlavorVersionsCurrentlyBuilding(t *testing.T) {
	tests := []struct {
		name        string
		buildStatus resource.FlavorVersionBuildStatus
	}{
		{
			name:        "build image",
			buildStatus: resource.FlavorVersionBuildStatusBuildImage,
		},
		{
			name:        "build checkpoint",
			buildStatus: resource.FlavorVersionBuildStatusBuildCheckpoint,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				c = fixture.Chunk(func(tmp *resource.Chunk) {
					tmp.Flavors = []resource.Flavor{
						fixture.Flavor(func(tmpF *resource.Flavor) {
							tmpF.Versions = []resource.FlavorVersion{
								fixture.FlavorVersion(func(tmpV *resource.FlavorVersion) {
									tmpV.BuildStatus = tt.buildStatus
								}),
							}
						}),
					}
				})
				logger          = slog.New(slog.NewTextHandler(os.Stdout, nil))
				mockArchiveRepo = mock.NewMockChunkArchiveRepository(t)
				mockChunkRepo   = mock.NewMockChunkRepository(t)
				mockInsRepo     = mock.NewMockInstanceRepository(t)
			)

			f := c.Flavors[0]

			mockChunkRepo.
				EXPECT().
				AllDeletedFlavors(mocky.Anything).
				Return(map[string]string{
					c.Flavors[0].ID: c.ID,
				}, nil)

			mockChunkRepo.
				EXPECT().
				FlavorByID(mocky.Anything, f.ID).
				Return(f, nil)

			mockInsRepo.
				EXPECT().
				CountInstancesByFlavorVersionID(mocky.Anything, f.Versions[0].ID).
				Return(uint(0), nil)

			mockArchiveRepo.AssertNotCalled(t, "ArchiveFlavorVersion", f.ID, f.Versions[0])

			mockChunkRepo.
				EXPECT().
				GetChunkByID(mocky.Anything, c.ID).
				Return(c, nil)

			w := worker.NewArchiveWorker(logger, mockChunkRepo, mockInsRepo, mockArchiveRepo)

			_ = w.Work(context.Background(), nil)
		})
	}
}

func TestArchiveWorkerDoesNotArchiveFlavorAndChunkWhenVersionsAndFlavorsRemain(t *testing.T) {
	var (
		c = fixture.Chunk()
		f = c.Flavors[0]

		logger          = slog.New(slog.NewTextHandler(os.Stdout, nil))
		mockArchiveRepo = mock.NewMockChunkArchiveRepository(t)
		mockChunkRepo   = mock.NewMockChunkRepository(t)
		mockInsRepo     = mock.NewMockInstanceRepository(t)
	)

	mockChunkRepo.
		EXPECT().
		AllDeletedFlavors(mocky.Anything).
		Return(map[string]string{
			f.ID: c.ID,
		}, nil)

	mockChunkRepo.
		EXPECT().
		FlavorByID(mocky.Anything, f.ID).
		Return(f, nil).
		Once()

	mockChunkRepo.
		EXPECT().
		FlavorByID(mocky.Anything, f.ID).
		Return(f, nil).
		Once()

	for _, v := range f.Versions {
		mockInsRepo.
			EXPECT().
			CountInstancesByFlavorVersionID(mocky.Anything, v.ID).
			Return(uint(1), nil)
	}

	mockChunkRepo.
		EXPECT().
		GetChunkByID(mocky.Anything, c.ID).
		Return(c, nil)

	w := worker.NewArchiveWorker(logger, mockChunkRepo, mockInsRepo, mockArchiveRepo)

	_ = w.Work(context.Background(), nil)

	mockArchiveRepo.AssertNotCalled(t, "ArchiveFlavor", mocky.Anything, mocky.Anything, mocky.Anything)
	mockArchiveRepo.AssertNotCalled(t, "ArchiveChunk", mocky.Anything, mocky.Anything, mocky.Anything)
}
