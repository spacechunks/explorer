package cli

import (
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	"github.com/spacechunks/explorer/cli/auth"
	"github.com/spacechunks/explorer/cli/state"
)

type Context struct {
	Config         state.Config
	State          state.Data
	Client         chunkv1alpha1.ChunkServiceClient
	InstanceClient instancev1alpha1.InstanceServiceClient
	UserClient     userv1alpha1.UserServiceClient
	Auth           auth.Service
}
