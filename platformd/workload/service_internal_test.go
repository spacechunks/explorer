package workload

import (
	"fmt"
	"testing"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	"github.com/spacechunks/explorer/platformd/cri"
)

func TestPodLogDir(t *testing.T) {
	tests := []struct {
		name     string
		instance *instancev1alpha1.Instance
		want     string
	}{
		{
			name: "clean names with no replacements needed",
			instance: &instancev1alpha1.Instance{
				Id: "inst-id",
				Chunk: &chunkv1alpha1.Chunk{
					Id:   "chunk-id",
					Name: "mychunk",
				},
				Flavor: &chunkv1alpha1.Flavor{
					Name: "myflavor",
				},
				FlavorVersion: &chunkv1alpha1.FlavorVersion{
					Id:      "fv-id",
					Version: "1.0.0",
				},
				Owner: &userv1alpha1.User{
					Id: "owner-id",
				},
			},
			want: fmt.Sprintf("%s/mychunk_myflavor_1.0.0_chunk-id_fv-id_inst-id_owner-id", cri.PodLogDir),
		},
		{
			name: "spaces in chunk and flavor names are replaced with dashes",
			instance: &instancev1alpha1.Instance{
				Id: "inst-id",
				Chunk: &chunkv1alpha1.Chunk{
					Id:   "chunk-id",
					Name: "my chunk",
				},
				Flavor: &chunkv1alpha1.Flavor{
					Name: "my flavor",
				},
				FlavorVersion: &chunkv1alpha1.FlavorVersion{
					Id:      "fv-id",
					Version: "1.0.0",
				},
				Owner: &userv1alpha1.User{
					Id: "owner-id",
				},
			},
			want: fmt.Sprintf("%s/my-chunk_my-flavor_1.0.0_chunk-id_fv-id_inst-id_owner-id", cri.PodLogDir),
		},
		{
			name: "underscores in chunk and flavor names are replaced with dashes",
			instance: &instancev1alpha1.Instance{
				Id: "inst-id",
				Chunk: &chunkv1alpha1.Chunk{
					Id:   "chunk-id",
					Name: "my_chunk",
				},
				Flavor: &chunkv1alpha1.Flavor{
					Name: "my_flavor",
				},
				FlavorVersion: &chunkv1alpha1.FlavorVersion{
					Id:      "fv-id",
					Version: "1.0.0",
				},
				Owner: &userv1alpha1.User{
					Id: "owner-id",
				},
			},
			want: fmt.Sprintf("%s/my-chunk_my-flavor_1.0.0_chunk-id_fv-id_inst-id_owner-id", cri.PodLogDir),
		},
		{
			name: "spaces and underscores together are both replaced with dashes",
			instance: &instancev1alpha1.Instance{
				Id: "inst-id",
				Chunk: &chunkv1alpha1.Chunk{
					Id:   "chunk-id",
					Name: "my chunk_name",
				},
				Flavor: &chunkv1alpha1.Flavor{
					Name: "my flavor_name",
				},
				FlavorVersion: &chunkv1alpha1.FlavorVersion{
					Id:      "fv-id",
					Version: "1.21.4",
				},
				Owner: &userv1alpha1.User{
					Id: "owner-id",
				},
			},
			want: fmt.Sprintf("%s/my-chunk-name_my-flavor-name_1.21.4_chunk-id_fv-id_inst-id_owner-id", cri.PodLogDir),
		},
		{
			name: "IDs are passed through unmodified",
			instance: &instancev1alpha1.Instance{
				Id: "550e8400-e29b-41d4-a716-446655440000",
				Chunk: &chunkv1alpha1.Chunk{
					Id:   "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
					Name: "chunk",
				},
				Flavor: &chunkv1alpha1.Flavor{
					Name: "flavor",
				},
				FlavorVersion: &chunkv1alpha1.FlavorVersion{
					Id:      "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
					Version: "2.0.0",
				},
				Owner: &userv1alpha1.User{
					Id: "6ba7b812-9dad-11d1-80b4-00c04fd430c8",
				},
			},
			want: fmt.Sprintf(
				"%s/chunk_flavor_2.0.0_6ba7b810-9dad-11d1-80b4-00c04fd430c8_6ba7b811-9dad-11d1-80b4-00c04fd430c8_550e8400-e29b-41d4-a716-446655440000_6ba7b812-9dad-11d1-80b4-00c04fd430c8", //nolint:lll
				cri.PodLogDir,
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := podLogDir(tt.instance)
			if got != tt.want {
				t.Errorf("podLogDir() = %q, want %q", got, tt.want)
			}
		})
	}
}
